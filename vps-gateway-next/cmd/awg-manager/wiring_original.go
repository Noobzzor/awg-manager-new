package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/frontend"
	"github.com/hoaxisr/awg-manager/internal/api"
	"github.com/hoaxisr/awg-manager/internal/auth"
	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/gateway/routerpolicy"
	"github.com/hoaxisr/awg-manager/internal/presets"
	"github.com/hoaxisr/awg-manager/internal/server"
	"github.com/hoaxisr/awg-manager/internal/singbox"
	"github.com/hoaxisr/awg-manager/internal/singbox/dnsrewrite"
	orch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
	"github.com/hoaxisr/awg-manager/internal/storage"
	"golang.org/x/sys/unix"
)

// Original mode is explicit, local and experimental. It reuses original
// authoring/HTTP and journaled slot application; no legacy migration/adoption.
func (a *app) prepareOriginalPolicyOwner() error {
	if a.localPolicyOwner != nil {
		if !a.originalRouterMode {
			return fmt.Errorf("cannot change policy owner during startup")
		}
		return a.localPolicyOwner.Verify(context.Background())
	}
	if !filepath.IsAbs(a.dataDir) || filepath.Clean(a.dataDir) != a.dataDir || filepath.Clean(a.singboxConfigDir) != filepath.Join(a.dataDir, "sing-box", "config.d") {
		return fmt.Errorf("original mode requires canonical config directory inside absolute data directory")
	}
	for _, addr := range []string{a.httpAddr, a.proxyAddr} {
		host, _, err := net.SplitHostPort(addr)
		if err != nil || !net.ParseIP(host).IsLoopback() {
			return fmt.Errorf("original mode requires loopback HTTP and proxy addresses")
		}
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("AWG_GATEWAY_ENABLE")), "true") {
		return fmt.Errorf("original mode requires explicit gateway ingress")
	}
	// Read-only orphan admission precedes owner claim and setupCore self-heal.
	if err := singbox.RefuseExistingEngine(a.singboxBinary); err != nil {
		return err
	}
	parent := a.dataDir
	for {
		if _, err := os.Lstat(parent); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return fmt.Errorf("original root unavailable")
		}
		parent = next
	}
	real, err := filepath.EvalSymlinks(parent)
	if err != nil || real != parent {
		return fmt.Errorf("original data directory ancestors must not redirect")
	}
	if _, err = os.Lstat(a.dataDir); err == nil {
		if err = checkOriginalTree(a.dataDir); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = os.MkdirAll(a.dataDir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(a.dataDir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("original data directory must be private")
	}
	owner, err := routerpolicy.AcquirePolicyOwner(context.Background(), a.dataDir, routerpolicy.PolicyOwnerOriginal)
	if err != nil {
		return err
	}
	a.originalRouterFresh = owner.FreshClaim()
	a.localPolicyOwner = owner
	a.originalRouterMode = true
	a.deferOnExit(func() { _ = owner.Close() })
	return nil
}
func checkOriginalTree(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) || info.Mode().Perm()&0022 != 0 {
			return fmt.Errorf("original state contains redirected, special or writable resources")
		}
		if info.Mode().IsRegular() {
			var st unix.Stat_t
			if err := unix.Lstat(path, &st); err != nil || st.Nlink != 1 {
				return fmt.Errorf("original state contains multiply linked resource")
			}
		}
		return nil
	})
}
func (a *app) verifyOriginalOwner(ctx context.Context) error {
	if a.localPolicyOwner == nil || !a.originalRouterMode {
		return fmt.Errorf("original policy lease absent")
	}
	if err := a.localPolicyOwner.Verify(ctx); err != nil {
		return err
	}
	if err := checkOriginalTree(a.dataDir); err != nil {
		return err
	}
	return nil
}

func (a *app) setupOriginalLocal() error {
	a.eventBus = events.NewBus()
	a.loggingService.SetEventBus(a.eventBus)
	state := routerpolicy.OriginalStateDir(a.dataDir)
	a.singboxOp = singbox.NewOperatorForRecovery(singbox.OperatorDeps{
		Dir: filepath.Join(a.dataDir, "sing-box"), ConfigDir: a.singboxConfigDir, Binary: a.singboxBinary,
		DisableNDMSProxy: true, AppLogger: a.loggingService, Bus: a.eventBus,
		InitialManuallyStopped: a.settings.SingboxManuallyStopped,
	})
	generation, err := singbox.NewOwnedGenerationProcess(a.singboxOp.Process(), a.singboxConfigDir, filepath.Join(state, "runtime"), a.singboxOp.Clash(), a.verifyOriginalOwner)
	if err != nil {
		return err
	}
	a.sbOrch = orch.NewWithAppliedPath(a.singboxConfigDir, generation, filepath.Join(state, "runtime-applied.json"))
	a.sbOrch.SetOperationCoordinator(&a.localConfigGate)
	a.sbOrch.SetValidator(&orchValidatorAdapter{v: singbox.NewValidator(a.singboxBinary)})
	a.sbOrch.SetShouldRun(func() bool { return !a.singboxOp.IsManuallyStopped() })
	a.singboxOp.SetOrch(a.sbOrch)
	a.deferOnExit(func() { _ = a.singboxOp.Process().Stop(); a.singboxOp.Process().Close() })
	a.deferOnExit(a.sbOrch.Close)
	for _, meta := range orch.KnownSlots() {
		if err = a.sbOrch.Register(meta); err != nil {
			return err
		}
	}
	if err = a.sbOrch.LoadRecoveryState(context.Background()); err != nil {
		return err
	}
	docs, err := router.OpenEditorDocuments(filepath.Join(state, "editor"))
	if err != nil {
		return err
	}
	if a.originalRouterFresh {
		if err = os.MkdirAll(filepath.Join(a.singboxConfigDir, "disabled"), 0700); err != nil {
			return err
		}
		base := []byte(`{"log":{"level":"warn"},"dns":{"servers":[{"type":"udp","tag":"dns-bootstrap","server":"1.1.1.1"}],"final":"dns-bootstrap"},"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct","default_domain_resolver":"dns-bootstrap"},"experimental":{"clash_api":{"external_controller":"127.0.0.1:9099"}}}`)
		if err = storage.AtomicWritePerm(filepath.Join(a.singboxConfigDir, "00-base.json"), base, 0600); err != nil {
			return err
		}
		proxy, err := localProxyInbound(a.proxyAddr)
		if err != nil {
			return err
		}
		if err = storage.AtomicWritePerm(filepath.Join(a.singboxConfigDir, "05-local-proxy.json"), proxy, 0600); err != nil {
			return err
		}
		if err = a.importOriginalWARP(); err != nil {
			return err
		}
		if err = a.sbOrch.LoadRecoveryState(context.Background()); err != nil {
			return err
		}
	}
	a.originalWARP, err = a.loadOriginalWARP()
	if err != nil {
		return err
	}
	if err = a.setupGatewayRuntime(); err != nil {
		return err
	}
	if a.gatewayRuntime == nil {
		return fmt.Errorf("original local mode requires explicit gateway ingress")
	}
	ingress := strings.TrimSpace(os.Getenv("AWG_GATEWAY_INTERFACE"))
	if ingress == "" {
		ingress = "awg0"
	}
	backend, err := routerpolicy.NewSlotApplyBackend(routerpolicy.SlotApplyDeps{
		Orchestrator: a.sbOrch, Documents: docs, IngressInterface: ingress, VerifyOwnership: a.verifyOriginalOwner,
		CheckProjectedRouting: generation.CheckProjectedRouting,
		ConfirmRunning:        func(ctx context.Context, _ routerpolicy.ApplyState) error { return generation.Confirm(ctx) },
	})
	if err != nil {
		return err
	}
	runtime := &originalLocalRuntime{a: a, docs: docs, backend: backend, generation: generation}
	runtime.applier, err = routerpolicy.NewApplier(filepath.Join(state, "apply-journal.json"), backend, &a.localConfigGate)
	if err != nil {
		return err
	}
	// Recover journal before publishing any writer or reconciling persisted peers.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = runtime.applier.Recover(ctx); err != nil {
		return err
	}
	a.localConfigGate.Lock()
	err = func() error {
		release, err := backend.AcquireApplyLease(ctx)
		if err != nil {
			return err
		}
		defer release()
		applied, err := docs.LoadApplied()
		if err != nil {
			return err
		}
		if a.originalRouterFresh && applied == nil {
			raw, err := json.Marshal(router.NewEmptyConfig())
			if err != nil {
				return err
			}
			candidate, err := backend.Candidate(raw)
			if err != nil {
				return err
			}
			if err = backend.Validate(ctx, candidate); err != nil {
				return err
			}
			if err = backend.Commit(ctx, candidate); err != nil {
				return err
			}
		} else if applied == nil {
			return fmt.Errorf("original applied state is absent; explicit recovery required")
		}
		snapshot, err := backend.Snapshot(ctx)
		if err != nil {
			return err
		}
		if err = backend.Activate(ctx); err != nil {
			return err
		}
		return backend.Confirm(ctx, snapshot)
	}()
	a.localConfigGate.Unlock()
	if err != nil {
		return err
	}
	a.sbOrch.SetRuntimeRecoveryRequired(false)
	if err = a.gatewayClients.ReconcilePeers(ctx, a.gatewayPeerApplier); err != nil {
		return err
	}
	a.originalRouterService, err = router.NewPortableService(router.PortableDeps{
		Settings: a.settingsStore, Catalog: presets.NewCatalog(presets.NewStore(a.dataDir)), Singbox: a.singboxOp, Orch: a.sbOrch,
		Events: a.eventBus, Runtime: runtime, Documents: docs, Coordinator: &a.localConfigGate, AppLog: a.loggingService,
		ResolvePresetRuleSets: a.resolveOriginalPresetRuleSets, Tunnels: a.originalWARP,
	})
	return err
}

type originalLocalRuntime struct {
	a          *app
	docs       *router.FileEditorDocuments
	backend    *routerpolicy.SlotApplyBackend
	applier    *routerpolicy.Applier
	generation *singbox.OwnedGenerationProcess
}

func (r *originalLocalRuntime) GetStatus(ctx context.Context) (router.Status, error) {
	r.a.localConfigGate.Lock()
	defer r.a.localConfigGate.Unlock()
	raw, err := r.docs.LoadEffective()
	if err != nil {
		return router.Status{}, err
	}
	cfg := router.NewEmptyConfig()
	if raw != nil {
		if err = json.Unmarshal(raw, cfg); err != nil {
			return router.Status{}, err
		}
	}
	live := r.generation.Confirm(ctx) == nil
	return router.Status{Enabled: true, Installed: true, Active: live, Final: cfg.Route.Final, RuleCount: len(cfg.Route.Rules), RuleSetCount: len(cfg.Route.RuleSet)}, nil
}
func (r *originalLocalRuntime) GetSettings(context.Context) (storage.SingboxRouterSettings, error) {
	return storage.SingboxRouterSettings{Enabled: true, RoutingMode: "policy-tun"}, nil
}
func (r *originalLocalRuntime) UpdateSettings(context.Context, storage.SingboxRouterSettings) error {
	return fmt.Errorf("original local engine settings are not yet supported")
}
func (r *originalLocalRuntime) SwitchRoutingMode(context.Context, string) error {
	return fmt.Errorf("original local routing mode is fixed to gateway TUN")
}
func (r *originalLocalRuntime) RenameExternalOutboundTag(context.Context, string, string) error {
	return fmt.Errorf("external outbounds are not yet supported in original local prototype")
}
func (r *originalLocalRuntime) Reconcile(ctx context.Context) error { return r.generation.Confirm(ctx) }
func (r *originalLocalRuntime) StagingStatus(context.Context) router.StagingStatus {
	r.a.localConfigGate.Lock()
	defer r.a.localConfigGate.Unlock()
	draft, err := r.docs.LoadEffective()
	if err != nil {
		return router.StagingStatus{HasDraft: true}
	}
	applied, err := r.docs.LoadApplied()
	return router.StagingStatus{HasDraft: err != nil || !bytes.Equal(draft, applied)}
}
func (r *originalLocalRuntime) ApplyStaging(ctx context.Context) (orch.ValidationResult, error) {
	var raw []byte
	outcome, err := r.applier.ApplyCurrent(ctx, func() (routerpolicy.ApplyState, error) {
		var err error
		raw, err = r.docs.LoadEffective()
		if err != nil {
			return routerpolicy.ApplyState{}, err
		}
		return r.backend.Candidate(raw)
	})
	if err != nil {
		return orch.ValidationResult{}, err
	}
	if !outcome.Applied || outcome.CleanupPending {
		return orch.ValidationResult{}, routerpolicy.ErrApplyRecoveryRequired
	}
	// Do not erase a draft saved after this transaction's admission/ACK.
	r.a.localConfigGate.Lock()
	defer r.a.localConfigGate.Unlock()
	current, err := r.docs.LoadEffective()
	if err == nil && bytes.Equal(current, raw) {
		err = r.docs.DiscardDraft()
	}
	return orch.ValidationResult{}, err
}
func (a *app) setupOriginalLocalServer() error {
	fs, err := frontend.FS()
	if err != nil {
		return err
	}
	a.sessionStore = auth.NewSessionStore(a.settingsStore.GetSessionTTL)
	a.deferOnExit(a.sessionStore.Stop)
	preview := os.Getenv("AWG_ORIGINAL_BROWSER_PREVIEW") == "1"
	deps := server.LocalDeps{
		Common:         server.CommonCapabilities{Settings: a.settingsStore, Sessions: a.sessionStore, LoggingService: a.loggingService, Bus: a.eventBus},
		RequestAllowed: originalLocalRequestAllowed,
	}
	if preview {
		gateway := newGatewayClientHTTP(a.gatewayClients, filepath.Join(a.dataDir, "gateway-clients.json"), a.gatewayPeerApplier)
		gateway.SetMutationAdmission(&a.localConfigGate, a.verifyOriginalOwner)
		gateway.SetClientConfig(a.gatewayIdentity.PublicKey, os.Getenv("AWG_GATEWAY_PUBLIC_ENDPOINT"))
		deps.Routing.Gateway = gateway
		if err := a.settingsStore.Update(func(settings *storage.Settings) error {
			settings.UsageLevel = storage.UsageLevelExpert
			return nil
		}); err != nil {
			return err
		}
		deps.OriginalBrowserPreview = true
		deps.Singbox.Status = a.singboxOp
		deps.Common.SettingsHandler = api.NewSettingsHandler(a.settingsStore, a.loggingService)
		deps.RequestAllowed = func(r *http.Request) bool {
			return originalLocalRequestAllowed(r) || originalBrowserRequestAllowed(r) || originalClientRequestAllowed(r)
		}
	}
	a.srv = server.NewLocal(server.Config{Version: version, FrontendFS: fs}, deps)
	if preview {
		a.srv.SetAWGOutboundsHandler(api.NewAWGOutboundsHandler(a.originalWARP))
		store := storage.NewDNSRewriteStore(filepath.Join(a.dataDir, "dns-rewrites.json"))
		a.srv.SetDNSRewritesHandler(api.NewDNSRewritesHandler(dnsrewrite.NewService(store, nil), a.loggingService))
	}
	a.srv.SetPresetCatalog(presets.NewCatalog(presets.NewStore(a.dataDir)))
	a.srv.SetSingboxRouterHandler(api.NewSingboxRouterHandler(originalInlineRuleSetService{a.originalRouterService}, a.loggingService))
	return nil
}

type originalInlineRuleSetService struct {
	router.Service
}

func (s originalInlineRuleSetService) AddRuleSet(ctx context.Context, set router.RuleSet) error {
	if set.Type != "inline" || set.Path != "" || set.URL != "" {
		return fmt.Errorf("original local mode requires inline rule-set content")
	}
	return s.Service.AddRuleSet(ctx, set)
}

func originalClientRequestAllowed(r *http.Request) bool {
	const clients = "/api/gateway/clients"
	if r.URL.Path == clients {
		return r.Method == http.MethodGet
	}
	if r.URL.Path == clients+"/create" {
		return r.Method == http.MethodPost
	}
	suffix, ok := strings.CutPrefix(r.URL.Path, clients+"/")
	if !ok {
		return false
	}
	id, action, hasAction := strings.Cut(suffix, "/")
	if id == "" {
		return false
	}
	if r.Method == http.MethodGet {
		return !hasAction || action == "config"
	}
	return r.Method == http.MethodPost && (action == "enable" || action == "disable" || action == "revoke")
}

func originalBrowserRequestAllowed(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet:
		switch r.URL.Path {
		case "/api/auth/status", "/api/system/info", "/api/settings/get", "/api/singbox/awg-outbounds/tags",
			"/api/singbox/router/settings", "/api/singbox/router/outbounds/list",
			"/api/singbox/router/dns/servers/list", "/api/singbox/router/dns/rules/list",
			"/api/singbox/router/dns/rewrites/list", "/api/singbox/router/dns/globals":
			return true
		}
	case http.MethodPost:
		switch r.URL.Path {
		case "/api/singbox/router/dns/servers/add", "/api/singbox/router/dns/servers/update",
			"/api/singbox/router/dns/rules/add", "/api/singbox/router/dns/rules/update",
			"/api/singbox/router/dns/rules/delete":
			return true
		}
	case http.MethodPut:
		return r.URL.Path == "/api/singbox/router/dns/globals"
	}
	return false
}

func originalLocalRequestAllowed(r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		return r.Method == http.MethodGet || r.Method == http.MethodHead
	}
	switch r.Method {
	case http.MethodGet:
		switch r.URL.Path {
		case "/api/health", "/api/capabilities", "/api/presets",
			"/api/singbox/router/presets/list", "/api/singbox/router/status", "/api/singbox/router/staging",
			"/api/singbox/router/rules/list", "/api/singbox/router/rulesets/list":
			return true
		}
	case http.MethodPost:
		switch r.URL.Path {
		case "/api/singbox/router/route/final", "/api/singbox/router/staging/apply", "/api/singbox/router/staging/discard",
			"/api/singbox/router/rules/add", "/api/singbox/router/rules/move", "/api/singbox/router/rulesets/add",
			"/api/singbox/router/presets/apply":
			return true
		}
	}
	return false
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/frontend"
	"github.com/hoaxisr/awg-manager/internal/api"
	"github.com/hoaxisr/awg-manager/internal/auth"
	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
	"github.com/hoaxisr/awg-manager/internal/dnsroute"
	"github.com/hoaxisr/awg-manager/internal/downloader"
	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/gateway/forwarding"
	gatewaygroups "github.com/hoaxisr/awg-manager/internal/gateway/groups"
	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
	gatewaypolicy "github.com/hoaxisr/awg-manager/internal/gateway/policy"
	gatewaysubscription "github.com/hoaxisr/awg-manager/internal/gateway/subscription"
	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/server"
	"github.com/hoaxisr/awg-manager/internal/singbox"
	singboxorch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/singbox/subscription"
)

// startJoinedWorker starts one owned background worker and returns a cleanup
// function that waits until the worker has observed cancellation.
func startJoinedWorker(run func(context.Context)) func() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		run(ctx)
	}()
	return func() {
		cancel()
		<-done
	}
}

// newLocalTunnelsHandler is the production local tunnel-list reader. It shares
// the store-owned transaction coordinator with Awg3Handler so no response can
// observe store state between mutation and slot/DNS commit or rollback.
func newLocalTunnelsHandler(store *awg3endpoint.Store, transaction awg3endpoint.TransactionLocker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			response.MethodNotAllowed(w)
			return
		}
		transaction.RLock()
		defer transaction.RUnlock()
		records, err := store.List()
		if err != nil {
			response.Error(w, err.Error(), "AWG3_LIST_ERROR")
			return
		}
		tunnels := make([]map[string]any, 0, len(records))
		for _, rec := range records {
			tunnels = append(tunnels, map[string]any{
				"id": rec.ID, "name": rec.Tag, "iface": rec.Tag,
				"type": "managed", "status": "configured", "available": true,
			})
		}
		response.Success(w, tunnels)
	}
}

func localProxyInbound(addr string) ([]byte, error) {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return nil, fmt.Errorf("invalid AWG_PROXY_ADDR %q: expected host:port", addr)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid AWG_PROXY_ADDR %q: port must be 1..65535", addr)
	}
	return json.MarshalIndent(map[string]any{"inbounds": []any{map[string]any{
		"type": "mixed", "tag": "local-proxy-in", "listen": host, "listen_port": port,
	}}}, "", "  ")
}

type localProxySlotSaver interface {
	SaveAndValidate(singboxorch.Slot, []byte) (singboxorch.ValidationResult, error)
}

func configureLocalProxy(saver localProxySlotSaver, addr string) error {
	proxySlot, err := localProxyInbound(addr)
	if err != nil {
		return err
	}
	res, err := saver.SaveAndValidate(singboxorch.SlotLocalProxy, proxySlot)
	if err != nil {
		return fmt.Errorf("validate required local proxy slot: %w", err)
	}
	if !res.Ok() {
		return fmt.Errorf("required local proxy configuration rejected")
	}
	return nil
}

// localCompositeTags discovers selector-like outbounds from sing-box's own
// Clash API. It deliberately does not construct router.Service: the portable
// runtime has no NDMS interfaces, policy routes or router NAT.
func localCompositeTags(op *singbox.Operator) func() map[string]struct{} {
	return func() map[string]struct{} {
		if op == nil || op.Clash() == nil {
			return map[string]struct{}{}
		}
		proxies, err := op.Clash().GetProxies()
		if err != nil {
			return map[string]struct{}{}
		}
		result := make(map[string]struct{})
		for tag, proxy := range proxies {
			switch strings.ToLower(proxy.Type) {
			case "selector", "urltest", "loadbalance":
				if proxy.Name != "" {
					tag = proxy.Name
				}
				result[tag] = struct{}{}
			}
		}
		return result
	}
}

// localNDMSProxyToggle keeps the subscription API honest in Docker mode.
// The portable runtime materializes subscription outbounds in sing-box and
// never allocates a Keenetic ProxyN interface.
type localNDMSProxyToggle struct{}

func (localNDMSProxyToggle) IsSingboxNDMSProxyEnabled() bool { return false }

// setupLocal constructs only portable storage/events/sing-box/orchestrator/
// AWG3 state. In particular it does not call the legacy setupSingbox phase,
// whose subscriptions, installer and updater are Keenetic/Entware-aware.
func (a *app) setupLocal() error {
	a.eventBus = events.NewBus()
	a.loggingService.SetEventBus(a.eventBus)

	core, err := buildSingboxCore(singboxCoreDeps{
		settings:               a.settingsStore,
		appLog:                 a.loggingService,
		bus:                    a.eventBus,
		bootLog:                a.bootLog,
		dataDir:                a.dataDir,
		dir:                    a.dataDir + "/sing-box",
		configDir:              a.singboxConfigDir,
		binary:                 a.singboxBinary,
		initialManuallyStopped: a.settings.SingboxManuallyStopped,
		ndmsProxyEnabled:       func() bool { return false },
		skipNDMSCleanup:        true,
		disableNDMSProxy:       true,
		localProxyAddr:         a.proxyAddr,
	})
	if err != nil {
		return err
	}
	a.singboxOp = core.op
	a.sbOrch = core.orch
	a.awg3Store = core.awg3Store
	a.awg3Svc = awg3endpoint.NewService(a.awg3Store, a.sbOrch, a.loggingService)
	// Register core cleanup before the first fallible post-construction step so
	// runWithCleanup tears down debounce/process state on startup failure.
	a.deferOnExit(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = a.singboxOp.QuiesceStop(ctx)
	})
	a.deferOnExit(a.sbOrch.Close)
	if err := configureLocalProxy(a.sbOrch, a.proxyAddr); err != nil {
		return err
	}
	if err := a.awg3Svc.Sync(); err != nil {
		return fmt.Errorf("reconcile required AWG3 state: %w", err)
	}

	// Local DNS routes keep the legacy DomainList persistence/API model while
	// projecting only backend=singbox records into the owned config slot.
	a.dnsRouteStore = dnsroute.NewStore(a.dataDir)
	if _, err := a.dnsRouteStore.Load(); err != nil {
		return fmt.Errorf("load required DNS routes state: %w", err)
	}
	upstream := os.Getenv("AWG_DNS_UPSTREAM")
	if upstream == "" {
		upstream = "1.1.1.1"
	}
	dnsReconciler := dnsroute.NewSingboxReconciler(a.sbOrch, a.awg3Store, upstream)
	a.localDNSRouteService = dnsroute.NewSingboxService(a.dnsRouteStore, dnsReconciler, a.loggingService)
	a.downloadSvc = downloader.NewService(downloader.Deps{})
	a.localDNSRouteService.SetDownloader(&dnsRouteDownloaderAdapter{svc: a.downloadSvc})
	if err := a.localDNSRouteService.Reconcile(context.Background()); err != nil {
		return fmt.Errorf("reconcile required DNS routes state: %w", err)
	}
	a.dnsRefreshScheduler = dnsroute.NewScheduler(a.localDNSRouteService, a.settingsStore, a.loggingService)

	// Portable subscriptions use the same sing-box operator adapter as the
	// router deployment, but without ProxyManager/NDMS dependencies. The
	// adapter owns 40-subscriptions.json and the service owns subscriptions.json.
	a.subStore, err = subscription.NewStore(filepath.Join(a.dataDir, "subscriptions.json"))
	if err != nil {
		return fmt.Errorf("create local subscription store: %w", err)
	}
	a.subAdapter = subscription.NewOperatorAdapter(a.sbOrch, nil, a.singboxOp.Clash())
	a.subAdapter.SetSingboxFeaturesFn(a.singboxOp.SingboxFeatures)
	if err := a.subAdapter.LoadFromDisk(a.singboxConfigDir); err != nil {
		return fmt.Errorf("load local subscription config: %w", err)
	}
	a.subSvc = subscription.NewService(a.subStore, a.subAdapter)
	a.subSvc.SetAppLogger(a.loggingService)
	a.subSvc.SetNDMSProxyEnabled(func() bool { return false })
	if err := a.subSvc.LoadHappKeys(); err != nil {
		a.bootLog.Warn("subscription-happ-keys", "load-from-disk", err.Error())
	}
	a.subGroupStore, err = subscription.NewGroupStore(filepath.Join(a.dataDir, "subscription-groups.json"))
	if err != nil {
		return fmt.Errorf("create local subscription group store: %w", err)
	}
	a.subSvc.SetGroupStore(a.subGroupStore)
	// A Docker deployment has no NDMS ProxyN namespace. Clear legacy proxy
	// indexes from a reused router volume before any local delete/reconcile
	// path can try to call the proxy registrar.
	for _, sub := range a.subStore.List() {
		if sub.ProxyIndex >= 0 {
			if err := a.subStore.SetProxyIndex(sub.ID, -1); err != nil {
				return fmt.Errorf("reset legacy subscription proxy index: %w", err)
			}
		}
	}
	for _, group := range a.subGroupStore.List() {
		if group.ProxyIndex >= 0 {
			if err := a.subGroupStore.SetProxyIndex(group.ID, -1); err != nil {
				return fmt.Errorf("reset legacy subscription group proxy index: %w", err)
			}
		}
	}
	a.singboxOp.SetSubscriptionProxySet(subProxySet{store: a.subStore, groups: a.subGroupStore})
	a.singboxOp.SetSubscriptionProxySync(a.subSvc.SyncProxies)
	if err := a.subSvc.Reconcile(context.Background()); err != nil {
		return fmt.Errorf("reconcile local subscription state: %w", err)
	}

	a.singboxHandler = api.NewSingboxHandler(a.singboxOp, a.eventBus, nil, nil, a.loggingService)

	// Register owned resources from longest- to shortest-lived. runOnExit is
	// LIFO: cancel the watchdog, close debounce work, then stop the process.
	watchdog := singbox.NewWatchdog(a.singboxOp, a.eventBus, slog.Default().With("component", "singbox-watchdog"))
	a.deferOnExit(startJoinedWorker(watchdog.Run))
	// Registered last so the scheduler is stopped before the orchestrator.
	a.dnsRefreshScheduler.Start()
	a.deferOnExit(a.dnsRefreshScheduler.Stop)
	if err := a.setupGatewayRuntime(); err != nil {
		return err
	}
	return nil
}

func (a *app) setupGatewayRuntime() error {
	if strings.ToLower(strings.TrimSpace(os.Getenv("AWG_GATEWAY_ENABLE"))) != "true" {
		return nil
	}
	subscriptionBaseURL, err := validateGatewaySubscriptionBaseURL(os.Getenv("AWG_GATEWAY_SUBSCRIPTION_BASE_URL"))
	if err != nil {
		return err
	}
	a.gatewaySubscriptionBaseURL = subscriptionBaseURL
	name := strings.TrimSpace(os.Getenv("AWG_GATEWAY_INTERFACE"))
	if name == "" {
		name = "awg0"
	}
	wan := strings.TrimSpace(os.Getenv("AWG_GATEWAY_WAN_INTERFACE"))
	if wan == "" {
		return fmt.Errorf("AWG_GATEWAY_WAN_INTERFACE is required when AWG_GATEWAY_ENABLE=true")
	}
	tool := strings.TrimSpace(os.Getenv("AWG_GATEWAY_TOOL"))
	if tool == "" {
		tool = "/usr/bin/wg"
	}
	port := 51820
	if raw := strings.TrimSpace(os.Getenv("AWG_GATEWAY_PORT")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 65535 {
			return fmt.Errorf("invalid AWG_GATEWAY_PORT %q", raw)
		}
		port = parsed
	}
	pool := netip.MustParsePrefix("10.66.0.0/24")
	if raw := strings.TrimSpace(os.Getenv("AWG_GATEWAY_CLIENT_POOL")); raw != "" {
		parsed, err := netip.ParsePrefix(raw)
		if err != nil {
			return fmt.Errorf("invalid AWG_GATEWAY_CLIENT_POOL %q", raw)
		}
		pool = parsed
	}
	serverAddress := strings.TrimSpace(os.Getenv("AWG_GATEWAY_SERVER_ADDRESS"))
	if serverAddress == "" {
		serverAddress = "10.66.0.1/24"
	}
	serverPrefix, err := netip.ParsePrefix(serverAddress)
	if err != nil || !pool.Contains(serverPrefix.Addr()) {
		return fmt.Errorf("invalid AWG_GATEWAY_SERVER_ADDRESS %q", serverAddress)
	}
	identity, err := gatewayingress.LoadOrCreateServerIdentity(filepath.Join(a.dataDir, "gateway-server-identity.json"), func() (string, string, error) {
		return gatewayingress.GenerateKeyPairWithBinary(context.Background(), tool)
	})
	if err != nil {
		return err
	}
	a.gatewayIdentity = identity
	a.gatewayRuntime = &gatewayingress.GatewayRuntime{
		Interface:       gatewayingress.NewLinuxInterfaceManagerWithBinary(tool, name, port, identity.PrivateKey),
		Forwarding:      forwarding.NewLinuxReconciler(),
		PolicyInterface: gatewaypolicy.GatewayTUNInterface,
	}
	a.gatewayRuntime.Interface.SetAddress(serverAddress)
	a.gatewayPeerApplier = gatewayingress.NewLinuxCommandApplier(tool, name)
	if err := a.gatewayRuntime.Bootstrap(context.Background(), pool, wan); err != nil {
		return fmt.Errorf("bootstrap gateway runtime: %w", err)
	}
	allocator, err := gatewayingress.NewAllocator(pool)
	if err != nil {
		return err
	}
	if err := allocator.ReserveAddress(serverPrefix.Addr()); err != nil {
		return fmt.Errorf("reserve gateway server address: %w", err)
	}
	keygen := func(_ string) (string, string, error) {
		return gatewayingress.GenerateKeyPairWithBinary(context.Background(), tool)
	}
	clientStore := filepath.Join(a.dataDir, "gateway-clients.json")
	clients, err := gatewayingress.Load(clientStore, allocator, keygen)
	if os.IsNotExist(errors.Unwrap(err)) || os.IsNotExist(err) {
		clients = gatewayingress.NewRegistry(allocator, keygen)
		err = nil
	}
	if err != nil {
		return fmt.Errorf("load gateway clients: %w", err)
	}
	a.gatewayClients = clients
	if err := a.gatewayClients.ReconcilePeers(context.Background(), a.gatewayPeerApplier); err != nil {
		return fmt.Errorf("reconcile persisted gateway peers: %w", err)
	}
	groupStore, err := gatewaygroups.NewFileStore(filepath.Join(a.dataDir, "gateway-client-groups.json"))
	if err != nil {
		return fmt.Errorf("configure gateway client groups: %w", err)
	}
	a.gatewayGroups, err = gatewaygroups.NewManager(groupStore, func(clientID string) bool {
		_, exists := a.gatewayClients.Get(clientID)
		return exists
	})
	if err != nil {
		return fmt.Errorf("load gateway client groups: %w", err)
	}
	subscriptionStore, err := gatewaysubscription.NewFileStore(filepath.Join(a.dataDir, "gateway-subscriptions.json"))
	if err != nil {
		return fmt.Errorf("configure gateway subscriptions: %w", err)
	}
	a.gatewaySubscriptions, err = gatewaysubscription.NewManager(subscriptionStore)
	if err != nil {
		return fmt.Errorf("load gateway subscriptions: %w", err)
	}
	policyStore, err := gatewaypolicy.NewFileStore(filepath.Join(a.dataDir, "gateway-policy-profiles.json"))
	if err != nil {
		return fmt.Errorf("configure gateway policy profiles: %w", err)
	}
	if _, err := policyStore.Load(); err != nil {
		return fmt.Errorf("load gateway policy profiles: %w", err)
	}
	a.gatewayPolicyStore = policyStore
	a.gatewayPolicy = gatewaypolicy.Runtime{
		Catalog:          a.awg3Svc,
		Saver:            a.sbOrch,
		ClientPool:       pool,
		IngressInterface: name,
		WANInterface:     wan,
	}
	return nil
}

func (a *app) setupLocalServer() {
	frontendFS, err := frontend.FS()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load embedded frontend: %v\n", err)
		os.Exit(1)
	}
	a.sessionStore = auth.NewSessionStore(a.settingsStore.GetSessionTTL)
	a.deferOnExit(a.sessionStore.Stop)
	awg3Handler := api.NewAwg3Handler(a.awg3Store, a.awg3Svc, nil, a.loggingService)
	awg3Handler.SetAfterMutation(func(candidate []awg3endpoint.Record) (func() error, error) {
		return a.localDNSRouteService.ReconcileAWG3Candidate(context.Background(), candidate)
	})
	dnsRouteHandler := api.NewDNSRouteHandler(a.localDNSRouteService, a.loggingService)
	dnsRouteHandler.SetEventBus(a.eventBus)
	settingsHandler := api.NewSettingsHandler(a.settingsStore, a.loggingService)
	settingsHandler.SetEventBus(a.eventBus)
	settingsHandler.SetApplyLoggingSettings(a.loggingService.ApplySettings)
	settingsHandler.SetApplySingboxLogSettings(func() error {
		return a.singboxOp.ApplyLogLevel(a.settingsStore.GetSingboxLogLevel())
	})
	settingsHandler.SetApplyBootstrapDNS(a.singboxOp.ApplyBootstrapDNS)
	settingsHandler.SetApplyClashPort(a.singboxOp.ApplyClashPort)
	localTunnels := newLocalTunnelsHandler(a.awg3Store, a.awg3Store.TransactionLock())
	localRefresh := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			response.MethodNotAllowed(w)
			return
		}
		if err := a.localDNSRouteService.RefreshAllSubscriptions(r.Context()); err != nil {
			response.Error(w, err.Error(), "DNS_ROUTE_REFRESH_ALL_ERROR")
			return
		}
		response.Success(w, map[string]any{"missing": []string{}})
	}
	gatewayHandler := newGatewayClientHTTP(a.gatewayClients, filepath.Join(a.dataDir, "gateway-clients.json"), a.gatewayPeerApplier)
	if gatewayHandler != nil {
		gatewayHandler.SetClientConfig(a.gatewayIdentity.PublicKey, os.Getenv("AWG_GATEWAY_PUBLIC_ENDPOINT"))
		gatewayHandler.SetGatewayGroups(a.gatewayGroups)
		if a.gatewayPolicyStore != nil {
			gatewayHandler.SetGatewayPolicy(a.gatewayPolicyStore, a.gatewayPolicy)
		}
		gatewayHandler.SetClientSubscriptions(a.gatewaySubscriptions)
		if err := gatewayHandler.SetSubscriptionBaseURL(a.gatewaySubscriptionBaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to configure gateway subscription URL: %v\n", err)
			os.Exit(1)
		}
	}
	a.srv = server.NewLocal(server.Config{
		Version:    version,
		FrontendFS: frontendFS,
	}, server.LocalDeps{
		Common: server.CommonCapabilities{
			Settings:        a.settingsStore,
			Sessions:        a.sessionStore,
			LoggingService:  a.loggingService,
			Bus:             a.eventBus,
			SettingsHandler: settingsHandler,
		},
		Singbox: server.SingboxCapabilities{
			Status:      a.singboxOp,
			Handler:     a.singboxHandler,
			Awg3Handler: awg3Handler,
		},
		Routing: server.RoutingCapabilities{
			DNSRoutes:       dnsRouteHandler,
			DNSRouteBackend: dnsroute.BackendSingbox,
			Tunnels:         localTunnels,
			Refresh:         localRefresh,
			Gateway:         gatewayHandler,
		},
	})
	// These handlers consume only the local orchestrator/config directory and
	// the sing-box loopback Clash API. They stay outside router wiring.
	a.srv.SetSingboxConfigHandler(api.NewSingboxConfigHandler(a.sbOrch.ConfigDir))
	a.srv.SetSingboxConfigEditorHandler(api.NewSingboxConfigEditorHandler(a.sbOrch, a.loggingService))
	a.srv.SetSingboxInboundsHandler(api.NewSingboxInboundsHandler(api.SingboxInboundsDeps{
		ConfigDir:        a.sbOrch.ConfigDir,
		NDMSProxyEnabled: func() bool { return false },
	}))
	a.clashProxy = api.NewClashProxy(a.singboxOp)
	a.srv.SetClashProxy(a.clashProxy)
	a.srv.SetSingboxProxiesHandler(api.NewSingboxProxiesHandler(
		a.clashProxy.ClashBaseURL,
		localCompositeTags(a.singboxOp),
		nil,
	))
	// Subscription CRUD is portable: it materializes directly into sing-box
	// slots and never creates NDMS ProxyN interfaces.
	subSched := subscription.NewScheduler(a.subStore, func(ctx context.Context, id string) error {
		_, err := a.subSvc.Refresh(ctx, id)
		return err
	})
	subSched.SetAppLogger(a.loggingService)
	subCtx, subCancel := context.WithCancel(context.Background())
	subSched.Start(subCtx)
	a.deferOnExit(func() {
		subCancel()
		subSched.Stop()
	})
	subHandler := api.NewSubscriptionHandler(a.subSvc, a.singboxOp, a.loggingService)
	subHandler.SetNDMSProxyToggler(localNDMSProxyToggle{})
	a.srv.SetSubscriptionHandler(subHandler)
}

func (a *app) setupLocalLifecycle() {
	a.shutdownCtx, a.shutdownCancel = context.WithCancel(context.Background())
	a.deferOnExit(a.shutdownCancel)
	// AWG_HTTP_ADDR is passed through as one exact net.Listen address. No
	// KeenDNS discovery, port fallback, interface expansion, or persistence.
	a.srv.SetListenAddr(a.httpAddr)
	a.srv.AddShutdownHook(a.shutdownCancel)
}

// registerProxyShutdown retains the legacy hook at its historical point in
// the Keenetic phase order while keeping it impossible to call in local mode.
func (a *app) registerProxyShutdown() {
	a.srv.AddShutdownHook(func() {
		if a.proxyMgr != nil {
			a.proxyMgr.Shutdown()
		}
	})
}

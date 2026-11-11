package router

import (
	"context"
	"fmt"
	"sync"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/presets"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

// PortableRuntime owns platform-dependent state and activation. Implementations
// must validate/apply against the gateway ingress, not the Keenetic router slot.
// Configuration CRUD remains the original ServiceImpl and creates drafts only.
type PortableRuntime interface {
	GetStatus(context.Context) (Status, error)
	GetSettings(context.Context) (storage.SingboxRouterSettings, error)
	UpdateSettings(context.Context, storage.SingboxRouterSettings) error
	Reconcile(context.Context) error
	SwitchRoutingMode(context.Context, string) error
	ApplyStaging(context.Context) (orchestrator.ValidationResult, error)
	StagingStatus(context.Context) StagingStatus
	RenameExternalOutboundTag(context.Context, string, string) error
}

// PortableDeps deliberately excludes netfilter, NDMS, OpkgTun and policy
// providers. A missing platform implementation is an error, never legacy mode.
type PortableDeps struct {
	Settings               *storage.SettingsStore
	Catalog                *presets.Catalog
	Singbox                SingboxController
	Orch                   *orchestrator.Orchestrator
	Events                 *events.Bus
	Bus                    StagingEventBus
	AppLog                 logging.AppLogger
	Runtime                PortableRuntime
	Documents              EditorDocuments
	Coordinator            sync.Locker
	ResolvePresetRuleSets  func(context.Context, Preset) ([]RuleSet, error)
	Tunnels                SingboxTunnelCatalog
	SubscriptionComposites *SubscriptionCompositesAdapter
}

// NewPortableService reuses the original editor without constructing router
// platform dependencies or refreshing any host netfilter hook.
func NewPortableService(d PortableDeps) (Service, error) {
	if d.Runtime == nil || d.Settings == nil || d.Singbox == nil || d.Orch == nil || d.Coordinator == nil || d.Documents == nil {
		return nil, fmt.Errorf("portable router requires runtime, settings, sing-box, orchestrator, canonical documents and shared coordinator")
	}
	return &ServiceImpl{
		deps: Deps{Settings: d.Settings, PresetCatalog: d.Catalog, Singbox: d.Singbox,
			Orch: d.Orch, Events: d.Events, Bus: d.Bus, AppLog: d.AppLog,
			SingboxTunnels: d.Tunnels, SubscriptionComposites: d.SubscriptionComposites},
		portable:               d.Runtime,
		portableDocuments:      d.Documents,
		portableGuard:          d.Coordinator,
		portablePresetRuleSets: d.ResolvePresetRuleSets,
		appLog:                 logging.NewScopedLogger(d.AppLog, logging.GroupRouting, logging.SubSingboxRouter),
		bypassLog:              logging.NewScopedLogger(d.AppLog, logging.GroupRouting, logging.SubBypassSet),
	}, nil
}

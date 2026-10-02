package dnsroute

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
	"github.com/hoaxisr/awg-manager/internal/logging"
	singboxorch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

// SingboxRouteOrchestrator is the config ownership surface required by local
// DNS routes. LoadEffective supplies the current cross-slot tag namespace and
// SaveAndValidate performs merged-config validation plus atomic slot replace.
type SingboxRouteOrchestrator interface {
	LoadEffective(singboxorch.Slot) ([]byte, error)
	SnapshotSlot(singboxorch.Slot) ([]byte, bool, error)
	SaveAndValidate(singboxorch.Slot, []byte) (singboxorch.ValidationResult, error)
	SetEnabled(singboxorch.Slot, bool) error
	RestoreSlot(singboxorch.Slot, []byte, bool) error
}

type AWG3RecordLister interface {
	List() ([]awg3endpoint.Record, error)
}

type awg3TransactionAwareLister interface {
	AWG3RecordLister
	TransactionLock() awg3endpoint.TransactionLocker
}

type dnsRouteTransactionLock interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

// SingboxReconciler compiles the complete persisted candidate on every call.
// It never performs NDMS interface discovery: RouteTarget is resolved only
// against local AWG3 record IDs/tags (plus direct/reject terminal targets).
type SingboxReconciler struct {
	orch        SingboxRouteOrchestrator
	endpoints   AWG3RecordLister
	dnsUpstream string
}

func NewSingboxReconciler(orch SingboxRouteOrchestrator, endpoints AWG3RecordLister, dnsUpstream string) *SingboxReconciler {
	return &SingboxReconciler{orch: orch, endpoints: endpoints, dnsUpstream: dnsUpstream}
}

func (r *SingboxReconciler) Reconcile(lists []DomainList) error {
	if endpoints, ok := r.endpoints.(awg3TransactionAwareLister); ok {
		lock := endpoints.TransactionLock()
		lock.RLock()
		defer lock.RUnlock()
	}
	return r.reconcileInAWG3Transaction(lists)
}

// reconcileInAWG3Transaction is used only while the caller already owns the
// AWG3 write transaction. Re-acquiring its read side would deadlock.
func (r *SingboxReconciler) reconcileInAWG3Transaction(lists []DomainList) error {
	records, err := r.endpoints.List()
	if err != nil {
		return fmt.Errorf("list AWG3 endpoints: %w", err)
	}
	return r.reconcileCandidateInAWG3Transaction(lists, records)
}

// reconcileCandidateInAWG3Transaction compiles only against the supplied AWG3
// snapshot. It must be called while the caller owns the AWG3 transaction lock.
func (r *SingboxReconciler) reconcileCandidateInAWG3Transaction(lists []DomainList, records []awg3endpoint.Record) error {
	if r == nil || r.orch == nil {
		return fmt.Errorf("sing-box DNS route reconciler is not configured")
	}
	outboundTags, dnsTags, err := r.effectiveReservedTags()
	if err != nil {
		return err
	}
	byTarget := make(map[string]string, len(records)*2)
	for _, rec := range records {
		byTarget[rec.ID] = rec.Tag
		byTarget[rec.Tag] = rec.Tag
	}
	fragment, err := CompileSingbox(lists, SingboxCompileOptions{
		DNSUpstream: r.dnsUpstream,
		ResolveTarget: func(target RouteTarget) (string, error) {
			switch target.TunnelID {
			case "direct", "reject", "block":
				return target.TunnelID, nil
			}
			if tag := byTarget[target.TunnelID]; tag != "" {
				return tag, nil
			}
			if tag := byTarget[target.Interface]; tag != "" {
				return tag, nil
			}
			return "", fmt.Errorf("AWG3 endpoint %q not found", target.TunnelID)
		},
		ReservedOutboundTags: outboundTags,
		ReservedDNSTags:      dnsTags,
	})
	if err != nil {
		return err
	}
	before, err := r.captureSlot()
	if err != nil {
		return err
	}
	result, err := r.orch.SaveAndValidate(singboxorch.SlotDNSRoutes, fragment)
	if err != nil {
		return fmt.Errorf("save sing-box DNS routes: %w", err)
	}
	if !result.Ok() {
		return fmt.Errorf("sing-box DNS routes rejected: %s", result.Error())
	}
	enabled := !bytes.Equal(bytes.TrimSpace(fragment), []byte("{}"))
	if err := r.orch.SetEnabled(singboxorch.SlotDNSRoutes, enabled); err != nil {
		applyErr := fmt.Errorf("set sing-box DNS routes enabled=%t: %w", enabled, err)
		if rollbackErr := r.restoreSlot(before); rollbackErr != nil {
			return fmt.Errorf("%v; restore prior DNS routes slot: %w", applyErr, rollbackErr)
		}
		return applyErr
	}
	return nil
}

type dnsRouteSlotSnapshot struct {
	data    []byte
	enabled bool
}

func (r *SingboxReconciler) captureSlot() (dnsRouteSlotSnapshot, error) {
	data, enabled, err := r.orch.SnapshotSlot(singboxorch.SlotDNSRoutes)
	if err != nil {
		return dnsRouteSlotSnapshot{}, fmt.Errorf("snapshot sing-box DNS routes slot: %w", err)
	}
	return dnsRouteSlotSnapshot{data: append([]byte(nil), data...), enabled: enabled}, nil
}

func (r *SingboxReconciler) restoreSlot(snapshot dnsRouteSlotSnapshot) error {
	return r.orch.RestoreSlot(singboxorch.SlotDNSRoutes, snapshot.data, snapshot.enabled)
}

func (r *SingboxReconciler) holdReloads() func() {
	if holder, ok := r.orch.(interface{ HoldReloads() func() }); ok {
		return holder.HoldReloads()
	}
	return func() {}
}

func (r *SingboxReconciler) effectiveReservedTags() (map[string]struct{}, map[string]struct{}, error) {
	outbounds := make(map[string]struct{})
	dnsServers := make(map[string]struct{})
	for _, meta := range singboxorch.KnownSlots() {
		if meta.Slot == singboxorch.SlotDNSRoutes {
			continue
		}
		data, err := r.orch.LoadEffective(meta.Slot)
		if err != nil {
			return nil, nil, fmt.Errorf("load effective sing-box slot %s: %w", meta.Slot, err)
		}
		if len(data) == 0 {
			continue
		}
		var fragment struct {
			Outbounds []struct {
				Tag string `json:"tag"`
			} `json:"outbounds"`
			Endpoints []struct {
				Tag string `json:"tag"`
			} `json:"endpoints"`
			DNS struct {
				Servers []struct {
					Tag string `json:"tag"`
				} `json:"servers"`
			} `json:"dns"`
		}
		if err := json.Unmarshal(data, &fragment); err != nil {
			return nil, nil, fmt.Errorf("parse effective sing-box slot %s: %w", meta.Slot, err)
		}
		for _, item := range fragment.Outbounds {
			if item.Tag != "" {
				outbounds[item.Tag] = struct{}{}
			}
		}
		for _, item := range fragment.Endpoints {
			if item.Tag != "" {
				outbounds[item.Tag] = struct{}{}
			}
		}
		for _, item := range fragment.DNS.Servers {
			if item.Tag != "" {
				dnsServers[item.Tag] = struct{}{}
			}
		}
	}
	return outbounds, dnsServers, nil
}

// SingboxService preserves the established DomainList CRUD implementation and
// response semantics while running each mutation against an isolated in-memory
// candidate. The candidate is compiled, validated and applied to sing-box before
// one durable store commit; reads share the transaction lock and therefore see
// either the old committed snapshot or the new one, never the candidate.
type SingboxService struct {
	mu         dnsRouteTransactionLock
	core       *ServiceImpl
	store      *Store
	reconciler *SingboxReconciler
	appLogger  logging.AppLogger
}

func NewSingboxService(store *Store, reconciler *SingboxReconciler, appLogger logging.AppLogger) *SingboxService {
	core := NewService(store, nil, nil, nil, appLogger)
	core.reconcileFn = func(context.Context) error { return nil }
	return &SingboxService{mu: &sync.RWMutex{}, core: core, store: store, reconciler: reconciler, appLogger: appLogger}
}

func (s *SingboxService) SetDownloader(d Downloader) { s.core.SetDownloader(d) }
func (s *SingboxService) Get(ctx context.Context, id string) (*DomainList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.core.Get(ctx, id)
}
func (s *SingboxService) List(ctx context.Context) ([]DomainList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.core.List(ctx)
}

func (s *SingboxService) mutate(run func(*ServiceImpl) error) error {
	unlockAWG := s.lockAWGRead()
	defer unlockAWG()
	s.mu.Lock()
	defer s.mu.Unlock()
	before, err := cloneStoreData(s.store.GetCached())
	if err != nil {
		return err
	}
	candidateStore := newMemoryStore(before)
	candidateCore := NewService(candidateStore, nil, nil, nil, s.appLogger)
	candidateCore.reconcileFn = func(context.Context) error { return nil }
	candidateCore.SetDownloader(s.core.downloader())
	if err := run(candidateCore); err != nil {
		return err
	}
	current := candidateStore.GetCached()
	if current == nil {
		return fmt.Errorf("store not loaded")
	}
	releaseReloads := s.reconciler.holdReloads()
	defer releaseReloads()
	slotBefore, err := s.reconciler.captureSlot()
	if err != nil {
		return err
	}
	if err := s.reconciler.reconcileInAWG3Transaction(current.Lists); err != nil {
		return err
	}
	if err := s.store.Save(current); err != nil {
		if rollbackErr := s.reconciler.restoreSlot(slotBefore); rollbackErr != nil {
			return fmt.Errorf("commit DNS routes: %v; restore active DNS routes: %w", err, rollbackErr)
		}
		return fmt.Errorf("commit DNS routes: %w", err)
	}
	return nil
}

func cloneStoreData(data *StoreData) (*StoreData, error) {
	if data == nil {
		return nil, fmt.Errorf("store not loaded")
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("snapshot DNS routes: %w", err)
	}
	var cloned StoreData
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, fmt.Errorf("snapshot DNS routes: %w", err)
	}
	return &cloned, nil
}

func (s *SingboxService) Create(ctx context.Context, list DomainList) (out *DomainList, err error) {
	list.Backend = BackendSingbox
	err = s.mutate(func(core *ServiceImpl) error { var e error; out, e = core.Create(ctx, list); return e })
	return
}
func (s *SingboxService) Update(ctx context.Context, list DomainList) (out *DomainList, err error) {
	list.Backend = BackendSingbox
	err = s.mutate(func(core *ServiceImpl) error { var e error; out, e = core.Update(ctx, list); return e })
	return
}
func (s *SingboxService) Delete(ctx context.Context, id string) error {
	return s.mutate(func(core *ServiceImpl) error { return core.Delete(ctx, id) })
}
func (s *SingboxService) DeleteBatch(ctx context.Context, ids []string) (count int, err error) {
	err = s.mutate(func(core *ServiceImpl) error { var e error; count, e = core.DeleteBatch(ctx, ids); return e })
	return
}
func (s *SingboxService) CreateBatch(ctx context.Context, lists []DomainList) (out []*DomainList, err error) {
	for i := range lists {
		lists[i].Backend = BackendSingbox
	}
	err = s.mutate(func(core *ServiceImpl) error { var e error; out, e = core.CreateBatch(ctx, lists); return e })
	return
}
func (s *SingboxService) SetEnabled(ctx context.Context, id string, enabled bool) error {
	return s.mutate(func(core *ServiceImpl) error { return core.SetEnabled(ctx, id, enabled) })
}
func (s *SingboxService) RefreshSubscriptions(ctx context.Context, id string) error {
	return s.mutate(func(core *ServiceImpl) error { return core.RefreshSubscriptions(ctx, id) })
}
func (s *SingboxService) RefreshAllSubscriptions(ctx context.Context) error {
	return s.mutate(func(core *ServiceImpl) error { return core.RefreshAllSubscriptions(ctx) })
}
func (s *SingboxService) Reconcile(context.Context) error {
	unlockAWG := s.lockAWGRead()
	defer unlockAWG()
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.store.GetCached()
	if data == nil {
		return fmt.Errorf("store not loaded")
	}
	return s.reconciler.reconcileInAWG3Transaction(data.Lists)
}

func (s *SingboxService) lockAWGRead() func() {
	if endpoints, ok := s.reconciler.endpoints.(awg3TransactionAwareLister); ok {
		lock := endpoints.TransactionLock()
		lock.RLock()
		return lock.RUnlock
	}
	return func() {}
}

// ReconcileAWG3Candidate rebuilds dependent routes against candidate while the
// caller owns the AWG3 write transaction. On success it returns an exact slot
// rollback closure for a later durable-commit failure.
func (s *SingboxService) ReconcileAWG3Candidate(_ context.Context, candidate []awg3endpoint.Record) (func() error, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.store.GetCached()
	if data == nil {
		return nil, fmt.Errorf("store not loaded")
	}
	before, err := s.reconciler.captureSlot()
	if err != nil {
		return nil, err
	}
	if err := s.reconciler.reconcileCandidateInAWG3Transaction(data.Lists, candidate); err != nil {
		if rollbackErr := s.reconciler.restoreSlot(before); rollbackErr != nil {
			return nil, fmt.Errorf("reconcile DNS routes: %v; restore prior DNS routes slot: %w", err, rollbackErr)
		}
		return nil, err
	}
	return func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.reconciler.restoreSlot(before)
	}, nil
}

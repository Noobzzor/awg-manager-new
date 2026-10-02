package subscription

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

// RefreshFunc is the callback the scheduler invokes for due subscriptions.
// In production it's Service.Refresh; tests inject a stub.
type RefreshFunc func(ctx context.Context, id string) error

// Scheduler walks the Store every tickInterval and refreshes any due item.
// Per-subscription serialization is enforced by the Service mutex; the
// scheduler runs each due item in its own goroutine.
type Scheduler struct {
	store        *Store
	doRefresh    RefreshFunc
	tickInterval time.Duration
	stop         chan struct{}
	stopOnce     sync.Once
	log          *logging.ScopedLogger
	loopWG       sync.WaitGroup
	workerWG     sync.WaitGroup

	// inflight guards against relaunching a refresh for a subscription whose
	// previous refresh is still running (#331). Without it, a refresh slower
	// than tickInterval is re-launched every tick and the launches queue up
	// behind the per-subscription mutex.
	mu       sync.Mutex
	inflight map[string]struct{}
}

func NewScheduler(store *Store, doRefresh RefreshFunc) *Scheduler {
	return &Scheduler{
		store:        store,
		doRefresh:    doRefresh,
		tickInterval: time.Minute,
		stop:         make(chan struct{}),
		inflight:     make(map[string]struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.loopWG.Add(1)
	go func() {
		defer s.loopWG.Done()
		s.loop(ctx)
	}()
}

func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
	})
	s.loopWG.Wait()
	s.workerWG.Wait()
}

func (s *Scheduler) SetAppLogger(app logging.AppLogger) {
	s.log = logging.NewScopedLogger(app, logging.GroupSingbox, logging.SubSBRuntime)
}

func (s *Scheduler) loop(ctx context.Context) {
	t := time.NewTicker(s.tickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case now := <-t.C:
			s.tick(ctx, now)
		}
	}
}

// tick is the internal step exercised by tests.
func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	due := s.store.MaybeRefresh(now)
	if s.log != nil && len(due) > 0 {
		s.log.Info("subscription-scheduler", "", fmt.Sprintf("tick due=%d", len(due)))
	}
	for _, sub := range due {
		id := sub.ID
		// Skip if a refresh for this subscription is already running.
		s.mu.Lock()
		if _, running := s.inflight[id]; running {
			s.mu.Unlock()
			continue
		}
		s.inflight[id] = struct{}{}
		s.mu.Unlock()

		s.workerWG.Add(1)
		go func() {
			defer s.workerWG.Done()
			defer func() {
				s.mu.Lock()
				delete(s.inflight, id)
				s.mu.Unlock()
			}()
			if err := s.doRefresh(ctx, id); err != nil && s.log != nil {
				s.log.Warn("subscription-scheduler", id, "refresh failed: "+err.Error())
			}
		}()
	}
}

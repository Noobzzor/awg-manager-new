// Package heavyop serializes memory-heavy sing-box work (config apply/reload).
package heavyop

import (
	"context"
	"sync"
)

// Gate ensures memory-heavy sing-box operations do not run concurrently.
// Channel-backed (not sync.Mutex) — TryLock'ом пользуется лечение
// отвалившегося tun, чтобы пропустить такт вместо ожидания.
type Gate struct {
	once sync.Once
	ch   chan struct{}
}

// Default is the process-wide gate shared by the orchestrator reload and the
// router's direct config apply.
var Default Gate

func (g *Gate) sem() chan struct{} {
	g.once.Do(func() { g.ch = make(chan struct{}, 1) })
	return g.ch
}

// Lock blocks until no other heavy operation is running.
func (g *Gate) Lock() {
	g.sem() <- struct{}{}
}

// LockContext waits for the same lifecycle gate, allowing a queued operation
// to be cancelled without acquiring or retaining the lock.
func (g *Gate) LockContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case g.sem() <- struct{}{}:
		if err := ctx.Err(); err != nil {
			g.Unlock()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Unlock releases the gate.
func (g *Gate) Unlock() {
	select {
	case <-g.sem():
	default:
		panic("heavyop: Unlock of unlocked Gate")
	}
}

// TryLock reports whether the gate was acquired without blocking.
func (g *Gate) TryLock() bool {
	select {
	case g.sem() <- struct{}{}:
		return true
	default:
		return false
	}
}

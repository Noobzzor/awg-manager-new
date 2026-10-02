// internal/singbox/restart_backoff.go
package singbox

import (
	"sync"
	"time"
)

// Restart-backoff policy (issue #456, anti crash-loop).
//
// The watchdog and the router reconcile loops tick every 30s. Without a
// gate, a sing-box that crashes instantly on start (e.g. OOM on rule-set
// load) would be respawned twice a minute forever, burning CPU/RAM on a
// low-end MIPS router and flooding the log. The policy below keeps the
// fast recovery for the common single-crash case and throttles only a
// genuine loop.
const (
	// restartFreeBudget: this many auto-restarts inside restartCrashWindow
	// are allowed immediately (no pause). Covers the common transient
	// crash — one restart usually fixes it; 3 leaves room for a flaky
	// upstream (rule-set fetch) without ever pausing.
	restartFreeBudget = 3

	// restartCrashWindow is the sliding window the free budget applies to.
	// 10 minutes ≈ 20 watchdog ticks — long enough that a loop is obvious,
	// short enough that an unrelated crash hours later starts fresh.
	restartCrashWindow = 10 * time.Minute

	// restartBackoffBase is the first suppression pause once the free
	// budget is exhausted; each subsequent over-budget attempt doubles it
	// (1m → 2m → 4m → 8m → cap).
	restartBackoffBase = 1 * time.Minute

	// restartBackoffCap bounds the exponential pause. 15 minutes keeps a
	// persistently-broken engine retrying a few times an hour — enough to
	// self-heal when the cause disappears (memory freed, network back)
	// without a manual restart, cheap enough to never hurt.
	restartBackoffCap = 15 * time.Minute

	// restartHealthyRun: a process that lived at least this long before
	// dying is NOT considered part of a crash loop — the counters reset so
	// the next crash gets the full free budget again.
	restartHealthyRun = 5 * time.Minute

	// crashHistoryCap bounds the in-memory crash ring surfaced via
	// CrashStats (time + reason). 5 is plenty for the UI («Падений за 10
	// мин») and keeps the Operator footprint fixed.
	crashHistoryCap = 5
)

// restartBackoff tracks consecutive automatic restarts of the sing-box
// process and decides whether the next one is allowed now or must wait.
// Zero value is ready to use (tests build Operator structs directly).
//
// Only AUTOMATIC recovery paths consult it (Operator.Reconcile liveness
// branch, router tproxy/fakeip reconciles via AutoRestartIfCrashed);
// manual Control("start"/"restart") stays ungated and calls Reset.
type restartBackoff struct {
	mu sync.Mutex

	// attempts holds timestamps of ALLOWED auto-restarts within the
	// sliding window; pruned on every Allow.
	attempts []time.Time

	// suppressedUntil is the end of the current pause; zero when none.
	suppressedUntil time.Time

	// denials counts consecutive over-budget suppressions — the exponent
	// for the pause. Reset only by Reset / a healthy run, NOT by a merely
	// expired window, so a persistent loop keeps long pauses.
	denials int

	// lastStart is the last known successful process start (any path —
	// auto or manual). NoteCrash compares against it to detect a healthy
	// run.
	lastStart time.Time
}

// restartToken is the receipt of one allowed auto-restart attempt.
// Allow records the attempt optimistically; the caller settles it once
// the outcome is known: Commit keeps the record (the attempt really
// consumed the budget — spawn happened or start failed), Rollback
// removes it (the spawn no-op'ed because another reconcile won the
// race — watchdog vs router; the single real restart must burn the
// budget exactly once, issue #456 FIX-E). Both are idempotent; the
// first settle wins.
type restartToken struct {
	b       *restartBackoff
	at      time.Time
	settled bool
}

// Commit finalises the attempt recorded by Allow. Safe on nil.
func (t *restartToken) Commit() {
	if t == nil {
		return
	}
	t.b.mu.Lock()
	t.settled = true
	t.b.mu.Unlock()
}

// Rollback refunds the attempt recorded by Allow (the start no-op'ed —
// process already running). No-op after Commit/Rollback, safe on nil,
// tolerant of a concurrent Reset having cleared the attempts.
func (t *restartToken) Rollback() {
	if t == nil {
		return
	}
	t.b.mu.Lock()
	defer t.b.mu.Unlock()
	if t.settled {
		return
	}
	t.settled = true
	// Удаляем ПОСЛЕДНЮЮ запись с нашим временем — именно её добавил Allow.
	for i := len(t.b.attempts) - 1; i >= 0; i-- {
		if t.b.attempts[i].Equal(t.at) {
			t.b.attempts = append(t.b.attempts[:i], t.b.attempts[i+1:]...)
			return
		}
	}
}

// Allow reports whether an automatic restart may happen at now. When
// allowed, tok is non-nil and the attempt is recorded against the
// window — the caller MUST settle the token (Commit when the attempt
// truly happened, Rollback when the start no-op'ed) so the budget
// stays honest under the watchdog/router race. When denied (tok nil),
// until is the end of the pause; newlySuppressed is true only for the
// call that STARTED this pause — callers use it to log once instead of
// on every 30s tick.
func (b *restartBackoff) Allow(now time.Time) (tok *restartToken, until time.Time, newlySuppressed bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if now.Before(b.suppressedUntil) {
		return nil, b.suppressedUntil, false
	}
	// Prune attempts that slid out of the window.
	kept := b.attempts[:0]
	for _, t := range b.attempts {
		if now.Sub(t) < restartCrashWindow {
			kept = append(kept, t)
		}
	}
	b.attempts = kept
	if len(b.attempts) < restartFreeBudget {
		b.attempts = append(b.attempts, now)
		return &restartToken{b: b, at: now}, time.Time{}, false
	}
	pause := restartBackoffBase << b.denials
	if pause > restartBackoffCap || pause <= 0 { // <=0 guards shift overflow
		pause = restartBackoffCap
	}
	b.denials++
	b.suppressedUntil = now.Add(pause)
	return nil, b.suppressedUntil, true
}

// NoteProcessStart records a successful process start (auto or manual) so
// NoteCrash can measure the run duration.
func (b *restartBackoff) NoteProcessStart(now time.Time) {
	b.mu.Lock()
	b.lastStart = now
	b.mu.Unlock()
}

// LastStart returns the last known successful process start (zero when
// none was recorded yet). Used by the dmesg OOM freshness check to
// discard OOM-killer traces older than the current run (issue #456 FIX-G).
func (b *restartBackoff) LastStart() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastStart
}

// NoteCrash records a non-deliberate process exit. A run that lasted at
// least restartHealthyRun resets the loop counters: the engine was fine
// for a while, so the next crash deserves the full free budget.
func (b *restartBackoff) NoteCrash(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.lastStart.IsZero() && now.Sub(b.lastStart) >= restartHealthyRun {
		b.resetLocked()
	}
}

// Reset clears all counters and any active pause. Called by manual
// Control actions — a user-initiated start/restart is an explicit intent
// that must never be delayed by history.
func (b *restartBackoff) Reset() {
	b.mu.Lock()
	b.resetLocked()
	b.mu.Unlock()
}

func (b *restartBackoff) resetLocked() {
	b.attempts = nil
	b.suppressedUntil = time.Time{}
	b.denials = 0
}

// SuppressedUntil returns the end of the active pause, or the zero time
// when auto-restart is not currently suppressed.
func (b *restartBackoff) SuppressedUntil(now time.Time) time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	if now.Before(b.suppressedUntil) {
		return b.suppressedUntil
	}
	return time.Time{}
}

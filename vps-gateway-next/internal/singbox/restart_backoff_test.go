package singbox

import (
	"testing"
	"time"
)

// Первые restartFreeBudget падений внутри окна перезапускаются сразу.
func TestRestartBackoff_FreeBudgetWithinWindow(t *testing.T) {
	var b restartBackoff
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < restartFreeBudget; i++ {
		tok, _, newly := b.Allow(now.Add(time.Duration(i) * 30 * time.Second))
		if tok == nil {
			t.Fatalf("attempt %d: want allowed, got denied", i+1)
		}
		tok.Commit()
		if newly {
			t.Fatalf("attempt %d: newlySuppressed must be false on allow", i+1)
		}
	}
}

// После исчерпания бюджета — экспоненциальная пауза с капом; повторные
// запросы внутри паузы не продлевают её и не считаются «новым» подавлением.
func TestRestartBackoff_ExponentialGrowthAndCap(t *testing.T) {
	var b restartBackoff
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < restartFreeBudget; i++ {
		b.Allow(now)
	}

	wantPauses := []time.Duration{
		1 * time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute,
		15 * time.Minute, 15 * time.Minute, // cap
	}
	cur := now
	for i, want := range wantPauses {
		tok, until, newly := b.Allow(cur)
		if tok != nil {
			t.Fatalf("denial %d: want denied, got allowed", i+1)
		}
		if !newly {
			t.Fatalf("denial %d: want newlySuppressed=true (new pause)", i+1)
		}
		if got := until.Sub(cur); got != want {
			t.Fatalf("denial %d: pause = %v, want %v", i+1, got, want)
		}
		// Тик внутри паузы: отказ без нового подавления, until не двигается.
		midTok, midUntil, midNewly := b.Allow(cur.Add(10 * time.Second))
		if midTok != nil || midNewly || !midUntil.Equal(until) {
			t.Fatalf("denial %d mid-pause tick: allowed=%v newly=%v until=%v (want denied, not-new, %v)",
				i+1, midTok != nil, midNewly, midUntil, until)
		}
		// Прыгаем за конец паузы. Окно попыток держим занятым: без свежих
		// попыток окно (10 мин) истечёт и бюджет вернётся — здесь проверяем
		// именно рост паузы, поэтому cur двигается на pause, а попытки
		// «обновляются» самим отказом? Нет: attempts не пополняются на отказе.
		// Поэтому продлеваем окно вручную через маленький сдвиг.
		cur = cur.Add(want)
		if cur.Sub(now) >= restartCrashWindow {
			// Попытки выпали из окна: заполняем бюджет заново, имитируя
			// продолжающийся crash-loop (перезапуск → мгновенное падение).
			for j := 0; j < restartFreeBudget; j++ {
				if tok, _, _ := b.Allow(cur); tok == nil {
					t.Fatalf("denial %d: re-fill attempt %d unexpectedly denied", i+1, j+1)
				}
			}
		}
	}
}

// Истечение окна возвращает бесплатный бюджет.
func TestRestartBackoff_WindowExpiryRestoresBudget(t *testing.T) {
	var b restartBackoff
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < restartFreeBudget; i++ {
		b.Allow(now)
	}
	if tok, _, _ := b.Allow(now.Add(time.Second)); tok != nil {
		t.Fatalf("budget exhausted: want denied")
	}
	later := now.Add(restartCrashWindow + time.Minute)
	if tok, _, _ := b.Allow(later); tok == nil {
		t.Fatalf("after window expiry: want allowed")
	}
}

// «Здоровый» аптайм (≥ restartHealthyRun) перед падением сбрасывает счётчики.
func TestRestartBackoff_HealthyRunResets(t *testing.T) {
	var b restartBackoff
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < restartFreeBudget; i++ {
		b.Allow(now)
	}
	if tok, _, _ := b.Allow(now); tok != nil {
		t.Fatalf("precondition: budget must be exhausted")
	}
	// Процесс стартовал и прожил дольше restartHealthyRun → NoteCrash сбрасывает.
	start := now.Add(time.Minute)
	b.NoteProcessStart(start)
	crash := start.Add(restartHealthyRun + time.Second)
	b.NoteCrash(crash)
	if tok, _, _ := b.Allow(crash); tok == nil {
		t.Fatalf("after healthy run: want allowed (counters reset)")
	}
}

// Короткий аптайм (< restartHealthyRun) НЕ сбрасывает счётчики.
func TestRestartBackoff_ShortRunDoesNotReset(t *testing.T) {
	var b restartBackoff
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < restartFreeBudget; i++ {
		b.Allow(now)
	}
	b.NoteProcessStart(now)
	b.NoteCrash(now.Add(30 * time.Second))
	if tok, _, _ := b.Allow(now.Add(31 * time.Second)); tok != nil {
		t.Fatalf("short run must not reset: want denied")
	}
}

// Ручной Reset (Control start/restart) снимает паузу и возвращает бюджет.
func TestRestartBackoff_ManualReset(t *testing.T) {
	var b restartBackoff
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < restartFreeBudget; i++ {
		b.Allow(now)
	}
	if tok, _, _ := b.Allow(now); tok != nil {
		t.Fatalf("precondition: want denied")
	}
	if until := b.SuppressedUntil(now); until.IsZero() {
		t.Fatalf("precondition: want suppressed")
	}
	b.Reset()
	if until := b.SuppressedUntil(now); !until.IsZero() {
		t.Fatalf("after Reset: want not suppressed, got until=%v", until)
	}
	if tok, _, _ := b.Allow(now); tok == nil {
		t.Fatalf("after Reset: want allowed")
	}
}

// SuppressedUntil отражает активную паузу и «гаснет» после её конца.
func TestRestartBackoff_SuppressedUntilQuery(t *testing.T) {
	var b restartBackoff
	now := time.Unix(1_700_000_000, 0)
	if until := b.SuppressedUntil(now); !until.IsZero() {
		t.Fatalf("fresh backoff: want zero, got %v", until)
	}
	for i := 0; i < restartFreeBudget; i++ {
		b.Allow(now)
	}
	_, until, _ := b.Allow(now)
	if got := b.SuppressedUntil(now.Add(time.Second)); !got.Equal(until) {
		t.Fatalf("during pause: SuppressedUntil = %v, want %v", got, until)
	}
	if got := b.SuppressedUntil(until.Add(time.Second)); !got.IsZero() {
		t.Fatalf("after pause end: want zero, got %v", got)
	}
}

// Rollback возвращает попытку в бюджет (FIX-E): проигравший гонку
// watchdog/router не жжёт Allow за чужой единственный рестарт.
func TestRestartBackoff_RollbackRefundsAttempt(t *testing.T) {
	var b restartBackoff
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < restartFreeBudget; i++ {
		tok, _, _ := b.Allow(now)
		if tok == nil {
			t.Fatalf("attempt %d: want allowed", i+1)
		}
		tok.Rollback()
	}
	// Все попытки откатили — бюджет полон, следующая разрешена.
	tok, _, _ := b.Allow(now.Add(time.Second))
	if tok == nil {
		t.Fatalf("after rollbacks: want allowed (budget refunded)")
	}
	tok.Commit()
}

// Commit фиксирует попытку; повторный Rollback после Commit — no-op
// (первое решение выигрывает), двойной Rollback не откатывает дважды.
func TestRestartBackoff_TokenSettleIdempotent(t *testing.T) {
	var b restartBackoff
	now := time.Unix(1_700_000_000, 0)

	tok, _, _ := b.Allow(now)
	if tok == nil {
		t.Fatalf("want allowed")
	}
	tok.Commit()
	tok.Rollback() // после Commit — no-op
	b.mu.Lock()
	kept := len(b.attempts)
	b.mu.Unlock()
	if kept != 1 {
		t.Fatalf("attempts after Commit+Rollback = %d, want 1 (commit wins)", kept)
	}

	tok2, _, _ := b.Allow(now)
	tok2.Rollback()
	tok2.Rollback() // двойной Rollback — no-op
	b.mu.Lock()
	kept = len(b.attempts)
	b.mu.Unlock()
	if kept != 1 {
		t.Fatalf("attempts after double Rollback = %d, want 1", kept)
	}
	// nil-token безопасен.
	var nilTok *restartToken
	nilTok.Commit()
	nilTok.Rollback()
}

// Rollback после Reset (ручное управление очистило попытки) безопасен.
func TestRestartBackoff_RollbackAfterResetSafe(t *testing.T) {
	var b restartBackoff
	now := time.Unix(1_700_000_000, 0)
	tok, _, _ := b.Allow(now)
	b.Reset()
	tok.Rollback() // попытки уже нет — просто no-op без паники
	if got, _, _ := b.Allow(now); got == nil {
		t.Fatalf("after Reset: want allowed")
	}
}

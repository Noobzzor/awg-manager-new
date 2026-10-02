package hydraroute

import (
	"sync"
	"testing"
)

// TestScheduleRestart_ConcurrentLockSafe ловит гонку на s.restartTimer:
// экспортный ScheduleRestart должен брать s.mu (как все callsite scheduleRestart
// и timer-callback). Запускать под -race. До фикса детектирует DATA RACE на
// чтении/записи s.restartTimer.
func TestScheduleRestart_ConcurrentLockSafe(t *testing.T) {
	svc := &Service{}
	svc.SetStatusForTest(true) // Installed=true → таймер реально ставится

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.ScheduleRestart("race-probe")
		}()
	}
	wg.Wait()

	// Не дать AfterFunc(2s) форкнуть /opt/bin/neo в CI.
	svc.mu.Lock()
	if svc.restartTimer != nil {
		svc.restartTimer.Stop()
	}
	svc.mu.Unlock()
}

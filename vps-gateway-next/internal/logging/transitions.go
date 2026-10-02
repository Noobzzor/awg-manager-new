package logging

import "sync"

// TransitionKind классифицирует одно наблюдение повторяющейся бинарной
// проверки (connectivity, ping и т.п.): значимы переходы состояния, а не
// каждое измерение. Логика уровней у вызывающего кода: NowFailing → Warn,
// Recovered → Info, StillOK/StillFailing/FirstOK → Debug.
type TransitionKind int

const (
	// TransitionFirstOK — первый успешный результат для цели.
	TransitionFirstOK TransitionKind = iota
	// TransitionStillOK — успех после успеха, ничего не изменилось.
	TransitionStillOK
	// TransitionNowFailing — переход в отказ (или первый результат — отказ).
	TransitionNowFailing
	// TransitionStillFailing — повторный отказ подряд.
	TransitionStillFailing
	// TransitionRecovered — успех после серии отказов.
	TransitionRecovered
)

// TransitionResult — классификация наблюдения. Failures — длина текущей
// серии отказов: для NowFailing/StillFailing — включая это наблюдение,
// для Recovered — сколько отказов было до восстановления.
type TransitionResult struct {
	Kind     TransitionKind
	Failures int
}

type transitionState struct {
	ok       bool
	failures int
}

// TransitionTracker хранит последний исход по каждой цели и классифицирует
// новые наблюдения. Потокобезопасен.
type TransitionTracker struct {
	mu   sync.Mutex
	last map[string]transitionState
}

func NewTransitionTracker() *TransitionTracker {
	return &TransitionTracker{last: make(map[string]transitionState)}
}

// Observe регистрирует результат очередной проверки цели и возвращает его
// классификацию относительно предыдущего состояния.
func (t *TransitionTracker) Observe(target string, ok bool) TransitionResult {
	t.mu.Lock()
	defer t.mu.Unlock()

	prev, known := t.last[target]
	switch {
	case ok && !known:
		t.last[target] = transitionState{ok: true}
		return TransitionResult{Kind: TransitionFirstOK}
	case ok && prev.ok:
		return TransitionResult{Kind: TransitionStillOK}
	case ok: // fail → ok
		t.last[target] = transitionState{ok: true}
		return TransitionResult{Kind: TransitionRecovered, Failures: prev.failures}
	case !known || prev.ok: // (первый результат | ok) → fail
		t.last[target] = transitionState{ok: false, failures: 1}
		return TransitionResult{Kind: TransitionNowFailing, Failures: 1}
	default: // fail → fail
		prev.failures++
		t.last[target] = prev
		return TransitionResult{Kind: TransitionStillFailing, Failures: prev.failures}
	}
}

// Forget сбрасывает состояние цели: следующий результат будет считаться
// первым (используется при остановке/удалении туннеля, чтобы после запуска
// отказ снова дал Warn, а не «повтор»).
func (t *TransitionTracker) Forget(target string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.last, target)
}

// Retain оставляет только перечисленные цели — остальные забываются.
// Используется циклами, наблюдающими изменяющееся множество объектов
// (мониторинг туннелей): удалённый объект не должен продолжать серию
// после пересоздания.
func (t *TransitionTracker) Retain(keep map[string]bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for target := range t.last {
		if !keep[target] {
			delete(t.last, target)
		}
	}
}

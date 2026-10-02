// Package perftrace — helper для performance diagnostics.
//
// Используется через defer в hot-path функциях (singbox.Operator:
// GetStatus/ListTunnels/AddTunnels/applyConfig и др.) для записи времени
// выполнения в app-log на debug-уровне.
package perftrace

import (
	"fmt"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

// LogDuration логирует время от start до now в миллисекундах. Nil-safe.
//
// Использование:
//
//	defer perftrace.LogDuration(o.runtimeLogger, "perf", "AddTunnels", "total", time.Now())
//
// Если функция возвращает раньше всех остальных defer'ов — это OK, время
// меряем с момента вызова LogDuration (последнее зарегистрированное
// значение time.Now() из стека defer'ов).
func LogDuration(log *logging.ScopedLogger, action, target, label string, start time.Time) {
	if log == nil {
		return
	}
	log.Info(action, target, fmt.Sprintf("%s took %dms", label, time.Since(start).Milliseconds()))
}

// Mark — однократная отметка момента (для промежуточных стадий).
// Возвращает новое начало для следующей секции.
//
// Использование:
//
//	stage := time.Now()
//	doParse()
//	stage = perftrace.Mark(log, "perf", "AddTunnels", "parse", stage)
//	doApply()
//	stage = perftrace.Mark(log, "perf", "AddTunnels", "apply", stage)
func Mark(log *logging.ScopedLogger, action, target, label string, start time.Time) time.Time {
	if log != nil {
		log.Info(action, target, fmt.Sprintf("%s took %dms", label, time.Since(start).Milliseconds()))
	}
	return time.Now()
}

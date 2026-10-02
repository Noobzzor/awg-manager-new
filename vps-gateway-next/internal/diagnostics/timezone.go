package diagnostics

import (
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

func convertLogEntriesToRouterTime(entries []logging.LogEntry, loc *time.Location) []logging.LogEntry {
	if len(entries) == 0 || loc == nil {
		return entries
	}
	out := make([]logging.LogEntry, len(entries))
	copy(out, entries)
	for i := range out {
		out[i].Timestamp = out[i].Timestamp.In(loc)
	}
	return out
}

func convertJournalWarningsToRouterTime(jw *JournalWarningsInfo, loc *time.Location) {
	if jw == nil || loc == nil {
		return
	}
	jw.AWGM.Entries = convertLogEntriesToRouterTime(jw.AWGM.Entries, loc)
	jw.Singbox.Entries = convertLogEntriesToRouterTime(jw.Singbox.Entries, loc)

	if jw.AWGM.BufferOldestTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, jw.AWGM.BufferOldestTimestamp); err == nil {
			jw.AWGM.BufferOldestTimestamp = t.In(loc).Format(time.RFC3339)
		}
	}
	if jw.Singbox.BufferOldestTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, jw.Singbox.BufferOldestTimestamp); err == nil {
			jw.Singbox.BufferOldestTimestamp = t.In(loc).Format(time.RFC3339)
		}
	}
}

func convertReportTimesToRouterTime(report *Report, loc *time.Location) {
	if report == nil || loc == nil {
		return
	}
	report.GeneratedAt = report.GeneratedAt.In(loc)
	report.BootHealth.DaemonStartedAt = report.BootHealth.DaemonStartedAt.In(loc)
}

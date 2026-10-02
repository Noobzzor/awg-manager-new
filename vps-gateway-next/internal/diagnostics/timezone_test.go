package diagnostics

import (
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

func TestConvertLogEntriesToRouterTime_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	srcTs := time.Date(2026, 5, 20, 18, 57, 54, 0, time.UTC)
	src := []logging.LogEntry{
		{Timestamp: srcTs, Level: "error", Message: "x"},
	}
	loc := time.FixedZone("MSK", 3*60*60)

	got := convertLogEntriesToRouterTime(src, loc)
	if got[0].Timestamp.Format(time.RFC3339) != "2026-05-20T21:57:54+03:00" {
		t.Fatalf("converted timestamp = %s", got[0].Timestamp.Format(time.RFC3339))
	}
	if src[0].Timestamp.Format(time.RFC3339) != "2026-05-20T18:57:54Z" {
		t.Fatalf("source entry was mutated: %s", src[0].Timestamp.Format(time.RFC3339))
	}
}

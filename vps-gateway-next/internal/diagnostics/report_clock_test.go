package diagnostics

import (
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/sys/routerclock"
)

func TestNewReport_PopulatesRouterClock(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("MSK", 3*60*60)
	gen := time.Date(2026, 5, 20, 22, 20, 43, 510751981, loc)
	clock := routerclock.Info{
		Now:           gen,
		ZoneName:      "MSK",
		OffsetMinutes: 180,
		Location:      loc,
		Source:        "/etc/TZ",
		RawTZ:         "MSK-3",
	}

	r := newReport(gen, clock)
	if r.RouterClock.Time != gen {
		t.Fatalf("time mismatch: got %s want %s", r.RouterClock.Time, gen)
	}
	if r.RouterClock.Timezone != "MSK" || r.RouterClock.OffsetMinutes != 180 {
		t.Fatalf("clock mismatch: %+v", r.RouterClock)
	}
	if r.RouterClock.Source != "/etc/TZ" || r.RouterClock.RawTZ != "MSK-3" {
		t.Fatalf("source/raw mismatch: %+v", r.RouterClock)
	}
}

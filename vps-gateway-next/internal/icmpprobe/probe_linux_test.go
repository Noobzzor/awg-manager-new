//go:build linux

package icmpprobe

import (
	"context"
	"os"
	"testing"
)

// Проверяет весь путь, включая SO_BINDTODEVICE: бинд к lo + echo на 127.0.0.1.
func TestByInterface_LoopbackRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root (raw socket + SO_BINDTODEVICE)")
	}
	res, err := ByInterface(context.Background(), "lo", "127.0.0.1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.LatencyMs < 1 {
		t.Fatalf("LatencyMs = %d, want >= 1", res.LatencyMs)
	}
}

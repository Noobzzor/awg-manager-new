package warp

import (
	"os"
	"testing"
)

// Explicit opt-in fixture emission lets the pinned real binary validate the
// actual renderer output. These are synthetic test credentials, not a provider.
func TestRenderEndpointNativeFixture(t *testing.T) {
	path := os.Getenv("AWGM_WARP_RENDER_FILE")
	if path == "" {
		t.Skip("explicit native renderer fixture not requested")
	}
	raw, err := RenderEndpoint(sampleProfile())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

package dnsroute

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
	"github.com/hoaxisr/awg-manager/internal/singbox/installer"
)

func verifiedAWG3SingboxBinary(t *testing.T) string {
	t.Helper()
	candidates := []string{os.Getenv("AWG3_SINGBOX_BIN"), os.Getenv("SINGBOX_BIN"), installer.DefaultBinaryPath}
	for _, name := range []string{"sing-box", "amnezia-box"} {
		if bin, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, bin)
		}
	}
	trusted := make(map[string]installer.BinarySpec, len(installer.EmbeddedBinaries))
	for _, spec := range installer.EmbeddedBinaries {
		trusted[strings.ToLower(spec.SHA256)] = spec
	}
	var unavailable []string
	seen := make(map[string]bool)
	for _, bin := range candidates {
		bin = strings.TrimSpace(bin)
		if bin == "" || seen[bin] {
			continue
		}
		seen[bin] = true
		st, err := os.Stat(bin)
		if err != nil || st.IsDir() || st.Mode().Perm()&0o111 == 0 {
			unavailable = append(unavailable, fmt.Sprintf("%s: not executable", bin))
			continue
		}
		f, err := os.Open(bin)
		if err != nil {
			unavailable = append(unavailable, fmt.Sprintf("%s: open: %v", bin, err))
			continue
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil || closeErr != nil {
			unavailable = append(unavailable, fmt.Sprintf("%s: sha256 read failed", bin))
			continue
		}
		digest := fmt.Sprintf("%x", h.Sum(nil))
		spec, verified := trusted[digest]
		if !verified || spec.Version != installer.RequiredVersion {
			unavailable = append(unavailable, fmt.Sprintf("%s: bytes do not match pinned metadata", bin))
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		out, err := exec.CommandContext(ctx, bin, "version").CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("candidate AWG binary %s failed version probe: %v\n%s", bin, err, out)
		}
		banner := string(out)
		if !strings.Contains(banner, "sing-box version "+installer.RequiredVersion) {
			t.Fatalf("candidate AWG binary %s is not pinned version %s:\n%s", bin, installer.RequiredVersion, banner)
		}
		if !strings.Contains(banner, "with_awg") {
			t.Fatalf("candidate AWG binary %s lacks verified with_awg capability:\n%s", bin, banner)
		}
		return bin
	}
	t.Skipf("verified %s with_awg binary unavailable for %s/%s (%s)", installer.RequiredVersion, runtime.GOOS, runtime.GOARCH, strings.Join(unavailable, "; "))
	return ""
}

func TestRealBinaryGeneratedAWGEndpointAndRouteCheck(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "awg3endpoint", "testdata", "routebox-client.json"))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := awg3endpoint.Parse(raw, "awg-amsterdam", nil)
	if err != nil {
		t.Fatalf("Parse AWG endpoint: %v", err)
	}

	var endpoint map[string]json.RawMessage
	if err := json.Unmarshal(rec.Endpoint, &endpoint); err != nil {
		t.Fatal(err)
	}
	var source struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"jc", "jmin", "jmax", "s1", "s2", "h1", "h2", "h3", "h4", "header_protection_key"} {
		if !bytes.Equal(bytes.TrimSpace(endpoint[key]), bytes.TrimSpace(source.Data[key])) {
			t.Fatalf("generated endpoint changed opaque AWG field %q: got %s want %s", key, endpoint[key], source.Data[key])
		}
	}
	// Keep check offline while preserving all AWG obfuscation fields.
	var peers []map[string]json.RawMessage
	if err := json.Unmarshal(endpoint["peers"], &peers); err != nil || len(peers) != 1 {
		t.Fatalf("peers: %v len=%d", err, len(peers))
	}
	peers[0]["address"] = json.RawMessage(`"192.0.2.1"`)
	endpoint["peers"], _ = json.Marshal(peers)
	endpoint["tag"], _ = json.Marshal(rec.Tag)
	endpointBytes, err := json.Marshal(map[string]any{"endpoints": []any{endpoint}})
	if err != nil {
		t.Fatal(err)
	}
	routeBytes, err := CompileSingbox([]DomainList{{
		ID: "real-awg-route", Backend: BackendSingbox, Enabled: true,
		Domains: []string{"example.com"},
		Routes:  []RouteTarget{{TunnelID: rec.ID, Fallback: "reject"}},
	}}, SingboxCompileOptions{
		DNSUpstream: "1.1.1.1",
		ResolveTarget: func(target RouteTarget) (string, error) {
			if target.TunnelID != rec.ID {
				return "", fmt.Errorf("unexpected target %q", target.TunnelID)
			}
			return rec.Tag, nil
		},
	})
	if err != nil {
		t.Fatalf("CompileSingbox: %v", err)
	}

	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "16-awg3.json"), endpointBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "19-dns-routes.json"), routeBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	bin := verifiedAWG3SingboxBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "check", "-C", configDir).CombinedOutput()
	if err != nil {
		t.Fatalf("%s check -C generated AWG endpoint+route: %v\n%s", bin, err, out)
	}
	if strings.Contains(strings.ToUpper(string(out)), "FATAL") {
		t.Fatalf("sing-box check emitted FATAL:\n%s", out)
	}
}

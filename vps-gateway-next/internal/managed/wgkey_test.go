package managed

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestReadKernelPrivateKey_ParsesBase64(t *testing.T) {
	bins := append([]string(nil), wgShowBins...)
	stub := func(ctx context.Context, name string, args ...string) (string, error) {
		if name != bins[0] || len(args) != 3 || args[0] != "show" || args[1] != "nwg63" || args[2] != "private-key" {
			t.Fatalf("unexpected argv: %s %v", name, args)
		}
		return "yKOZJtI2nQbSWzo8zvmBSjjnSkn89AkLXbekWTgKQ08=\n", nil
	}
	key, err := readKernelPrivateKeyWith(context.Background(), "nwg63", stub)
	if err != nil {
		t.Fatalf("readKernelPrivateKey: %v", err)
	}
	if key != "yKOZJtI2nQbSWzo8zvmBSjjnSkn89AkLXbekWTgKQ08=" {
		t.Errorf("key: got %q", key)
	}
}

func TestReadKernelPrivateKey_FallbackToSecondBinary(t *testing.T) {
	bins := append([]string(nil), wgShowBins...)
	stub := func(ctx context.Context, name string, args ...string) (string, error) {
		if name == bins[0] {
			return "", fmt.Errorf("%s: no such file or directory", name)
		}
		if name == bins[1] {
			return "fallback-key=\n", nil
		}
		return "", fmt.Errorf("unexpected binary %s", name)
	}
	key, err := readKernelPrivateKeyWith(context.Background(), "nwg63", stub)
	if err != nil {
		t.Fatalf("readKernelPrivateKey: %v", err)
	}
	if key != "fallback-key=" {
		t.Fatalf("key: got %q, want %q", key, "fallback-key=")
	}
}

func TestReadKernelPrivateKey_FallbackAfterNonMissingError(t *testing.T) {
	bins := append([]string(nil), wgShowBins...)
	stub := func(ctx context.Context, name string, args ...string) (string, error) {
		if name == bins[0] {
			return "", errors.New("Unable to access interface: No such device")
		}
		if name == bins[1] {
			return "fallback-after-error=\n", nil
		}
		return "", fmt.Errorf("unexpected binary %s", name)
	}
	key, err := readKernelPrivateKeyWith(context.Background(), "nwg63", stub)
	if err != nil {
		t.Fatalf("readKernelPrivateKey: %v", err)
	}
	if key != "fallback-after-error=" {
		t.Fatalf("key: got %q, want %q", key, "fallback-after-error=")
	}
}

func TestReadKernelPrivateKey_AllBinariesMissing(t *testing.T) {
	var called []string
	stub := func(ctx context.Context, name string, args ...string) (string, error) {
		called = append(called, name)
		return "", fmt.Errorf("%s: no such file or directory", name)
	}
	_, err := readKernelPrivateKeyWith(context.Background(), "nwg99", stub)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.HasPrefix(err.Error(), "wireguard tools not found: tried ") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(called, wgShowBins) {
		t.Fatalf("called bins: got %v, want %v", called, wgShowBins)
	}
}

func TestReadKernelPrivateKey_PropagatesToolError(t *testing.T) {
	stub := func(ctx context.Context, name string, args ...string) (string, error) {
		return "", errors.New("Unable to access interface: No such device")
	}
	_, err := readKernelPrivateKeyWith(context.Background(), "nwg99", stub)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

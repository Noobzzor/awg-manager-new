package managed

import (
	"context"
	"fmt"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/sys/exec"
)

var wgShowBins = []string{
	awgBin,
	"/opt/sbin/wg",
	"/opt/bin/awg",
	"/opt/bin/wg",
}

// wgRunner is the indirection seam for tests. Production wires
// realWgRunner (which calls internal/sys/exec.Run on wgBin); tests pass
// stubs without forking real binaries.
type wgRunner func(ctx context.Context, name string, args ...string) (string, error)

func realWgRunner(ctx context.Context, name string, args ...string) (string, error) {
	result, err := exec.Run(ctx, name, args...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, exec.FormatError(result, err))
	}
	return result.Stdout, nil
}

func readKernelPrivateKeyWith(ctx context.Context, kernelName string, run wgRunner) (string, error) {
	if kernelName == "" {
		return "", fmt.Errorf("readKernelPrivateKey: empty kernel name")
	}
	var firstNonMissingErr error
	var sawMissing bool
	for _, bin := range wgShowBins {
		out, err := run(ctx, bin, "show", kernelName, "private-key")
		if err == nil {
			return strings.TrimSpace(out), nil
		}
		if isBinaryMissingError(err) {
			sawMissing = true
			continue
		}
		if firstNonMissingErr == nil {
			firstNonMissingErr = err
		}
		continue
	}
	if firstNonMissingErr != nil {
		return "", firstNonMissingErr
	}
	if sawMissing {
		return "", fmt.Errorf("wireguard tools not found: tried %s", strings.Join(wgShowBins, ", "))
	}
	return "", fmt.Errorf("readKernelPrivateKey: failed for %s", kernelName)
}

func isBinaryMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such file or directory") ||
		strings.Contains(msg, "file not found")
}

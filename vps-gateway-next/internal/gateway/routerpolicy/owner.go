package routerpolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type PolicyOwner string

const (
	PolicyOwnerGateway  PolicyOwner = "gateway"
	PolicyOwnerOriginal PolicyOwner = "original"
)

var ErrGatewayPolicyOwnership = errors.New("gateway policy owner admission denied")

func OriginalStateDir(dataDir string) string { return filepath.Join(dataDir, "original-router") }

// ReadPolicyOwner inspects persisted ownership without creating any state.
func ReadPolicyOwner(dataDir string) (PolicyOwner, error) {
	if !filepath.IsAbs(dataDir) {
		return "", fmt.Errorf("policy data directory must be absolute")
	}
	dir := OriginalStateDir(dataDir)
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return PolicyOwnerGateway, nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect policy owner directory")
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return "", fmt.Errorf("policy owner directory must be private and regular")
	}
	path := filepath.Join(dir, "owner.json")
	info, err = os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("original policy state has no readable owner")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 4096 {
		return "", fmt.Errorf("policy owner must be a small private regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open policy owner")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil || len(raw) > 4096 {
		return "", fmt.Errorf("read bounded policy owner")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return "", fmt.Errorf("invalid policy owner document")
	}
	seen := map[string]bool{}
	version := 0
	mode := ""
	for d.More() {
		keyToken, err := d.Token()
		key, ok := keyToken.(string)
		if err != nil || !ok || seen[key] {
			return "", fmt.Errorf("ambiguous policy owner fields")
		}
		seen[key] = true
		switch key {
		case "version":
			err = d.Decode(&version)
		case "mode":
			err = d.Decode(&mode)
		default:
			return "", fmt.Errorf("unknown policy owner field")
		}
		if err != nil {
			return "", fmt.Errorf("invalid policy owner field")
		}
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return "", fmt.Errorf("invalid policy owner document")
	}
	if _, err := d.Token(); err != io.EOF || version != 1 {
		return "", fmt.Errorf("invalid policy owner version or trailing data")
	}
	owner := PolicyOwner(mode)
	if owner != PolicyOwnerGateway && owner != PolicyOwnerOriginal {
		return "", fmt.Errorf("unknown policy owner")
	}
	if owner == PolicyOwnerGateway {
		if _, err := os.Lstat(filepath.Join(dir, "apply-journal.json")); !os.IsNotExist(err) {
			return "", fmt.Errorf("gateway owner conflicts with unresolved original recovery")
		}
	}
	return owner, nil
}

// RequireGatewayOwner guards the legacy startup and HTTP writer paths.
func RequireGatewayOwner(ctx context.Context, dataDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	owner, err := ReadPolicyOwner(dataDir)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGatewayPolicyOwnership, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if owner != PolicyOwnerGateway {
		return ErrGatewayPolicyOwnership
	}
	return nil
}

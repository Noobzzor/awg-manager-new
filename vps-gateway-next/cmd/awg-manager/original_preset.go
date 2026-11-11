package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/router"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

const (
	originalGitHubRuleSetURL = "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-github.srs"
	originalGitHubRuleSetSHA = "438f12d297174b168b3b6acd4572c360c083c24641736bdaefb4ed2cb3d0295f"
	originalPresetLimit      = 8 * 1024 * 1024
)

// resolveOriginalPresetRuleSets runs under the original editor's writer lock.
// The first pilot accepts one catalog identity and one pinned public payload.
func (a *app) resolveOriginalPresetRuleSets(ctx context.Context, preset router.Preset) ([]router.RuleSet, error) {
	if preset.ID != "github" || len(preset.RuleSets) != 1 ||
		preset.RuleSets[0] != (router.RuleRef{Tag: "geosite-github", URL: originalGitHubRuleSetURL}) {
		return nil, fmt.Errorf("this preset has no verified local rule-set")
	}
	if err := a.verifyOriginalOwner(ctx); err != nil {
		return nil, err
	}
	dir := filepath.Join(a.singboxConfigDir, "rule-sets", "presets")
	path := filepath.Join(dir, "sha256-"+originalGitHubRuleSetSHA+".srs")
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0400 || info.Size() < 1 || info.Size() > originalPresetLimit {
			return nil, fmt.Errorf("cached preset file has unsafe attributes")
		}
		raw, err := os.ReadFile(path)
		if err != nil || !isPinnedGitHubRuleSet(raw) {
			return nil, fmt.Errorf("cached GitHub preset differs from its verified file")
		}
	case os.IsNotExist(err):
		if err := a.downloadOriginalGitHubPreset(ctx, dir, path); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	return []router.RuleSet{{Tag: "geosite-github", Type: "local", Format: "binary", Path: path}}, nil
}

func isPinnedGitHubRuleSet(raw []byte) bool {
	return fmt.Sprintf("%x", sha256.Sum256(raw)) == originalGitHubRuleSetSHA
}

func (a *app) downloadOriginalGitHubPreset(ctx context.Context, dir, path string) error {
	client := &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, originalGitHubRuleSetURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download GitHub preset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download GitHub preset: HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, originalPresetLimit+1))
	if err != nil || len(raw) > originalPresetLimit || !isPinnedGitHubRuleSet(raw) {
		return fmt.Errorf("downloaded GitHub preset differs from its verified file")
	}
	if err := a.verifyOriginalOwner(ctx); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := storage.AtomicWritePerm(path, raw, 0400); err != nil {
		return err
	}
	for _, parent := range []string{dir, filepath.Dir(dir), a.singboxConfigDir} {
		handle, err := os.Open(parent)
		if err != nil {
			return err
		}
		syncErr := handle.Sync()
		closeErr := handle.Close()
		if syncErr != nil {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hoaxisr/awg-manager/internal/gateway/warp"
	"github.com/hoaxisr/awg-manager/internal/singbox/awgoutbounds"
	orch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

const originalWARPProfileFile = "AWG_ORIGINAL_WARP_PROFILE_FILE"

// importOriginalWARP runs only during fresh original-mode initialization,
// before any engine or policy writer is published.
func (a *app) importOriginalWARP() error {
	path := os.Getenv(originalWARPProfileFile)
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("original WARP profile must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open original WARP profile: %w", err)
	}
	defer file.Close()
	profile, err := warp.DecodeProfile(file)
	if err != nil {
		return err
	}
	raw, err := warp.RenderEndpoint(profile)
	if err != nil {
		return err
	}
	if err := warp.NewStore(a.dataDir).Save(profile); err != nil {
		return err
	}
	for _, slot := range orch.KnownSlots() {
		if slot.Slot == orch.SlotWARP {
			return storage.AtomicWritePerm(filepath.Join(a.singboxConfigDir, slot.Filename), raw, 0600)
		}
	}
	return fmt.Errorf("original WARP slot is unavailable")
}

// On restart, only advertise the endpoint if the owned slot still matches the
// encrypted profile. A missing or changed endpoint must not become DIRECT.
func (a *app) loadOriginalWARP() (originalWARPCatalog, error) {
	profile, err := warp.NewStore(a.dataDir).Load()
	if err != nil {
		return false, err
	}
	raw, enabled, err := a.sbOrch.SnapshotSlot(orch.SlotWARP)
	if err != nil {
		return false, err
	}
	if profile == nil && raw == nil {
		return false, nil
	}
	if profile != nil && enabled {
		expected, err := warp.RenderEndpoint(*profile)
		if err == nil && bytes.Equal(raw, expected) {
			return true, nil
		}
	}
	return false, fmt.Errorf("original WARP profile and endpoint slot differ; recovery required")
}

type originalWARPCatalog bool

func (configured originalWARPCatalog) ListTunnelTags(context.Context) ([]string, error) {
	if !configured {
		return nil, nil
	}
	return []string{warp.Tag}, nil
}

func (configured originalWARPCatalog) ListTags(context.Context) ([]awgoutbounds.TagInfo, error) {
	if !configured {
		return nil, nil
	}
	return []awgoutbounds.TagInfo{{Tag: warp.Tag, Label: "WARP", Kind: "warp"}}, nil
}

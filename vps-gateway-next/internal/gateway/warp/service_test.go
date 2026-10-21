package warp

import (
	"bytes"
	"errors"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type serviceSaver struct {
	calls  int
	slot   orchestrator.Slot
	data   []byte
	err    error
	reject bool
}

func (s *serviceSaver) SaveAndValidate(slot orchestrator.Slot, data []byte) (orchestrator.ValidationResult, error) {
	s.calls++
	s.slot = slot
	s.data = append([]byte(nil), data...)
	if s.reject {
		return orchestrator.ValidationResult{Errors: []orchestrator.ValidationError{{Message: "PRIVATE_SENTINEL"}}}, nil
	}
	return orchestrator.ValidationResult{}, s.err
}
func TestServiceSyncAndProjection(t *testing.T) {
	store := NewStore(t.TempDir())
	p := sampleProfile()
	if err := store.Save(p); err != nil {
		t.Fatal(err)
	}
	saver := &serviceSaver{}
	s := NewService(store, saver)
	if _, err := s.WARPConfig(); err == nil {
		t.Fatal("unsynced provider available")
	}
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	cfg, err := s.WARPConfig()
	if err != nil || cfg == nil || cfg.Tag != "warp" || cfg.IPv4 != p.IPv4 || cfg.MTU != p.MTU || len(cfg.EncryptedSecrets) != 0 || saver.slot != orchestrator.SlotWARP || !bytes.Contains(saver.data, []byte(`"wireguard"`)) {
		t.Fatal("provider missing or wrong")
	}
	p.MTU = 1400
	if err := store.Save(p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WARPConfig(); err == nil {
		t.Fatal("changed profile accepted without slot sync")
	}
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	cfg, err = s.WARPConfig()
	if err != nil || cfg.MTU != 1400 {
		t.Fatal("resync failed")
	}
}
func TestServiceClearsMissingProfileSlot(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	saver := &serviceSaver{}
	s := NewService(store, saver)
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	cfg, err := s.WARPConfig()
	if err != nil || cfg != nil || !bytes.Contains(saver.data, []byte(`"endpoints": []`)) {
		t.Fatal("empty profile not cleared")
	}
	if err := store.Save(sampleProfile()); err != nil {
		t.Fatal(err)
	}
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "gateway-warp.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WARPConfig(); err == nil {
		t.Fatal("removed profile accepted before slot sync")
	}
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	if cfg, err := s.WARPConfig(); err != nil || cfg != nil {
		t.Fatal("missing profile failed")
	}
}
func TestServiceFailuresDoNotExposeProviderOrValidatorSecrets(t *testing.T) {
	for _, mode := range []string{"validator error", "failed result", "lost key"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			store := NewStore(dir)
			if err := store.Save(sampleProfile()); err != nil {
				t.Fatal(err)
			}
			saver := &serviceSaver{}
			switch mode {
			case "validator error":
				saver.err = errors.New("PRIVATE_SENTINEL")
			case "failed result":
				saver.reject = true
			case "lost key":
				if err := os.Remove(filepath.Join(dir, ".device-key")); err != nil {
					t.Fatal(err)
				}
			}
			s := NewService(store, saver)
			err := s.Sync()
			if err == nil || strings.Contains(err.Error(), "PRIVATE_SENTINEL") {
				t.Fatal("sync failure missing or secret leaked")
			}
			if cfg, err := s.WARPConfig(); err == nil || cfg != nil {
				t.Fatal("failed sync exposed provider")
			}
			if mode == "lost key" && saver.calls != 0 {
				t.Fatal("slot written with unreadable profile")
			}
		})
	}
}

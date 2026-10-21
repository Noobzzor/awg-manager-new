package warp

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

func TestStoreRoundtripEncryptsAndProjectsSafely(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	p := sampleProfile()
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "gateway-warp.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{p.PrivateKey, p.PublicKey, p.Endpoint, `"reserved"`} {
		if bytes.Contains(raw, []byte(v)) {
			t.Fatal("plaintext stored")
		}
	}
	for _, name := range []string{"gateway-warp.json", storage.DeviceKeyFile} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("permissions: %s %v", name, err)
		}
	}
	reopened := NewStore(dir)
	got, err := reopened.Load()
	if err != nil || got == nil || got.PrivateKey != p.PrivateKey || got.Reserved[2] != 255 {
		t.Fatalf("roundtrip: %v", err)
	}
	got.Reserved[0] = 42
	again, err := reopened.Load()
	if err != nil || again.Reserved[0] != 0 {
		t.Fatal("caller mutation affected saved state")
	}
	d, err := reopened.Describe()
	if err != nil || !d.Configured || d.Tag != "warp" {
		t.Fatalf("describe: %v", err)
	}
}

func TestStoreMissingProfileDoesNotCreateKey(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	p, err := s.Load()
	if err != nil || p != nil {
		t.Fatal("missing profile must be unconfigured")
	}
	d, err := s.Describe()
	if err != nil || d.Configured {
		t.Fatal("missing configured")
	}
	if _, err := os.Stat(filepath.Join(dir, storage.DeviceKeyFile)); !os.IsNotExist(err) {
		t.Fatal("read created key")
	}
}

func TestStoreMissingOrForeignKeyRefusesReadAndReplacement(t *testing.T) {
	for _, mode := range []string{"missing", "foreign", "short"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			s := NewStore(dir)
			if err := s.Save(sampleProfile()); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "gateway-warp.json")
			before, _ := os.ReadFile(path)
			kp := filepath.Join(dir, storage.DeviceKeyFile)
			if mode == "missing" {
				if err := os.Remove(kp); err != nil {
					t.Fatal(err)
				}
			} else {
				key := bytes.Repeat([]byte{7}, 32)
				if mode == "short" {
					key = key[:3]
				}
				if err := os.WriteFile(kp, key, 0600); err != nil {
					t.Fatal(err)
				}
			}
			keyBefore, _ := os.ReadFile(kp)
			got, err := s.Load()
			if err == nil || got != nil {
				t.Fatal("unreadable credential accepted")
			}
			if _, err := s.Describe(); err == nil {
				t.Fatal("unreadable state described as usable")
			}
			if err := s.Save(sampleProfile()); err == nil {
				t.Fatal("unreadable profile overwritten")
			}
			after, _ := os.ReadFile(path)
			keyAfter, _ := os.ReadFile(kp)
			if !bytes.Equal(before, after) || !bytes.Equal(keyBefore, keyAfter) {
				t.Fatal("failed operation changed durable state")
			}
			if mode == "missing" {
				if _, err := os.Stat(kp); !os.IsNotExist(err) {
					t.Fatal("key recreated")
				}
			}
		})
	}
}

func TestStoreRejectsCorruptVersionOversizeAndInvalidEncryptedProfile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Save(sampleProfile()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "gateway-warp.json")
	valid, _ := os.ReadFile(path)
	for _, raw := range []string{`{`, `{"version":2,"ciphertext":"x"}`, `{"version":1,"ciphertext":"x","unknown":true}`, string(valid) + ` {}`, strings.Repeat(" ", 65537)} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		p, err := s.Load()
		if err == nil || p != nil {
			t.Fatal("corrupt file accepted")
		}
	}
	cipher := storage.NewDeviceCipher(dir)
	token, err := cipher.Encrypt(`{"privateKey":"PRIVATE_SENTINEL"}`)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"version": 1, "ciphertext": token})
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := s.Load()
	if err == nil || p != nil || strings.Contains(err.Error(), "PRIVATE_SENTINEL") {
		t.Fatal("invalid decrypted profile accepted or leaked")
	}
}

func TestStoreInvalidOrFailedSaveRetainsPreviousProfile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	p := sampleProfile()
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "gateway-warp.json")
	before, _ := os.ReadFile(path)
	invalid := p
	invalid.PrivateKey = "PRIVATE_SENTINEL"
	if err := s.Save(invalid); err == nil {
		t.Fatal("invalid save accepted")
	}
	s.writeAtomic = func(string, []byte, os.FileMode) error { return errors.New("injected write failure") }
	p.MTU = 1400
	if err := s.Save(p); err == nil {
		t.Fatal("write failure accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("previous record lost")
	}
	got, err := s.Load()
	if err != nil || got.MTU != 1280 {
		t.Fatal("previous profile not usable")
	}
}

func TestStoreConcurrentReadersAndWriters(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Save(sampleProfile()); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			p := sampleProfile()
			p.MTU = 1280 + i
			for n := 0; n < 8; n++ {
				if err := s.Save(p); err != nil {
					t.Error(err)
					return
				}
				got, err := s.Load()
				if err != nil || got == nil {
					t.Errorf("concurrent read: %v", err)
					return
				}
				if err := got.Validate(); err != nil {
					t.Error(err)
					return
				}
			}
		}(i)
	}
	workers.Wait()
}

func TestStoreRejectsNullReservedInEncryptedProfile(t *testing.T) {
	dir := t.TempDir()
	raw, _ := json.Marshal(sampleProfile())
	plaintext := strings.Replace(string(raw), `[0,127,255]`, `[null,127,255]`, 1)
	token, err := storage.NewDeviceCipher(dir).Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	envelope, _ := json.Marshal(map[string]any{"version": 1, "ciphertext": token})
	if err := os.WriteFile(filepath.Join(dir, "gateway-warp.json"), envelope, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := NewStore(dir).Load()
	if err == nil || got != nil {
		t.Fatal("invalid encrypted reserved value accepted")
	}
}

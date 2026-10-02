package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// migrateToV30: легаси-одиночный Server.Interface поднимается в Interfaces[],
// старое поле сохраняет значение (downgrade-совместимость).
func TestMigrateToV30_LiftsLegacyInterface(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":29,"server":{"port":2222,"interface":"br0"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	store := NewSettingsStore(dir)
	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(settings.Server.Interfaces) != 1 || settings.Server.Interfaces[0] != "br0" {
		t.Errorf("Interfaces = %v, want [br0]", settings.Server.Interfaces)
	}
	if settings.Server.Interface != "br0" {
		t.Errorf("legacy Interface = %q, want br0 (kept for downgrade)", settings.Server.Interface)
	}
	if settings.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", settings.SchemaVersion, CurrentSchemaVersion)
	}
}

// Пустой легаси-Interface ("" = 0.0.0.0) мигрирует в пустой список — та же
// семантика «все интерфейсы».
func TestMigrateToV30_EmptyLegacyStaysAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":29,"server":{"port":2222,"interface":""}}`), 0644); err != nil {
		t.Fatal(err)
	}
	store := NewSettingsStore(dir)
	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(settings.Server.Interfaces) != 0 {
		t.Errorf("Interfaces = %v, want empty (bind all)", settings.Server.Interfaces)
	}
}

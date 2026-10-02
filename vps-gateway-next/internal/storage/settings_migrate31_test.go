package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// migrateToV31: вводит поля автоустановки обновлений (issue #559) с
// дефолтами — выключено, каждые 7 суток, окно 05:00.
func TestMigrateToV31_SetsAutoInstallDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":30,"updates":{"checkEnabled":true,"channel":"stable"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	store := NewSettingsStore(dir)
	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if settings.Updates.AutoInstallEnabled {
		t.Errorf("AutoInstallEnabled = true, want false (default)")
	}
	if settings.Updates.AutoInstallIntervalDays != 7 {
		t.Errorf("AutoInstallIntervalDays = %d, want 7", settings.Updates.AutoInstallIntervalDays)
	}
	if settings.Updates.AutoInstallTime != "05:00" {
		t.Errorf("AutoInstallTime = %q, want 05:00", settings.Updates.AutoInstallTime)
	}
	if settings.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", settings.SchemaVersion, CurrentSchemaVersion)
	}
}

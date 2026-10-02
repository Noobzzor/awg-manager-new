package main

import (
	"testing"

	"github.com/hoaxisr/awg-manager/internal/updater"
)

func TestGenerate_GroupsAndOrder(t *testing.T) {
	subjects := []string{
		"feat(frontend): селектор канала",
		"fix(updater): выбор версии",
		"refactor(x): cleanup",
		"perf(y): faster loop",
	}
	got := Generate(subjects, "2.11.2+r95", "2026-05-25")
	want := "## [2.11.2+r95] - 2026-05-25\n\n" +
		"### Добавлено\n- feat(frontend): селектор канала\n\n" +
		"### Исправлено\n- fix(updater): выбор версии\n\n" +
		"### Рефакторинг\n- refactor(x): cleanup\n\n" +
		"### Производительность\n- perf(y): faster loop\n\n"
	if got != want {
		t.Errorf("Generate mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGenerate_FiltersNoise(t *testing.T) {
	subjects := []string{
		"chore: bump deps",
		"ci: tweak workflow",
		"docs: readme",
		"test(x): add test",
		"style: format",
		"Merge branch 'develop' into x",
		"Update .gitignore",
		"upd(singbox): bump",
		"fix(scope) - dash separator not colon",
	}
	if got := Generate(subjects, "1.0.0", "2026-01-01"); got != "" {
		t.Errorf("expected empty output for all-noise input, got:\n%s", got)
	}
}

func TestGenerate_PrefixVariants(t *testing.T) {
	subjects := []string{
		"fix: no scope",
		"feat(a,b): comma scope",
		"feat!: breaking no scope",
		"fix(x)!: breaking with scope",
	}
	got := Generate(subjects, "1.0.0", "2026-01-01")
	want := "## [1.0.0] - 2026-01-01\n\n" +
		"### Добавлено\n- feat(a,b): comma scope\n- feat!: breaking no scope\n\n" +
		"### Исправлено\n- fix: no scope\n- fix(x)!: breaking with scope\n\n"
	if got != want {
		t.Errorf("prefix variants mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGenerate_EmptyInput(t *testing.T) {
	if got := Generate(nil, "1.0.0", "2026-01-01"); got != "" {
		t.Errorf("expected empty for nil input, got %q", got)
	}
}

func TestGenerate_RoundTripParses(t *testing.T) {
	subjects := []string{
		"feat(frontend): селектор канала",
		"fix(updater): выбор версии",
		"refactor(x): cleanup",
		"chore: bump",
		"Merge branch 'develop'",
	}
	block := Generate(subjects, "2.11.2+r95", "2026-05-25")

	entries, err := updater.ParseChangelog(block)
	if err != nil {
		t.Fatalf("ParseChangelog error: %v", err)
	}
	e, ok := entries["2.11.2+r95"]
	if !ok {
		t.Fatalf("version 2.11.2+r95 not parsed; got keys %v", keysOf(entries))
	}
	if e.Date != "2026-05-25" {
		t.Errorf("date = %q, want 2026-05-25", e.Date)
	}
	if len(e.Groups) != 3 {
		t.Fatalf("groups = %d, want 3 (Добавлено/Исправлено/Рефакторинг)", len(e.Groups))
	}
	wantHeadings := []string{"Добавлено", "Исправлено", "Рефакторинг"}
	for i, want := range wantHeadings {
		if e.Groups[i].Heading != want {
			t.Errorf("group[%d].Heading = %q, want %q", i, e.Groups[i].Heading, want)
		}
		if len(e.Groups[i].Items) != 1 {
			t.Errorf("group[%d] items = %d, want 1", i, len(e.Groups[i].Items))
		}
	}
}

func keysOf(m map[string]updater.Entry) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

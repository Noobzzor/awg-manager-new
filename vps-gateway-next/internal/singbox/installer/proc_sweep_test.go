package installer

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestProcCmdline_ParsesNullSeparatedArgv(t *testing.T) {
	dir := t.TempDir()
	pidDir := filepath.Join(dir, "12345")
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		t.Fatal(err)
	}
	cmdline := []byte("/opt/etc/awg-manager/singbox/sing-box\x00run\x00-C\x00/opt/etc/awg-manager/singbox/config.json\x00")
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), cmdline, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readProcCmdline(filepath.Join(pidDir, "cmdline"))
	if err != nil {
		t.Fatalf("readProcCmdline: %v", err)
	}
	want := []string{"/opt/etc/awg-manager/singbox/sing-box", "run", "-C", "/opt/etc/awg-manager/singbox/config.json"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSweepOrphanProcesses_KillsOnlyMatchingArgv(t *testing.T) {
	dir := t.TempDir()
	procDir := filepath.Join(dir, "proc")

	mkProc := func(pid string, cmdline []byte) {
		os.MkdirAll(filepath.Join(procDir, pid), 0755)
		os.WriteFile(filepath.Join(procDir, pid, "cmdline"), cmdline, 0644)
	}
	// Our orphan
	mkProc("11111", []byte("/opt/etc/awg-manager/singbox/sing-box\x00run\x00-C\x00/opt/etc/awg-manager/singbox/config.json\x00"))
	// User's own sing-box (different binary path)
	mkProc("22222", []byte("/opt/bin/sing-box\x00run\x00-C\x00/opt/etc/sing-box/config.json\x00"))
	// Random other process
	mkProc("33333", []byte("/usr/bin/dnsmasq\x00-d\x00"))
	// Tracked (current managed) process — same argv as our orphan
	mkProc("44444", []byte("/opt/etc/awg-manager/singbox/sing-box\x00run\x00-C\x00/opt/etc/awg-manager/singbox/config.json\x00"))
	// Non-numeric directory entry (should be ignored)
	os.MkdirAll(filepath.Join(procDir, "self"), 0755)

	expectArgv := []string{"/opt/etc/awg-manager/singbox/sing-box", "run", "-C", "/opt/etc/awg-manager/singbox/config.json"}
	trackedPid := 44444

	var killed []int
	killer := func(pid int) error { killed = append(killed, pid); return nil }

	if err := sweepOrphanProcesses(procDir, expectArgv, trackedPid, killer); err != nil {
		t.Fatalf("sweepOrphanProcesses: %v", err)
	}

	if len(killed) != 1 || killed[0] != 11111 {
		t.Errorf("killed = %v, want [11111] only — strict argv match must spare other processes", killed)
	}
}

func TestSweepOrphanProcesses_NoTrackedPid(t *testing.T) {
	dir := t.TempDir()
	procDir := filepath.Join(dir, "proc")
	os.MkdirAll(filepath.Join(procDir, "5555"), 0755)
	os.WriteFile(filepath.Join(procDir, "5555", "cmdline"),
		[]byte("/opt/etc/awg-manager/singbox/sing-box\x00run\x00-C\x00/opt/etc/awg-manager/singbox/config.json\x00"),
		0644)

	expectArgv := []string{"/opt/etc/awg-manager/singbox/sing-box", "run", "-C", "/opt/etc/awg-manager/singbox/config.json"}

	var killed []int
	killer := func(pid int) error { killed = append(killed, pid); return nil }

	// trackedPid=0 means no current managed process — every match is an orphan.
	if err := sweepOrphanProcesses(procDir, expectArgv, 0, killer); err != nil {
		t.Fatalf("sweepOrphanProcesses: %v", err)
	}
	if len(killed) != 1 || killed[0] != 5555 {
		t.Errorf("killed = %v, want [5555]", killed)
	}
}

func TestSweepOrphanProcesses_HandlesMissingProcDir(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-proc")

	expectArgv := []string{"/foo/bar"}
	killer := func(pid int) error { return nil }

	if err := sweepOrphanProcesses(missing, expectArgv, 0, killer); err == nil {
		t.Error("expected error when procDir missing, got nil")
	}
}

func TestSweepOrphanProcesses_KillerErrorDoesNotAbort(t *testing.T) {
	dir := t.TempDir()
	procDir := filepath.Join(dir, "proc")

	mkProc := func(pid string, cmdline []byte) {
		os.MkdirAll(filepath.Join(procDir, pid), 0755)
		os.WriteFile(filepath.Join(procDir, pid, "cmdline"), cmdline, 0644)
	}
	expectArgv := []string{"/opt/etc/awg-manager/singbox/sing-box", "run", "-C", "/opt/etc/awg-manager/singbox/config.json"}
	mkProc("1", []byte("/opt/etc/awg-manager/singbox/sing-box\x00run\x00-C\x00/opt/etc/awg-manager/singbox/config.json\x00"))
	mkProc("2", []byte("/opt/etc/awg-manager/singbox/sing-box\x00run\x00-C\x00/opt/etc/awg-manager/singbox/config.json\x00"))

	var attempted []int
	killer := func(pid int) error {
		attempted = append(attempted, pid)
		if pid == 1 {
			return os.ErrPermission
		}
		return nil
	}

	if err := sweepOrphanProcesses(procDir, expectArgv, 0, killer); err != nil {
		t.Fatalf("sweep should tolerate killer errors: %v", err)
	}
	if len(attempted) != 2 {
		t.Errorf("expected both pids attempted (killer failure must not abort sweep), got %v", attempted)
	}
}

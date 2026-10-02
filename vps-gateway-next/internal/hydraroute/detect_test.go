package hydraroute

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDetect_NotInstalled(t *testing.T) {
	bin, pid := withFakeHydraPaths(t)
	_ = pid
	_ = bin

	got := Detect()
	if got.Installed {
		t.Fatalf("Installed = true, want false")
	}
	if got.Running {
		t.Fatalf("Running = true, want false")
	}
	if got.ProcessState != StateNotInstalled {
		t.Fatalf("ProcessState = %q, want %q", got.ProcessState, StateNotInstalled)
	}
}

func TestDetect_InstalledStopped(t *testing.T) {
	bin, _ := withFakeHydraPaths(t)
	mustWriteExecutable(t, bin)

	got := Detect()
	if !got.Installed {
		t.Fatalf("Installed = false, want true")
	}
	if got.Running {
		t.Fatalf("Running = true, want false")
	}
	if got.ProcessState != StateStopped {
		t.Fatalf("ProcessState = %q, want %q", got.ProcessState, StateStopped)
	}
}

func TestDetect_Running(t *testing.T) {
	bin, pid := withFakeHydraPaths(t)
	mustWriteExecutable(t, bin)
	self := os.Getpid()
	mustWriteText(t, pid, []byte(strconv.Itoa(self)))

	got := Detect()
	if !got.Installed {
		t.Fatalf("Installed = false, want true")
	}
	if !got.Running {
		t.Fatalf("Running = false, want true")
	}
	if got.PID != self {
		t.Fatalf("PID = %d, want %d", got.PID, self)
	}
	if got.ProcessState != StateRunning {
		t.Fatalf("ProcessState = %q, want %q", got.ProcessState, StateRunning)
	}
}

func TestDetect_DeadStalePID(t *testing.T) {
	bin, pid := withFakeHydraPaths(t)
	mustWriteExecutable(t, bin)
	mustWriteText(t, pid, []byte("999999"))

	got := Detect()
	if !got.Installed {
		t.Fatalf("Installed = false, want true")
	}
	if got.Running {
		t.Fatalf("Running = true, want false")
	}
	if got.StalePID != 999999 {
		t.Fatalf("StalePID = %d, want 999999", got.StalePID)
	}
	if got.ProcessState != StateDead {
		t.Fatalf("ProcessState = %q, want %q", got.ProcessState, StateDead)
	}
}

func TestParseVersionOutput(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "2.4.1", want: "2.4.1"},
		{in: "v2.4.1", want: "2.4.1"},
		{in: "HydraRoute Neo 2.4.1", want: "2.4.1"},
		{in: "hrneo version 2.4.1", want: "2.4.1"},
	}
	for _, tc := range tests {
		got := parseVersionOutput(tc.in)
		if got != tc.want {
			t.Fatalf("parseVersionOutput(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func withFakeHydraPaths(t *testing.T) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	oldBin, oldNeo, oldPID := hrneoBinary, neoCommand, pidFile
	hrneoBinary = filepath.Join(tmp, "hrneo")
	neoCommand = filepath.Join(tmp, "neo")
	pidFile = filepath.Join(tmp, "hrneo.pid")
	t.Cleanup(func() {
		hrneoBinary = oldBin
		neoCommand = oldNeo
		pidFile = oldPID
	})
	return hrneoBinary, pidFile
}

func mustWriteExecutable(t *testing.T, path string) {
	t.Helper()
	mustWriteText(t, path, []byte("#!/bin/sh\nexit 0\n"))
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

func mustWriteText(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

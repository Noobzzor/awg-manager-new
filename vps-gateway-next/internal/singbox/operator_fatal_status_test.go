package singbox

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

type fatalStatusValidator struct{ err error }

func (v fatalStatusValidator) Validate(context.Context, string) error { return v.err }

func TestGetStatusIncludesRejectedCandidateFatal(t *testing.T) {
	const validatorDetail = "generated AWG route rejected private_key=super-secret"
	runtimeDir := t.TempDir()
	op := NewOperator(OperatorDeps{
		Dir:              runtimeDir,
		ConfigDir:        filepath.Join(runtimeDir, "config.d"),
		Binary:           filepath.Join(runtimeDir, "sing-box"),
		DisableNDMSProxy: true,
	})
	orch := orchestrator.NewWithAppliedPath(op.ConfigDir(), op.Process(), filepath.Join(runtimeDir, "applied.json"))
	t.Cleanup(orch.Close)
	if err := orch.Register(orchestrator.SlotMeta{Slot: orchestrator.SlotAwg3, Filename: "16-awg3.json", AlwaysOn: true, HasContent: func() bool { return true }}); err != nil {
		t.Fatal(err)
	}
	if err := orch.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	op.SetOrch(orch)
	orch.SetValidator(fatalStatusValidator{err: errors.New(validatorDetail)})

	res, err := orch.SaveAndValidate(orchestrator.SlotAwg3, []byte(`{"endpoints":[{"type":"awg","tag":"bad"}]}`))
	if err != nil {
		t.Fatalf("SaveAndValidate infra error: %v", err)
	}
	if res.Ok() {
		t.Fatal("invalid candidate unexpectedly accepted")
	}
	got := op.GetStatus(context.Background()).LastError
	if !strings.Contains(got, "sing-box configuration validation failed") {
		t.Fatalf("status LastError = %q", got)
	}
	if strings.Contains(got, validatorDetail) || strings.Contains(got, "super-secret") {
		t.Fatalf("status LastError leaked validator diagnostics: %q", got)
	}
}

func TestGetStatusIncludesProcessFatal(t *testing.T) {
	runtimeDir := t.TempDir()
	op := NewOperator(OperatorDeps{
		Dir:              runtimeDir,
		ConfigDir:        filepath.Join(runtimeDir, "config.d"),
		Binary:           filepath.Join(runtimeDir, "sing-box"),
		DisableNDMSProxy: true,
	})
	op.setLastError("runtime process fatal")
	if got := op.GetStatus(context.Background()).LastError; got != "runtime process fatal" {
		t.Fatalf("status LastError = %q", got)
	}
}

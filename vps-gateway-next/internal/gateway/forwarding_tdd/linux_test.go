package forwarding

import (
	"errors"
	"strings"
	"testing"

	systemexec "github.com/hoaxisr/awg-manager/internal/sys/exec"
)

func TestApplyNFTBatchUsesAtomicDeleteAndReplaceWhenTableExists(t *testing.T) {
	var gotBatch string
	err := applyNFTBatch("table inet awgm_gateway {}", func(batch string) (*systemexec.Result, error) {
		gotBatch = batch
		return &systemexec.Result{}, nil
	}, func() (*systemexec.Result, error) {
		return &systemexec.Result{Stdout: `table inet awgm_gateway { comment "AWGM_GATEWAY_OWNER" }`}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotBatch, "delete table inet awgm_gateway\n") || !strings.Contains(gotBatch, "table inet awgm_gateway {}") {
		t.Fatalf("batch does not atomically delete and replace owned table: %q", gotBatch)
	}
}

func TestApplyNFTBatchRefusesToDeleteUnownedTable(t *testing.T) {
	applied := false
	err := applyNFTBatch("table inet awgm_gateway {}", func(string) (*systemexec.Result, error) {
		applied = true
		return &systemexec.Result{}, nil
	}, func() (*systemexec.Result, error) {
		return &systemexec.Result{Stdout: `table inet awgm_gateway {\n	comment "OTHER_OWNER"\n}`}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("error = %v, want unowned-table refusal", err)
	}
	if applied {
		t.Fatal("nft batch ran after finding an unowned table")
	}
}

func TestApplyNFTBatchCreatesTableWhenMissing(t *testing.T) {
	var gotBatch string
	err := applyNFTBatch("table inet awgm_gateway {}", func(batch string) (*systemexec.Result, error) {
		gotBatch = batch
		return &systemexec.Result{}, nil
	}, func() (*systemexec.Result, error) {
		return &systemexec.Result{Stderr: "No such file or directory"}, errNFTTableMissing
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBatch != "table inet awgm_gateway {}" {
		t.Fatalf("initial batch = %q, want definition only", gotBatch)
	}
}

func TestApplyNFTBatchStopsWhenTableInspectionFailsUnexpectedly(t *testing.T) {
	inspectErr := errors.New("permission denied")
	applied := false
	err := applyNFTBatch("table inet awgm_gateway {}", func(string) (*systemexec.Result, error) {
		applied = true
		return &systemexec.Result{}, nil
	}, func() (*systemexec.Result, error) {
		return &systemexec.Result{Stderr: inspectErr.Error()}, inspectErr
	})
	if !errors.Is(err, inspectErr) {
		t.Fatalf("error = %v, want wrapped inspection error", err)
	}
	if applied {
		t.Fatal("nft batch ran after table inspection failed")
	}
}

func TestApplyNFTBatchReturnsFailedReplacementError(t *testing.T) {
	applyErr := errors.New("invalid replacement")
	var gotBatch string
	err := applyNFTBatch("table inet awgm_gateway {}", func(batch string) (*systemexec.Result, error) {
		gotBatch = batch
		return &systemexec.Result{Stderr: "syntax error"}, applyErr
	}, func() (*systemexec.Result, error) {
		return &systemexec.Result{Stdout: `table inet awgm_gateway { comment "AWGM_GATEWAY_OWNER" }`}, nil
	})
	if !errors.Is(err, applyErr) || !strings.Contains(err.Error(), "syntax error") {
		t.Fatalf("error = %v, want apply error with stderr", err)
	}
	if !strings.HasPrefix(gotBatch, "delete table inet awgm_gateway\n") {
		t.Fatalf("failed replacement batch = %q", gotBatch)
	}
}

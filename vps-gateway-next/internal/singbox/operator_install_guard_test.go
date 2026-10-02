package singbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/installer"
)

// blockingDownloader signals startedCh once DownloadFile is entered, then
// blocks until the test closes releaseCh. Models the "in-flight window" of a
// slow install so a concurrent second call can observe the guard.
type blockingDownloader struct {
	startedCh chan struct{}
	releaseCh chan struct{}
}

func (d *blockingDownloader) DownloadFile(_ context.Context, req installer.DownloadFileRequest) (installer.DownloadFileResult, error) {
	close(d.startedCh)
	<-d.releaseCh
	return installer.DownloadFileResult{}, errors.New("stubbed: download stopped for test")
}

// TestOperator_Update_ConcurrentCalls_GuardRejectsSecond verifies that while
// one Update() is mid-flight (blocked in Download), a concurrent second
// Update() call is rejected immediately with ErrInstallInProgress instead of
// racing the first into the download/activate/restart sequence.
func TestOperator_Update_ConcurrentCalls_GuardRejectsSecond(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "sing-box")
	if err := os.WriteFile(binary, []byte("old-content"), 0o755); err != nil {
		t.Fatal(err)
	}
	op := NewOperator(OperatorDeps{Dir: dir, Binary: binary})
	inst := installer.New(binary, "test-arch", installer.BinarySpec{
		Version: "1.2.3", URL: "u", SHA256: "different-sha", Size: 10 << 20,
	}, nil)
	inst.SetFreeDiskFn(func(string) (int64, bool) { return 200 << 20, true })

	dl := &blockingDownloader{startedCh: make(chan struct{}), releaseCh: make(chan struct{})}
	inst.SetDownloader(dl)
	op.SetInstaller(inst)

	firstErrCh := make(chan error, 1)
	go func() {
		firstErrCh <- op.Update(context.Background())
	}()

	<-dl.startedCh // первый вызов внутри guard'а, держит его

	secondErr := op.Update(context.Background())
	if !errors.Is(secondErr, ErrInstallInProgress) {
		t.Fatalf("second Update err = %v, want ErrInstallInProgress", secondErr)
	}

	close(dl.releaseCh)
	firstErr := <-firstErrCh
	if errors.Is(firstErr, ErrInstallInProgress) {
		t.Fatalf("first Update unexpectedly got ErrInstallInProgress: %v", firstErr)
	}

	// Guard released after completion — a subsequent call must not be
	// rejected by a stuck flag.
	dl2 := &blockingDownloader{startedCh: make(chan struct{}), releaseCh: make(chan struct{})}
	close(dl2.releaseCh) // не блокируем — сразу отдаём ошибку
	inst.SetDownloader(dl2)
	if err := op.Update(context.Background()); errors.Is(err, ErrInstallInProgress) {
		t.Fatalf("Update after release still guarded: %v", err)
	}
}

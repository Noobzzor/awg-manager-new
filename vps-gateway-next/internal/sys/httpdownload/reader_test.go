package httpdownload

import (
	"bytes"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

type fixedChunkReader struct {
	data []byte
	pos  int
	step int
}

func (r *fixedChunkReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := r.step
	if n > len(p) {
		n = len(p)
	}
	remain := len(r.data) - r.pos
	if n > remain {
		n = remain
	}
	copy(p[:n], r.data[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}

func TestReader_PassthroughBytes(t *testing.T) {
	src := bytes.Repeat([]byte("x"), 1024)
	pr := NewReader(bytes.NewReader(src), int64(len(src)), nil)
	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, src) {
		t.Errorf("payload mismatch: got %d bytes, want %d", len(got), len(src))
	}
	if pr.BytesRead() != int64(len(src)) {
		t.Errorf("BytesRead = %d, want %d", pr.BytesRead(), len(src))
	}
}

func TestReader_EmitsAtLeastOnceOnEOF(t *testing.T) {
	// 1 KB of data — well under emitBytes threshold, so we should still
	// see a final emit triggered by io.EOF.
	src := bytes.Repeat([]byte("x"), 1024)
	var calls atomic.Int32
	var lastDownloaded atomic.Int64
	pr := NewReader(bytes.NewReader(src), int64(len(src)), func(downloaded, total int64) {
		calls.Add(1)
		lastDownloaded.Store(downloaded)
	})
	if _, err := io.ReadAll(pr); err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if calls.Load() == 0 {
		t.Error("expected at least one progress emit (EOF flush)")
	}
	if got := lastDownloaded.Load(); got != int64(len(src)) {
		t.Errorf("last downloaded = %d, want %d (final frame)", got, len(src))
	}
}

func TestReader_DoesNotEmitOnEverySmallRead(t *testing.T) {
	// Many tiny reads should not cause progress callback spam.
	src := bytes.Repeat([]byte("x"), 64*1024)
	var calls atomic.Int32
	r := &fixedChunkReader{data: src, step: 16}
	current := time.Unix(0, 0)
	now := func() time.Time { return current }
	pr := newReaderWithClock(r, int64(len(src)), func(downloaded, total int64) {
		calls.Add(1)
	}, now)
	if _, err := io.ReadAll(pr); err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("expected exactly final emit with frozen clock, got %d", got)
	}
}

func TestReader_PeriodicEmitByTime(t *testing.T) {
	src := bytes.Repeat([]byte("x"), 1536)
	var calls atomic.Int32
	r := &fixedChunkReader{data: src, step: 512}
	current := time.Unix(0, 0)
	now := func() time.Time { return current }
	pr := newReaderWithClock(r, int64(len(src)), func(downloaded, total int64) {
		calls.Add(1)
	}, now)
	buf := make([]byte, 512)
	for {
		_, err := pr.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		current = current.Add(210 * time.Millisecond)
	}
	if got := calls.Load(); got < 2 {
		t.Errorf("expected periodic emits, got %d", got)
	}
}

func TestReader_NilProgressIsSafe(t *testing.T) {
	src := bytes.Repeat([]byte("x"), 1024)
	pr := NewReader(bytes.NewReader(src), 0, nil)
	if _, err := io.ReadAll(pr); err != nil {
		t.Fatalf("ReadAll with nil progress: %v", err)
	}
}

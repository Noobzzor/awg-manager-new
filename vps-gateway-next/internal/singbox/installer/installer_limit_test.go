package installer

import "testing"

func TestDownloadByteLimit(t *testing.T) {
	const (
		fallback = 128 << 20
		reserve  = 16 << 20
	)
	cases := []struct {
		name   string
		free   int64
		freeOK bool
		want   int64
	}{
		{"free unknown → fallback", 0, false, fallback},
		{"plenty (external storage, GBs)", 4 << 30, true, (4 << 30) - reserve},
		{"enough for a 72MB binary", 200 << 20, true, (200 << 20) - reserve},
		{"tiny disk → below a 72MB binary", 40 << 20, true, (40 << 20) - reserve},
		{"free below reserve → clamps to 0", 8 << 20, true, 0},
	}
	for _, c := range cases {
		got := downloadByteLimit(c.free, c.freeOK, fallback, reserve)
		if got != c.want {
			t.Errorf("%s: downloadByteLimit(%d,%v) = %d, want %d", c.name, c.free, c.freeOK, got, c.want)
		}
	}
}

package semver

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// equal
		{"1.0.0", "1.0.0", 0},
		{"2.3.11", "2.3.11", 0},
		// missing components treated as 0
		{"1.2", "1.2.0", 0},
		{"1.2.0", "1.2", 0},
		{"", "0.0.0", 0},
		{"1", "1.0.0", 0},
		{"1.2.3", "1.2.3.0", 0},
		// cross-digit carry-over (real-world release tags)
		{"2.3.10", "2.3.11", -1},
		{"2.3.11", "2.3.10", 1},
		{"2.7.10", "2.7.3", 1},
		{"10.0.0", "9.99.99", 1},
		// simple less/greater
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"2.0.0", "1.99.99", 1},
		{"1.99.99", "2.0.0", -1},
		{"0.0.1", "0.0.2", -1},
		{"1.2.3.1", "1.2.3", 1},
		{"2.4.0", "2.3.99", 1},
		// non-numeric component parses as 0 (per contract)
		{"1.x.3", "1.0.3", 0},
		// GitHub / CI build revision (+rN) — same release as repo
		{"2.11.2+r70", "2.11.2", 0},
		{"2.11.2", "2.11.2+r71", 0},
		{"2.11.2+r1", "2.11.2+r99", 0},
		// pre-release on patch component
		{"2.11.2-rc1", "2.11.2", 0},
		{"2.11.2-beta", "2.11.2", 0},
		{"2.11.2-rc1", "2.11.2+r70", 0},
		// numeric ordering still works with build tags present
		{"2.11.2+r70", "2.11.3", -1},
		{"2.11.1+r5", "2.11.2", -1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestBase(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"2.11.2+r70", "2.11.2"},
		{"2.11.2", "2.11.2"},
		{"  2.11.2+r1  ", "2.11.2"},
		{"2.11.2-rc1", "2.11.2-rc1"},
	}
	for _, c := range cases {
		if got := Base(c.in); got != c.want {
			t.Errorf("Base(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Regression: Go strconv.Atoi("2+r70") errors → 0, which made 2.11.2+r70 < 2.11.2.
func TestCompare_BuildRevisionNotLessThanRelease(t *testing.T) {
	if got := Compare("2.11.2+r70", "2.11.2"); got < 0 {
		t.Fatalf("build revision must not compare less than release: got %d", got)
	}
}

func TestCompareWithRevision(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2.11.2+r70", "2.11.2+r71", -1},
		{"2.11.2+r71", "2.11.2+r70", 1},
		{"2.11.2+r70", "2.11.2+r70", 0},
		{"2.11.2", "2.11.2+r1", -1},
		{"2.11.2+r1", "2.11.2", 1},
		{"2.11.2", "2.11.2", 0},
		{"2.11.2+r99", "2.11.3", -1},
		{"2.11.3", "2.11.2+r99", 1},
		{"2.11.2+r5", "2.11.10+r1", -1},
		{"2.11.2+rX", "2.11.2", 0},
		{"2.11.2+r", "2.11.2", 0},
	}
	for _, c := range cases {
		if got := CompareWithRevision(c.a, c.b); got != c.want {
			t.Errorf("CompareWithRevision(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

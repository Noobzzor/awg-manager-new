package signature

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The exact default I1 emitted by the docker-amneziawg container
// (root/etc/s6-overlay/s6-rc.d/init-amneziawg-confs/run:103): a QUIC Initial
// padded to the RFC 9000 §14.1 minimum of 1200 bytes. Its trailing <r 1178>
// exceeds the per-tag limit, so NDMS rejects the whole value on import.
const containerDefaultI1 = "<b 0xc3><b 0x00000001><b 0x08><r 8><b 0x00><b 0x00><b 0x449e><r 4><r 1178>"

func TestSplitOversizedTags_ContainerDefaultI1(t *testing.T) {
	got, note := SplitOversizedTags(containerDefaultI1)
	want := "<b 0xc3><b 0x00000001><b 0x08><r 8><b 0x00><b 0x00><b 0x449e><r 4><r 1000><r 178>"
	if got != want {
		t.Errorf("SplitOversizedTags(containerDefaultI1)\n got %q\nwant %q", got, want)
	}
	if note != "<r 1178> → <r 1000><r 178>" {
		t.Errorf("note = %q", note)
	}
}

func TestSplitOversizedTags_PreservesUnaffected(t *testing.T) {
	// Anything already within the limit and in canonical form must come back
	// byte-identical with an empty note: a gratuitous rewrite of a working
	// config is a regression, not a fix.
	cases := []string{
		"",
		"<b 0x170303><r 32><t>",
		"<r 1000>",
		"<rc 16><rd 8><c>",
		"<r 0>", // NDMS accepts it; not ours to drop
		"<b 0xc3><b 0x00000001>",
	}
	for _, in := range cases {
		if got, note := SplitOversizedTags(in); got != in || note != "" {
			t.Errorf("SplitOversizedTags(%q)=%q, %q; want unchanged, empty note", in, got, note)
		}
	}
}

func TestSplitOversizedTags_AllRandomKinds(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<r 1178>", "<r 1000><r 178>"},
		{"<rc 1500>", "<rc 1000><rc 500>"},
		{"<rd 2500>", "<rd 1000><rd 1000><rd 500>"},
		{"<r 2000>", "<r 1000><r 1000>"},
		{"<r 1001>", "<r 1000><r 1>"},
	}
	for _, c := range cases {
		if got, _ := SplitOversizedTags(c.in); got != c.want {
			t.Errorf("SplitOversizedTags(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestSplitOversizedTags_ByteCountPreserved(t *testing.T) {
	// The whole point: splitting must be wire-equivalent. Same total random
	// bytes in, same total out — otherwise we have silently changed the
	// packet the peer expects.
	re := regexp.MustCompile(`<(r|rc|rd) (\d+)>`)
	sum := func(s string) int {
		total := 0
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			n, _ := strconv.Atoi(m[2])
			total += n
		}
		return total
	}
	for _, in := range []string{containerDefaultI1, "<rd 4321><b 0xff><rc 9999>", "<r 100000>"} {
		got, _ := SplitOversizedTags(in)
		if sum(got) != sum(in) {
			t.Errorf("byte count changed for %q: %d -> %d", in, sum(in), sum(got))
		}
		for _, m := range re.FindAllStringSubmatch(got, -1) {
			n, _ := strconv.Atoi(m[2])
			if n > MaxTagBytes {
				t.Errorf("SplitOversizedTags(%q) left an oversized tag %q", in, m[0])
			}
		}
	}
}

func TestSplitOversizedTags_ToleratesOddSpacingAndJunk(t *testing.T) {
	// Third-party configs are not required to match our formatting. Tokens we
	// do not understand must survive untouched rather than be dropped.
	if got, _ := SplitOversizedTags("<r1178>"); got != "<r 1000><r 178>" {
		t.Errorf("no-space form: got %q", got)
	}
	if got, _ := SplitOversizedTags("<r  1178 >"); got != "<r 1000><r 178>" {
		t.Errorf("extra-space form: got %q", got)
	}
	for _, in := range []string{"<t>", "<z 5000>", "plain text", "<b 0xdead><unknown>"} {
		if got, note := SplitOversizedTags(in); got != in || note != "" {
			t.Errorf("SplitOversizedTags(%q)=%q, %q; want unchanged, empty note", in, got, note)
		}
	}
}

// A size beyond MaxSplittableTagBytes must be left alone, not expanded. Without
// the ceiling, <r 999999999999999999> spins ~10^15 Builder appends and takes
// the process out on memory — and nothing validates I1-I5 on import, so such a
// value can reach us from a .conf.
func TestSplitOversizedTags_LeavesAbsurdSizesAlone(t *testing.T) {
	for _, in := range []string{
		"<r 999999999999999999>",
		"<r 100001>",
		"<rd 2000000000>",
		"<r 99999999999999999999999999>", // beyond int64: Atoi fails
	} {
		done := make(chan string, 1)
		go func() { out, note := SplitOversizedTags(in); done <- out + "|" + note }()
		select {
		case got := <-done:
			if got != in+"|" {
				t.Errorf("SplitOversizedTags(%q)=%q, want unchanged with empty note", in, got)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("SplitOversizedTags(%q) did not return — missing size ceiling", in)
		}
	}
	// The largest size we still rewrite.
	if got, _ := SplitOversizedTags("<r 100000>"); !strings.HasPrefix(got, "<r 1000>") {
		t.Errorf("<r 100000> should still be split, got %q", got)
	}
}

// The regex tolerates spacing upstream's parser rejects, so every matched
// token is re-emitted canonically — not only the oversized ones. Otherwise a
// config gets "fixed", logged as fixed, and still rejected on another token.
func TestSplitOversizedTags_CanonicalisesCompliantTokens(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<r500>", "<r 500>"},
		{"<r  500 >", "<r 500>"},
		{"<rc16>", "<rc 16>"},
		{"<r1178><r500>", "<r 1000><r 178><r 500>"},
	}
	for _, c := range cases {
		if got, _ := SplitOversizedTags(c.in); got != c.want {
			t.Errorf("SplitOversizedTags(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

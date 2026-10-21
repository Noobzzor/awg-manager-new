//go:build linux

package ingress

import (
	"strings"
	"testing"
)

func TestSelectedAWGPeerOutputFragmentsAndValidation(t *testing.T) {
	row := ownedPeerFixtureKey + " 42 99"
	for _, size := range []int{1, 7, 4096} {
		collector := &selectedAWGPeerOutput{key: ownedPeerFixtureKey}
		data := strings.Repeat("unrelated 0 0\n", 1000) + row
		for len(data) > 0 {
			end := size
			if end > len(data) {
				end = len(data)
			}
			if _, err := collector.Write([]byte(data[:end])); err != nil {
				t.Fatal(err)
			}
			data = data[end:]
		}
		selected, err := collector.finish()
		if err != nil || selected != row+"\n" || len(collector.pending) > 1024 {
			t.Fatal("fragmented selected row lost or storage unbounded")
		}
	}
	for _, data := range []string{row + "\n" + row + "\n", strings.Repeat("x", 1025) + "\n" + row} {
		collector := &selectedAWGPeerOutput{key: ownedPeerFixtureKey}
		_, _ = collector.Write([]byte(data))
		if _, err := collector.finish(); err == nil {
			t.Fatal("duplicate/oversized stream accepted")
		}
	}
}

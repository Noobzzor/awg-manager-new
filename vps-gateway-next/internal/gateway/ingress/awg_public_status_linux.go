//go:build linux

package ingress

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Keep one selected row and one bounded line. Aggregate valid peer output has
// no count/byte admission limit; the subprocess has a finite runtime budget.
type selectedAWGPeerOutput struct {
	key        string
	pending    []byte
	selected   string
	matched    bool
	failed     bool
	discarding bool
}

func (b *selectedAWGPeerOutput) line() {
	fields := bytes.Fields(b.pending)
	if len(fields) > 0 && bytes.Equal(fields[0], []byte(b.key)) {
		if b.matched {
			b.failed = true
		} else {
			b.matched = true
			b.selected = string(b.pending) + "\n"
		}
	}
	b.pending = b.pending[:0]
}

func (b *selectedAWGPeerOutput) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		newline := bytes.IndexByte(p, '\n')
		end := len(p)
		if newline >= 0 {
			end = newline
		}
		if !b.discarding {
			if len(b.pending)+end > 1024 {
				b.failed = true
				b.discarding = true
				b.pending = b.pending[:0]
			} else {
				b.pending = append(b.pending, p[:end]...)
			}
		}
		if newline < 0 {
			break
		}
		if !b.discarding {
			b.line()
		}
		b.discarding = false
		p = p[end+1:]
	}
	return n, nil
}

func (b *selectedAWGPeerOutput) finish() (string, error) {
	if len(b.pending) > 0 && !b.discarding {
		b.line()
	}
	if b.failed {
		return "", fmt.Errorf("invalid owned AWG public status stream")
	}
	return b.selected, nil
}

func runUserspaceAWGSelectedPublicField(parent context.Context, binary, name, field, key string) (string, error) {
	if !validOwnedPeerKey(key) {
		return "", fmt.Errorf("invalid owned AWG peer public key")
	}
	switch field {
	case "endpoints", "latest-handshakes", "transfer":
	default:
		return "", fmt.Errorf("invalid owned AWG public field")
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "show", name, field)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	command.WaitDelay = 250 * time.Millisecond
	output := &selectedAWGPeerOutput{key: key, pending: make([]byte, 0, 1024)}
	command.Stdout, command.Stderr = output, io.Discard
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("owned AWG public status command failed")
	}
	return output.finish()
}

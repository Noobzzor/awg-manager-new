//go:build !linux

package connections

import "errors"

// KillParams identifies a conntrack entry by its original-direction 5-tuple.
type KillParams struct {
	Src      string
	Dst      string
	SrcPort  int
	DstPort  int
	Protocol string
}

// Kill is linux-only (ctnetlink); на других ОС всегда ошибка.
func Kill(p KillParams) error {
	return errors.New("conntrack kill is linux-only")
}

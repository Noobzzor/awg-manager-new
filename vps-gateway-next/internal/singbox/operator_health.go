package singbox

import (
	"context"
	"time"
)

// IsResponsive checks the active engine API without changing process supervision.
// The caller's cancellation and a one-second ceiling bound the local probe.
func (o *Operator) IsResponsive(ctx context.Context) bool {
	if o.clash == nil {
		return false
	}
	probe, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err := o.clash.Version(probe)
	return err == nil
}

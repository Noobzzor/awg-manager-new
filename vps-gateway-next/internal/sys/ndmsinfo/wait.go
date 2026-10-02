package ndmsinfo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/ndms/query"
)

// WaitForNDMS polls NDMS every second until it responds to a basic RCI
// query (GetDefaultGatewayInterface). Returns as soon as NDMS answers,
// even when the answer is "no default route yet" — the key signal is that
// the RCI endpoint is alive and the interface subsystem has initialized.
//
// Logs every failed probe with the error for post-boot diagnostics, and
// logs the successful probe with elapsed time.
//
// The caller MUST supply a context with a deadline or timeout — this
// function loops until NDMS responds OR the context is cancelled/expired.
func WaitForNDMS(ctx context.Context, routes *query.RouteStore, log *logging.ScopedLogger) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	start := time.Now()

	// Try once before entering the poll loop — NDMS may already be up.
	if err := ndmsResponds(ctx, routes); err == nil {
		log.Debug("ndms-wait", "", "NDMS ready immediately")
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := ndmsResponds(ctx, routes); err == nil {
				log.Debug("ndms-wait", "",
					fmt.Sprintf("NDMS ready after %s polling", time.Since(start).Round(time.Second)))
				return nil
			} else {
				log.Debug("ndms-wait", "",
					fmt.Sprintf("NDMS not ready yet (probe: %v)", err))
			}
		}
	}
}

// ndmsResponds performs a lightweight NDMS health probe and returns nil
// when the RCI endpoint is reachable. Returns the error on failure.
//
// Uses GetDefaultGatewayInterface because it's the simplest read-path
// query available at every consumer call-site (it's what the boot
// goroutine already checks after the wait). A successful response
// ("there is a default gateway") AND ErrNoDefaultRoute ("NDMS answered
// but no default route exists yet") both signal a working RCI endpoint.
func ndmsResponds(ctx context.Context, routes *query.RouteStore) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := routes.GetDefaultGatewayInterface(ctx)
	if err == nil || errors.Is(err, query.ErrNoDefaultRoute) {
		return nil
	}
	return err
}

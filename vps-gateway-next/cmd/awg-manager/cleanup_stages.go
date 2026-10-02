package main

import (
	"errors"
	"fmt"
	"github.com/hoaxisr/awg-manager/internal/cleanup"
)

// runCleanupStages preserves best-effort removal without reporting incomplete
// cleanup as success. A failed core must never supply even a typed-nil cleaner.
func runCleanupStages(init func() (cleanup.SingboxCleaner, error), resources func(cleanup.SingboxCleaner) error, policyTun, files func() error) error {
	cleaner, initErr := init()
	if initErr != nil {
		cleaner = nil
		initErr = fmt.Errorf("cleanup initialization: %w", initErr)
	}
	resourceErr := resources(cleaner)
	policyErr := policyTun()
	fileErr := files()
	return errors.Join(initErr, resourceErr, policyErr, fileErr)
}

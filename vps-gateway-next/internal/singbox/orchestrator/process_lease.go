package orchestrator

import "context"

// ProcessLease excludes public process mutations while allowing its owner to
// activate or restore state. Reads such as IsRunning remain available.
type ProcessLease interface {
	ProcessController
	StartSpawned() (bool, error)
	Close()
}

type ProcessLeaseProvider interface {
	AcquireProcessLease(context.Context) (ProcessLease, error)
}

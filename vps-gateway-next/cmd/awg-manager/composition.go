package main

// compositionHooks makes the deployment split executable as a small, testable
// call graph. Production supplies bound app methods; tests replace every phase
// to prove both order and negative capability guarantees.
type compositionHooks struct {
	core, ndms, tunnels, services, orchestrator, eventWiring func()
	server, deviceProxy, router, listen, shutdown            func()
	singbox                                                  func() error
	proxyShutdown, boot                                      func()
	local                                                    func() error
	localServer, localLifecycle                              func()
	serve                                                    func()
}

// runPlatformSetup keeps router-specific clock and Entware CA discovery out
// of generic Linux mode while preserving their historical Keenetic order.
func runPlatformSetup(mode deploymentMode, installRouterClock, setupCACerts func()) {
	if mode != deploymentModeKeenetic {
		return
	}
	installRouterClock()
	setupCACerts()
}

func runComposition(mode deploymentMode, h compositionHooks) error {
	h.core()
	if mode == deploymentModeSingbox {
		if err := h.local(); err != nil {
			return err
		}
		h.localServer()
		h.localLifecycle()
		h.serve()
		return nil
	}

	// Keep the historical Keenetic setup order byte-for-byte visible here.
	h.ndms()
	h.tunnels()
	h.services()
	h.orchestrator()
	h.eventWiring()
	if err := h.singbox(); err != nil {
		return err
	}
	h.server()
	h.deviceProxy()
	h.router()
	h.listen()
	h.shutdown()
	h.proxyShutdown()
	h.boot()
	h.serve()
	return nil
}

// runWithCleanup guarantees that all registered lifecycle cleanup completes
// before a startup/serve error is returned to main for a non-zero process exit.
func runWithCleanup(run func() error, cleanup func()) (err error) {
	defer cleanup()
	return run()
}

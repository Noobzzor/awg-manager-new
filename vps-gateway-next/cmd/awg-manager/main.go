package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/sys/routerclock"
)

const defaultDataDir = "/opt/etc/awg-manager"

// version is set via ldflags at build time
var version = "dev"

func main() {
	mode, err := parseDeploymentMode(os.LookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	runtime := runtimeSettings{dataDir: defaultDataDir}
	if mode == deploymentModeSingbox {
		runtime = localRuntimeSettings(os.Getenv)
	}

	dataDir := flag.String("data-dir", runtime.dataDir, "Data directory path")
	showVersion := flag.Bool("version", false, "Show version and exit")
	cleanup := flag.Bool("cleanup", false, "Stop and delete all tunnels, then exit (for uninstall)")
	serviceAction := flag.String("service", "", "Service management (start|stop|restart|status)")
	forceBoot := flag.Bool("force-boot", false, "Simulate boot mode (for testing boot path on running router)")
	pprofListen := flag.String("pprof-listen", "", "Dedicated TCP address for Go /debug/pprof only (recommended: 127.0.0.1:6060); empty disables standalone pprof")
	slowReqMS := flag.Int("slow-request-ms", 0, "Log HTTP handlers slower than this (ms) to stderr via slog (0 disables); long-lived SSE/WS routes are excluded")
	flag.Parse()

	// `-data-dir` обязан соблюдаться целиком: иначе демон в песочнице пишет
	// .conf туннелей, файлы релея, модули и скрипты роутера в БОЕВОЙ каталог
	// (F168, наблюдалось на стенде 08.09). Ставим сразу после разбора флагов:
	// ниже по main из того же каталога работают и --cleanup, и --service, и
	// сторы с операторами читают эти пути уже при конструировании.
	applyDataDir(*dataDir)
	runtime.dataDir = *dataDir
	if mode == deploymentModeSingbox && os.Getenv("SINGBOX_CONFIG_DIR") == "" {
		runtime.singboxConfigDir = filepath.Join(*dataDir, "sing-box", "config.d")
	}

	oneShots := oneShotHooks{
		localCleanup:  runLocalCleanup,
		legacyCleanup: runCleanup,
		legacyService: runService,
	}
	// Local one-shots must resolve before any platform setup can acquire an
	// Entware/NDMS dependency. runPlatformSetup is currently a local no-op,
	// and this ordering makes the boundary explicit and regression-testable.
	if mode == deploymentModeSingbox {
		handled, err := runOneShot(mode, runtime, *cleanup, *serviceAction, oneShots)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if handled {
			return
		}
	}

	runPlatformSetup(mode, func() { routerclock.InstallAsLocal() }, ensureCACerts)

	if *showVersion {
		fmt.Printf("awg-manager version %s\n", version)
		os.Exit(0)
	}

	if handled, err := runOneShot(mode, runtime, *cleanup, *serviceAction, oneShots); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	} else if handled {
		return
	}

	a := &app{
		mode:             mode,
		dataDir:          *dataDir,
		httpAddr:         runtime.httpAddr,
		singboxBinary:    runtime.singboxBinary,
		singboxConfigDir: runtime.singboxConfigDir,
		proxyAddr:        runtime.proxyAddr,
		forceBoot:        *forceBoot,
		pprofListen:      strings.TrimSpace(*pprofListen),
		slowReqMS:        *slowReqMS,
	}
	var serveErr error
	err = runWithCleanup(func() error {
		if err := runComposition(mode, compositionHooks{
			core:           a.setupCore,
			ndms:           a.setupNDMS,
			tunnels:        a.setupTunnels,
			services:       a.setupServices,
			orchestrator:   a.setupOrchestrator,
			eventWiring:    a.setupEventWiring,
			singbox:        a.setupSingbox,
			server:         a.setupServer,
			deviceProxy:    a.setupDeviceProxy,
			router:         a.setupRouter,
			listen:         a.setupListen,
			shutdown:       a.setupShutdown,
			proxyShutdown:  a.registerProxyShutdown,
			boot:           a.startBootSequence,
			local:          a.setupLocal,
			localServer:    a.setupLocalServer,
			localLifecycle: a.setupLocalLifecycle,
			serve:          func() { serveErr = a.serve() },
		}); err != nil {
			return err
		}
		return serveErr
	}, a.runOnExit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

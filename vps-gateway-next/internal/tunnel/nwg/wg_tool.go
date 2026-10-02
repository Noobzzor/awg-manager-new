package nwg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	sysexec "github.com/hoaxisr/awg-manager/internal/sys/exec"
)

// Установка IPv6-endpoint'а на ASC-прошивках (≥5.01.A.4): NDMS не принимает
// v6-endpoint через RCI ни в импорте, ни в peer-командах (подтверждено
// автором на устройстве) — endpoint выставляется напрямую в ядро через
// wireguard-tools по kernel-имени интерфейса (nwgN).
//
// NDMS перезаписывает выставленный так endpoint своей заглушкой при любом
// переприменении конфига (ребут роутера, up/down интерфейса, WAN-failover,
// рестарт от его ping-check) — за возвратом следит endpoint-страж
// (endpoint_guard.go), а после рестарта демона реестр стража наполняет
// заново Start (EventReconnect/EventBoot → ActionStartNativeWG,
// см. orchestrator/decide.go).

// wgToolCandidates — места, где Entware/прошивка держат бинарь wg
// (пакет wireguard-tools).
var wgToolCandidates = []string{"/opt/bin/wg", "/opt/sbin/wg", "/opt/usr/bin/wg", "/usr/bin/wg"}

// wgToolLookup возвращает путь к первому исполняемому wg (кандидаты, затем
// PATH) или "". Переопределяется в тестах.
var wgToolLookup = func() string {
	for _, p := range wgToolCandidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return p
		}
	}
	if p, err := exec.LookPath("wg"); err == nil {
		return p
	}
	return ""
}

// wgToolRun запускает wg с аргументами. Переопределяется в тестах.
var wgToolRun = func(ctx context.Context, binary string, args ...string) error {
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := sysexec.Run(runCtx, binary, args...)
	if err != nil {
		if res != nil && res.Stderr != "" {
			return fmt.Errorf("%w: %s", err, res.Stderr)
		}
		return err
	}
	return nil
}

// wgToolOutput — как wgToolRun, но возвращает stdout (для `wg show`).
// Переопределяется в тестах.
var wgToolOutput = func(ctx context.Context, binary string, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := sysexec.Run(runCtx, binary, args...)
	if err != nil {
		if res != nil && res.Stderr != "" {
			return "", fmt.Errorf("%w: %s", err, res.Stderr)
		}
		return "", err
	}
	return res.Stdout, nil
}

// wgSetRetryDelay — пауза между ретраями wg set в startNative (гонка с
// асинхронным поднятием интерфейса NDMS). Переопределяется в тестах.
var wgSetRetryDelay = 700 * time.Millisecond

// errWGToolMissing — единый текст «нет wireguard-tools» с подсказкой.
func errWGToolMissing() error {
	return fmt.Errorf("IPv6 endpoint на прошивке с нативным ASC выставляется только через wireguard-tools, бинарь wg не найден — установите пакет: opkg install wireguard-tools")
}

// setKernelPeerEndpoint выставляет endpoint пира напрямую в ядро:
// `wg set <nwgN> peer <pubkey> endpoint <[v6]:port>`.
func setKernelPeerEndpoint(ctx context.Context, ifaceName, pubkey, endpoint string) error {
	bin := wgToolLookup()
	if bin == "" {
		return errWGToolMissing()
	}
	if err := wgToolRun(ctx, bin, "set", ifaceName, "peer", pubkey, "endpoint", endpoint); err != nil {
		return fmt.Errorf("wg set %s endpoint %s: %w", ifaceName, endpoint, err)
	}
	return nil
}

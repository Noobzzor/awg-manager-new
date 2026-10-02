package main

import (
	"path/filepath"

	"github.com/hoaxisr/awg-manager/internal/obfuscator"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
	"github.com/hoaxisr/awg-manager/internal/sys/kmod"
	"github.com/hoaxisr/awg-manager/internal/tunnel"
)

const (
	defaultLocalDataDir          = "/data"
	defaultLocalHTTPAddr         = "0.0.0.0:2222"
	defaultLocalSingboxBinary    = "/usr/local/bin/sing-box"
	defaultLocalSingboxConfigDir = "/data/sing-box/config.d"
	defaultLocalProxyAddr        = "127.0.0.1:1080"
)

type runtimeSettings struct {
	dataDir          string
	httpAddr         string
	singboxBinary    string
	singboxConfigDir string
	proxyAddr        string
}

func localRuntimeSettings(getenv func(string) string) runtimeSettings {
	dataDir := envOrDefault(getenv, "AWG_DATA_DIR", defaultLocalDataDir)
	configDir := getenv("SINGBOX_CONFIG_DIR")
	if configDir == "" {
		configDir = filepath.Join(dataDir, "sing-box", "config.d")
	}
	return runtimeSettings{
		dataDir:          dataDir,
		httpAddr:         envOrDefault(getenv, "AWG_HTTP_ADDR", defaultLocalHTTPAddr),
		singboxBinary:    envOrDefault(getenv, "SINGBOX_BIN", defaultLocalSingboxBinary),
		singboxConfigDir: configDir,
		proxyAddr:        envOrDefault(getenv, "AWG_PROXY_ADDR", defaultLocalProxyAddr),
	}
}

func envOrDefault(getenv func(string) string, key, fallback string) string {
	if value := getenv(key); value != "" {
		return value
	}
	return fallback
}

// applyDataDir перенацеливает пакеты, чьи каталоги производны от каталога
// данных. Пакеты держат их отдельными переменными (их же подменяют тесты), и
// без этой раздачи флаг `-data-dir` соблюдался наполовину: запуск «в
// песочнице» всё равно писал в /opt/etc/awg-manager.
//
// Сюда попадает то, что демон ПИШЕТ как свои данные и что умеет
// перенацеливаться. Осознанно не тронуты: бинари Entware (/opt/bin); белый
// список файлового редактора (`sys/files/sandbox.go`) — там боевой каталог
// зашит константой, и с нестандартным -data-dir корень «AWG Manager» в UI
// покажет чужой каталог; дерево sing-box (бинарь, config.d, cache.db) — его
// пути константны и требуют правки сигнатур.
//
// Под флагом работает и `--cleanup` (он удаляет туннели и конфиги релея из
// того же каталога). `--service` перенацеленных путей не читает вовсе.
func applyDataDir(dataDir string) {
	// Абсолютный путь обязателен: значение уезжает в тело шелл-скрипта хука
	// ndm, а его запускает роутер со своим рабочим каталогом.
	if abs, err := filepath.Abs(dataDir); err == nil {
		dataDir = abs
	}
	tunnel.ConfDir = dataDir
	obfuscator.ConfDir = filepath.Join(dataDir, "obfuscator")
	kmod.ModulesDir = filepath.Join(dataDir, "modules")
	router.SetDataDir(dataDir)

	if dataDir == defaultDataDir {
		return
	}
	// Дальше — только для НЕбоевого каталога. Хук ndm лежит вне каталога
	// данных, но его тело ссылается на перенацеленные файлы: оставив хук на
	// месте, песочница переписала бы боевой скрипт ссылками на /tmp, и после
	// её ухода ndm восстанавливал бы правила по мёртвым путям.
	router.SetNetfilterHookPath(filepath.Join(dataDir, "ndm-netfilter.d", "50-awgm-tproxy.sh"))
}

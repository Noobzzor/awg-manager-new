#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

failures=0
fail() { printf 'contract failure: %s\n' "$*" >&2; failures=$((failures + 1)); }
require() { grep -Fq -- "$2" "$1" || fail "$1 lacks: $2"; }

[ "$(grep -c '^FROM ' Dockerfile)" -eq 4 ] || fail 'Dockerfile must have exactly four stages including target verification'
require Dockerfile 'FROM --platform=$BUILDPLATFORM node:22-bookworm-slim@sha256:48e4b67d85f87bd551df43704e24d252f56cc5f8e9718841aace50f19948f0f9 AS frontend'
require Dockerfile 'npm ci'
require Dockerfile 'npm run check && npm test && npm run build'
require Dockerfile 'COPY internal/presets/defaults.json /src/internal/presets/defaults.json'
require Dockerfile 'go build -mod=vendor -trimpath -tags embed_frontend'
require Dockerfile 'ARG AMNEZIA_BOX_COMMIT=d231098f6f1a7bd90c690ecda1cf5e9176b32c2a'
require Dockerfile 'ARG CRONET_GO_COMMIT=45832ab074849607406baa3e3a2c4660274602ed'
require Dockerfile 'python3-requests'
require Dockerfile 'file git gnupg'
require Dockerfile 'case "$TARGETARCH" in amd64|arm64)'
require Dockerfile 'with_musl,with_awg,with_low_memory'
require Dockerfile 'EXPOSE 2222'
require Dockerfile 'install -d -o awg -g awg -m 0750 /data /data/sing-box /data/sing-box/config.d'
require Dockerfile 'FROM --platform=$TARGETPLATFORM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS singbox-verify'
require Dockerfile 'RUN /usr/local/bin/sing-box version | grep -F "sing-box version $AMNEZIA_BOX_VERSION"'
if grep -Fq '/out/sing-box version' Dockerfile; then
  fail 'builder must not execute the TARGETARCH sing-box binary on BUILDPLATFORM'
fi

require docker-compose.yml 'NET_ADMIN'
require docker-compose.yml 'SETUID'
require docker-compose.yml 'SETGID'
if grep -Fq -- 'CHOWN' docker-compose.yml; then
  fail 'Compose must not retain CHOWN capability'
fi
require docker-compose.yml '/dev/net/tun:/dev/net/tun'
require docker-compose.yml 'AWG_MODE: singbox'
require docker-compose.yml 'AWG_DATA_DIR: /data'
require docker-compose.yml 'AWG_HTTP_ADDR: 0.0.0.0:2222'
require docker-compose.yml 'SINGBOX_BIN: /usr/local/bin/sing-box'
require docker-compose.yml 'SINGBOX_CONFIG_DIR: /data/sing-box/config.d'
require docker-compose.yml 'AWG_PROXY_ADDR: 0.0.0.0:1080'
require docker-compose.yml '127.0.0.1:${AWG_HTTP_PORT:-2222}:2222'
require docker-compose.yml '127.0.0.1:${AWG_PROXY_PORT:-1080}:1080'
require docker-compose.yml 'http://127.0.0.1:2222/readyz'
if grep -Fq 'http://127.0.0.1:2222/healthz' docker-compose.yml; then
  fail 'Compose healthcheck must use readiness, not liveness'
fi
if grep -Eq '^[[:space:]]*privileged:' docker-compose.yml; then
  fail 'Compose must not enable privileged mode'
fi

require docker/entrypoint.sh '[ ! -c /dev/net/tun ]'
require docker/entrypoint.sh '"$SINGBOX_BIN" version >/dev/null 2>&1'
require docker/entrypoint.sh '/usr/bin/install -d -m 0750'
require docker/entrypoint.sh '--ambient-caps=+net_admin'
require docker/entrypoint.sh 'exec setpriv'

require cmd/awg-manager/paths.go 'AWG_PROXY_ADDR'
require cmd/awg-manager/wiring_local.go 'localProxyInbound'
require README.md 'AWG_PROXY_PORT'
require README.md 'AWG_PROXY_ADDR'
require README.md 'аутентификац'

if [ "$failures" -ne 0 ]; then
  printf 'docker static contracts: FAIL (%s)\n' "$failures" >&2
  exit 1
fi
printf 'docker static contracts: PASS\n'

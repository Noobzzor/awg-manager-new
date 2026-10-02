#!/bin/sh
set -eu

: "${AWG_DATA_DIR:=/data}"
: "${SINGBOX_BIN:=/usr/local/bin/sing-box}"
: "${SINGBOX_CONFIG_DIR:=$AWG_DATA_DIR/sing-box/config.d}"

case "$AWG_DATA_DIR:$SINGBOX_CONFIG_DIR:$SINGBOX_BIN" in
  /*:/*:/*) ;;
  *) echo "fatal: AWG_DATA_DIR, SINGBOX_CONFIG_DIR and SINGBOX_BIN must be absolute paths" >&2; exit 64 ;;
esac

if [ ! -c /dev/net/tun ] || [ ! -r /dev/net/tun ] || [ ! -w /dev/net/tun ]; then
  echo "fatal: /dev/net/tun must be mounted as a readable and writable character device" >&2
  exit 78
fi
if [ ! -x "$SINGBOX_BIN" ]; then
  echo "fatal: sing-box executable is unavailable at SINGBOX_BIN" >&2
  exit 78
fi

if [ "${AWG_GATEWAY_ENABLE:-false}" = "true" ]; then
  /usr/bin/install -d -m 0750 \
    "$AWG_DATA_DIR" "$AWG_DATA_DIR/sing-box" "$SINGBOX_CONFIG_DIR" /run/awg-manager
else
  setpriv --reuid=awg --regid=awg --init-groups -- \
    /usr/bin/install -d -m 0750 \
    "$AWG_DATA_DIR" "$AWG_DATA_DIR/sing-box" "$SINGBOX_CONFIG_DIR" /run/awg-manager
fi

# Preflight only: never print configuration or imported tunnel material.
if ! "$SINGBOX_BIN" version >/dev/null 2>&1; then
  echo "fatal: sing-box version preflight failed" >&2
  exit 78
fi

# Server-side WireGuard/AWG ingress needs to create and configure a kernel
# interface. Keep the portable default unprivileged; gateway mode is an
# explicit opt-in and retains the container's narrowly-scoped NET_ADMIN cap.
if [ "${AWG_GATEWAY_ENABLE:-false}" = "true" ]; then
  exec "$@"
fi

exec setpriv \
  --reuid=awg --regid=awg --init-groups \
  --inh-caps=+net_admin --ambient-caps=+net_admin \
  -- "$@"

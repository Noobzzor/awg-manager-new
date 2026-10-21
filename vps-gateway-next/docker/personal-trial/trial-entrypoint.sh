#!/bin/sh
set -eu
: "${AWG_GATEWAY_PUBLIC_ENDPOINT:?Set AWG_GATEWAY_PUBLIC_ENDPOINT to the server address and UDP port}"
if [ "${AWG_GATEWAY_ENABLE:-}" != true ] || [ "${AWG_GATEWAY_BACKEND:-}" != userspace-awg ]; then
  echo 'fatal: this trial requires the userspace AWG gateway backend' >&2
  exit 78
fi
if ! sha256sum -c /usr/local/share/awgm-trial/SHA256SUMS >/dev/null 2>&1; then
  echo 'fatal: trial binary integrity check failed' >&2
  exit 78
fi
exec /usr/local/bin/entrypoint.sh "$@"

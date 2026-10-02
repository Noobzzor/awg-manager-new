#!/usr/bin/env bash
set -Eeuo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
AWG_GATEWAY_TEST_MANAGER_RESTART=1 exec bash "$SCRIPT_DIR/test-gateway-vpn-egress-packet.sh"

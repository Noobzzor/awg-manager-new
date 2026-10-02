#!/usr/bin/env bash
# Verify real WireGuard egress stops when the server removes an active peer.
# Uses isolated, disposable Docker containers and tmpfs only; no project volumes.
set -Eeuo pipefail

IMAGE="${AWG_GATEWAY_TEST_IMAGE:-awg-manager:2.19.0}"
TEST_ID="$(python -c 'import uuid; print(uuid.uuid4().hex[:10])')"
NETWORK="awgm-peer-isolation-${TEST_ID}"
GATEWAY="awgm-peer-isolation-gw-${TEST_ID}"
CLIENT="awgm-peer-isolation-client-${TEST_ID}"
PORT="${AWG_GATEWAY_TEST_PORT:-51824}"
CLIENT_ADDRESS="10.66.0.254/32"
PUBLIC_IP_CHECK="api.ipify.org"

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  printf 'Required local image not found: %s\n' "$IMAGE" >&2
  exit 2
fi
if docker container inspect "$GATEWAY" >/dev/null 2>&1 || docker container inspect "$CLIENT" >/dev/null 2>&1; then
  printf 'Refusing to touch a pre-existing test container.\n' >&2
  exit 2
fi
if docker network inspect "$NETWORK" >/dev/null 2>&1; then
  printf 'Refusing to touch a pre-existing test network.\n' >&2
  exit 2
fi

cleanup() {
  docker rm -f "$CLIENT" "$GATEWAY" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker network create "$NETWORK" >/dev/null
docker run -d --name "$GATEWAY" --network "$NETWORK" \
  --cap-drop ALL --cap-add NET_ADMIN --cap-add DAC_OVERRIDE --cap-add SETUID --cap-add SETGID \
  --device /dev/net/tun --sysctl net.ipv4.ip_forward=1 \
  --tmpfs /data:rw,size=64m,mode=0750,uid=10001,gid=10001 \
  --tmpfs /run:rw,size=16m,mode=0750,uid=10001,gid=10001 \
  --tmpfs /tmp:rw,size=64m,mode=1777 \
  -e AWG_GATEWAY_ENABLE=true \
  -e AWG_GATEWAY_INTERFACE=awg0 \
  -e AWG_GATEWAY_TOOL=/usr/bin/wg \
  -e AWG_GATEWAY_WAN_INTERFACE=eth0 \
  -e AWG_GATEWAY_PORT=51820 \
  -e AWG_GATEWAY_CLIENT_POOL=10.66.0.0/24 \
  -e AWG_GATEWAY_SERVER_ADDRESS=10.66.0.1/24 \
  -e AWG_HTTP_ADDR=0.0.0.0:2222 \
  "$IMAGE" >/dev/null

docker run -d --name "$CLIENT" --network "$NETWORK" \
  --cap-drop ALL --cap-add NET_ADMIN --device /dev/net/tun \
  --tmpfs /run:rw,size=16m,mode=0750 \
  --entrypoint /bin/sh "$IMAGE" -c 'sleep 600' >/dev/null

ready=0
for _ in $(seq 1 40); do
  if docker exec "$GATEWAY" curl -fsS http://127.0.0.1:2222/readyz >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "$ready" != 1 ]]; then
  printf 'Gateway test container did not become ready.\n' >&2
  docker logs "$GATEWAY" >&2
  exit 1
fi

GATEWAY_IP="$(docker inspect -f "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}" "$GATEWAY")"
SERVER_PUBLIC="$(docker exec "$GATEWAY" /usr/bin/wg show awg0 public-key)"
CLIENT_PUBLIC="$(docker exec "$CLIENT" sh -ec 'umask 077; wg genkey > /run/client.key; wg pubkey < /run/client.key')"
if [[ -z "$GATEWAY_IP" || -z "$SERVER_PUBLIC" || -z "$CLIENT_PUBLIC" ]]; then
  printf 'Could not initialize ephemeral test peer.\n' >&2
  exit 1
fi

docker exec "$GATEWAY" /usr/bin/wg set awg0 peer "$CLIENT_PUBLIC" \
  allowed-ips "$CLIENT_ADDRESS"
docker exec "$CLIENT" sh -ec '
  ip link add wg-test0 type wireguard
  ip address add "$1" dev wg-test0
  wg set wg-test0 private-key /run/client.key peer "$2" \
    endpoint "$3:51820" allowed-ips 0.0.0.0/0 persistent-keepalive 5
  ip link set wg-test0 up
  ip route replace default dev wg-test0
' sh "$CLIENT_ADDRESS" "$SERVER_PUBLIC" "$GATEWAY_IP"

PUBLIC_IP="$(docker exec "$CLIENT" getent ahostsv4 "$PUBLIC_IP_CHECK" | awk 'NR == 1 {print $1}')"
if [[ -z "$PUBLIC_IP" ]]; then
  printf 'Could not resolve the public-IP test endpoint before enabling the tunnel.\n' >&2
  exit 1
fi

ACTIVE_EXIT_IP="$(docker exec "$CLIENT" curl -4fsS --connect-timeout 5 --max-time 12 \
  --resolve "$PUBLIC_IP_CHECK:443:$PUBLIC_IP" "https://$PUBLIC_IP_CHECK")"
if [[ ! "$ACTIVE_EXIT_IP" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'Active peer did not return a valid IPv4 egress address.\n' >&2
  exit 1
fi

PEER_HANDSHAKE="$(docker exec "$CLIENT" /usr/bin/wg show wg-test0 latest-handshakes | awk 'NR == 1 {print $2}')"
if [[ ! "$PEER_HANDSHAKE" =~ ^[1-9][0-9]*$ ]]; then
  printf 'WireGuard handshake was not established.\n' >&2
  exit 1
fi

# This is the exact peer-removal operation used by CommandApplier.RemovePeer.
docker exec "$GATEWAY" /usr/bin/wg set awg0 peer "$CLIENT_PUBLIC" remove
REMAINING_PEERS="$(docker exec "$GATEWAY" /usr/bin/wg show awg0 peers)"
if [[ -n "$REMAINING_PEERS" ]]; then
  printf 'Removed peer is still present on the gateway interface.\n' >&2
  exit 1
fi

if docker exec "$CLIENT" curl -4fsS --connect-timeout 3 --max-time 6 \
  --resolve "$PUBLIC_IP_CHECK:443:$PUBLIC_IP" "https://$PUBLIC_IP_CHECK" >/dev/null 2>&1; then
  printf 'FAIL: client traffic still reached the public network after peer removal.\n' >&2
  exit 1
fi

printf 'PASS: live handshake and public egress verified; peer removal stopped egress.\n'

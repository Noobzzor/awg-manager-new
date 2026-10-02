#!/usr/bin/env bash
# Prove a client packet reaches a local fixture only through an imported AWG endpoint.
# All networks are Docker-internal; all state is disposable tmpfs; no host ports/volumes.
set -Eeuo pipefail

IMAGE="${AWG_GATEWAY_TEST_IMAGE:-awg-manager:gateway-packet-test-20260929-dns-order-r1}"
TEST_DESTINATION="203.0.113.10"
DIRECT_TEST_DESTINATION="203.0.113.11"
ID="$(python -c 'import uuid; print(uuid.uuid4().hex[:10])')"
TEST_MANAGER_RESTART="${AWG_GATEWAY_TEST_MANAGER_RESTART:-0}"
OUTER_NET="awgm-vpn-outer-${ID}"
CLIENT_NET="awgm-vpn-client-${ID}"
EXIT_NET="awgm-vpn-exit-${ID}"
GATEWAY="awgm-vpn-gateway-${ID}"
CLIENT="awgm-vpn-client-${ID}"
CLIENT2="awgm-vpn-client2-${ID}"
VPN_EXIT="awgm-vpn-exit-${ID}"
SINK="awgm-vpn-sink-${ID}"
DIRECT_SINK="awgm-vpn-direct-sink-${ID}"

GW_ALIAS="awg-vpn-gateway-${ID}"
CLIENT_ID="vpn-packet-${ID}"
CLIENT2_ID="vpn-packet-${ID}-other"
PROFILE_ID="vpn-packet-${ID}"
DIRECT_PATH="direct-control-${ID}"
DIRECT_SUCCESS_PATH="direct-success-${ID}"
VPN_PATH="via-vpn-${ID}"
SPOOF_PATH="spoofed-client-${ID}"
VPN_TAG="vpn-${ID}"
CREATED_NETWORK_IDS=()
CREATED_CONTAINER_IDS=()

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}
[[ "$TEST_MANAGER_RESTART" == 0 || "$TEST_MANAGER_RESTART" == 1 ]] || fail "AWG_GATEWAY_TEST_MANAGER_RESTART must be 0 or 1"

cleanup() {
  result=$?
  trap - EXIT
  local resource
  # IDs bind cleanup to resources this run created, even if a name is reused.
  for resource in "${CREATED_CONTAINER_IDS[@]}"; do docker rm -f "$resource" >/dev/null 2>&1 || true; done
  for resource in "${CREATED_NETWORK_IDS[@]}"; do docker network rm "$resource" >/dev/null 2>&1 || true; done
  exit "$result"
}
trap cleanup EXIT

if ! IMAGE_ID="$(docker image inspect -f '{{.Id}}' "$IMAGE" 2>/dev/null)" || [[ -z "$IMAGE_ID" ]]; then
  fail "local test image not found: $IMAGE"
fi
if ! BUSYBOX_IMAGE_ID="$(docker image inspect -f '{{.Id}}' busybox:latest 2>/dev/null)" || [[ -z "$BUSYBOX_IMAGE_ID" ]]; then
  fail "local BusyBox fixture image not found; refusing to pull an image"
fi
for name in "$CLIENT" "$CLIENT2" "$GATEWAY" "$VPN_EXIT" "$SINK" "$DIRECT_SINK"; do
  if docker container inspect "$name" >/dev/null 2>&1; then
    fail "refusing to touch pre-existing container $name"
  fi
done
for name in "$CLIENT_NET" "$OUTER_NET" "$EXIT_NET"; do
  if docker network inspect "$name" >/dev/null 2>&1; then
    fail "refusing to touch pre-existing network $name"
  fi
done

# All three bridges are internal: Gateway can reach the VPN peer's outer
# interface, but only that peer is attached to the fixture's separate exit net.
CREATED_NETWORK_IDS+=("$(docker network create --internal "$OUTER_NET")")
CREATED_NETWORK_IDS+=("$(docker network create --internal "$CLIENT_NET")")
CREATED_NETWORK_IDS+=("$(docker network create --internal "$EXIT_NET")")
for name in "$OUTER_NET" "$CLIENT_NET" "$EXIT_NET"; do
  [[ "$(docker network inspect -f '{{.Internal}}' "$name")" == true ]] || fail "$name is not internal"
done

CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$SINK" --network "$EXIT_NET" \
  --tmpfs /www:rw,size=1m \
  --entrypoint /bin/sh "$BUSYBOX_IMAGE_ID" \
  -ec 'exec httpd -vv -f -p 8080 -h /www')")
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$DIRECT_SINK" --network "$OUTER_NET" \
  --tmpfs /www:rw,size=1m \
  --entrypoint /bin/sh "$BUSYBOX_IMAGE_ID" \
  -ec 'exec httpd -vv -f -p 8080 -h /www')")

CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$VPN_EXIT" --network "$OUTER_NET" \
  --cap-drop ALL --cap-add NET_ADMIN --sysctl net.ipv4.ip_forward=1 \
  --tmpfs /data:rw,size=16m,mode=0750 \
  --tmpfs /run:rw,size=8m --tmpfs /tmp:rw,size=8m \
  --entrypoint /bin/sh "$IMAGE_ID" -ec 'sleep 1800')")
docker network connect "$EXIT_NET" "$VPN_EXIT"

GATEWAY_COMMAND='exec /usr/local/bin/entrypoint.sh /usr/local/bin/awg-manager'
if [[ "$TEST_MANAGER_RESTART" == 1 ]]; then
  GATEWAY_COMMAND='manager_launches=0
manager_pid=
stop_gateway_manager() {
  if [ -n "$manager_pid" ]; then
    kill -TERM "$manager_pid" 2>/dev/null || true
    wait "$manager_pid" 2>/dev/null || true
  fi
}
trap "stop_gateway_manager; exit 0" TERM INT
while [ "$manager_launches" -lt 3 ]; do
  manager_launches=$((manager_launches + 1))
  printf "%s\n" "$manager_launches" >/tmp/awg-manager-launch-count
  /usr/local/bin/entrypoint.sh /usr/local/bin/awg-manager &
  manager_pid=$!
  if wait "$manager_pid"; then manager_status=0; else manager_status=$?; fi
  manager_pid=
  if [ "$manager_launches" -ge 3 ]; then exit "$manager_status"; fi
  sleep 0.2
done
exit 1'
fi
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$GATEWAY" --network "$OUTER_NET" \
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
  -e AWG_GATEWAY_PUBLIC_ENDPOINT="${GW_ALIAS}:51820" \
  -e AWG_HTTP_ADDR=0.0.0.0:2222 \
  --entrypoint /bin/sh "$IMAGE_ID" -ec \
  "$GATEWAY_COMMAND")")
docker network connect --alias "$GW_ALIAS" "$CLIENT_NET" "$GATEWAY"
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$CLIENT" --network "$CLIENT_NET" \
  --cap-drop ALL --cap-add NET_ADMIN --device /dev/net/tun \
  --tmpfs /data:rw,size=16m,mode=0750 \
  --tmpfs /run:rw,size=16m --tmpfs /tmp:rw,size=8m \
  --entrypoint /bin/sh "$IMAGE_ID" -ec 'sleep 1800')")
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$CLIENT2" --network "$CLIENT_NET" \
  --cap-drop ALL --cap-add NET_ADMIN --device /dev/net/tun \
  --tmpfs /data:rw,size=16m,mode=0750 \
  --tmpfs /run:rw,size=16m --tmpfs /tmp:rw,size=8m \
  --entrypoint /bin/sh "$IMAGE_ID" -ec 'sleep 1800')")

# Keep the upstream peer's outer endpoint literal and the fixture on a network
# that the Gateway container never joins.
network_ip() {
  docker inspect "$1" | python -c 'import json,sys; print(json.load(sys.stdin)[0]["NetworkSettings"]["Networks"][sys.argv[1]]["IPAddress"])' "$2"
}
VPN_OUTER_IP="$(network_ip "$VPN_EXIT" "$OUTER_NET")"
SINK_IP="$(network_ip "$SINK" "$EXIT_NET")"
DIRECT_SINK_IP="$(network_ip "$DIRECT_SINK" "$OUTER_NET")"
[[ -n "$VPN_OUTER_IP" && -n "$SINK_IP" && -n "$DIRECT_SINK_IP" ]] || fail "could not resolve isolated peer/fixture addresses"
EXIT_IFACE="$(docker exec "$VPN_EXIT" sh -ec 'route="$(ip route get "$1")"; set -- $route; while [ "$#" -gt 0 ]; do if [ "$1" = dev ]; then shift; printf "%s" "$1"; exit 0; fi; shift; done; exit 1' sh "$SINK_IP")"
[[ -n "$EXIT_IFACE" && "$EXIT_IFACE" != eth0 ]] || fail "VPN peer exit interface is not isolated from its outer interface"

# Generate ephemeral WireGuard keys in the containers. Only public keys leave
# their owner container; private keys stay in tmpfs and are never printed.
VPN_SERVER_PUBLIC="$(docker exec "$VPN_EXIT" sh -ec 'umask 077; wg genkey >/run/server.key; wg pubkey </run/server.key')"
VPN_CLIENT_PUBLIC="$(docker exec "$GATEWAY" sh -ec 'umask 077; wg genkey >/run/vpn-client.key; wg pubkey </run/vpn-client.key')"
[[ -n "$VPN_SERVER_PUBLIC" && -n "$VPN_CLIENT_PUBLIC" ]] || fail "could not generate disposable VPN keys"
docker exec "$VPN_EXIT" sh -ec \
  'ip link add wg0 type wireguard; ip addr add 10.99.0.1/24 dev wg0; wg set wg0 private-key /run/server.key listen-port 51820 peer "$1" allowed-ips 10.99.0.2/32,10.66.0.0/24; ip link set wg0 up' \
  sh "$VPN_CLIENT_PUBLIC"

# Route only the test destination through the synthetic VPN exit to the local
# fixture, and provide a conntrack return path. This is inside VPN_EXIT only.
docker exec "$VPN_EXIT" sh -ec \
  'iptables -P FORWARD DROP; iptables -A FORWARD -i wg0 -o "$1" -p tcp -d "$2" --dport 8080 -m conntrack --ctstate NEW,ESTABLISHED -j ACCEPT; iptables -A FORWARD -i "$1" -o wg0 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT; iptables -t nat -A PREROUTING -i wg0 -d "$3" -p tcp --dport 8080 -j DNAT --to-destination "$2:8080"; iptables -t nat -A POSTROUTING -o "$1" -d "$2" -p tcp --dport 8080 -j MASQUERADE' \
  sh "$EXIT_IFACE" "$SINK_IP" "$TEST_DESTINATION"
docker exec "$SINK" sh -ec 'printf "fixture:%s" "$1" >"/www/$2"; dd if=/dev/zero bs=1024 count=32 >>"/www/$2" 2>/dev/null' sh "$ID" "$VPN_PATH"

# Direct traffic to a separate synthetic destination is DNATed only on the
# Gateway container's output path. The VPN fixture remains unreachable there.
DIRECT_FIXTURE_RULESET="table ip awgm_direct_fixture {
 chain output {
  type nat hook output priority -100; policy accept;
  ip daddr $DIRECT_TEST_DESTINATION tcp dport 8080 dnat to $DIRECT_SINK_IP:8080
 }
}"
printf '%s' "$DIRECT_FIXTURE_RULESET" | docker exec -i "$GATEWAY" nft -c -f -
printf '%s' "$DIRECT_FIXTURE_RULESET" | docker exec -i "$GATEWAY" nft -f -
docker exec "$DIRECT_SINK" sh -ec 'printf "direct:%s" "$1" >"/www/$2"; dd if=/dev/zero bs=1024 count=32 >>"/www/$2" 2>/dev/null' sh "$ID" "$DIRECT_SUCCESS_PATH"

# Assert disposable containers have no published host ports or Docker volumes.
for container in "$GATEWAY" "$CLIENT" "$CLIENT2" "$VPN_EXIT" "$SINK" "$DIRECT_SINK"; do
  [[ -z "$(docker port "$container")" ]] || fail "$container unexpectedly publishes host ports"
  mounts="$(docker inspect -f '{{range .Mounts}}{{.Type}}:{{.Destination}} {{end}}' "$container")"
  [[ "$mounts" != *volume* ]] || fail "$container unexpectedly uses a Docker volume"
done

ready=0
for _ in $(seq 1 60); do
  if docker exec "$GATEWAY" curl -fsS --max-time 2 http://127.0.0.1:2222/readyz >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[[ "$ready" == 1 ]] || fail "gateway did not become ready"

applied_hash() {
  docker exec "$GATEWAY" sha256sum /var/run/awg-manager/singbox-applied.json 2>/dev/null \
    | cut -d ' ' -f 1 || true
}
wait_for_apply() {
  local before="$1" current
  for _ in $(seq 1 60); do
    current="$(applied_hash)"
    if [[ -n "$current" && "$current" != "$before" ]] && \
       docker exec "$GATEWAY" ip link show awgm0 >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}
create_client() {
  local client_id="$1"
  docker exec "$GATEWAY" curl -fsS --max-time 5 \
    -H 'Content-Type: application/json' -X POST \
    --data-binary "{\"id\":\"$client_id\",\"label\":\"Local VPN packet test\"}" \
    http://127.0.0.1:2222/api/gateway/clients/create >/dev/null
}
put_multi_client_profile() {
  local body
  body="$(printf '{\"id\":\"%s\",\"name\":\"Local multi-client packet test\",\"defaultAction\":\"block\",\"rules\":[{\"id\":\"client-vpn\",\"action\":\"vpn\",\"outbound\":\"%s\",\"priority\":10,\"clientIds\":[\"%s\"],\"ports\":[8080],\"enabled\":true},{\"id\":\"client-direct\",\"action\":\"direct\",\"outbound\":\"direct\",\"priority\":10,\"clientIds\":[\"%s\"],\"ports\":[8080],\"enabled\":true}]}' \
    "$PROFILE_ID" "$VPN_TAG" "$CLIENT_ID" "$CLIENT2_ID")"
  docker exec "$GATEWAY" curl -fsS --max-time 5 \
    -H 'Content-Type: application/json' -X PUT --data-binary "$body" \
    "http://127.0.0.1:2222/api/gateway/policies/profiles/$PROFILE_ID" >/dev/null
}
create_profile() {
  local body
  body="$(printf '{\"id\":\"%s\",\"name\":\"Local VPN packet test\",\"defaultAction\":\"direct\"}' "$PROFILE_ID")"
  docker exec "$GATEWAY" curl -fsS --max-time 5 \
    -H 'Content-Type: application/json' -X POST --data-binary "$body" \
    http://127.0.0.1:2222/api/gateway/policies/profiles/create >/dev/null
}
apply_profile() {
  docker exec "$GATEWAY" curl -fsS --max-time 5 -X POST \
    "http://127.0.0.1:2222/api/gateway/policies/profiles/$PROFILE_ID/apply" >/dev/null
}
configure_client() {
  local client_container="$1" client_id="$2"
  docker exec "$GATEWAY" curl -fsS --max-time 5 \
    "http://127.0.0.1:2222/api/gateway/clients/$client_id/config" |
    docker exec -i "$client_container" sh -ec '
      umask 077
      cat > /run/awg-client.conf
      client_address="$(awk -F " = " "/^Address = / { print \$2; exit }" /run/awg-client.conf)"
      gateway_ip="$(getent ahostsv4 "$1" | awk "NR == 1 { print \$1 }")"
      test -n "$client_address" && test -n "$gateway_ip"
      ip route add "$gateway_ip/32" dev eth0
      ip link add awg-client type wireguard
      wg-quick strip /run/awg-client.conf | wg setconf awg-client /dev/stdin
      ip address add "$client_address" dev awg-client
      ip link set mtu 1420 up dev awg-client
      ip route replace default dev awg-client
      rm -f /run/awg-client.conf
    ' sh "$GW_ALIAS"
}
wait_for_client_handshake() {
  local client_container="$1"
  local handshake=0 timestamp
  for _ in $(seq 1 40); do
    timestamp="$(docker exec "$client_container" wg show awg-client latest-handshakes 2>/dev/null | awk 'NR == 1 {print $2}')"
    if [[ "$timestamp" =~ ^[1-9][0-9]*$ ]]; then handshake=1; break; fi
    sleep 0.5
  done
  [[ "$handshake" == 1 ]] || fail "WireGuard client-to-Gateway handshake did not complete"
}
server_counters() {
  local values
  values="$(docker exec "$VPN_EXIT" wg show wg0 transfer)"
  local peer received sent
  read -r peer received sent <<<"$values"
  printf '%s %s\n' "$received" "$sent"
}
fixture_hit() {
  docker logs "$SINK" 2>&1 | grep -Fq "GET /$1"
}
direct_fixture_hit() {
  docker logs "$DIRECT_SINK" 2>&1 | grep -Fq "GET /$1"
}
client_address() {
  docker exec "$1" ip -4 -o addr show dev awg-client | awk '$3 == "inet" {split($4, address, "/"); print address[1]; exit}'
}
client_tx_bytes() {
  docker exec "$1" wg show awg-client transfer | awk 'NR == 1 { print $3 }'
}
manager_launch_count() {
  docker exec "$GATEWAY" sh -ec '
    if [ -r /tmp/awg-manager-launch-count ]; then
      read -r count </tmp/awg-manager-launch-count
      printf "%s" "$count"
    else
      printf 0
    fi
  ' 2>/dev/null || printf 0
}
client_config_identity() {
  local client_id="$1"
  docker exec "$GATEWAY" curl -fsS --max-time 5 \
    "http://127.0.0.1:2222/api/gateway/clients/$client_id/config" |
    python -c 'import sys; print("\n".join(line for line in sys.stdin.read().splitlines() if line.startswith(("Address = ", "PublicKey = "))))'
}

create_client "$CLIENT_ID"
create_client "$CLIENT2_ID"
configure_client "$CLIENT" "$CLIENT_ID"
configure_client "$CLIENT2" "$CLIENT2_ID"
wait_for_client_handshake "$CLIENT"
wait_for_client_handshake "$CLIENT2"
CLIENT_ADDRESS="$(client_address "$CLIENT")"
CLIENT2_ADDRESS="$(client_address "$CLIENT2")"
[[ -n "$CLIENT_ADDRESS" && -n "$CLIENT2_ADDRESS" && "$CLIENT_ADDRESS" != "$CLIENT2_ADDRESS" ]] || fail "Gateway clients do not have distinct tunnel addresses"

# Import exactly one AWG endpoint; API payload and private key remain inside the
# disposable Gateway container. The only VPN peer is the local test server.
docker exec "$GATEWAY" sh -ec \
  'umask 077; printf '\''{"tag":"%s","config":{"type":"awg","tag":"%s","useIntegratedTun":false,"private_key":"%s","address":["10.99.0.2/32"],"peers":[{"address":"%s","port":51820,"public_key":"%s","allowed_ips":["0.0.0.0/0"]}]}}\n'\'' "$1" "$1" "$(cat /run/vpn-client.key)" "$2" "$3" >/run/awg3-import.json; curl -fsS --max-time 8 -H '\''Content-Type: application/json'\'' -X POST --data-binary @/run/awg3-import.json http://127.0.0.1:2222/api/awg3-endpoints >/dev/null; rm -f /run/awg3-import.json /run/vpn-client.key' \
  sh "vpn-${ID}" "$VPN_OUTER_IP" "$VPN_SERVER_PUBLIC"

# DIRECT is a VPN-only reachability control, not a general DIRECT-outbound
# proof: this TEST-NET target is available only behind the isolated VPN exit.
create_profile
HASH_BEFORE="$(applied_hash)"
apply_profile
wait_for_apply "$HASH_BEFORE" || fail "DIRECT control reload was not observed"
if docker exec "$CLIENT" curl -4fsS --connect-timeout 2 --max-time 5 \
  "http://$TEST_DESTINATION:8080/$DIRECT_PATH" >/dev/null 2>&1; then
  fail "DIRECT control unexpectedly received a response"
fi
if fixture_hit "$DIRECT_PATH"; then fail "DIRECT control reached the fixture"; fi
printf 'PASS: DIRECT policy could not bypass the VPN-only isolated path.\n'

# Keep both client-specific rules active together, then send DIRECT and VPN
# requests concurrently and verify their distinct replies return to the right client.
put_multi_client_profile
HASH_BEFORE="$(applied_hash)"
apply_profile
wait_for_apply "$HASH_BEFORE" || fail "two-client policy reload was not observed"
BASE_COUNTS="$(server_counters)"
read -r BASE_RX BASE_TX <<<"$BASE_COUNTS"
[[ "$BASE_RX" =~ ^[0-9]+$ && "$BASE_TX" =~ ^[0-9]+$ ]] || fail "invalid baseline VPN counters"
DIRECT_EXPECTED_HASH="$(docker exec "$DIRECT_SINK" sha256sum "/www/$DIRECT_SUCCESS_PATH" | awk '{print $1}')"
VPN_EXPECTED_HASH="$(docker exec "$SINK" sha256sum "/www/$VPN_PATH" | awk '{print $1}')"
[[ -n "$DIRECT_EXPECTED_HASH" && -n "$VPN_EXPECTED_HASH" && "$DIRECT_EXPECTED_HASH" != "$VPN_EXPECTED_HASH" ]] || fail "could not establish distinct fixture response hashes"
START_AT=$(( $(date +%s) + 10 ))
docker exec "$CLIENT2" sh -ec \
  'printf "%s" "$1" >/run/direct-ready; while [ "$(date +%s)" -lt "$1" ]; do sleep 0.02; done; curl -4fsS --limit-rate 8k --connect-timeout 4 --max-time 20 "http://$2:8080/$3" >/run/direct-response' \
  sh "$START_AT" "$DIRECT_TEST_DESTINATION" "$DIRECT_SUCCESS_PATH" &
DIRECT_PID=$!
docker exec "$CLIENT" sh -ec \
  'printf "%s" "$1" >/run/vpn-ready; while [ "$(date +%s)" -lt "$1" ]; do sleep 0.02; done; curl -4fsS --limit-rate 8k --connect-timeout 4 --max-time 20 "http://$2:8080/$3" >/run/vpn-response' \
  sh "$START_AT" "$TEST_DESTINATION" "$VPN_PATH" &
VPN_PID=$!
flows_ready=0
for _ in $(seq 1 50); do
  if docker exec "$CLIENT2" test -s /run/direct-ready && docker exec "$CLIENT" test -s /run/vpn-ready; then
    flows_ready=1
    break
  fi
  sleep 0.1
done
[[ "$flows_ready" == 1 && "$(date +%s)" -lt "$START_AT" ]] || fail "both client flows were not armed before the shared start barrier"
while [[ "$(date +%s)" -lt "$START_AT" ]]; do sleep 0.05; done
flows_overlapped=0
for _ in $(seq 1 40); do
  if docker exec "$CLIENT2" pgrep -x curl >/dev/null 2>&1 && docker exec "$CLIENT" pgrep -x curl >/dev/null 2>&1; then
    flows_overlapped=1
    break
  fi
  sleep 0.05
done
[[ "$flows_overlapped" == 1 ]] || fail "DIRECT and VPN curl processes did not overlap"
direct_rc=0
vpn_rc=0
wait "$DIRECT_PID" || direct_rc=$?
wait "$VPN_PID" || vpn_rc=$?
if [[ "$direct_rc" != 0 || "$vpn_rc" != 0 ]]; then
  printf 'Diagnostic: direct_rc=%s vpn_rc=%s\n' "$direct_rc" "$vpn_rc" >&2
  if direct_fixture_hit "$DIRECT_SUCCESS_PATH"; then printf 'Diagnostic: DIRECT fixture observed the request.\n' >&2; else printf 'Diagnostic: DIRECT fixture did not observe the request.\n' >&2; fi
  if fixture_hit "$VPN_PATH"; then printf 'Diagnostic: VPN fixture observed the request.\n' >&2; else printf 'Diagnostic: VPN fixture did not observe the request.\n' >&2; fi
  docker exec "$GATEWAY" nft list table ip awgm_direct_fixture >&2 || true
  docker exec "$GATEWAY" nft list table inet awgm_gateway >&2 || true
fi
[[ "$direct_rc" == 0 ]] || fail "DIRECT client did not receive its concurrent fixture response"
[[ "$vpn_rc" == 0 ]] || fail "VPN client did not receive its concurrent fixture response"
DIRECT_RESPONSE_HASH="$(docker exec "$CLIENT2" sha256sum /run/direct-response | awk '{print $1}')"
VPN_RESPONSE_HASH="$(docker exec "$CLIENT" sha256sum /run/vpn-response | awk '{print $1}')"
[[ "$DIRECT_RESPONSE_HASH" == "$DIRECT_EXPECTED_HASH" ]] || fail "DIRECT reply did not match the complete DIRECT fixture body"
[[ "$VPN_RESPONSE_HASH" == "$VPN_EXPECTED_HASH" ]] || fail "VPN reply did not match the complete VPN fixture body"
direct_fixture_hit "$DIRECT_SUCCESS_PATH" || fail "DIRECT client packet missed its isolated fixture"
fixture_hit "$VPN_PATH" || fail "VPN client packet missed its isolated fixture"

COUNTS="$(server_counters)"
read -r RX TX <<<"$COUNTS"
[[ "$RX" =~ ^[0-9]+$ && "$TX" =~ ^[0-9]+$ ]] || fail "invalid final VPN counters"
(( RX > BASE_RX && TX > BASE_TX )) || fail "VPN server WireGuard RX/TX counters did not both increase"
printf 'PASS: concurrent DIRECT/VPN flows returned distinct fixture replies to their assigned clients.\n'

# Forge the other client's in-pool source address in each direction. The
# client-side WireGuard TX counter proves each negative packet was emitted;
# neither destination policy's fixture may observe it.
CLIENT1_TX_BEFORE="$(client_tx_bytes "$CLIENT")"
[[ "$CLIENT1_TX_BEFORE" =~ ^[0-9]+$ ]] || fail "invalid client-one WireGuard TX baseline"
docker exec "$CLIENT" ip address add "$CLIENT2_ADDRESS/32" dev awg-client
if docker exec "$CLIENT" curl -4fsS --interface "$CLIENT2_ADDRESS" --connect-timeout 2 --max-time 5 \
  "http://$DIRECT_TEST_DESTINATION:8080/$SPOOF_PATH" >/dev/null 2>&1; then
  fail "client one spoofing client two unexpectedly received a response"
fi
docker exec "$CLIENT" ip address del "$CLIENT2_ADDRESS/32" dev awg-client
CLIENT1_TX_AFTER="$(client_tx_bytes "$CLIENT")"
[[ "$CLIENT1_TX_AFTER" =~ ^[0-9]+$ ]] && (( CLIENT1_TX_AFTER > CLIENT1_TX_BEFORE )) || fail "client-one spoof packet was not emitted on WireGuard"
if direct_fixture_hit "$SPOOF_PATH"; then fail "client-one spoof reached client-two DIRECT policy fixture"; fi

CLIENT2_TX_BEFORE="$(client_tx_bytes "$CLIENT2")"
[[ "$CLIENT2_TX_BEFORE" =~ ^[0-9]+$ ]] || fail "invalid client-two WireGuard TX baseline"
SPOOF_VPN_PATH="${SPOOF_PATH}-vpn"
docker exec "$CLIENT2" ip address add "$CLIENT_ADDRESS/32" dev awg-client
if docker exec "$CLIENT2" curl -4fsS --interface "$CLIENT_ADDRESS" --connect-timeout 2 --max-time 5 \
  "http://$TEST_DESTINATION:8080/$SPOOF_VPN_PATH" >/dev/null 2>&1; then
  fail "client two spoofing client one unexpectedly received a response"
fi
docker exec "$CLIENT2" ip address del "$CLIENT_ADDRESS/32" dev awg-client
CLIENT2_TX_AFTER="$(client_tx_bytes "$CLIENT2")"
[[ "$CLIENT2_TX_AFTER" =~ ^[0-9]+$ ]] && (( CLIENT2_TX_AFTER > CLIENT2_TX_BEFORE )) || fail "client-two spoof packet was not emitted on WireGuard"
if fixture_hit "$SPOOF_VPN_PATH"; then fail "client-two spoof reached client-one VPN policy fixture"; fi
printf 'PASS: both peers emitted in-pool source-spoof attempts; neither crossed into the other client policy.\n'
if [[ "$TEST_MANAGER_RESTART" == 1 ]]; then
  RESTART_PATH="via-vpn-after-manager-restart-${ID}"
  docker exec "$SINK" sh -ec 'printf "fixture:%s\n" "$1" >"/www/$2"' sh "$ID" "$RESTART_PATH"
  LAUNCHES_BEFORE="$(manager_launch_count)"
  [[ "$LAUNCHES_BEFORE" == 1 ]] || fail "manager supervisor launch count before restart=$LAUNCHES_BEFORE, want 1"
  CLIENT_IDENTITY_BEFORE="$(client_config_identity "$CLIENT_ID")"
  [[ -n "$CLIENT_IDENTITY_BEFORE" ]] || fail "could not read safe client identity before manager restart"
  MANAGER_PID="$(docker exec "$GATEWAY" pgrep -xo awg-manager 2>/dev/null || true)"
  [[ "$MANAGER_PID" =~ ^[1-9][0-9]*$ ]] || fail "gateway manager PID not found for controlled process restart"
  RESTART_BASE_COUNTS="$(server_counters)"
  read -r RESTART_BASE_RX RESTART_BASE_TX <<<"$RESTART_BASE_COUNTS"
  [[ "$RESTART_BASE_RX" =~ ^[0-9]+$ && "$RESTART_BASE_TX" =~ ^[0-9]+$ ]] || fail "invalid pre-restart VPN counters"
  docker exec "$GATEWAY" kill -TERM "$MANAGER_PID"
  restart_ready=0
  for _ in $(seq 1 90); do
    launches="$(manager_launch_count)"
    if [[ "$launches" =~ ^[0-9]+$ ]] && (( launches > 2 )); then
      fail "manager supervisor launched more than once during restart"
    fi
    if [[ "$launches" == 2 ]] && docker exec "$GATEWAY" curl -fsS --max-time 2 http://127.0.0.1:2222/readyz >/dev/null 2>&1; then
      restart_ready=1
      break
    fi
    sleep 1
  done
  [[ "$restart_ready" == 1 ]] || fail "manager did not return ready after process restart"
  CLIENT_IDENTITY_AFTER="$(client_config_identity "$CLIENT_ID")" || fail "client config readback failed after manager restart"
  [[ "$CLIENT_IDENTITY_AFTER" == "$CLIENT_IDENTITY_BEFORE" ]] || fail "client address/public key changed after manager process restart"
  RESTART_RESPONSE="$(docker exec "$CLIENT" curl -4fsS --connect-timeout 4 --max-time 15 \
    "http://$TEST_DESTINATION:8080/$RESTART_PATH")" || fail "VPN packet did not return after manager process restart"
  [[ "$RESTART_RESPONSE" == "fixture:$ID" ]] || fail "unexpected fixture response after manager process restart"
  fixture_hit "$RESTART_PATH" || fail "VPN packet after manager process restart missed the fixture"
  RESTART_FINAL_COUNTS="$(server_counters)"
  read -r RESTART_RX RESTART_TX <<<"$RESTART_FINAL_COUNTS"
  [[ "$RESTART_RX" =~ ^[0-9]+$ && "$RESTART_TX" =~ ^[0-9]+$ ]] || fail "invalid post-restart VPN counters"
  (( RESTART_RX > RESTART_BASE_RX && RESTART_TX > RESTART_BASE_TX )) || fail "VPN server RX/TX counters did not increase after manager process restart"
  printf 'PASS: app process restart restored client state and VPN packet path.\n'
fi
printf 'PASS: isolated VPN egress packet test completed (VPN-only DIRECT control + VPN return path).\n'

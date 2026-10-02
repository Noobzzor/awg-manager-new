#!/usr/bin/env bash
# Prove client-specific gateway DIRECT/BLOCK and fail-closed behavior with real WG packets.
# Everything is disposable: private Docker networks, tmpfs-only state, no ports.
set -Eeuo pipefail

IMAGE="${AWG_GATEWAY_TEST_IMAGE:-awg-manager:gateway-packet-test-20260928-dnat-r1}"
TEST_DESTINATION="203.0.113.10"
ID="$(python -c 'import uuid; print(uuid.uuid4().hex[:10])')"
EGRESS_NET="awgm-policy-egress-${ID}"
CLIENT_NET="awgm-policy-client-${ID}"
GATEWAY="awgm-policy-gateway-${ID}"
CLIENT="awgm-policy-client-${ID}"
CLIENT2="awgm-policy-client2-${ID}"
SINK="awgm-policy-sink-${ID}"
GW_ALIAS="awg-gateway-${ID}"
CLIENT_ID="packet-${ID}"
CLIENT2_ID="packet-${ID}-other"
PROFILE_ID="packet-${ID}"
DIRECT_PATH="direct-${ID}"
SECOND_PATH="unmatched-${ID}"
BLOCK_PATH="block-${ID}"
DOWN_PATH="down-${ID}"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

cleanup() {
  result=$?
  trap - EXIT
  if [[ "${AWG_GATEWAY_TEST_KEEP:-0}" == 1 ]]; then
    printf 'DEBUG: retained only this run: gateway=%s clients=%s,%s sink=%s networks=%s,%s\n' \
      "$GATEWAY" "$CLIENT" "$CLIENT2" "$SINK" "$CLIENT_NET" "$EGRESS_NET" >&2
    exit "$result"
  fi
  docker rm -f -v "$CLIENT2" "$CLIENT" "$GATEWAY" "$SINK" >/dev/null 2>&1 || true
  docker network rm "$CLIENT_NET" "$EGRESS_NET" >/dev/null 2>&1 || true
  exit "$result"
}
trap cleanup EXIT

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  fail "local test image not found: $IMAGE"
fi
for name in "$GATEWAY" "$CLIENT" "$CLIENT2" "$SINK"; do
  if docker container inspect "$name" >/dev/null 2>&1; then
    fail "refusing to touch pre-existing container $name"
  fi
done
for name in "$CLIENT_NET" "$EGRESS_NET"; do
  if docker network inspect "$name" >/dev/null 2>&1; then
    fail "refusing to touch pre-existing network $name"
  fi
done

# Keep the gateway and sink on an internal Docker network. A default route via
# its bridge is injected only so Sing-box can identify eth0; the only allowed
# test destination is DNATed to the local observable HTTP fixture below.
docker network create --internal "$EGRESS_NET" >/dev/null
docker network create --internal "$CLIENT_NET" >/dev/null
EGRESS_GATEWAY="$(docker network inspect -f '{{(index .IPAM.Config 0).Gateway}}' "$EGRESS_NET")"
[[ -n "$EGRESS_GATEWAY" ]] || fail "could not resolve internal egress bridge gateway"

docker run -d --name "$SINK" --network "$EGRESS_NET" \
  --tmpfs /www:rw,size=1m \
  --entrypoint /bin/sh busybox:latest \
  -ec 'exec httpd -vv -f -p 8080 -h /www' >/dev/null

docker run -d --name "$GATEWAY" --network "$EGRESS_NET" \
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
  --entrypoint /bin/sh "$IMAGE" -ec \
  'ip route add default via "$1" dev eth0; exec /usr/local/bin/entrypoint.sh /usr/local/bin/awg-manager' \
  sh "$EGRESS_GATEWAY" >/dev/null

docker network connect --alias "$GW_ALIAS" "$CLIENT_NET" "$GATEWAY"
docker run -d --name "$CLIENT" --network "$CLIENT_NET" \
  --cap-drop ALL --cap-add NET_ADMIN --device /dev/net/tun \
  --tmpfs /data:rw,size=16m,mode=0750 \
  --tmpfs /run:rw,size=16m,mode=0750 \
  --entrypoint /bin/sh "$IMAGE" -ec 'sleep 1800' >/dev/null
docker run -d --name "$CLIENT2" --network "$CLIENT_NET" \
  --cap-drop ALL --cap-add NET_ADMIN --device /dev/net/tun \
  --tmpfs /data:rw,size=16m,mode=0750 \
  --tmpfs /run:rw,size=16m,mode=0750 \
  --entrypoint /bin/sh "$IMAGE" -ec 'sleep 1800' >/dev/null

# Assert no published host ports or persistent Docker volumes were introduced.
if [[ -n "$(docker port "$GATEWAY")" || -n "$(docker port "$CLIENT")" || -n "$(docker port "$CLIENT2")" || -n "$(docker port "$SINK")" ]]; then
  fail "test containers unexpectedly publish host ports"
fi
for container in "$GATEWAY" "$CLIENT" "$CLIENT2" "$SINK"; do
  MOUNTS="$(docker inspect -f '{{range .Mounts}}{{.Type}}:{{.Destination}} {{end}}' "$container")"
  if [[ "$MOUNTS" == *volume* ]]; then
    fail "$container unexpectedly uses a Docker volume"
  fi
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

# Apply completion is asynchronous in the manager. The tmpfs applied-state
# hash lets the harness wait for the reload operation; packet assertions below
# remain the functional evidence.
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

put_profile() {
  local action="$1" body
  body="$(printf '{\"id\":\"%s\",\"name\":\"Local packet test\",\"defaultAction\":\"%s\"}' "$PROFILE_ID" "$action")"
  docker exec "$GATEWAY" curl -fsS --max-time 5 \
    -H 'Content-Type: application/json' -X PUT \
    --data-binary "$body" \
    "http://127.0.0.1:2222/api/gateway/policies/profiles/$PROFILE_ID" >/dev/null
}
apply_profile() {
  docker exec "$GATEWAY" curl -fsS --max-time 5 -X POST \
    "http://127.0.0.1:2222/api/gateway/policies/profiles/$PROFILE_ID/apply" >/dev/null
}

create_client() {
  local client_id="$1"
  docker exec "$GATEWAY" curl -fsS --max-time 5 \
    -H 'Content-Type: application/json' -X POST \
    --data-binary "{\"id\":\"$client_id\",\"label\":\"Local packet test\"}" \
    http://127.0.0.1:2222/api/gateway/clients/create >/dev/null
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

wait_for_handshake() {
  local client_container="$1" label="$2" handshake=0 timestamp
  for _ in $(seq 1 40); do
    timestamp="$(docker exec "$client_container" wg show awg-client latest-handshakes 2>/dev/null | awk 'NR == 1 {print $2}')"
    if [[ "$timestamp" =~ ^[1-9][0-9]*$ ]]; then
      handshake=1
      break
    fi
    sleep 0.5
  done
  [[ "$handshake" == 1 ]] || fail "$label WireGuard client handshake did not complete"
}

create_client "$CLIENT_ID"
create_client "$CLIENT2_ID"

PROFILE_BODY="$(printf '{\"id\":\"%s\",\"name\":\"Local packet test\",\"defaultAction\":\"block\",\"rules\":[{\"id\":\"client-direct\",\"action\":\"direct\",\"outbound\":\"direct\",\"priority\":10,\"clientIds\":[\"%s\"],\"ports\":[8080],\"enabled\":true}]}' "$PROFILE_ID" "$CLIENT_ID")"
docker exec "$GATEWAY" curl -fsS --max-time 5 \
  -H 'Content-Type: application/json' -X POST --data-binary "$PROFILE_BODY" \
  http://127.0.0.1:2222/api/gateway/policies/profiles/create >/dev/null
HASH_BEFORE="$(applied_hash)"
apply_profile
wait_for_apply "$HASH_BEFORE" || fail "client-specific policy reload was not observed"

docker exec "$SINK" sh -ec 'printf "fixture:%s\n" "$1" > "/www/$1"' sh "$DIRECT_PATH"
docker exec "$SINK" sh -ec 'printf "fixture:%s\n" "$1" > "/www/$1"' sh "$SECOND_PATH"
docker exec "$SINK" sh -ec 'printf "fixture:%s\n" "$1" > "/www/$1"' sh "$BLOCK_PATH"
docker exec "$SINK" sh -ec 'printf "fixture:%s\n" "$1" > "/www/$1"' sh "$DOWN_PATH"
SINK_IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$SINK")"
[[ -n "$SINK_IP" ]] || fail "could not resolve local fixture address"
FIXTURE_RULESET="$(printf 'table ip awgm_test_fixture {\n chain output {\n  type nat hook output priority -100; policy accept;\n  ip daddr %s tcp dport 8080 dnat to %s:8080\n }\n}\n' "$TEST_DESTINATION" "$SINK_IP")"
printf '%s\n' "$FIXTURE_RULESET" | docker exec -i "$GATEWAY" nft -c -f -
printf '%s\n' "$FIXTURE_RULESET" | docker exec -i "$GATEWAY" nft -f -

# The generated client config contains a private key. Pipe it directly between
# containers; never print, persist, or include it in diagnostics.
configure_client "$CLIENT" "$CLIENT_ID"
configure_client "$CLIENT2" "$CLIENT2_ID"
wait_for_handshake "$CLIENT" "selected"
wait_for_handshake "$CLIENT2" "unselected"

# The VPN ingress must not expose the manager's admin API; this destination is
# local and non-DNATed, so it must remain blocked by the gateway input guard.
if docker exec "$CLIENT" curl -4sS -o /dev/null --connect-timeout 2 --max-time 5 \
  http://10.66.0.1:2222/readyz >/dev/null 2>&1; then
  fail "WireGuard client reached the gateway admin API"
fi

DIRECT_RESPONSE="$(docker exec "$CLIENT" curl -4fsS --connect-timeout 3 --max-time 8 \
  "http://$TEST_DESTINATION:8080/$DIRECT_PATH")"
[[ "$DIRECT_RESPONSE" == "fixture:$DIRECT_PATH" ]] || fail "DIRECT response did not return through the client tunnel"
if ! docker logs "$SINK" 2>&1 | grep -Fq "GET /$DIRECT_PATH"; then
  fail "DIRECT request did not reach the local fixture"
fi
printf 'PASS: client-specific DIRECT rule reached the fixture and response returned.\n'

if docker exec "$CLIENT2" curl -4fsS --connect-timeout 2 --max-time 5 \
  "http://$TEST_DESTINATION:8080/$SECOND_PATH" >/dev/null 2>&1; then
  fail "unselected client received a response instead of the profile BLOCK default"
fi
if docker logs "$SINK" 2>&1 | grep -Fq "GET /$SECOND_PATH"; then
  fail "unselected client reached the local fixture"
fi
printf 'PASS: unselected client stayed BLOCKed by the profile default.\n'

# BLOCK must produce no fixture hit and no client response after the applied hash
# changes, not merely after the profile API accepted the request.
put_profile block
HASH_BEFORE="$(applied_hash)"
apply_profile
wait_for_apply "$HASH_BEFORE" || fail "BLOCK policy reload was not observed"
if docker exec "$CLIENT" curl -4fsS --connect-timeout 2 --max-time 5 \
  "http://$TEST_DESTINATION:8080/$BLOCK_PATH" >/dev/null 2>&1; then
  fail "BLOCK request received a response"
fi
if docker logs "$SINK" 2>&1 | grep -Fq "GET /$BLOCK_PATH"; then
  fail "BLOCK request reached the local fixture"
fi
printf 'PASS: BLOCK produced neither a fixture hit nor a client response.\n'

# Restore DIRECT, observe its active reload, then stop only this test gateway's
# sing-box process. A direct-capable policy must still fail closed when the
# traffic engine is absent.
put_profile direct
HASH_BEFORE="$(applied_hash)"
apply_profile
wait_for_apply "$HASH_BEFORE" || fail "DIRECT restore reload was not observed"
docker exec "$GATEWAY" pkill -KILL -x sing-box
engine_down=0
for _ in $(seq 1 20); do
  if ! docker exec "$GATEWAY" pgrep -x sing-box >/dev/null 2>&1; then
    engine_down=1
    break
  fi
  sleep 0.25
done
[[ "$engine_down" == 1 ]] || fail "Sing-box process did not stop"
if docker exec "$CLIENT" curl -4fsS --connect-timeout 2 --max-time 5 \
  "http://$TEST_DESTINATION:8080/$DOWN_PATH" >/dev/null 2>&1; then
  fail "client received a response while Sing-box was stopped"
fi
if docker logs "$SINK" 2>&1 | grep -Fq "GET /$DOWN_PATH"; then
  fail "traffic reached the local fixture while Sing-box was stopped"
fi
printf 'PASS: DIRECT policy failed closed after Sing-box stopped.\n'
printf 'PASS: isolated packet-path test completed (two WG clients, client-specific DIRECT, default BLOCK, return path, engine-down).\n'

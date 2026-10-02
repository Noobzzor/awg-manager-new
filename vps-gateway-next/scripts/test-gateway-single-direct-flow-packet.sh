#!/usr/bin/env bash
# One DIRECT flow from client .3 with packet, Sing-box, nft and conntrack evidence.
set -Eeuo pipefail

IMAGE="${AWG_GATEWAY_TEST_IMAGE:-awg-manager:gateway-packet-test-20260929-dns-order-r1}"
GO_IMAGE="${GO_IMAGE:-golang:1.26-bookworm}"
BUSYBOX_IMAGE="busybox:latest"
CAPTURE_SOURCE="${CAPTURE_SOURCE:-C:/Users/olesh/hermes_work/_snapshots/awg-manager_docker_gateway_gate_20260928/scripts/gateway_direct_flow_capture.go}"
EXPECT_CURL_SUCCESS="${EXPECT_CURL_SUCCESS:-0}"
EXPECT_RESPONSE_HASH_MATCH="${EXPECT_RESPONSE_HASH_MATCH:-0}"
DIRECT_TARGET_MODE="${DIRECT_TARGET_MODE:-synthetic-dnat}"
DIRECT_TEST_DESTINATION=''
DIRECT_SINK_EXTRA_ARGS=()
ID="$(python -c 'import uuid; print(uuid.uuid4().hex[:10])')"
OUTER_NET="awgm-one-direct-outer-${ID}"
CLIENT_NET="awgm-one-direct-client-${ID}"
EXIT_NET="awgm-one-direct-exit-${ID}"
GATEWAY="awgm-one-direct-gateway-${ID}"
CLIENT="awgm-one-direct-client1-${ID}"
CLIENT2="awgm-one-direct-client2-${ID}"
VPN_EXIT="awgm-one-direct-vpn-exit-${ID}"
SINK="awgm-one-direct-vpn-sink-${ID}"
DIRECT_SINK="awgm-one-direct-sink-${ID}"
CAP_GATEWAY="awgm-one-direct-cap-gw-${ID}"
CAP_SINK="awgm-one-direct-cap-sink-${ID}"
GW_ALIAS="awg-one-direct-gateway-${ID}"
CLIENT_ID="one-direct-${ID}"
CLIENT2_ID="one-direct-${ID}-other"
PROFILE_ID="one-direct-${ID}"
VPN_TAG="one-direct-vpn-${ID}"
DIRECT_PATH="direct-single-${ID}"
CREATED_NETWORK_IDS=()
CREATED_CONTAINER_IDS=()

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
cleanup() {
  result=$?
  trap - EXIT
  local resource
  for resource in "${CREATED_CONTAINER_IDS[@]}"; do docker rm -f "$resource" >/dev/null 2>&1 || true; done
  for resource in "${CREATED_NETWORK_IDS[@]}"; do docker network rm "$resource" >/dev/null 2>&1 || true; done
  exit "$result"
}
trap cleanup EXIT

case "$DIRECT_TARGET_MODE" in
  synthetic-dnat|local-direct) ;;
  routed-synthetic|routed-synthetic-via-peer) DIRECT_SINK_EXTRA_ARGS=(--cap-drop ALL --cap-add NET_ADMIN) ;;
  fixture-local-delivery) DIRECT_SINK_EXTRA_ARGS=(--cap-drop ALL --cap-add NET_ADMIN) ;;
  *) fail "unsupported DIRECT_TARGET_MODE=$DIRECT_TARGET_MODE" ;;
esac

for image in "$IMAGE" "$GO_IMAGE" "$BUSYBOX_IMAGE"; do
  docker image inspect -f '{{.Id}}' "$image" >/dev/null 2>&1 || fail "required local image missing; will not pull: $image"
done
IMAGE_ID="$(docker image inspect -f '{{.Id}}' "$IMAGE")"
GO_IMAGE_ID="$(docker image inspect -f '{{.Id}}' "$GO_IMAGE")"
BUSYBOX_IMAGE_ID="$(docker image inspect -f '{{.Id}}' "$BUSYBOX_IMAGE")"
for name in "$GATEWAY" "$CLIENT" "$CLIENT2" "$VPN_EXIT" "$SINK" "$DIRECT_SINK" "$CAP_GATEWAY" "$CAP_SINK"; do
  docker container inspect "$name" >/dev/null 2>&1 && fail "refusing to touch pre-existing container $name"
done
for name in "$OUTER_NET" "$CLIENT_NET" "$EXIT_NET"; do
  docker network inspect "$name" >/dev/null 2>&1 && fail "refusing to touch pre-existing network $name"
done

CREATED_NETWORK_IDS+=("$(docker network create --internal "$OUTER_NET")")
CREATED_NETWORK_IDS+=("$(docker network create --internal "$CLIENT_NET")")
CREATED_NETWORK_IDS+=("$(docker network create --internal "$EXIT_NET")")
for name in "$OUTER_NET" "$CLIENT_NET" "$EXIT_NET"; do
  [[ "$(docker network inspect -f '{{.Internal}}' "$name")" == true ]] || fail "$name is not internal"
done

CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$DIRECT_SINK" --network "$OUTER_NET" \
  "${DIRECT_SINK_EXTRA_ARGS[@]}" --tmpfs /www:rw,size=1m --entrypoint /bin/sh "$BUSYBOX_IMAGE_ID" \
  -ec 'exec httpd -vv -f -p 8080 -h /www')")
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$SINK" --network "$EXIT_NET" \
  --tmpfs /www:rw,size=1m --entrypoint /bin/sh "$BUSYBOX_IMAGE_ID" \
  -ec 'exec httpd -vv -f -p 8080 -h /www')")
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$VPN_EXIT" --network "$OUTER_NET" \
  --cap-drop ALL --cap-add NET_ADMIN --sysctl net.ipv4.ip_forward=1 \
  --tmpfs /data:rw,size=16m,mode=0750 --tmpfs /run:rw,size=8m --tmpfs /tmp:rw,size=8m \
  --entrypoint /bin/sh "$IMAGE_ID" -ec 'sleep 1800')")
docker network connect "$EXIT_NET" "$VPN_EXIT"
GATEWAY_COMMAND='exec /usr/local/bin/entrypoint.sh /usr/local/bin/awg-manager'
CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$GATEWAY" --network "$OUTER_NET" \
  --cap-drop ALL --cap-add NET_ADMIN --cap-add DAC_OVERRIDE --cap-add SETUID --cap-add SETGID \
  --device /dev/net/tun --sysctl net.ipv4.ip_forward=1 \
  --tmpfs /data:rw,size=64m,mode=0750,uid=10001,gid=10001 \
  --tmpfs /run:rw,size=16m,mode=0750,uid=10001,gid=10001 --tmpfs /tmp:rw,size=64m,mode=1777 \
  -e AWG_GATEWAY_ENABLE=true -e AWG_GATEWAY_INTERFACE=awg0 -e AWG_GATEWAY_TOOL=/usr/bin/wg \
  -e AWG_GATEWAY_WAN_INTERFACE=eth0 -e AWG_GATEWAY_PORT=51820 \
  -e AWG_GATEWAY_CLIENT_POOL=10.66.0.0/24 -e AWG_GATEWAY_SERVER_ADDRESS=10.66.0.1/24 \
  -e AWG_GATEWAY_PUBLIC_ENDPOINT="${GW_ALIAS}:51820" -e AWG_HTTP_ADDR=0.0.0.0:2222 \
  --entrypoint /bin/sh "$IMAGE_ID" -ec "$GATEWAY_COMMAND")")
docker network connect --alias "$GW_ALIAS" "$CLIENT_NET" "$GATEWAY"
for client in "$CLIENT" "$CLIENT2"; do
  CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$client" --network "$CLIENT_NET" \
    --cap-drop ALL --cap-add NET_ADMIN --device /dev/net/tun \
    --tmpfs /data:rw,size=16m,mode=0750 --tmpfs /run:rw,size=16m --tmpfs /tmp:rw,size=8m \
    --entrypoint /bin/sh "$IMAGE_ID" -ec 'sleep 1800')")
done

network_ip() {
  docker inspect "$1" | python -c 'import json,sys; print(json.load(sys.stdin)[0]["NetworkSettings"]["Networks"][sys.argv[1]]["IPAddress"])' "$2"
}
VPN_OUTER_IP="$(network_ip "$VPN_EXIT" "$OUTER_NET")"
SINK_IP="$(network_ip "$SINK" "$EXIT_NET")"
DIRECT_SINK_IP="$(network_ip "$DIRECT_SINK" "$OUTER_NET")"
GW_OUTER_IP="$(network_ip "$GATEWAY" "$OUTER_NET")"
[[ -n "$VPN_OUTER_IP" && -n "$SINK_IP" && -n "$DIRECT_SINK_IP" && -n "$GW_OUTER_IP" ]] || fail 'fixture IP discovery failed'
case "$DIRECT_TARGET_MODE" in
  synthetic-dnat) DIRECT_TEST_DESTINATION='203.0.113.11' ;;
  local-direct) DIRECT_TEST_DESTINATION="$DIRECT_SINK_IP" ;;
  routed-synthetic|routed-synthetic-via-peer|fixture-local-delivery) DIRECT_TEST_DESTINATION='203.0.113.11' ;;
esac
printf 'PROBE_TARGET_MODE=%s target=%s direct_sink=%s gateway_outer=%s\n' "$DIRECT_TARGET_MODE" "$DIRECT_TEST_DESTINATION" "$DIRECT_SINK_IP" "$GW_OUTER_IP"
if [[ "$DIRECT_TARGET_MODE" == routed-synthetic || "$DIRECT_TARGET_MODE" == routed-synthetic-via-peer || "$DIRECT_TARGET_MODE" == fixture-local-delivery ]]; then
  docker exec "$DIRECT_SINK" ip -4 addr add "$DIRECT_TEST_DESTINATION/32" dev eth0
  if [[ "$DIRECT_TARGET_MODE" == routed-synthetic-via-peer || "$DIRECT_TARGET_MODE" == fixture-local-delivery ]]; then
    docker exec "$GATEWAY" ip -4 route add table main "$DIRECT_TEST_DESTINATION/32" via "$DIRECT_SINK_IP" dev eth0
    printf '%s\n' 'PROBE_STAGE=fixture-alias-and-via-peer-route-installed'
  else
    docker exec "$GATEWAY" ip -4 route add table main "$DIRECT_TEST_DESTINATION/32" dev eth0 scope link
    printf '%s\n' 'PROBE_STAGE=routed-synthetic-alias-and-main-route-installed'
  fi
  printf '%s\n' '--- ROUTED SYNTHETIC SINK ADDRESS ---'
  docker exec "$DIRECT_SINK" ip -4 addr show dev eth0
  printf '%s\n' '--- ROUTED SYNTHETIC GATEWAY ROUTE ---'
  docker exec "$GATEWAY" ip -4 route show table main "$DIRECT_TEST_DESTINATION"
fi

run_fixture_local_delivery() {
  local duration=60 ready_deadline rc target tag expected actual expected_hash match local_gateway_ifaces='eth0' local_capture_start_seconds
  local bridge_path="bridge-${ID}" alias_path="alias-${ID}"
  printf '%s\n' 'PROBE_CLASSIFICATION=fixture-delivery-listener-observability-not-direct-policy'
  docker exec "$DIRECT_SINK" sh -ec 'printf "bridge:%s" "$1" >"/www/$2"; dd if=/dev/zero bs=1024 count=8 >>"/www/$2" 2>/dev/null; printf "alias:%s" "$1" >"/www/$3"; dd if=/dev/zero bs=1024 count=8 >>"/www/$3" 2>/dev/null' sh "$ID" "$bridge_path" "$alias_path"
  printf '%s\n' '--- FIXTURE ADDRESS/LISTENER READBACK ---'
  docker exec "$GATEWAY" sh -c 'ip -4 addr show; ip -4 route show table all; cat /sys/class/net/eth0/address'
  docker exec "$DIRECT_SINK" sh -c 'ip -4 addr show; ip -4 route show table all; cat /sys/class/net/eth0/address; if command -v ss >/dev/null 2>&1; then ss -lntp; elif command -v netstat >/dev/null 2>&1; then netstat -lnt; else cat /proc/net/tcp; fi'
  printf '%s\n' '--- FIXTURE RULE READBACK ---'
  docker exec "$GATEWAY" sh -c 'nft list ruleset 2>/dev/null || true; iptables-save 2>/dev/null || true'

  for capture_spec in "gateway:$CAP_GATEWAY:$local_gateway_ifaces:$DIRECT_SINK_IP,$GW_OUTER_IP,$DIRECT_TEST_DESTINATION" "sink:$CAP_SINK:eth0:$DIRECT_SINK_IP,$GW_OUTER_IP,$DIRECT_TEST_DESTINATION"; do
    IFS=: read -r tag capture ifaces ips <<<"$capture_spec"
    CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$capture" --network "container:$([[ "$tag" == gateway ]] && printf '%s' "$GATEWAY" || printf '%s' "$DIRECT_SINK")" --cap-drop ALL --cap-add NET_RAW --tmpfs /tmp:rw,size=512m --entrypoint /bin/sh "$GO_IMAGE_ID" -ec 'sleep 1800')")
    docker cp "$CAPTURE_SOURCE" "$capture:/capture-transfer.go" >/dev/null
    docker exec "$capture" cp /capture-transfer.go /tmp/capture.go
    docker exec "$capture" sh -ec 'cd /tmp; mkdir -p /tmp/gocache /tmp/gotmp; GOCACHE=/tmp/gocache GOTMPDIR=/tmp/gotmp GO111MODULE=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off /usr/local/go/bin/go build -trimpath -o /packet-capture capture.go'
    printf 'LOCAL_CAPTURE_NAMESPACE_MAC tag=%s mac=' "$tag"
    docker exec "$capture" cat /sys/class/net/eth0/address
  done
  local_capture_start_seconds=$SECONDS
  for capture_spec in "gateway:$CAP_GATEWAY:$local_gateway_ifaces:$DIRECT_SINK_IP,$GW_OUTER_IP,$DIRECT_TEST_DESTINATION" "sink:$CAP_SINK:eth0:$DIRECT_SINK_IP,$GW_OUTER_IP,$DIRECT_TEST_DESTINATION"; do
    IFS=: read -r tag capture ifaces ips <<<"$capture_spec"
    docker exec -d "$capture" sh -c "/packet-capture --ifaces $ifaces --ips $ips --port 8080 --duration ${duration}s > /tmp/capture.log 2>&1 & pid=\$!; printf '%s\\n' \"\$pid\" > /tmp/capture.pid; wait \"\$pid\"; rc=\$?; printf 'CAPTURE_EXIT_CODE=%s\\n' \"\$rc\" >> /tmp/capture.log; exit \"\$rc\""
  done
  for capture in "$CAP_GATEWAY" "$CAP_SINK"; do
    ready_deadline=$((SECONDS + 20))
    while (( SECONDS < ready_deadline )); do
      if docker exec "$capture" sh -c 'grep -q "^CAPTURE_READY " /tmp/capture.log' 2>/dev/null; then break; fi
      sleep 1
    done
    if ! docker exec "$capture" sh -c 'grep -q "^CAPTURE_READY " /tmp/capture.log' 2>/dev/null; then
      printf '%s\n' "--- local capture readiness failure: $capture ---"
      docker exec "$capture" sh -c 'cat /tmp/capture.log 2>/dev/null || true; ls -l /packet-capture /tmp/capture.go 2>/dev/null || true' || true
      docker inspect -f 'id={{.Id}} state={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}}' "$capture" || true
      fail "local capture not ready: $capture"
    fi
  done
  for container in "$GATEWAY" "$DIRECT_SINK"; do
    docker exec -d "$container" sh -c "ip monitor neigh > /tmp/local-neigh.log 2>&1 & printf '%s\\n' \$! > /tmp/local-neigh.pid; wait \$(cat /tmp/local-neigh.pid)"
    docker exec "$container" sh -c 'ip neigh show dev eth0 > /tmp/local-neigh-before.log; ip -4 route show table all > /tmp/local-routes-before.log'
  done

  dump_local_capture_log() {
    local capture="$1"
    printf '%s\n' "--- LOCAL CAPTURE LOG: $capture ---"
    docker exec "$capture" sh -c 'cat /tmp/capture.log 2>/dev/null || true' || true
    docker inspect -f 'id={{.Id}} state={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}}' "$capture" || true
  }
  assert_local_captures_active() {
    local remaining=$((duration - (SECONDS - local_capture_start_seconds))) capture pid active
    if (( remaining < 12 )); then
      for capture in "$CAP_GATEWAY" "$CAP_SINK"; do dump_local_capture_log "$capture"; done
      fail "local capture window too short immediately before request: remaining=${remaining}s required=12s"
    fi
    for capture in "$CAP_GATEWAY" "$CAP_SINK"; do
      pid="$(docker exec "$capture" sh -c 'cat /tmp/capture.pid 2>/dev/null || true' 2>/dev/null || true)"
      active="$(docker exec "$capture" sh -c 'pid="$1"; if [ -n "$pid" ] && [ -r "/proc/$pid/exe" ] && [ "$(readlink "/proc/$pid/exe" 2>/dev/null || true)" = "/packet-capture" ]; then printf yes; else printf no; fi' sh "$pid" 2>/dev/null || printf no)"
      if [[ "$active" != yes ]] || ! docker exec "$capture" sh -c 'grep -q "^CAPTURE_READY " /tmp/capture.log' 2>/dev/null; then
        dump_local_capture_log "$capture"
        fail "local capture inactive before request: capture=$capture pid=${pid:-missing} active=$active"
      fi
    done
    printf 'PROBE_STAGE=local-captures-active-immediately-before-request remaining_seconds=%s\n' "$remaining"
  }
  assert_local_captures_active

  for request_spec in "bridge:$DIRECT_SINK_IP:48081:$bridge_path" "alias:$DIRECT_TEST_DESTINATION:48082:$alias_path"; do
    IFS=: read -r tag target port path <<<"$request_spec"
    printf 'LOCAL_REQUEST_BEGIN tag=%s target=%s port=%s path=%s\n' "$tag" "$target" "$port" "$path"
    docker exec "$GATEWAY" sh -c 'date -u; ip -4 route get "$1"; ip neigh show dev eth0; if [ -r /proc/net/nf_conntrack ]; then grep -E "172\\.23\\.0\\.4|203\\.0\\.113\\.11|172\\.23\\.0\\.2" /proc/net/nf_conntrack || true; fi' sh "$target" >"/tmp/local-pre-request-$tag-$ID.txt" 2>&1
    cat "/tmp/local-pre-request-$tag-$ID.txt"
    assert_local_captures_active
    docker exec "$GATEWAY" sh -c 'set +e; curl -q --noproxy "*" --proxy "" --http1.1 --retry 0 --connect-timeout 3 --max-time 8 --local-port "$2" -o "/tmp/local-$3.body" -w "HTTP=%{http_code} LOCAL=%{local_ip}:%{local_port} REMOTE=%{remote_ip}:%{remote_port} BYTES=%{size_download}\\n" "http://$1:8080/$4"; rc=$?; printf "LOCAL_CURL_EXIT=%s\\n" "$rc"; sha256sum "/tmp/local-$3.body" 2>/dev/null || true' sh "$target" "$port" "$tag" "$path" >"/tmp/local-request-$tag-$ID.txt" 2>&1 || true
    rc="$(sed -n 's/^LOCAL_CURL_EXIT=//p' "/tmp/local-request-$tag-$ID.txt" | tail -n 1)"
    expected="$(docker exec "$DIRECT_SINK" sha256sum "/www/$path" | awk '{print $1}')"
    actual="$(grep -E '^[0-9a-f]{64}  /tmp/local-' "/tmp/local-request-$tag-$ID.txt" | awk '{print $1}' | tail -n 1 || true)"
    expected_hash="$expected"; match=no; [[ -n "$actual" && "$actual" == "$expected_hash" ]] && match=yes
    printf 'LOCAL_RESULT tag=%s target=%s curl_exit=%s hash_match=%s expected_hash=%s actual_hash=%s\n' "$tag" "$target" "${rc:-missing}" "$match" "$expected_hash" "${actual:-missing}"
    cat "/tmp/local-request-$tag-$ID.txt"
    docker exec "$GATEWAY" sh -c 'date -u; if [ -r /proc/net/nf_conntrack ]; then grep -E "172\\.23\\.0\\.4|203\\.0\\.113\\.11|172\\.23\\.0\\.2" /proc/net/nf_conntrack || true; fi; ip neigh show dev eth0' >"/tmp/local-post-request-$tag-$ID.txt" 2>&1
    cat "/tmp/local-post-request-$tag-$ID.txt"
  done
  for container in "$GATEWAY" "$DIRECT_SINK"; do
    docker exec "$container" sh -c 'ip neigh show dev eth0 > /tmp/local-neigh-after.log; cat /tmp/local-neigh-before.log; cat /tmp/local-neigh.log 2>/dev/null || true; cat /tmp/local-neigh-after.log; cat /tmp/local-routes-before.log'
    docker exec "$container" sh -c 'if [ -r /tmp/local-neigh.pid ]; then kill "$(cat /tmp/local-neigh.pid)" 2>/dev/null || true; fi' || true
  done
  for capture in "$CAP_GATEWAY" "$CAP_SINK"; do
    for _ in $(seq 1 $((duration + 15))); do
      if docker exec "$capture" sh -c 'grep -q "^CAPTURE_EXIT_CODE=" /tmp/capture.log' 2>/dev/null; then break; fi
      sleep 1
    done
    local log done_count exit_marker_count exit_zero_count stats_count unexpected
    log="$(docker exec "$capture" sh -c 'cat /tmp/capture.log')"
    printf '%s\n' "$log"
    done_count="$(printf '%s\n' "$log" | grep -cx 'CAPTURE_DONE' || true)"
    exit_marker_count="$(printf '%s\n' "$log" | grep -c '^CAPTURE_EXIT_CODE=' || true)"
    exit_zero_count="$(printf '%s\n' "$log" | grep -cx 'CAPTURE_EXIT_CODE=0' || true)"
    stats_count="$(printf '%s\n' "$log" | grep -Ec '^PACKET_STATISTICS iface=eth0 packets=[0-9]+ drops=0$' || true)"
    unexpected="$(printf '%s\n' "$log" | grep -E '^(CAPTURE_FAILED|CAPTURE_ERROR|PACKET_STATISTICS iface=eth0 packets=unknown|PACKET_STATISTICS iface=eth0 .*drops=[1-9])' || true)"
    if [[ "$done_count" != 1 || "$exit_marker_count" != 1 || "$exit_zero_count" != 1 || "$stats_count" != 1 || -n "$unexpected" ]]; then
      dump_local_capture_log "$capture"
      fail "local capture completion/statistics invalid: capture=$capture done=$done_count exit_markers=$exit_marker_count exit_zero=$exit_zero_count stats=$stats_count unexpected=${unexpected:-none}"
    fi
  done
  printf '%s\n' 'LOCAL_DELIVERY_CONTROL_COMPLETE'
}

if [[ "$DIRECT_TARGET_MODE" == fixture-local-delivery ]]; then
  run_fixture_local_delivery
  exit 0
fi
EXIT_IFACE="$(docker exec "$VPN_EXIT" sh -ec 'route="$(ip route get "$1")"; set -- $route; while [ "$#" -gt 0 ]; do if [ "$1" = dev ]; then shift; printf "%s" "$1"; exit 0; fi; shift; done; exit 1' sh "$SINK_IP")"
[[ -n "$EXIT_IFACE" && "$EXIT_IFACE" != eth0 ]] || fail 'VPN fixture exit is not isolated'

VPN_SERVER_PUBLIC="$(docker exec "$VPN_EXIT" sh -ec 'umask 077; wg genkey >/run/server.key; wg pubkey </run/server.key')"
VPN_CLIENT_PUBLIC="$(docker exec "$GATEWAY" sh -ec 'umask 077; wg genkey >/run/vpn-client.key; wg pubkey </run/vpn-client.key')"
[[ -n "$VPN_SERVER_PUBLIC" && -n "$VPN_CLIENT_PUBLIC" ]] || fail 'disposable WireGuard key generation failed'
docker exec "$VPN_EXIT" sh -ec \
  'ip link add wg0 type wireguard; ip addr add 10.99.0.1/24 dev wg0; wg set wg0 private-key /run/server.key listen-port 51820 peer "$1" allowed-ips 10.99.0.2/32,10.66.0.0/24; ip link set wg0 up' \
  sh "$VPN_CLIENT_PUBLIC"
docker exec "$VPN_EXIT" sh -ec \
  'iptables -P FORWARD DROP; iptables -A FORWARD -i wg0 -o "$1" -p tcp -d "$2" --dport 8080 -m conntrack --ctstate NEW,ESTABLISHED -j ACCEPT; iptables -A FORWARD -i "$1" -o wg0 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT; iptables -t nat -A PREROUTING -i wg0 -d "$3" -p tcp --dport 8080 -j DNAT --to-destination "$2:8080"; iptables -t nat -A POSTROUTING -o "$1" -d "$2" -p tcp --dport 8080 -j MASQUERADE' \
  sh "$EXIT_IFACE" "$SINK_IP" 203.0.113.10
docker exec "$SINK" sh -ec 'printf "fixture:%s" "$1" >"/www/$2"; dd if=/dev/zero bs=1024 count=32 >>"/www/$2" 2>/dev/null' sh "$ID" "vpn-unused-${ID}"
docker exec "$DIRECT_SINK" sh -ec 'printf "direct:%s" "$1" >"/www/$2"; dd if=/dev/zero bs=1024 count=32 >>"/www/$2" 2>/dev/null' sh "$ID" "$DIRECT_PATH"

ready=0
for _ in $(seq 1 60); do
  if docker exec "$GATEWAY" curl -fsS --max-time 2 http://127.0.0.1:2222/readyz >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
[[ "$ready" == 1 ]] || fail 'Gateway did not become ready'
applied_hash() { docker exec "$GATEWAY" sha256sum /var/run/awg-manager/singbox-applied.json 2>/dev/null | cut -d ' ' -f 1 || true; }
wait_for_apply() {
  local before="$1" current
  for _ in $(seq 1 60); do
    current="$(applied_hash)"
    if [[ -n "$current" && "$current" != "$before" ]] && docker exec "$GATEWAY" ip link show awgm0 >/dev/null 2>&1; then return 0; fi
    sleep 0.5
  done
  return 1
}
create_client() {
  docker exec "$GATEWAY" curl -fsS --max-time 5 -H 'Content-Type: application/json' -X POST \
    --data-binary "{\"id\":\"$1\",\"label\":\"One-flow local packet probe\"}" \
    http://127.0.0.1:2222/api/gateway/clients/create >/dev/null
}
configure_client() {
  local client_container="$1" client_id="$2"
  docker exec "$GATEWAY" curl -fsS --max-time 5 "http://127.0.0.1:2222/api/gateway/clients/$client_id/config" |
    docker exec -i "$client_container" sh -ec '
      umask 077; cat > /run/awg-client.conf
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
wait_handshake() {
  local client="$1" timestamp
  for _ in $(seq 1 40); do
    timestamp="$(docker exec "$client" wg show awg-client latest-handshakes 2>/dev/null | awk 'NR == 1 {print $2}')"
    if [[ "$timestamp" =~ ^[1-9][0-9]*$ ]]; then return 0; fi
    sleep 0.5
  done
  fail "handshake did not complete for $client"
}
client_address() {
  docker exec "$1" ip -4 -o addr show dev awg-client | awk '$3 == "inet" {split($4,address,"/"); print address[1]; exit}'
}

create_client "$CLIENT_ID"
create_client "$CLIENT2_ID"
configure_client "$CLIENT" "$CLIENT_ID"
configure_client "$CLIENT2" "$CLIENT2_ID"
wait_handshake "$CLIENT"
wait_handshake "$CLIENT2"
CLIENT_ADDRESS="$(client_address "$CLIENT")"
CLIENT2_ADDRESS="$(client_address "$CLIENT2")"
printf 'CLIENT_ADDRESSES client1=%s client2=%s\n' "$CLIENT_ADDRESS" "$CLIENT2_ADDRESS"
[[ "$CLIENT_ADDRESS" == 10.66.0.2 && "$CLIENT2_ADDRESS" == 10.66.0.3 ]] || fail 'client allocation differs from required .2/.3 order'

# One local AWG endpoint is kept only so the existing two-client policy compiles; it sends no test traffic.
docker exec "$GATEWAY" sh -ec \
  'umask 077; printf '\''{"tag":"%s","config":{"type":"awg","tag":"%s","useIntegratedTun":false,"private_key":"%s","address":["10.99.0.2/32"],"peers":[{"address":"%s","port":51820,"public_key":"%s","allowed_ips":["0.0.0.0/0"]}]}}\n'\'' "$1" "$1" "$(dd if=/run/vpn-client.key 2>/dev/null)" "$2" "$3" >/run/awg3-import.json; curl -fsS --max-time 8 -H '\''Content-Type: application/json'\'' -X POST --data-binary @/run/awg3-import.json http://127.0.0.1:2222/api/awg3-endpoints >/dev/null; rm -f /run/awg3-import.json /run/vpn-client.key' \
  sh "$VPN_TAG" "$VPN_OUTER_IP" "$VPN_SERVER_PUBLIC"

PROFILE_BODY="$(printf '{\"id\":\"%s\",\"name\":\"One-flow local packet probe\",\"defaultAction\":\"direct\"}' "$PROFILE_ID")"
docker exec "$GATEWAY" curl -fsS --max-time 5 -H 'Content-Type: application/json' -X POST --data-binary "$PROFILE_BODY" \
  http://127.0.0.1:2222/api/gateway/policies/profiles/create >/dev/null
MULTI_BODY="$(printf '{\"id\":\"%s\",\"name\":\"One-flow local packet probe\",\"defaultAction\":\"block\",\"rules\":[{\"id\":\"client-vpn\",\"action\":\"vpn\",\"outbound\":\"%s\",\"priority\":10,\"clientIds\":[\"%s\"],\"ports\":[8080],\"enabled\":true},{\"id\":\"client-direct\",\"action\":\"direct\",\"outbound\":\"direct\",\"priority\":10,\"clientIds\":[\"%s\"],\"ports\":[8080],\"enabled\":true}]}' \
  "$PROFILE_ID" "$VPN_TAG" "$CLIENT_ID" "$CLIENT2_ID")"
docker exec "$GATEWAY" curl -fsS --max-time 5 -H 'Content-Type: application/json' -X PUT --data-binary "$MULTI_BODY" \
  "http://127.0.0.1:2222/api/gateway/policies/profiles/$PROFILE_ID" >/dev/null
HASH_BEFORE="$(applied_hash)"
docker exec "$GATEWAY" curl -fsS --max-time 5 -X POST \
  "http://127.0.0.1:2222/api/gateway/policies/profiles/$PROFILE_ID/apply" >/dev/null
wait_for_apply "$HASH_BEFORE" || fail 'policy apply was not observed'

DIRECT_FIXTURE_SNAT="${DIRECT_FIXTURE_SNAT:-0}"
if [[ "$DIRECT_TARGET_MODE" == local-direct || "$DIRECT_TARGET_MODE" == routed-synthetic || "$DIRECT_TARGET_MODE" == routed-synthetic-via-peer ]]; then
  [[ "$DIRECT_FIXTURE_SNAT" != 1 ]] || fail 'DIRECT_FIXTURE_SNAT is forbidden in no-fixture routed mode'
  if docker exec "$GATEWAY" nft list table ip awgm_direct_fixture >/dev/null 2>&1; then
    fail 'awgm_direct_fixture unexpectedly exists in local-direct mode'
  fi
  printf '%s\n' 'PROBE_STAGE=no-output-dnat-no-fixture-snat'
else
  DIRECT_FIXTURE_POSTROUTING=''
  if [[ "$DIRECT_FIXTURE_SNAT" == 1 ]]; then
    DIRECT_FIXTURE_POSTROUTING=" chain postrouting {
  type nat hook postrouting priority 100; policy accept;
  ip saddr 198.18.0.1 ip daddr $DIRECT_SINK_IP tcp dport 8080 snat to $GW_OUTER_IP
 }"
  fi
  DIRECT_FIXTURE_RULESET="table ip awgm_direct_fixture {
 chain output {
  type nat hook output priority -100; policy accept;
  ip daddr $DIRECT_TEST_DESTINATION tcp dport 8080 dnat to $DIRECT_SINK_IP:8080
 }
 $DIRECT_FIXTURE_POSTROUTING
}"
  printf '%s' "$DIRECT_FIXTURE_RULESET" | docker exec -i "$GATEWAY" nft -c -f -
  printf '%s' "$DIRECT_FIXTURE_RULESET" | docker exec -i "$GATEWAY" nft -f -
fi

# Enable per-flow Sing-box tracing in this disposable instance only.
docker exec "$GATEWAY" curl -fsS --max-time 8 -H 'Content-Type: application/json' -X POST \
  --data-binary '{"logging":{"singboxLogLevel":"trace"}}' http://127.0.0.1:2222/api/settings/update |
  python -c 'import json,sys; level=json.load(sys.stdin).get("data",{}).get("logging",{}).get("singboxLogLevel"); assert level=="trace", f"unexpected log level {level!r}"; print("SINGBOX_LOG_LEVEL=trace")'
printf '%s\n' 'PROBE_STAGE=after-trace-update'
CLASH_PORT="$(docker exec "$GATEWAY" dd if=/data/sing-box/config.d/00-base.json 2>/dev/null | python -c 'import json,sys; addr=json.load(sys.stdin)["experimental"]["clash_api"]["external_controller"]; host,sep,port=addr.rpartition(":"); assert sep and host=="127.0.0.1"; print(port)')"
printf 'PROBE_STAGE=clash-port-discovered port=%s\n' "$CLASH_PORT"
docker exec -d -e CLASH_PORT="$CLASH_PORT" "$GATEWAY" sh -ec \
  'curl -sSN --max-time 60 -o /tmp/probe-singbox.log -w "%{http_code}" "http://127.0.0.1:${CLASH_PORT}/logs?level=trace" >/tmp/probe-singbox.status 2>/tmp/probe-singbox.err & echo $! >/tmp/probe-singbox.pid; wait'
printf '%s\n' 'PROBE_STAGE=trace-stream-started'

NFTRACE_RULESET="table inet awgm_probe_${ID} {
 chain pre { type filter hook prerouting priority raw; policy accept;
  iifname \"awg0\" ip saddr 10.66.0.3 ip daddr $DIRECT_TEST_DESTINATION tcp dport 8080 counter meta nftrace set 1
  iifname \"awgm0\" ip saddr 10.66.0.3 ip daddr $DIRECT_TEST_DESTINATION tcp dport 8080 counter meta nftrace set 1
  iifname \"eth0\" ip saddr $DIRECT_SINK_IP tcp sport 8080 counter meta nftrace set 1
  iifname \"awg0\" ip daddr 10.66.0.3 tcp flags & rst != 0 counter meta nftrace set 1
  iifname \"awgm0\" ip daddr 10.66.0.3 tcp flags & rst != 0 counter meta nftrace set 1
 }
 chain probe_forward { type filter hook forward priority raw; policy accept;
  iifname \"awg0\" oifname \"awgm0\" ip saddr 10.66.0.3 ip daddr $DIRECT_TEST_DESTINATION tcp dport 8080 counter meta nftrace set 1
 }
 chain probe_output { type filter hook output priority raw; policy accept;
  ip daddr $DIRECT_TEST_DESTINATION tcp dport 8080 counter meta nftrace set 1
  ip daddr $DIRECT_SINK_IP tcp dport 8080 counter meta nftrace set 1
  ip daddr 10.66.0.3 tcp flags & rst != 0 counter meta nftrace set 1
 }
}"
printf '%s' "$NFTRACE_RULESET" | docker exec -i "$GATEWAY" nft -c -f -
printf '%s' "$NFTRACE_RULESET" | docker exec -i "$GATEWAY" nft -f -
printf '%s\n' 'PROBE_STAGE=nft-rules-installed'
docker exec -d "$GATEWAY" sh -ec 'nft monitor trace >/tmp/probe-nft-trace.log 2>&1 & echo $! >/tmp/probe-nft-trace.pid; wait'
printf '%s\n' 'PROBE_STAGE=nft-monitor-started'
printf '%s\n' '--- ROUTE READBACK ---'
docker exec "$GATEWAY" ip -4 route get "$DIRECT_TEST_DESTINATION"
printf '%s\n' '--- POLICY RULE READBACK ---'
docker exec "$GATEWAY" ip -4 rule show
printf '%s\n' '--- ROUTE TABLE READBACK ---'
docker exec "$GATEWAY" ip -4 route show table all
printf '%s\n' '--- ACTIVE NFT RULESET READBACK ---'
docker exec "$GATEWAY" nft list ruleset

for container in "$GATEWAY" "$CLIENT" "$CLIENT2" "$VPN_EXIT" "$SINK" "$DIRECT_SINK"; do
  [[ -z "$(docker port "$container")" ]] || fail "$container unexpectedly publishes a host port"
  mounts="$(docker inspect -f '{{range .Mounts}}{{.Type}}:{{.Destination}} {{end}}' "$container")"
  [[ "$mounts" != *volume* && "$mounts" != *bind* ]] || fail "$container unexpectedly uses a volume or bind mount"
done
printf '%s\n' 'PROBE_STAGE=container-safety-checks-passed'

for capture in "$CAP_GATEWAY" "$CAP_SINK"; do
  if [[ "$capture" == "$CAP_GATEWAY" ]]; then netns="$GATEWAY"; else netns="$DIRECT_SINK"; fi
  if [[ "$capture" == "$CAP_GATEWAY" ]]; then
    CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$capture" --network "container:$netns" --cap-drop ALL --cap-add NET_RAW \
      --tmpfs /tmp:rw,size=512m --entrypoint /bin/sh "$GO_IMAGE_ID" -ec 'sleep 1800')")
  else
    CREATED_CONTAINER_IDS+=("$(docker run -d --pull never --name "$capture" --network "container:$netns" --cap-drop ALL --cap-add NET_RAW \
      --tmpfs /tmp:rw,size=512m --entrypoint /bin/sh "$GO_IMAGE_ID" -ec 'sleep 1800')")
  fi
  printf 'PROBE_STAGE=capture-container-created name=%s\n' "$capture"
  docker cp "$CAPTURE_SOURCE" "$capture:/capture-transfer.go" >/dev/null
  printf 'PROBE_STAGE=capture-source-copied name=%s\n' "$capture"
  docker inspect -f 'id={{.Id}} state={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}} network={{.HostConfig.NetworkMode}}' "$capture" >/tmp/awg-capture-inspect-${ID}.txt
  docker exec "$capture" sh -c 'if [ -L /capture-transfer.go ]; then printf SYMLINK; elif [ -f /capture-transfer.go ]; then printf REGULAR; else printf ABSENT; fi; wc -c /capture-transfer.go; sha256sum /capture-transfer.go' >/tmp/awg-capture-list-${ID}.txt
  docker exec "$capture" cp /capture-transfer.go /tmp/capture.go
  if ! docker exec "$capture" sh -ec 'if [ -L /tmp/capture.go ]; then exit 1; fi; test -f /tmp/capture.go; wc -c /tmp/capture.go; sha256sum /tmp/capture.go' >/tmp/awg-capture-readback-${ID}.txt 2>/tmp/awg-capture-readback-${ID}.err; then
    printf 'capture readback failed for %s\n' "$capture" >&2
    cat /tmp/awg-capture-inspect-${ID}.txt >&2 || true
    cat /tmp/awg-capture-list-${ID}.txt >&2 || true
    cat /tmp/awg-capture-readback-${ID}.err >&2 || true
    exit 1
  fi
  printf 'PROBE_STAGE=capture-source-readback name=%s\n' "$capture"
done
printf '%s\n' 'PROBE_STAGE=all-capture-containers-ready'
docker exec "$CAP_GATEWAY" sh -ec 'cd /tmp; mkdir -p /tmp/gocache /tmp/gotmp; GOCACHE=/tmp/gocache GOTMPDIR=/tmp/gotmp GO111MODULE=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off /usr/local/go/bin/go build -trimpath -o /packet-capture capture.go'
printf '%s\n' 'PROBE_STAGE=gateway-capture-built'
docker exec "$CAP_SINK" sh -ec 'cd /tmp; mkdir -p /tmp/gocache /tmp/gotmp; GOCACHE=/tmp/gocache GOTMPDIR=/tmp/gotmp GO111MODULE=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off /usr/local/go/bin/go build -trimpath -o /packet-capture capture.go'
printf '%s\n' 'PROBE_STAGE=sink-capture-built'

CAPTURE_WAIT_SECONDS="${CAPTURE_WAIT_SECONDS:-20}"
CAPTURE_DURATION_SECONDS="${CAPTURE_DURATION_SECONDS:-15}"
CAPTURE_MIN_REMAINING_SECONDS="${CAPTURE_MIN_REMAINING_SECONDS:-12}"
dump_capture_log() {
  local label="$1" capture="$2"
  printf '%s\n' "--- ${label} CAPTURE LOG ---"
  docker exec "$capture" sh -c 'cat /tmp/capture.log 2>/dev/null || true' || true
  printf '%s\n' "--- ${label} CAPTURE INSPECT ---"
  docker inspect -f 'id={{.Id}} state={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}}' "$capture" || true
}
wait_capture_ready() {
  local label="$1" capture="$2" deadline=$((SECONDS + CAPTURE_WAIT_SECONDS))
  while (( SECONDS < deadline )); do
    if docker exec "$capture" sh -c 'grep -q "^CAPTURE_READY " /tmp/capture.log' 2>/dev/null; then
      printf 'PROBE_STAGE=capture-ready label=%s\n' "$label"
      return 0
    fi
    if docker exec "$capture" sh -c 'grep -Eq "^(CAPTURE_ERROR|CAPTURE_FAILED)" /tmp/capture.log' 2>/dev/null; then
      dump_capture_log "$label" "$capture"
      fail "${label} capture failed before CAPTURE_READY"
    fi
    sleep 1
  done
  dump_capture_log "$label" "$capture"
  fail "${label} capture did not become ready within ${CAPTURE_WAIT_SECONDS}s"
}
wait_capture_finished() {
  local label="$1" capture="$2" deadline=$((SECONDS + CAPTURE_WAIT_SECONDS)) code=''
  while (( SECONDS < deadline )); do
    if code="$(docker exec "$capture" sh -c 'sed -n "s/^CAPTURE_EXIT_CODE=//p" /tmp/capture.log | tail -n 1' 2>/dev/null)" && [[ -n "$code" ]]; then
      dump_capture_log "$label" "$capture"
      [[ "$code" == 0 ]] || fail "${label} capture exited with code ${code}"
      printf 'PROBE_STAGE=capture-finished label=%s exit=%s\n' "$label" "$code"
      return 0
    fi
    sleep 1
  done
  dump_capture_log "$label" "$capture"
  fail "${label} capture did not finish within ${CAPTURE_WAIT_SECONDS}s"
}
assert_capture_completed() {
  local label="$1" capture="$2" expected_ifaces="$3" log='' done_count=0 exit_count=0 exit_zero_count=0 stats='' bad='' actual_ifaces='' expected_sorted='' actual_sorted='' actual_count=0 unique_count=0
  log="$(docker exec "$capture" sh -c 'cat /tmp/capture.log 2>/dev/null || true' 2>/dev/null || true)"
  done_count="$(printf '%s\n' "$log" | grep -c '^CAPTURE_DONE$' || true)"
  exit_count="$(printf '%s\n' "$log" | grep -c '^CAPTURE_EXIT_CODE=' || true)"
  exit_zero_count="$(printf '%s\n' "$log" | grep -c '^CAPTURE_EXIT_CODE=0$' || true)"
  stats="$(printf '%s\n' "$log" | grep '^PACKET_STATISTICS ' || true)"
  actual_ifaces="$(printf '%s\n' "$stats" | awk '{for (i=1; i<=NF; i++) if ($i ~ /^iface=/) {sub(/^iface=/, "", $i); print $i}}' | sort)"
  expected_sorted="$(printf '%s\n' "$expected_ifaces" | tr ',' '\n' | sort)"
  actual_sorted="$actual_ifaces"
  actual_count="$(printf '%s\n' "$actual_ifaces" | awk 'NF {count++} END {print count+0}')"
  unique_count="$(printf '%s\n' "$actual_ifaces" | awk 'NF {if (!seen[$0]++) count++} END {print count+0}')"
  bad="$(printf '%s\n' "$stats" | awk 'NF && $0 !~ /^PACKET_STATISTICS iface=[^ ]+ packets=[0-9]+ drops=0$/ {print}')"
  if [[ "$done_count" != 1 || "$exit_count" != 1 || "$exit_zero_count" != 1 || "$actual_count" != "$unique_count" || "$actual_sorted" != "$expected_sorted" || -n "$bad" ]]; then
    dump_capture_log "$label" "$capture"
    fail "${label} capture lifecycle/statistics invalid: done=${done_count} exit=${exit_count} exit0=${exit_zero_count} ifaces=${actual_sorted//$'\n'/,} expected=${expected_sorted//$'\n'/,}"
  fi
  printf 'PROBE_STAGE=capture-completed-and-statistics-verified label=%s interfaces=%s\n%s\n' "$label" "${actual_sorted//$'\n'/,}" "$stats"
}
assert_capture_preflight() {
  local label="$1" capture="$2" done_count=0 exit_count=0 pid='' active=''
  done_count="$(docker exec "$capture" sh -c 'grep -c "^CAPTURE_DONE$" /tmp/capture.log 2>/dev/null || true' 2>/dev/null || true)"
  exit_count="$(docker exec "$capture" sh -c 'grep -c "^CAPTURE_EXIT_CODE=" /tmp/capture.log 2>/dev/null || true' 2>/dev/null || true)"
  pid="$(docker exec "$capture" sh -c 'cat /tmp/capture.pid 2>/dev/null || true' 2>/dev/null || true)"
  active="$(docker exec "$capture" sh -c 'pid="$1"; if [ -n "$pid" ] && [ -r "/proc/$pid/exe" ] && [ "$(readlink "/proc/$pid/exe" 2>/dev/null || true)" = "/packet-capture" ]; then printf yes; else printf no; fi' sh "$pid" 2>/dev/null || printf no)"
  if [[ "$done_count" != 0 || "$exit_count" != 0 || "$active" != yes ]]; then
    dump_capture_log "$label" "$capture"
    fail "${label} capture is not active before curl: pid=${pid:-missing} active=${active} done=${done_count} exit=${exit_count}"
  fi
  printf 'PROBE_STAGE=capture-active-before-curl label=%s pid=%s\n' "$label" "$pid"
}
assert_capture_window_before_curl() {
  local elapsed=$((SECONDS - CAPTURE_START_SECONDS)) remaining=$((CAPTURE_DURATION_SECONDS - elapsed))
  if (( remaining < CAPTURE_MIN_REMAINING_SECONDS )); then
    dump_capture_log gateway "$CAP_GATEWAY"
    dump_capture_log sink "$CAP_SINK"
    fail "insufficient capture window immediately before curl: remaining=${remaining}s required=${CAPTURE_MIN_REMAINING_SECONDS}s"
  fi
  assert_capture_preflight gateway "$CAP_GATEWAY"
  assert_capture_preflight sink "$CAP_SINK"
  printf 'PROBE_STAGE=final-capture-window-verified-immediately-before-curl remaining_seconds=%s\n' "$remaining"
}

CAPTURE_START_SECONDS=$SECONDS
docker exec -d "$CAP_GATEWAY" sh -c \
  "/packet-capture --ifaces awg0,awgm0,eth0,eth1 --ips 10.66.0.3,$DIRECT_TEST_DESTINATION,$DIRECT_SINK_IP,$GW_OUTER_IP --duration ${CAPTURE_DURATION_SECONDS}s > /tmp/capture.log 2>&1 & pid=\$!; printf '%s\\n' \"\$pid\" > /tmp/capture.pid; wait \"\$pid\"; rc=\$?; printf 'CAPTURE_EXIT_CODE=%s\\n' \"\$rc\" >> /tmp/capture.log; exit \"\$rc\""
docker exec -d "$CAP_SINK" sh -c \
  "/packet-capture --ifaces eth0 --ips $DIRECT_TEST_DESTINATION,$DIRECT_SINK_IP,$GW_OUTER_IP --duration ${CAPTURE_DURATION_SECONDS}s > /tmp/capture.log 2>&1 & pid=\$!; printf '%s\\n' \"\$pid\" > /tmp/capture.pid; wait \"\$pid\"; rc=\$?; printf 'CAPTURE_EXIT_CODE=%s\\n' \"\$rc\" >> /tmp/capture.log; exit \"\$rc\""
wait_capture_ready gateway "$CAP_GATEWAY"
wait_capture_ready sink "$CAP_SINK"
CAPTURE_ELAPSED_SECONDS=$((SECONDS - CAPTURE_START_SECONDS))
CAPTURE_REMAINING_SECONDS=$((CAPTURE_DURATION_SECONDS - CAPTURE_ELAPSED_SECONDS))
if (( CAPTURE_REMAINING_SECONDS < CAPTURE_MIN_REMAINING_SECONDS )); then
  dump_capture_log gateway "$CAP_GATEWAY"
  dump_capture_log sink "$CAP_SINK"
  fail "insufficient capture window before curl: remaining=${CAPTURE_REMAINING_SECONDS}s required=${CAPTURE_MIN_REMAINING_SECONDS}s"
fi
assert_capture_preflight gateway "$CAP_GATEWAY"
assert_capture_preflight sink "$CAP_SINK"
printf 'PROBE_STAGE=initial-capture-window-verified remaining_seconds=%s\n' "$CAPTURE_REMAINING_SECONDS"
printf '%s\n' 'PROBE_STAGE=all-captures-ready-before-curl'
printf '%s\n' '--- ETHERNET IDENTITY AND NEIGHBOR READBACK BEFORE CURL ---'
printf '%s\n' 'gateway eth0 MAC:'
docker exec "$GATEWAY" cat /sys/class/net/eth0/address
printf '%s\n' 'sink eth0 MAC:'
docker exec "$DIRECT_SINK" cat /sys/class/net/eth0/address
printf '%s\n' 'gateway route to target:'
docker exec "$GATEWAY" ip -4 route get "$DIRECT_TEST_DESTINATION"
printf '%s\n' 'gateway neighbors eth0:'
docker exec "$GATEWAY" ip -4 neigh show dev eth0
printf '%s\n' 'gateway neighbor details eth0:'
docker exec "$GATEWAY" ip -s -4 neigh show dev eth0
printf '%s\n' 'sink neighbors eth0:'
docker exec "$DIRECT_SINK" ip -4 neigh show dev eth0

conntrack_snapshot() {
  docker exec "$GATEWAY" sh -ec '
    found=0
    for f in /proc/net/nf_conntrack /proc/net/ip_conntrack; do
      if [ -r "$f" ]; then
        printf "CONNTRACK_FILE=%s\n" "$f"
        awk -v a="$1" -v b="$2" -v c="$3" "index(\$0,a)||index(\$0,b)||index(\$0,c) {print}" "$f"
        found=1
      fi
    done
    if [ "$found" -eq 0 ]; then printf "NF_CONNTRACK_PROC_UNAVAILABLE\n"; fi
  ' sh 10.66.0.3 "$DIRECT_TEST_DESTINATION" "$DIRECT_SINK_IP"
}
CT_BEFORE="$(conntrack_snapshot)"
assert_capture_window_before_curl
printf 'PROBE_FLOW=10.66.0.3 -> %s:8080; only this client application request will be sent.\n' "$DIRECT_TEST_DESTINATION"
if docker exec "$CLIENT2" curl -4fsS --retry 0 --connect-timeout 4 --max-time 10 \
  "http://$DIRECT_TEST_DESTINATION:8080/$DIRECT_PATH" -o /run/direct-response; then
  CURL_STATUS=0
  RESPONSE_HASH="$(docker exec "$CLIENT2" sha256sum /run/direct-response | awk '{print $1}')"
  EXPECTED_HASH="$(docker exec "$DIRECT_SINK" sha256sum "/www/$DIRECT_PATH" | awk '{print $1}')"
else
  CURL_STATUS=$?
  RESPONSE_HASH=none
  EXPECTED_HASH="$(docker exec "$DIRECT_SINK" sha256sum "/www/$DIRECT_PATH" | awk '{print $1}')"
fi
printf 'CURL_STATUS=%s RESPONSE_HASH_MATCH=%s\n' "$CURL_STATUS" "$( [[ "$RESPONSE_HASH" == "$EXPECTED_HASH" ]] && printf yes || printf no )"
wait_capture_finished gateway "$CAP_GATEWAY"
wait_capture_finished sink "$CAP_SINK"
assert_capture_completed gateway "$CAP_GATEWAY" 'awg0,awgm0,eth0,eth1'
assert_capture_completed sink "$CAP_SINK" 'eth0'

docker exec "$GATEWAY" sh -ec 'if read -r pid </tmp/probe-nft-trace.pid; then kill "$pid" 2>/dev/null || true; fi' 2>/dev/null || true
docker exec "$GATEWAY" sh -ec 'if read -r pid </tmp/probe-singbox.pid; then kill "$pid" 2>/dev/null || true; fi' 2>/dev/null || true
CT_AFTER="$(conntrack_snapshot)"
printf '%s\n' '--- CONNTRACK BEFORE ---' "$CT_BEFORE" '--- CONNTRACK AFTER ---' "$CT_AFTER"
printf '%s\n' '--- GATEWAY PACKETS (raw socket; only matching TCP flow) ---'
docker exec "$CAP_GATEWAY" dd if=/tmp/capture.log 2>/dev/null | python -c 'import sys; rows=sys.stdin.read().splitlines(); print("\n".join(rows) if rows else "NO_GATEWAY_PACKET_CAPTURE_OUTPUT")'
printf '%s\n' '--- DIRECT SINK PACKETS (raw socket; only matching TCP flow) ---'
docker exec "$CAP_SINK" dd if=/tmp/capture.log 2>/dev/null | python -c 'import sys; rows=sys.stdin.read().splitlines(); print("\n".join(rows) if rows else "NO_SINK_PACKET_CAPTURE_OUTPUT")'
printf '%s\n' '--- NFT TRACE (flow-filtered) ---'
docker exec "$GATEWAY" dd if=/tmp/probe-nft-trace.log 2>/dev/null | python -c 'import sys; needles=("10.66.0.3",sys.argv[1],sys.argv[2],"reject","dnat","awgm0"); rows=[x.rstrip() for x in sys.stdin if any(n in x for n in needles)]; print("\n".join(rows[-160:]) if rows else "NO_MATCHING_NFT_TRACE")' "$DIRECT_TEST_DESTINATION" "$DIRECT_SINK_IP"
printf '%s\n' '--- NFT COUNTERS AND DIRECT DNAT RULE ---'
docker exec "$GATEWAY" nft list table "inet" "awgm_probe_${ID}"
if docker exec "$GATEWAY" nft list table ip awgm_direct_fixture >/dev/null 2>&1; then
  :
else
  printf '%s\n' 'NO_AWGM_DIRECT_FIXTURE_TABLE'
fi
printf '%s\n' '--- SING-BOX TRACE LOG (filtered and secret-redacted) ---'
docker exec "$GATEWAY" dd if=/tmp/probe-singbox.log 2>/dev/null | python -c 'import json,re,sys; needles=("10.66.0.3",sys.argv[1],"router:","inbound","outbound","direct","dial","reject"); found=[]
for raw in sys.stdin:
 try: item=json.loads(raw); text=str(item.get("payload",raw))
 except Exception: text=raw.rstrip()
 text=re.sub(r"(?i)(private[_ -]?key|preshared[_ -]?key|secret|token|password)(\s*[:=]\s*)[^\s,;]+",r"\1\2[REDACTED]",text)
 text=re.sub(r"(?<![A-Za-z0-9])[A-Za-z0-9+/]{40,}={0,2}(?![A-Za-z0-9])","[REDACTED]",text)
 if any(n.lower() in text.lower() for n in needles): found.append(text)
print("\n".join(found[-120:]) if found else "NO_RELEVANT_SINGBOX_TRACE")' "$DIRECT_TEST_DESTINATION"
printf '%s\n' '--- DIRECT SINK HTTP LOG (request path only) ---'
docker logs "$DIRECT_SINK" 2>&1 | python -c 'import sys; rows=[x.rstrip() for x in sys.stdin if "GET /direct-single-" in x or "GET /" in x]; print("\n".join(rows[-20:]) if rows else "NO_HTTP_GET_LOG")'
printf 'PROBE_FINISHED id=%s client=%s direct_fixture=%s curl_status=%s\n' "$ID" "$CLIENT2_ADDRESS" "$DIRECT_SINK_IP" "$CURL_STATUS"
if [[ "$EXPECT_CURL_SUCCESS" == 1 && "$CURL_STATUS" != 0 ]]; then
  printf 'EXPECTATION_FAILED curl_status=%s expected=0\n' "$CURL_STATUS"
  exit 1
fi
if [[ "$EXPECT_RESPONSE_HASH_MATCH" == 1 && "$RESPONSE_HASH" != "$EXPECTED_HASH" ]]; then
  printf 'EXPECTATION_FAILED response_hash_match=no expected=yes\n'
  exit 1
fi

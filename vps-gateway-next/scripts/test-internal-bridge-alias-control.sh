#!/usr/bin/env bash
# Fixture-only control: no AWG, Sing-box, client flow or host capture.
set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
STAMP="$(date +%Y%m%dT%H%M%S)-$RANDOM"
OUT="${CONTROL_EVIDENCE_DIR:-$ROOT/review-packs/2026-09-30-direct-flow/50-internal-bridge-control/$STAMP}"
NET="awgm-alias-control-$STAMP"
SENDER="awgm-alias-sender-$STAMP"
SINK="awgm-alias-sink-$STAMP"
SUBNET='198.19.240.0/24'
SENDER_IP='198.19.240.4'
SINK_IP='198.19.240.2'
INSIDE_ALIAS='198.19.240.11'
OUTSIDE_ALIAS='203.0.113.11'
CID_SENDER='' CID_SINK='' NID=''
mkdir -p "$OUT"
record() {
  local label="$1" rc; shift
  printf '%q ' "$@" >> "$OUT/commands.txt"; printf '\n' >> "$OUT/commands.txt"
  if "$@" >"$OUT/$label.stdout.txt" 2>"$OUT/$label.stderr.txt"; then rc=0; else rc=$?; fi
  printf '%s\n' "$rc" > "$OUT/$label.exit.txt"
  return "$rc"
}
cleanup() {
  local rc=$? cleanup_rc=0
  trap - EXIT
  for cid in "$CID_SENDER" "$CID_SINK"; do
    if [[ -n "$cid" ]]; then docker rm -f "$cid" >> "$OUT/cleanup.txt" 2>&1 || cleanup_rc=1; fi
  done
  if [[ -n "$NID" ]]; then docker network rm "$NID" >> "$OUT/cleanup.txt" 2>&1 || cleanup_rc=1; fi
  for cid in "$CID_SENDER" "$CID_SINK"; do
    if [[ -n "$cid" ]] && docker inspect "$cid" >/dev/null 2>&1; then cleanup_rc=1; fi
  done
  if [[ -n "$NID" ]] && docker network inspect "$NID" >/dev/null 2>&1; then cleanup_rc=1; fi
  docker ps -a --filter name=awg-manager-peer-test --filter name=awg-manager-api-test --filter name=awg-manager-full --format '{{.Names}} {{.Status}}' > "$OUT/protected-after.txt" 2>&1 || cleanup_rc=1
  printf 'probe_exit=%s cleanup_exit=%s\n' "$rc" "$cleanup_rc" >> "$OUT/result.txt"
  printf 'evidence=%s probe_exit=%s cleanup_exit=%s\n' "$OUT" "$rc" "$cleanup_rc"
  (( cleanup_rc == 0 )) || exit 1
  exit "$rc"
}
# No resource is created until the daemon and local images are available.
record daemon docker info --format '{{.ServerVersion}}'
IMAGE_ID="$(docker image inspect -f '{{.Id}}' "${CONTROL_IMAGE:-awg-manager:gateway-packet-test-20260929-dns-order-r1}")"
SINK_IMAGE_ID="$(docker image inspect -f '{{.Id}}' busybox:latest)"
record protected-before docker ps -a --filter name=awg-manager-peer-test --filter name=awg-manager-api-test --filter name=awg-manager-full --format '{{.Names}} {{.Status}}'
trap cleanup EXIT
# Unique IDs only; refuse any name collision, never touch pre-existing resources.
for name in "$SENDER" "$SINK"; do
  if docker container inspect "$name" >/dev/null 2>&1; then exit 1; fi
done
if docker network inspect "$NET" >/dev/null 2>&1; then exit 1; fi
NID="$(docker network create --internal --subnet "$SUBNET" "$NET")"
[[ "$(docker network inspect -f '{{.Internal}}' "$NID")" == true ]]
CID_SINK="$(docker run -d --pull never --name "$SINK" --network "$NID" --ip "$SINK_IP" --cap-drop ALL --cap-add NET_ADMIN --security-opt no-new-privileges --read-only --tmpfs /www:rw,size=1m --tmpfs /tmp:rw,size=1m --entrypoint /bin/sh "$SINK_IMAGE_ID" -ec 'printf "fixture-only-alias-control\n" >/www/probe; exec httpd -f -p 8080 -h /www')"
CID_SENDER="$(docker run -d --pull never --name "$SENDER" --network "$NID" --ip "$SENDER_IP" --cap-drop ALL --cap-add NET_ADMIN --security-opt no-new-privileges --read-only --tmpfs /tmp:rw,size=1m --entrypoint /bin/sh "$IMAGE_ID" -ec 'exec sleep 180')"
record sink-alias-inside docker exec "$CID_SINK" ip addr add "$INSIDE_ALIAS/32" dev eth0
record sink-alias-outside docker exec "$CID_SINK" ip addr add "$OUTSIDE_ALIAS/32" dev eth0
record sender-route-inside docker exec "$CID_SENDER" ip route add "$INSIDE_ALIAS/32" via "$SINK_IP" dev eth0
record sender-route-outside docker exec "$CID_SENDER" ip route add "$OUTSIDE_ALIAS/32" via "$SINK_IP" dev eth0
record network docker network inspect "$NID" --format '{{.Id}} internal={{.Internal}} ipam={{json .IPAM.Config}} endpoints={{json .Containers}}'
record safety docker inspect "$CID_SENDER" "$CID_SINK" --format '{{.Id}} privileged={{.HostConfig.Privileged}} caps={{json .HostConfig.CapAdd}} mounts={{json .Mounts}} ports={{json .HostConfig.PortBindings}}'
record sink-state docker exec "$CID_SINK" sh -c 'ip addr show dev eth0; ip route show table all; netstat -lnt; cat /sys/class/net/eth0/address'
record sender-state docker exec "$CID_SENDER" sh -c 'ip addr show dev eth0; ip rule; ip route show table all; cat /sys/class/net/eth0/address'
# Show the alias listener works locally before interpreting remote failures.
record sink-self-alias docker exec "$CID_SINK" sh -ec 'wget -q -T 3 -O /tmp/self "http://$1:8080/probe"; sha256sum /tmp/self /www/probe; cmp /tmp/self /www/probe' sh "$OUTSIDE_ALIAS"
EXPECTED="$(docker exec "$CID_SINK" sha256sum /www/probe | cut -d ' ' -f1)"
for spec in "bridge:$SINK_IP" "inside:$INSIDE_ALIAS" "outside:$OUTSIDE_ALIAS"; do
  IFS=: read -r label target <<< "$spec"
  record "route-$label" docker exec "$CID_SENDER" ip route get "$target"
  set +e
  record "request-$label" docker exec "$CID_SENDER" curl -q --noproxy '*' --proxy '' --retry 0 --connect-timeout 2 --max-time 4 -fsS -o "/tmp/$label.body" "http://$target:8080/probe"
  rc=$?
  set -e
  actual='missing'
  if (( rc == 0 )); then actual="$(docker exec "$CID_SENDER" sha256sum "/tmp/$label.body" | cut -d ' ' -f1)"; fi
  printf 'request=%s destination=%s curl_exit=%s expected_hash=%s actual_hash=%s\n' "$label" "$target" "$rc" "$EXPECTED" "$actual" | tee -a "$OUT/result.txt"
  if [[ "$label" == outside ]]; then
    [[ "$rc" == 28 ]] || { printf 'prediction=NOT_CONFIRMED\n' >> "$OUT/result.txt"; exit 1; }
  else
    [[ "$rc" == 0 && "$actual" == "$EXPECTED" ]] || exit 1
  fi
  record "neighbor-$label" docker exec "$CID_SENDER" ip neigh show dev eth0
done
printf 'classification=FIXTURE_ONLY prediction=CONFIRMED product_gateway=NOT_STARTED host_rule=NOT_OBSERVED\n' >> "$OUT/result.txt"

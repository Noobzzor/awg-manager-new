#!/usr/bin/env bash
# Build and passively exercise the capture helper on loopback in a no-network container.
set -uo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="$ROOT/scripts/gateway_direct_flow_capture.go"
SOURCE_NATIVE='C:/Users/olesh/hermes_work/_snapshots/awg-manager_docker_gateway_gate_20260928/scripts/gateway_direct_flow_capture.go'
PACK_ROOT="${REVIEW_PACK_DIR:-$ROOT/review-packs/2026-09-30-direct-flow}"
EVIDENCE_ROOT="$PACK_ROOT/06-loopback-readiness"
MODE="${1:-}"
RUN_SUFFIX="$(date +%s)-$$-$RANDOM-$RANDOM"
CONTAINER_NAME="awg-direct-loopback-readiness-$RUN_SUFFIX"
EVIDENCE="$EVIDENCE_ROOT/$RUN_SUFFIX"
IMAGE="golang:1.26-bookworm"
EXPECTED_SOURCE_SIZE=15945
EXPECTED_SOURCE_SHA256='d5d07cfcd16d8b280e3f2257442377d421beff2b73d1496d91070aa67a3e1482'
CID=''
CREATE_ATTEMPTED=0
LAST_RC=0
FIRST_FAILURE=0
READINESS_RESULT='NOT_RUN'

mkdir -p "$EVIDENCE"
printf 'started_at=%s\nmode=%s\nroot=%s\nsource=%s\nsource_native=%s\ncontainer_name=%s\nimage=%s\nexpected_source_size=%s\nexpected_source_sha256=%s\n' \
  "$(date -Iseconds)" "$MODE" "$ROOT" "$SOURCE" "$SOURCE_NATIVE" "$CONTAINER_NAME" "$IMAGE" \
  "$EXPECTED_SOURCE_SIZE" "$EXPECTED_SOURCE_SHA256" > "$EVIDENCE/invocation.txt"

record() {
  local label="$1"; shift
  local rc=0
  {
    printf '[%s]' "$label"
    printf ' %q' "$@"
    printf '\n'
  } >> "$EVIDENCE/commands.txt"
  if "$@" >"$EVIDENCE/$label.stdout.txt" 2>"$EVIDENCE/$label.stderr.txt"; then rc=0; else rc=$?; fi
  printf '%s\n' "$rc" > "$EVIDENCE/$label.exit.txt"
  LAST_RC="$rc"
  if (( rc != 0 && FIRST_FAILURE == 0 )); then FIRST_FAILURE="$rc"; fi
  return 0
}

read_first_line() {
  [[ -r "$1" ]] || return 1
  local value=''
  IFS= read -r value < "$1" || [[ -n "$value" ]] || [[ ! -s "$1" ]] || return 1
  value="${value%$'\r'}"
  printf '%s' "$value"
}

cleanup() {
  local shell_rc=$?
  local recovered_id=''
  trap - EXIT
  if (( shell_rc != 0 && FIRST_FAILURE == 0 )); then FIRST_FAILURE="$shell_rc"; fi
  if [[ -z "$CID" && "$CREATE_ATTEMPTED" == 1 ]]; then
    record '89-recover-created-id' docker inspect -f '{{.Id}}' "$CONTAINER_NAME"
    if (( LAST_RC == 0 )); then
      recovered_id="$(read_first_line "$EVIDENCE/89-recover-created-id.stdout.txt")" || recovered_id=''
      if [[ "$recovered_id" =~ ^[0-9a-f]{64}$ ]]; then
        CID="$recovered_id"
        printf 'recovered_container_id=%s\n' "$CID" >> "$EVIDENCE/invocation.txt"
      fi
    fi
  fi
  if [[ -n "$CID" ]]; then
    record '90-final-inspect' docker inspect -f 'id={{.Id}} image={{.Image}} path={{.Path}} args={{json .Args}} state={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}} oom={{.State.OOMKilled}} network={{.HostConfig.NetworkMode}} capdrop={{json .HostConfig.CapDrop}} capadd={{json .HostConfig.CapAdd}} tmpfs={{json .HostConfig.Tmpfs}} mounts={{json .Mounts}} ports={{json .HostConfig.PortBindings}}' "$CID"
    record '91-remove-by-id' docker rm -f "$CID"
    record '92-post-cleanup-name-list' docker ps -a --filter "name=^/${CONTAINER_NAME}$" --format '{{.ID}} {{.Names}} {{.Status}}'
    if (( LAST_RC == 0 )) && [[ -n "$(read_first_line "$EVIDENCE/92-post-cleanup-name-list.stdout.txt")" ]]; then FIRST_FAILURE=72; fi
    record '93-protected-containers-after' docker ps -a --filter name=awg-manager-peer-test --filter name=awg-manager-api-test --filter name=awg-manager-full --format '{{.ID}} {{.Names}} {{.Status}}'
  elif [[ "$CREATE_ATTEMPTED" == 1 ]]; then
    record '89-unresolved-created-name-list' docker ps -a --filter "name=^/${CONTAINER_NAME}$" --format '{{.ID}} {{.Names}} {{.Status}}'
    if (( FIRST_FAILURE == 0 )); then FIRST_FAILURE=73; fi
  fi
  if (( FIRST_FAILURE == 0 )); then
    printf 'diagnostic=COMPLETE\nmode=%s\nreadiness=%s\nexit=0\ncontainer_id=%s\n' "$MODE" "$READINESS_RESULT" "$CID" > "$EVIDENCE/result.txt"
  else
    printf 'diagnostic=INCOMPLETE\nmode=%s\nreadiness=%s\nexit=%s\ncontainer_id=%s\n' "$MODE" "$READINESS_RESULT" "$FIRST_FAILURE" "$CID" > "$EVIDENCE/result.txt"
  fi
  printf 'finished_at=%s\n' "$(date -Iseconds)" >> "$EVIDENCE/result.txt"
  printf 'readiness=%s evidence=%s\n' "$READINESS_RESULT" "$EVIDENCE"
  exit "$FIRST_FAILURE"
}
trap cleanup EXIT

if [[ "$MODE" != '--run' ]]; then
  printf 'usage: %s --run\n' "$0" >&2
  FIRST_FAILURE=64
  exit 64
fi
if [[ ! -s "$SOURCE" ]]; then
  printf 'helper source missing or empty\n' > "$EVIDENCE/precondition.stderr.txt"
  FIRST_FAILURE=41
  exit 41
fi

record '01-image-inspect' docker image inspect -f '{{.Id}}' "$IMAGE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
IMAGE_ID="$(read_first_line "$EVIDENCE/01-image-inspect.stdout.txt")" || { FIRST_FAILURE=46; exit 46; }
if [[ "$IMAGE_ID" != 'sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d' ]]; then FIRST_FAILURE=47; exit 47; fi
printf 'image_id=%s\n' "$IMAGE_ID" >> "$EVIDENCE/invocation.txt"
record '02-name-precheck' docker ps -a --filter "name=^/${CONTAINER_NAME}$" --format '{{.ID}} {{.Names}}'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
if [[ -n "$(read_first_line "$EVIDENCE/02-name-precheck.stdout.txt")" ]]; then FIRST_FAILURE=48; exit 48; fi
record '03-protected-containers-before' docker ps -a --filter name=awg-manager-peer-test --filter name=awg-manager-api-test --filter name=awg-manager-full --format '{{.ID}} {{.Names}} {{.Status}}'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '03-host-source-size' wc -c "$SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '03-host-source-sha256' sha256sum "$SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
read -r HOST_SOURCE_SIZE _ < "$EVIDENCE/03-host-source-size.stdout.txt" || { FIRST_FAILURE=49; exit 49; }
read -r HOST_SOURCE_HASH _ < "$EVIDENCE/03-host-source-sha256.stdout.txt" || { FIRST_FAILURE=49; exit 49; }
if [[ "$HOST_SOURCE_SIZE" != "$EXPECTED_SOURCE_SIZE" || "$HOST_SOURCE_HASH" != "$EXPECTED_SOURCE_SHA256" ]]; then
  FIRST_FAILURE=50
  printf 'pinned_source_gate=FAIL\nactual_size=%s\nactual_sha256=%s\n' "$HOST_SOURCE_SIZE" "$HOST_SOURCE_HASH" > "$EVIDENCE/03-pinned-source-gate.txt"
  exit 50
fi
printf 'pinned_source_gate=PASS\nsize=%s\nsha256=%s\n' "$HOST_SOURCE_SIZE" "$HOST_SOURCE_HASH" > "$EVIDENCE/03-pinned-source-gate.txt"

CREATE_ATTEMPTED=1
record '04-container-create' docker run -d --pull never --network none --cap-drop ALL --cap-add NET_RAW \
  --tmpfs /tmp:rw,size=1g,mode=1777 --name "$CONTAINER_NAME" --entrypoint /bin/sh \
  "$IMAGE_ID" -ec 'sleep 600'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
CID="$(read_first_line "$EVIDENCE/04-container-create.stdout.txt")" || CID=''
if [[ ! "$CID" =~ ^[0-9a-f]{64}$ ]]; then FIRST_FAILURE=43; CID=''; exit 43; fi
printf 'container_id=%s\n' "$CID" >> "$EVIDENCE/invocation.txt"
record '05-inspect-before-copy' docker inspect -f 'id={{.Id}} image={{.Image}} state={{.State.Status}} network={{.HostConfig.NetworkMode}} capdrop={{json .HostConfig.CapDrop}} capadd={{json .HostConfig.CapAdd}} tmpfs={{json .HostConfig.Tmpfs}} mounts={{json .Mounts}} ports={{json .HostConfig.PortBindings}}' "$CID"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '05-source-path-absent' docker exec "$CID" sh -c '[ ! -e /capture-helper-control.go ] && [ ! -L /capture-helper-control.go ] && [ ! -e /tmp/gateway_direct_flow_capture.go ] && [ ! -L /tmp/gateway_direct_flow_capture.go ]'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '06-docker-cp-control' docker cp "$SOURCE_NATIVE" "$CID:/capture-helper-control.go"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-type' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /capture-helper-control.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-size' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then wc -c < "$1"; else printf ABSENT; fi' sh /capture-helper-control.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-sha256' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then sha256sum "$1"; else printf ABSENT; fi' sh /capture-helper-control.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
read -r CONTROL_TYPE < "$EVIDENCE/06-control-type.stdout.txt"
read -r CONTROL_SIZE < "$EVIDENCE/06-control-size.stdout.txt"
read -r CONTROL_HASH _ < "$EVIDENCE/06-control-sha256.stdout.txt"
if [[ "$CONTROL_TYPE" != REGULAR_FILE || "$CONTROL_SIZE" != "$HOST_SOURCE_SIZE" || "$CONTROL_HASH" != "$HOST_SOURCE_HASH" ]]; then
  FIRST_FAILURE=51
  printf 'control_source_gate=FAIL\n' > "$EVIDENCE/06-control-source-gate.txt"
  exit 51
fi
printf 'control_source_gate=PASS\nsize=%s\nsha256=%s\n' "$CONTROL_SIZE" "$CONTROL_HASH" > "$EVIDENCE/06-control-source-gate.txt"
record '06-copy-source-to-tmpfs' docker exec "$CID" cp /capture-helper-control.go /tmp/gateway_direct_flow_capture.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-runtime-mounts' docker exec "$CID" sh -c 'set -f; root_seen=0; tmp_seen=0; root_noexec=0; tmp_noexec=0; tmpfs_seen=0; while IFS= read -r line; do set -- $line; mountpoint=$5; opts=$6; if [ "$mountpoint" = "/" ]; then root_seen=1; printf "ROOT_MOUNTINFO=%s\n" "$line"; case ",$opts," in *,noexec,*) root_noexec=1;; esac; elif [ "$mountpoint" = "/tmp" ]; then tmp_seen=1; printf "TMP_MOUNTINFO=%s\n" "$line"; case ",$opts," in *,noexec,*) tmp_noexec=1;; esac; case "$line" in *" - tmpfs tmpfs "*) tmpfs_seen=1;; esac; fi; done < /proc/self/mountinfo; [ "$root_seen" -eq 1 ] && [ "$root_noexec" -eq 0 ] && [ "$tmp_seen" -eq 1 ] && [ "$tmpfs_seen" -eq 1 ] && [ "$tmp_noexec" -eq 1 ]'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '08-timeout-discovery' docker exec "$CID" sh -c 'command -v timeout'
if (( LAST_RC != 0 )); then FIRST_FAILURE=52; exit 52; fi
TIMEOUT_BIN="$(read_first_line "$EVIDENCE/08-timeout-discovery.stdout.txt")" || { FIRST_FAILURE=53; exit 53; }
record '08-timeout-version' docker exec "$CID" "$TIMEOUT_BIN" --version
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
TIMEOUT_VERSION="$(<"$EVIDENCE/08-timeout-version.stdout.txt")"
if [[ "$TIMEOUT_VERSION" != *'GNU coreutils'* ]]; then FIRST_FAILURE=54; exit 54; fi

record '09-copy-tmp-size' docker exec "$CID" sh -c 'wc -c < /tmp/gateway_direct_flow_capture.go'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '09-copy-tmp-sha256' docker exec "$CID" sha256sum /tmp/gateway_direct_flow_capture.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '09-copy-tmp-type' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /tmp/gateway_direct_flow_capture.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
read -r TMP_SOURCE_SIZE < "$EVIDENCE/09-copy-tmp-size.stdout.txt"
read -r TMP_SOURCE_HASH _ < "$EVIDENCE/09-copy-tmp-sha256.stdout.txt"
read -r TMP_SOURCE_TYPE < "$EVIDENCE/09-copy-tmp-type.stdout.txt"
if [[ "$TMP_SOURCE_TYPE" != REGULAR_FILE || "$TMP_SOURCE_SIZE" != "$HOST_SOURCE_SIZE" || "$TMP_SOURCE_HASH" != "$HOST_SOURCE_HASH" ]]; then
  FIRST_FAILURE=55
  printf 'tmpfs_source_gate=FAIL\n' > "$EVIDENCE/09-tmpfs-source-gate.txt"
  exit 55
fi
printf 'tmpfs_source_gate=PASS\nsize=%s\nsha256=%s\n' "$TMP_SOURCE_SIZE" "$TMP_SOURCE_HASH" > "$EVIDENCE/09-tmpfs-source-gate.txt"

record '10-gofmt-diff' docker exec "$CID" sh -c 'cd /tmp && /usr/local/go/bin/gofmt -d gateway_direct_flow_capture.go'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
if [[ -s "$EVIDENCE/10-gofmt-diff.stdout.txt" ]]; then FIRST_FAILURE=56; READINESS_RESULT='GOFMT_FAIL'; exit 56; fi
printf 'gofmt_gate=PASS\n' > "$EVIDENCE/10-gofmt-result.txt"
record '11-build-tempdirs' docker exec "$CID" sh -c 'mkdir -p /tmp/gocache /tmp/gotmp && [ -d /tmp/gocache ] && [ -w /tmp/gocache ] && [ -d /tmp/gotmp ] && [ -w /tmp/gotmp ] && printf BUILD_TEMP_DIRS_READY'
if (( LAST_RC != 0 )); then READINESS_RESULT='BUILD_TEMP_SETUP_FAIL'; exit "$LAST_RC"; fi
record '12-go-build' docker exec -w /tmp "$CID" "$TIMEOUT_BIN" --signal=KILL 120s env GOCACHE=/tmp/gocache GOTMPDIR=/tmp/gotmp GO111MODULE=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0 /usr/local/go/bin/go build -trimpath -o /capture-helper gateway_direct_flow_capture.go
if (( LAST_RC != 0 )); then READINESS_RESULT='BUILD_FAIL'; exit "$LAST_RC"; fi
record '13-binary-metadata' docker exec "$CID" sh -c 'if [ -L /capture-helper ] || [ ! -f /capture-helper ] || [ ! -x /capture-helper ]; then printf INVALID_BINARY; exit 62; fi; wc -c < /capture-helper; sha256sum /capture-helper'
if (( LAST_RC != 0 )); then READINESS_RESULT='BINARY_INVALID'; exit "$LAST_RC"; fi

record '14-capture-run' docker exec "$CID" "$TIMEOUT_BIN" --signal=KILL 5s /capture-helper -ifaces lo -ips 10.66.0.3 -duration 1s
CAPTURE_STDOUT="$(<"$EVIDENCE/14-capture-run.stdout.txt")"
CAPTURE_STDERR="$(<"$EVIDENCE/14-capture-run.stderr.txt")"
if (( LAST_RC != 0 )); then
  READINESS_RESULT='RUN_FAIL_OR_WATCHDOG'
  printf 'readiness=FAIL\nexit=%s\n' "$LAST_RC" > "$EVIDENCE/14-readiness-result.txt"
  exit "$LAST_RC"
fi
if [[ -n "$CAPTURE_STDERR" ]]; then
  FIRST_FAILURE=63
  READINESS_RESULT='UNEXPECTED_STDERR'
  printf 'readiness=FAIL\nreason=nonempty-stderr\n' > "$EVIDENCE/14-readiness-result.txt"
  exit 63
fi
READY_COUNT=0
CAPTURE_READY_COUNT=0
DONE_COUNT=0
STATISTICS_COUNT=0
PHASE=0
UNEXPECTED=''
while IFS= read -r line; do
  case "$line" in
    READY_BOUND\ *)
      if [[ "$line" =~ ^READY_BOUND[[:space:]]iface=lo[[:space:]]ifindex=([1-9][0-9]*)$ ]] && (( PHASE == 0 )); then
        READY_COUNT=$((READY_COUNT + 1)); PHASE=1
      else
        UNEXPECTED+="invalid_or_out_of_order_ready_bound: $line"$'\n'
      fi
      ;;
    CAPTURE_READY\ *)
      if [[ "$line" == 'CAPTURE_READY interfaces=1 duration=1s' ]] && (( PHASE == 1 )); then
        CAPTURE_READY_COUNT=$((CAPTURE_READY_COUNT + 1)); PHASE=2
      else
        UNEXPECTED+="invalid_or_out_of_order_capture_ready: $line"$'\n'
      fi
      ;;
    PACKET_STATISTICS\ *)
      if [[ "$line" =~ ^PACKET_STATISTICS[[:space:]]iface=lo[[:space:]]packets=([0-9]+)[[:space:]]drops=([0-9]+)$ ]] && (( PHASE == 2 )); then
        STATISTICS_COUNT=$((STATISTICS_COUNT + 1)); PHASE=2
      else
        UNEXPECTED+="invalid_or_out_of_order_packet_statistics: $line"$'\n'
      fi
      ;;
    CAPTURE_DONE)
      if (( PHASE == 2 )); then DONE_COUNT=$((DONE_COUNT + 1)); PHASE=3; else UNEXPECTED+="invalid_or_out_of_order_capture_done"$'\n'; fi
      ;;
    CAPTURE_ERROR*|CAPTURE_FAILED*|CAPTURE_READ_ERROR*|PANIC:*|panic:*) UNEXPECTED+="$line"$'\n' ;;
    PACKET\ *) UNEXPECTED+="unexpected_packet: $line"$'\n' ;;
    *) UNEXPECTED+="unexpected_stdout: $line"$'\n' ;;
  esac
done < "$EVIDENCE/14-capture-run.stdout.txt"
if (( READY_COUNT != 1 || CAPTURE_READY_COUNT != 1 || STATISTICS_COUNT != 1 || DONE_COUNT != 1 || PHASE != 3 )) || [[ -n "$UNEXPECTED" ]]; then
  FIRST_FAILURE=64
  READINESS_RESULT='MARKER_OR_PACKET_GATE_FAIL'
  printf 'readiness=FAIL\nready_bound_count=%s\ncapture_ready_count=%s\npacket_statistics_count=%s\ncapture_done_count=%s\nunexpected=%s\n' "$READY_COUNT" "$CAPTURE_READY_COUNT" "$STATISTICS_COUNT" "$DONE_COUNT" "$UNEXPECTED" > "$EVIDENCE/14-readiness-result.txt"
  exit 64
fi
record '15-host-source-size-after' wc -c "$SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '15-host-source-sha256-after' sha256sum "$SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
read -r SOURCE_SIZE_AFTER _ < "$EVIDENCE/15-host-source-size-after.stdout.txt"
read -r SOURCE_HASH_AFTER _ < "$EVIDENCE/15-host-source-sha256-after.stdout.txt"
if [[ "$SOURCE_SIZE_AFTER" != "$EXPECTED_SOURCE_SIZE" || "$SOURCE_HASH_AFTER" != "$EXPECTED_SOURCE_SHA256" ]]; then FIRST_FAILURE=65; exit 65; fi
READINESS_RESULT='LOOPBACK_READY'
printf 'readiness=PASS\nready_bound_count=%s\ncapture_ready_count=%s\npacket_statistics_count=%s\ncapture_done_count=%s\nsource_size=%s\nsource_sha256=%s\n' "$READY_COUNT" "$CAPTURE_READY_COUNT" "$STATISTICS_COUNT" "$DONE_COUNT" "$SOURCE_SIZE_AFTER" "$SOURCE_HASH_AFTER" > "$EVIDENCE/15-readiness-result.txt"
exit 0

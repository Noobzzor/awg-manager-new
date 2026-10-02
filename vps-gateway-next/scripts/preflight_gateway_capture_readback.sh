#!/usr/bin/env bash
# Verify Docker copy and source readback only; does not build or run capture code.
set -uo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="$ROOT/scripts/gateway_direct_flow_capture.go"
SOURCE_NATIVE='C:/Users/olesh/hermes_work/_snapshots/awg-manager_docker_gateway_gate_20260928/scripts/gateway_direct_flow_capture.go'
PACK_ROOT="${REVIEW_PACK_DIR:-$ROOT/review-packs/2026-09-30-direct-flow}"
EVIDENCE_ROOT="$PACK_ROOT/02-source-readback"
IMAGE="golang:1.26-bookworm"
MODE="${1:-}"
RUN_SUFFIX="$(date +%s)-$$-$RANDOM-$RANDOM"
CONTAINER_NAME="awg-direct-source-readback-$RUN_SUFFIX"
EVIDENCE="$EVIDENCE_ROOT/$RUN_SUFFIX"
CID=''
LAST_RC=0
FIRST_FAILURE=0

mkdir -p "$EVIDENCE"
printf 'started_at=%s\nmode=%s\nroot=%s\nsource=%s\nsource_native=%s\ncontainer_name=%s\nimage=%s\n' \
  "$(date -Iseconds)" "$MODE" "$ROOT" "$SOURCE" "$SOURCE_NATIVE" "$CONTAINER_NAME" "$IMAGE" > "$EVIDENCE/invocation.txt"

record() {
  local label="$1"; shift
  local rc=0
  {
    printf '[%s]' "$label"
    printf ' %q' "$@"
    printf '\n'
  } >> "$EVIDENCE/commands.txt"
  if "$@" >"$EVIDENCE/$label.stdout.txt" 2>"$EVIDENCE/$label.stderr.txt"; then
    rc=0
  else
    rc=$?
  fi
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

if [[ "$MODE" == '--self-test' ]]; then
  TEST_DIR="$EVIDENCE/self-test"
  mkdir -p "$TEST_DIR/path with spaces"
  PREVIOUS_IMAGE_FILE="$EVIDENCE_ROOT/01-image-inspect.stdout.txt"
  if ! IMAGE_ID="$(read_first_line "$PREVIOUS_IMAGE_FILE")"; then
    printf 'SELFTEST=FAIL step=read-existing-image-id\n'
    exit 51
  fi
  if [[ ! "$IMAGE_ID" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    printf 'SELFTEST=FAIL step=validate-image-id value=%s\n' "$IMAGE_ID"
    exit 52
  fi
  printf '%s\n' 'path with spaces ok' > "$TEST_DIR/path with spaces/value.txt"
  printf 'crlf value\r\n' > "$TEST_DIR/crlf.txt"
  printf '%s' 'no final newline' > "$TEST_DIR/no-final-newline.txt"
  printf '%s\n' 'sha256:bad' > "$TEST_DIR/malformed-image-id.txt"
  if [[ "$(read_first_line "$TEST_DIR/path with spaces/value.txt")" != 'path with spaces ok' ]]; then
    printf 'SELFTEST=FAIL step=path-with-spaces\n'; exit 53
  fi
  if [[ "$(read_first_line "$TEST_DIR/crlf.txt")" != 'crlf value' ]]; then
    printf 'SELFTEST=FAIL step=crlf\n'; exit 54
  fi
  if [[ "$(read_first_line "$TEST_DIR/no-final-newline.txt")" != 'no final newline' ]]; then
    printf 'SELFTEST=FAIL step=no-final-newline\n'; exit 55
  fi
  if [[ "$(read_first_line "$TEST_DIR/malformed-image-id.txt")" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    printf 'SELFTEST=FAIL step=malformed-id-accepted\n'; exit 56
  fi
  if read_first_line "$TEST_DIR/missing.txt" >/dev/null 2>&1; then
    printf 'SELFTEST=FAIL step=missing-file-accepted\n'; exit 57
  fi
  : > "$TEST_DIR/empty.txt"
  if ! read_first_line "$TEST_DIR/empty.txt" >/dev/null 2>&1; then
    printf 'SELFTEST=FAIL step=empty-file-read\n'; exit 58
  fi
  record 'self-test-name-precheck' docker ps -a --filter "name=^/${CONTAINER_NAME}$" --format '{{.ID}} {{.Names}}'
  if (( LAST_RC != 0 )); then printf 'SELFTEST=FAIL step=docker-name-precheck exit=%s\n' "$LAST_RC"; exit "$LAST_RC"; fi
  if [[ -n "$(read_first_line "$EVIDENCE/self-test-name-precheck.stdout.txt")" ]]; then
    printf 'SELFTEST=FAIL step=unexpected-name-collision\n'; exit 59
  fi
  printf 'SELFTEST=PASS image_id=%s no_helper_created=true\n' "$IMAGE_ID"
  exit 0
fi

if [[ "$MODE" != '--create-helper' && "$MODE" != '--two-stage-copy' ]]; then
  printf 'usage: %s --self-test | --create-helper | --two-stage-copy\n' "$0" >&2
  exit 64
fi

cleanup() {
  local shell_rc=$?
  trap - EXIT
  if (( shell_rc != 0 && FIRST_FAILURE == 0 )); then FIRST_FAILURE="$shell_rc"; fi
  if [[ -n "$CID" ]]; then
    record '08-final-inspect' docker inspect -f 'id={{.Id}} image={{.Image}} path={{.Path}} args={{json .Args}} user={{.Config.User}} workdir={{.Config.WorkingDir}} state={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}} oom={{.State.OOMKilled}} network={{.HostConfig.NetworkMode}} capdrop={{json .HostConfig.CapDrop}} capadd={{json .HostConfig.CapAdd}} tmpfs={{json .HostConfig.Tmpfs}} mounts={{json .Mounts}} ports={{json .HostConfig.PortBindings}} networks={{json .NetworkSettings.Networks}}' "$CID"
    record '09-remove-by-id' docker rm -f "$CID"
  fi
  if (( FIRST_FAILURE == 0 )); then
    printf 'diagnostic=COMPLETE\nexit=0\ncontainer_id=%s\n' "$CID" > "$EVIDENCE/result.txt"
  else
    printf 'diagnostic=INCOMPLETE\nexit=%s\ncontainer_id=%s\n' "$FIRST_FAILURE" "$CID" > "$EVIDENCE/result.txt"
  fi
  printf 'finished_at=%s\n' "$(date -Iseconds)" >> "$EVIDENCE/result.txt"
  exit "$FIRST_FAILURE"
}
trap cleanup EXIT

if [[ ! -s "$SOURCE" ]]; then
  printf 'source missing or empty: %s\n' "$SOURCE" > "$EVIDENCE/precondition.stderr.txt"
  printf '41\n' > "$EVIDENCE/precondition.exit.txt"
  FIRST_FAILURE=41
  exit 41
fi

record '01-image-inspect' docker image inspect -f '{{.Id}}' "$IMAGE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
IMAGE_ID="$(read_first_line "$EVIDENCE/01-image-inspect.stdout.txt")" || { FIRST_FAILURE=46; exit 46; }
if [[ ! "$IMAGE_ID" =~ ^sha256:[0-9a-f]{64}$ ]]; then FIRST_FAILURE=47; exit 47; fi
printf 'image_id=%s\n' "$IMAGE_ID" >> "$EVIDENCE/invocation.txt"

record '02-name-precheck' docker ps -a --filter "name=^/${CONTAINER_NAME}$" --format '{{.ID}} {{.Names}}'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
PREEXISTING="$(read_first_line "$EVIDENCE/02-name-precheck.stdout.txt")" || { FIRST_FAILURE=48; exit 48; }
if [[ -n "$PREEXISTING" ]]; then
  printf 'refusing pre-existing container name %s\n' "$CONTAINER_NAME" > "$EVIDENCE/name-collision.stderr.txt"
  printf '42\n' > "$EVIDENCE/name-collision.exit.txt"
  FIRST_FAILURE=42
  exit 42
fi

record '03-host-source-size' wc -c "$SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '03-host-source-sha256' sha256sum "$SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '04-container-create' docker run -d --pull never --network none --cap-drop ALL --cap-add NET_RAW \
  --tmpfs /tmp:rw,size=1g,mode=1777 --name "$CONTAINER_NAME" --entrypoint /bin/sh \
  "$IMAGE_ID" -ec 'sleep 300'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
CID="$(read_first_line "$EVIDENCE/04-container-create.stdout.txt")" || CID=''
if [[ -z "$CID" ]]; then
  record '04-recover-created-id' docker inspect -f '{{.Id}}' "$CONTAINER_NAME"
  if (( LAST_RC == 0 )); then CID="$(read_first_line "$EVIDENCE/04-recover-created-id.stdout.txt")" || CID=''; fi
fi
if [[ ! "$CID" =~ ^[0-9a-f]{64}$ ]]; then FIRST_FAILURE=43; exit 43; fi
printf 'container_id=%s\n' "$CID" >> "$EVIDENCE/invocation.txt"

record '05-inspect-before-copy' docker inspect -f 'id={{.Id}} image={{.Image}} path={{.Path}} args={{json .Args}} user={{.Config.User}} workdir={{.Config.WorkingDir}} state={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}} oom={{.State.OOMKilled}} network={{.HostConfig.NetworkMode}} capdrop={{json .HostConfig.CapDrop}} capadd={{json .HostConfig.CapAdd}} tmpfs={{json .HostConfig.Tmpfs}} mounts={{json .Mounts}} ports={{json .HostConfig.PortBindings}} networks={{json .NetworkSettings.Networks}}' "$CID"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '06-paths-before-copy' docker exec "$CID" sh -c 'for path in /capture-transfer-control.go /tmp/capture.go; do if [ -L "$path" ]; then printf "EXISTS_SYMLINK %s\n" "$path"; elif [ -e "$path" ]; then printf "EXISTS %s\n" "$path"; else printf "ABSENT %s\n" "$path"; fi; done; [ ! -e /capture-transfer-control.go ] && [ ! -L /capture-transfer-control.go ] && [ ! -e /tmp/capture.go ] && [ ! -L /tmp/capture.go ]'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '06-docker-cp-control' docker cp "$SOURCE_NATIVE" "$CID:/capture-transfer-control.go"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-type-before-inner-copy' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /capture-transfer-control.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-size-before-inner-copy' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then wc -c < "$1"; else printf ABSENT; fi' sh /capture-transfer-control.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-sha256-before-inner-copy' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then sha256sum "$1"; else printf ABSENT; fi' sh /capture-transfer-control.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
read -r HOST_SIZE_PRE _ < "$EVIDENCE/03-host-source-size.stdout.txt" || { FIRST_FAILURE=49; exit 49; }
read -r HOST_HASH_PRE _ < "$EVIDENCE/03-host-source-sha256.stdout.txt" || { FIRST_FAILURE=49; exit 49; }
read -r CONTROL_TYPE_PRE < "$EVIDENCE/06-control-type-before-inner-copy.stdout.txt"
read -r CONTROL_SIZE_PRE < "$EVIDENCE/06-control-size-before-inner-copy.stdout.txt"
read -r CONTROL_HASH_PRE _ < "$EVIDENCE/06-control-sha256-before-inner-copy.stdout.txt"
if [[ "$CONTROL_TYPE_PRE" != REGULAR_FILE || "$CONTROL_SIZE_PRE" != "$HOST_SIZE_PRE" || "$CONTROL_HASH_PRE" != "$HOST_HASH_PRE" ]]; then
  FIRST_FAILURE=51
  printf 'control_source_gate=FAIL\n' > "$EVIDENCE/06-control-source-gate.txt"
  exit 51
fi
printf 'control_source_gate=PASS\nsize=%s\nsha256=%s\n' "$CONTROL_SIZE_PRE" "$CONTROL_HASH_PRE" > "$EVIDENCE/06-control-source-gate.txt"

if [[ "$MODE" == '--create-helper' ]]; then
  record '06-docker-cp-tmpfs' docker cp "$SOURCE_NATIVE" "$CID:/tmp/capture.go"
else
  record '06-internal-cp-to-tmpfs' docker exec "$CID" cp /capture-transfer-control.go /tmp/capture.go
fi
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '07-exec-marker' docker exec "$CID" sh -c 'printf EXEC_READY'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-command-discovery' docker exec "$CID" sh -c 'command -v sh; command -v wc; command -v sha256sum; command -v ls'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-runtime-metadata' docker exec "$CID" sh -c 'printf "pwd="; pwd; while IFS= read -r line; do case "$line" in *" /tmp "*) printf "tmp_mount=%s\\n" "$line";; esac; done < /proc/self/mountinfo'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '07-control-type' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /capture-transfer-control.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-control-size' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then wc -c < "$1"; else printf ABSENT; fi' sh /capture-transfer-control.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-control-sha256' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then sha256sum "$1"; else printf ABSENT; fi' sh /capture-transfer-control.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-tmp-type' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /tmp/capture.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-tmp-size' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then wc -c < "$1"; else printf ABSENT; fi' sh /tmp/capture.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-tmp-sha256' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then sha256sum "$1"; else printf ABSENT; fi' sh /tmp/capture.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '10-host-source-size-after' wc -c "$SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '10-host-source-sha256-after' sha256sum "$SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

read -r HOST_SIZE _ < "$EVIDENCE/03-host-source-size.stdout.txt" || { FIRST_FAILURE=49; exit 49; }
read -r HOST_HASH _ < "$EVIDENCE/03-host-source-sha256.stdout.txt" || { FIRST_FAILURE=49; exit 49; }
read -r HOST_SIZE_AFTER _ < "$EVIDENCE/10-host-source-size-after.stdout.txt" || { FIRST_FAILURE=49; exit 49; }
read -r HOST_HASH_AFTER _ < "$EVIDENCE/10-host-source-sha256-after.stdout.txt" || { FIRST_FAILURE=49; exit 49; }
read -r CONTROL_TYPE < "$EVIDENCE/07-control-type.stdout.txt"
read -r CONTROL_SIZE < "$EVIDENCE/07-control-size.stdout.txt"
read -r CONTROL_HASH _ < "$EVIDENCE/07-control-sha256.stdout.txt"
read -r TMP_TYPE < "$EVIDENCE/07-tmp-type.stdout.txt"
read -r TMP_SIZE < "$EVIDENCE/07-tmp-size.stdout.txt"
read -r TMP_HASH _ < "$EVIDENCE/07-tmp-sha256.stdout.txt"
CONTROL_MATCH=false
TMP_MATCH=false
[[ "$CONTROL_TYPE" == REGULAR_FILE && "$CONTROL_SIZE" == "$HOST_SIZE" && "$CONTROL_HASH" == "$HOST_HASH" ]] && CONTROL_MATCH=true
[[ "$TMP_TYPE" == REGULAR_FILE && "$TMP_SIZE" == "$HOST_SIZE" && "$TMP_HASH" == "$HOST_HASH" ]] && TMP_MATCH=true
if [[ "$HOST_SIZE" != "$HOST_SIZE_AFTER" || "$HOST_HASH" != "$HOST_HASH_AFTER" ]]; then
  FIRST_FAILURE=50
  SOURCE_STABLE=false
else
  SOURCE_STABLE=true
fi
if [[ "$CONTROL_MATCH" == true && "$TMP_MATCH" == true ]]; then
  TRANSFER_RESULT=both_match
elif [[ "$CONTROL_MATCH" == true ]]; then
  TRANSFER_RESULT=control_only
elif [[ "$TMP_MATCH" == true ]]; then
  TRANSFER_RESULT=tmpfs_only
else
  TRANSFER_RESULT=neither_match
fi
printf 'host_size_before=%s\nhost_size_after=%s\nhost_sha256_before=%s\nhost_sha256_after=%s\ncontrol_type=%s\ncontrol_size=%s\ncontrol_sha256=%s\ncontrol_matches_host=%s\ntmp_type=%s\ntmp_size=%s\ntmp_sha256=%s\ntmp_matches_host=%s\nsource_stable=%s\ntransfer_result=%s\n' \
  "$HOST_SIZE" "$HOST_SIZE_AFTER" "$HOST_HASH" "$HOST_HASH_AFTER" \
  "$CONTROL_TYPE" "$CONTROL_SIZE" "$CONTROL_HASH" "$CONTROL_MATCH" \
  "$TMP_TYPE" "$TMP_SIZE" "$TMP_HASH" "$TMP_MATCH" "$SOURCE_STABLE" "$TRANSFER_RESULT" > "$EVIDENCE/11-ab-comparison.txt"
printf 'A_B_RESULT=%s source_stable=%s evidence=%s\n' "$TRANSFER_RESULT" "$SOURCE_STABLE" "$EVIDENCE"
exit 0

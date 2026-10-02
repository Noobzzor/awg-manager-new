#!/usr/bin/env bash
# Run isolated RED/GREEN tests for the diagnostic AF_PACKET helper only.
set -uo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="$ROOT/scripts/gateway_direct_flow_capture.go"
TEST_SOURCE="$ROOT/scripts/gateway_direct_flow_capture_test.go"
LINK_TYPE_TEST_SOURCE="$ROOT/scripts/gateway_direct_flow_capture_packet_test.go"
SOURCE_NATIVE='C:/Users/olesh/hermes_work/_snapshots/awg-manager_docker_gateway_gate_20260928/scripts/gateway_direct_flow_capture.go'
TEST_SOURCE_NATIVE='C:/Users/olesh/hermes_work/_snapshots/awg-manager_docker_gateway_gate_20260928/scripts/gateway_direct_flow_capture_test.go'
LINK_TYPE_TEST_SOURCE_NATIVE='C:/Users/olesh/hermes_work/_snapshots/awg-manager_docker_gateway_gate_20260928/scripts/gateway_direct_flow_capture_packet_test.go'
PACK_ROOT="${REVIEW_PACK_DIR:-$ROOT/review-packs/2026-09-30-direct-flow}"
EVIDENCE_ROOT="$PACK_ROOT/05-capture-helper-tests"
MODE="${1:-}"
RUN_SUFFIX="$(date +%s)-$$-$RANDOM-$RANDOM"
CONTAINER_NAME="awg-direct-capture-test-$RUN_SUFFIX"
EVIDENCE="$EVIDENCE_ROOT/$RUN_SUFFIX"
IMAGE="golang:1.26-bookworm"
CID=''
LAST_RC=0
FIRST_FAILURE=0
TEST_RESULT='NOT_RUN'
EXPECTED_RED=0

mkdir -p "$EVIDENCE"
printf 'started_at=%s\nmode=%s\nroot=%s\nsource=%s\ntest_source=%s\nlink_type_test_source=%s\ncontainer_name=%s\nimage=%s\n' \
  "$(date -Iseconds)" "$MODE" "$ROOT" "$SOURCE" "$TEST_SOURCE" "$LINK_TYPE_TEST_SOURCE" "$CONTAINER_NAME" "$IMAGE" > "$EVIDENCE/invocation.txt"

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
  trap - EXIT
  if (( shell_rc != 0 && FIRST_FAILURE == 0 )); then FIRST_FAILURE="$shell_rc"; fi
  if [[ -n "$CID" ]]; then
    record '90-final-inspect' docker inspect -f 'id={{.Id}} image={{.Image}} path={{.Path}} args={{json .Args}} state={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}} oom={{.State.OOMKilled}} network={{.HostConfig.NetworkMode}} capdrop={{json .HostConfig.CapDrop}} capadd={{json .HostConfig.CapAdd}} tmpfs={{json .HostConfig.Tmpfs}} mounts={{json .Mounts}} ports={{json .HostConfig.PortBindings}}' "$CID"
    record '91-remove-by-id' docker rm -f "$CID"
    record '92-post-cleanup-name-list' docker ps -a --filter "name=^/${CONTAINER_NAME}$" --format '{{.ID}} {{.Names}} {{.Status}}'
    if (( LAST_RC == 0 )) && [[ -n "$(read_first_line "$EVIDENCE/92-post-cleanup-name-list.stdout.txt")" ]]; then FIRST_FAILURE=72; fi
    record '93-protected-containers-after' docker ps -a --filter name=awg-manager-peer-test --filter name=awg-manager-api-test --filter name=awg-manager-full --format '{{.ID}} {{.Names}} {{.Status}}'
  fi
  if (( FIRST_FAILURE == 0 )); then
    printf 'diagnostic=COMPLETE\nmode=%s\ntest_result=%s\nexit=0\ncontainer_id=%s\n' "$MODE" "$TEST_RESULT" "$CID" > "$EVIDENCE/result.txt"
  else
    printf 'diagnostic=INCOMPLETE\nmode=%s\ntest_result=%s\nexit=%s\ncontainer_id=%s\n' "$MODE" "$TEST_RESULT" "$FIRST_FAILURE" "$CID" > "$EVIDENCE/result.txt"
  fi
  printf 'finished_at=%s\n' "$(date -Iseconds)" >> "$EVIDENCE/result.txt"
  printf 'test_result=%s evidence=%s\n' "$TEST_RESULT" "$EVIDENCE"
  exit "$FIRST_FAILURE"
}
trap cleanup EXIT

if [[ "$MODE" != '--red' && "$MODE" != '--parser-red' && "$MODE" != '--decoder-red' && "$MODE" != '--linktype-red' && "$MODE" != '--metadata-red' && "$MODE" != '--race-red' && "$MODE" != '--green' ]]; then
  printf 'usage: %s --red | --parser-red | --decoder-red | --linktype-red | --metadata-red | --race-red | --green\n' "$0" >&2
  FIRST_FAILURE=64
  exit 64
fi
if [[ ! -s "$SOURCE" || ! -s "$TEST_SOURCE" || ! -s "$LINK_TYPE_TEST_SOURCE" ]]; then
  printf 'source or test source missing/empty\n' > "$EVIDENCE/precondition.stderr.txt"
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
if [[ -n "$(read_first_line "$EVIDENCE/02-name-precheck.stdout.txt")" ]]; then FIRST_FAILURE=42; exit 42; fi

record '03-protected-containers-before' docker ps -a --filter name=awg-manager-peer-test --filter name=awg-manager-api-test --filter name=awg-manager-full --format '{{.ID}} {{.Names}} {{.Status}}'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '03-source-size' wc -c "$SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '03-source-sha256' sha256sum "$SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '03-test-source-size' wc -c "$TEST_SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '03-test-source-sha256' sha256sum "$TEST_SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '03-linktype-test-size' wc -c "$LINK_TYPE_TEST_SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '03-linktype-test-sha256' sha256sum "$LINK_TYPE_TEST_SOURCE"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '04-container-create' docker run -d --pull never --network none --cap-drop ALL --cap-add NET_RAW \
  --tmpfs /tmp:rw,size=1g,mode=1777 --name "$CONTAINER_NAME" --entrypoint /bin/sh \
  "$IMAGE_ID" -ec 'sleep 600'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
CID="$(read_first_line "$EVIDENCE/04-container-create.stdout.txt")" || CID=''
if [[ ! "$CID" =~ ^[0-9a-f]{64}$ ]]; then FIRST_FAILURE=43; exit 43; fi
printf 'container_id=%s\n' "$CID" >> "$EVIDENCE/invocation.txt"
record '05-inspect-before-copy' docker inspect -f 'id={{.Id}} image={{.Image}} state={{.State.Status}} network={{.HostConfig.NetworkMode}} capdrop={{json .HostConfig.CapDrop}} capadd={{json .HostConfig.CapAdd}} tmpfs={{json .HostConfig.Tmpfs}} mounts={{json .Mounts}} ports={{json .HostConfig.PortBindings}}' "$CID"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '05-source-paths-absent' docker exec "$CID" sh -c '[ ! -e /capture-helper.go ] && [ ! -L /capture-helper.go ] && [ ! -e /capture-helper_test.go ] && [ ! -L /capture-helper_test.go ] && [ ! -e /capture-helper_linktype_test.go ] && [ ! -L /capture-helper_linktype_test.go ] && [ ! -e /tmp/capture-helper.go ] && [ ! -L /tmp/capture-helper.go ] && [ ! -e /tmp/capture-helper_test.go ] && [ ! -L /tmp/capture-helper_test.go ] && [ ! -e /tmp/capture-helper_linktype_test.go ] && [ ! -L /tmp/capture-helper_linktype_test.go ]'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '06-docker-cp-source' docker cp "$SOURCE_NATIVE" "$CID:/capture-helper.go"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-docker-cp-test' docker cp "$TEST_SOURCE_NATIVE" "$CID:/capture-helper_test.go"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-docker-cp-linktype-test' docker cp "$LINK_TYPE_TEST_SOURCE_NATIVE" "$CID:/capture-helper_linktype_test.go"
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '06-control-source-type' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /capture-helper.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-source-size' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then wc -c < "$1"; else printf ABSENT; fi' sh /capture-helper.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-source-sha256' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then sha256sum "$1"; else printf ABSENT; fi' sh /capture-helper.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-test-type' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /capture-helper_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-test-size' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then wc -c < "$1"; else printf ABSENT; fi' sh /capture-helper_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-test-sha256' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then sha256sum "$1"; else printf ABSENT; fi' sh /capture-helper_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-linktype-test-type' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /capture-helper_linktype_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-linktype-test-size' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then wc -c < "$1"; else printf ABSENT; fi' sh /capture-helper_linktype_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-control-linktype-test-sha256' docker exec "$CID" sh -c 'if [ -f "$1" ] && [ ! -L "$1" ]; then sha256sum "$1"; else printf ABSENT; fi' sh /capture-helper_linktype_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

read -r HOST_SOURCE_SIZE _ < "$EVIDENCE/03-source-size.stdout.txt" || { FIRST_FAILURE=51; exit 51; }
read -r HOST_SOURCE_HASH _ < "$EVIDENCE/03-source-sha256.stdout.txt" || { FIRST_FAILURE=51; exit 51; }
read -r HOST_TEST_SIZE _ < "$EVIDENCE/03-test-source-size.stdout.txt" || { FIRST_FAILURE=51; exit 51; }
read -r HOST_TEST_HASH _ < "$EVIDENCE/03-test-source-sha256.stdout.txt" || { FIRST_FAILURE=51; exit 51; }
read -r HOST_LINK_TYPE_TEST_SIZE _ < "$EVIDENCE/03-linktype-test-size.stdout.txt" || { FIRST_FAILURE=51; exit 51; }
read -r HOST_LINK_TYPE_TEST_HASH _ < "$EVIDENCE/03-linktype-test-sha256.stdout.txt" || { FIRST_FAILURE=51; exit 51; }
read -r CONTROL_SOURCE_TYPE < "$EVIDENCE/06-control-source-type.stdout.txt"
read -r CONTROL_SOURCE_SIZE < "$EVIDENCE/06-control-source-size.stdout.txt"
read -r CONTROL_SOURCE_HASH _ < "$EVIDENCE/06-control-source-sha256.stdout.txt"
read -r CONTROL_TEST_TYPE < "$EVIDENCE/06-control-test-type.stdout.txt"
read -r CONTROL_TEST_SIZE < "$EVIDENCE/06-control-test-size.stdout.txt"
read -r CONTROL_TEST_HASH _ < "$EVIDENCE/06-control-test-sha256.stdout.txt"
read -r CONTROL_LINK_TYPE_TEST_TYPE < "$EVIDENCE/06-control-linktype-test-type.stdout.txt"
read -r CONTROL_LINK_TYPE_TEST_SIZE < "$EVIDENCE/06-control-linktype-test-size.stdout.txt"
read -r CONTROL_LINK_TYPE_TEST_HASH _ < "$EVIDENCE/06-control-linktype-test-sha256.stdout.txt"
if [[ "$CONTROL_SOURCE_TYPE" != REGULAR_FILE || "$CONTROL_SOURCE_SIZE" != "$HOST_SOURCE_SIZE" || "$CONTROL_SOURCE_HASH" != "$HOST_SOURCE_HASH" || "$CONTROL_TEST_TYPE" != REGULAR_FILE || "$CONTROL_TEST_SIZE" != "$HOST_TEST_SIZE" || "$CONTROL_TEST_HASH" != "$HOST_TEST_HASH" || "$CONTROL_LINK_TYPE_TEST_TYPE" != REGULAR_FILE || "$CONTROL_LINK_TYPE_TEST_SIZE" != "$HOST_LINK_TYPE_TEST_SIZE" || "$CONTROL_LINK_TYPE_TEST_HASH" != "$HOST_LINK_TYPE_TEST_HASH" ]]; then
  FIRST_FAILURE=52
  printf 'control_source_gate=FAIL\n' > "$EVIDENCE/06-control-source-gate.txt"
  exit 52
fi
printf 'control_source_gate=PASS\nhelper_size=%s\nhelper_sha256=%s\ntest_size=%s\ntest_sha256=%s\nlinktype_test_size=%s\nlinktype_test_sha256=%s\n' "$HOST_SOURCE_SIZE" "$HOST_SOURCE_HASH" "$HOST_TEST_SIZE" "$HOST_TEST_HASH" "$HOST_LINK_TYPE_TEST_SIZE" "$HOST_LINK_TYPE_TEST_HASH" > "$EVIDENCE/06-control-source-gate.txt"

record '06-copy-source-to-tmpfs' docker exec "$CID" cp /capture-helper.go /tmp/gateway_direct_flow_capture.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-copy-test-to-tmpfs' docker exec "$CID" cp /capture-helper_test.go /tmp/gateway_direct_flow_capture_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '06-copy-linktype-test-to-tmpfs' docker exec "$CID" cp /capture-helper_linktype_test.go /tmp/gateway_direct_flow_capture_linktype_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi

record '07-runtime-metadata' docker exec "$CID" sh -c 'printf "pwd="; pwd; while IFS= read -r line; do case "$line" in *" /tmp "*) printf "tmp_mount=%s\\n" "$line";; *" / "*) printf "root_mount=%s\\n" "$line";; esac; done < /proc/self/mountinfo'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-copy-source-size' docker exec "$CID" sh -c 'wc -c < /tmp/gateway_direct_flow_capture.go'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-copy-source-sha256' docker exec "$CID" sha256sum /tmp/gateway_direct_flow_capture.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-copy-test-size' docker exec "$CID" sh -c 'wc -c < /tmp/gateway_direct_flow_capture_test.go'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-copy-test-sha256' docker exec "$CID" sha256sum /tmp/gateway_direct_flow_capture_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-copy-linktype-test-size' docker exec "$CID" sh -c 'wc -c < /tmp/gateway_direct_flow_capture_linktype_test.go'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-copy-linktype-test-sha256' docker exec "$CID" sha256sum /tmp/gateway_direct_flow_capture_linktype_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-tmp-source-type' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /tmp/gateway_direct_flow_capture.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-tmp-test-type' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /tmp/gateway_direct_flow_capture_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
record '07-tmp-linktype-test-type' docker exec "$CID" sh -c 'if [ -L "$1" ]; then printf SYMLINK; elif [ -f "$1" ]; then printf REGULAR_FILE; elif [ -e "$1" ]; then printf PRESENT_NONREGULAR; else printf ABSENT; fi' sh /tmp/gateway_direct_flow_capture_linktype_test.go
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
read -r TMP_SOURCE_SIZE < "$EVIDENCE/07-copy-source-size.stdout.txt"
read -r TMP_SOURCE_HASH _ < "$EVIDENCE/07-copy-source-sha256.stdout.txt"
read -r TMP_TEST_SIZE < "$EVIDENCE/07-copy-test-size.stdout.txt"
read -r TMP_TEST_HASH _ < "$EVIDENCE/07-copy-test-sha256.stdout.txt"
read -r TMP_LINK_TYPE_TEST_SIZE < "$EVIDENCE/07-copy-linktype-test-size.stdout.txt"
read -r TMP_LINK_TYPE_TEST_HASH _ < "$EVIDENCE/07-copy-linktype-test-sha256.stdout.txt"
read -r TMP_SOURCE_TYPE < "$EVIDENCE/07-tmp-source-type.stdout.txt"
read -r TMP_TEST_TYPE < "$EVIDENCE/07-tmp-test-type.stdout.txt"
read -r TMP_LINK_TYPE_TEST_TYPE < "$EVIDENCE/07-tmp-linktype-test-type.stdout.txt"
if [[ "$TMP_SOURCE_TYPE" != REGULAR_FILE || "$TMP_SOURCE_SIZE" != "$HOST_SOURCE_SIZE" || "$TMP_SOURCE_HASH" != "$HOST_SOURCE_HASH" || "$TMP_TEST_TYPE" != REGULAR_FILE || "$TMP_TEST_SIZE" != "$HOST_TEST_SIZE" || "$TMP_TEST_HASH" != "$HOST_TEST_HASH" || "$TMP_LINK_TYPE_TEST_TYPE" != REGULAR_FILE || "$TMP_LINK_TYPE_TEST_SIZE" != "$HOST_LINK_TYPE_TEST_SIZE" || "$TMP_LINK_TYPE_TEST_HASH" != "$HOST_LINK_TYPE_TEST_HASH" ]]; then
  FIRST_FAILURE=53
  printf 'tmpfs_source_gate=FAIL\n' > "$EVIDENCE/07-transfer-gate.txt"
  exit 53
fi
printf 'tmpfs_source_gate=PASS\nhelper_size=%s\nhelper_sha256=%s\ntest_size=%s\ntest_sha256=%s\n' "$TMP_SOURCE_SIZE" "$TMP_SOURCE_HASH" "$TMP_TEST_SIZE" "$TMP_TEST_HASH" > "$EVIDENCE/07-transfer-gate.txt"

record '08-gofmt-diff' docker exec "$CID" sh -c 'cd /tmp && /usr/local/go/bin/gofmt -d gateway_direct_flow_capture.go gateway_direct_flow_capture_test.go gateway_direct_flow_capture_linktype_test.go'
if (( LAST_RC != 0 )); then exit "$LAST_RC"; fi
if [[ -s "$EVIDENCE/08-gofmt-diff.stdout.txt" ]]; then
  FIRST_FAILURE=54
  printf 'gofmt_gate=FAIL\n' > "$EVIDENCE/08-gofmt-result.txt"
  exit 54
fi
printf 'gofmt_gate=PASS\n' > "$EVIDENCE/08-gofmt-result.txt"
record '08-go-test-build' docker exec "$CID" sh -c 'mkdir -p /tmp/gocache /tmp/gotmp; cd /tmp && GOCACHE=/tmp/gocache GOTMPDIR=/tmp/gotmp GO111MODULE=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off /usr/local/go/bin/go test -c -o /capture_test gateway_direct_flow_capture.go gateway_direct_flow_capture_test.go gateway_direct_flow_capture_linktype_test.go'
if [[ "$MODE" == '--red' ]]; then
  GO_ERR="$(<"$EVIDENCE/08-go-test-build.stderr.txt")"
  if (( LAST_RC != 0 )) && [[ "$GO_ERR" == *'cannot use capture'* ]] && [[ "$GO_ERR" == *'setReceiveTimeout'* ]] && [[ "$GO_ERR" == *'openPacketSockets'* ]]; then
    TEST_RESULT='RED_EXPECTED'
    EXPECTED_RED=1
    FIRST_FAILURE=0
    printf 'red_gate=PASS expected_missing_error_surface\n' > "$EVIDENCE/08-test-result.txt"
    exit 0
  fi
  FIRST_FAILURE=61
  TEST_RESULT='RED_NOT_REPRODUCED'
  printf 'red_gate=FAIL build_exit=%s\n' "$LAST_RC" > "$EVIDENCE/08-test-result.txt"
  exit 61
fi
if [[ "$MODE" == '--parser-red' ]]; then
  GO_ERR="$(<"$EVIDENCE/08-go-test-build.stderr.txt")"
  if (( LAST_RC != 0 )) && [[ "$GO_ERR" == *'undefined: decodeIPv4Frame'* ]]; then
    TEST_RESULT='PARSER_RED_EXPECTED'
    FIRST_FAILURE=0
    printf 'parser_red_gate=PASS expected_missing_l3_decoder\n' > "$EVIDENCE/08-parser-test-result.txt"
    exit 0
  fi
  FIRST_FAILURE=62
  TEST_RESULT='PARSER_RED_NOT_REPRODUCED'
  printf 'parser_red_gate=FAIL build_exit=%s\n' "$LAST_RC" > "$EVIDENCE/08-parser-test-result.txt"
  exit 62
fi
if [[ "$MODE" == '--linktype-red' ]]; then
  GO_ERR="$(<"$EVIDENCE/08-go-test-build.stderr.txt")"
  if (( LAST_RC != 0 )) && [[ "$GO_ERR" == *'undefined: decodeIPv4FrameForHardware'* ]]; then
    TEST_RESULT='LINKTYPE_RED_EXPECTED'
    FIRST_FAILURE=0
    printf 'linktype_red_gate=PASS expected_missing_hardware_type_decoder\n' > "$EVIDENCE/08-linktype-test-result.txt"
    exit 0
  fi
  FIRST_FAILURE=65
  TEST_RESULT='LINKTYPE_RED_NOT_REPRODUCED'
  printf 'linktype_red_gate=FAIL build_exit=%s\n' "$LAST_RC" > "$EVIDENCE/08-linktype-test-result.txt"
  exit 65
fi
if [[ "$MODE" == '--metadata-red' ]]; then
  GO_ERR="$(<"$EVIDENCE/08-go-test-build.stderr.txt")"
  if (( LAST_RC != 0 )) && [[ "$GO_ERR" == *'undefined: summarizeTCPPacket'* ]] && [[ "$GO_ERR" == *'undefined: decodeKernelTimestampData'* ]] && [[ "$GO_ERR" == *'undefined: readPacketStatistics'* ]] && [[ "$GO_ERR" == *'undefined: parseKernelTimestamp'* ]]; then
    TEST_RESULT='METADATA_RED_EXPECTED'
    FIRST_FAILURE=0
    printf 'metadata_red_gate=PASS expected_missing_packet_metadata\n' > "$EVIDENCE/08-metadata-test-result.txt"
    exit 0
  fi
  FIRST_FAILURE=66
  TEST_RESULT='METADATA_RED_NOT_REPRODUCED'
  printf 'metadata_red_gate=FAIL build_exit=%s\n' "$LAST_RC" > "$EVIDENCE/08-metadata-test-result.txt"
  exit 66
fi
if (( LAST_RC != 0 )); then
  if [[ "$MODE" == '--race-red' ]]; then TEST_RESULT='RACE_RED_BUILD_FAILED'; else TEST_RESULT='GREEN_BUILD_FAILED'; fi
  exit "$LAST_RC"
fi

record '09-test-binary-metadata' docker exec "$CID" sh -c 'if [ -L /capture_test ] || [ ! -f /capture_test ] || [ ! -x /capture_test ]; then printf "INVALID_TEST_BINARY\\n"; exit 62; fi; wc -c < /capture_test; sha256sum /capture_test'
if (( LAST_RC != 0 )); then TEST_RESULT='GREEN_BINARY_INVALID'; exit "$LAST_RC"; fi
if [[ "$MODE" == '--decoder-red' ]]; then
  record '09-decoder-red-test-run' docker exec "$CID" /capture_test -test.v -test.run '^TestDecodeIPv4FrameDoesNotTreatIPv4LikeMACAsRawPacket$' -test.timeout=30s
  DECODER_OUTPUT="$(<"$EVIDENCE/09-decoder-red-test-run.stdout.txt")$(<"$EVIDENCE/09-decoder-red-test-run.stderr.txt")"
  if (( LAST_RC != 0 )) && [[ "$DECODER_OUTPUT" == *'treated an IPv4-like Ethernet MAC as a raw IPv4 header'* ]]; then
    TEST_RESULT='DECODER_RED_EXPECTED'
    FIRST_FAILURE=0
    printf 'decoder_red_gate=PASS expected_l2_l3_ambiguity\n' > "$EVIDENCE/09-decoder-test-result.txt"
    exit 0
  fi
  FIRST_FAILURE=64
  TEST_RESULT='DECODER_RED_NOT_REPRODUCED'
  printf 'decoder_red_gate=FAIL runner_exit=%s\n' "$LAST_RC" > "$EVIDENCE/09-decoder-test-result.txt"
  exit 64
fi
if [[ "$MODE" == '--race-red' ]]; then
  record '09-race-red-test-run' docker exec "$CID" /capture_test -test.v -test.run '^TestRunCapturesCollectsBeforeCompleting$' -test.timeout=30s
  RACE_OUTPUT="$(<"$EVIDENCE/09-race-red-test-run.stdout.txt")$(<"$EVIDENCE/09-race-red-test-run.stderr.txt")"
  if (( LAST_RC != 0 )) && [[ "$RACE_OUTPUT" == *'RACE_RED_EXPECTED:'* ]]; then
    TEST_RESULT='RACE_RED_EXPECTED'
    FIRST_FAILURE=0
    printf 'race_red_gate=PASS\n' > "$EVIDENCE/09-race-test-result.txt"
    exit 0
  fi
  FIRST_FAILURE=63
  TEST_RESULT='RACE_RED_NOT_REPRODUCED'
  printf 'race_red_gate=FAIL runner_exit=%s\n' "$LAST_RC" > "$EVIDENCE/09-race-test-result.txt"
  exit 63
fi
record '09-go-test-run' docker exec "$CID" /capture_test -test.v -test.timeout=30s
if (( LAST_RC != 0 )); then TEST_RESULT='GREEN_TEST_FAILED'; exit "$LAST_RC"; fi
TEST_RESULT='GREEN_PASS'
printf 'green_gate=PASS\n' > "$EVIDENCE/09-test-result.txt"
exit 0

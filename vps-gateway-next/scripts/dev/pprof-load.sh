#!/usr/bin/env bash
# Stress-test script for awg-manager during pprof collection.
#
# Usage:
#   ./scripts/pprof-load.sh [HOST] [SECONDS] [CONCURRENCY]
#
# Defaults:
#   HOST        = 192.168.1.1:2222
#   SECONDS     = 35   (чуть больше 30 чтобы перекрыть окно съёма)
#   CONCURRENCY = 4    (параллельных воркеров)
#
# Пример:
#   ./scripts/pprof-load.sh 192.168.1.1:2222 30 6

set -euo pipefail

HOST="${1:-192.168.1.1:2222}"
DURATION="${2:-35}"
WORKERS="${3:-4}"
BASE="http://$HOST/api"

# Безопасные GET-эндпоинты — не меняют состояние.
ENDPOINTS=(
    "$BASE/health"
    "$BASE/boot-status"
    "$BASE/tunnels/list"
    "$BASE/tunnels/all"
    "$BASE/tunnels/traffic"
    "$BASE/pingcheck/status"
    "$BASE/pingcheck/logs"
    "$BASE/logs"
    "$BASE/logs/subgroups"
    "$BASE/settings/get"
    "$BASE/routing/tunnels"
    "$BASE/routing/dns-routes"
    "$BASE/routing/client-routes"
    "$BASE/client-routes"
    "$BASE/dns-routes/list"
    "$BASE/system-tunnels"
    "$BASE/diagnostics/status"
    "$BASE/connections"
    "$BASE/singbox/status"
    "$BASE/singbox/tunnels"
    "$BASE/singbox/subscriptions"
    "$BASE/managed-servers"
)

COUNT=${#ENDPOINTS[@]}

end_time=$(( $(date +%s) + DURATION ))

echo "► Загружаем $BASE на $DURATION секунд ($WORKERS воркеров)"
echo "  Ctrl+C чтобы остановить досрочно"
echo

worker() {
    local id=$1
    local req=0
    while [[ $(date +%s) -lt $end_time ]]; do
        local url="${ENDPOINTS[$(( req % COUNT ))]}"
        curl -sf --max-time 5 \
             --cookie-jar /tmp/awg-pprof-cookies-$id.txt \
             --cookie /tmp/awg-pprof-cookies-$id.txt \
             -o /dev/null "$url" 2>/dev/null || true
        req=$(( req + 1 ))
    done
    echo "  воркер $id завершён: $req запросов"
}

# Запуск воркеров в фоне
pids=()
for i in $(seq 1 "$WORKERS"); do
    worker "$i" &
    pids+=($!)
done

# Прогресс-бар
elapsed=0
while [[ $(date +%s) -lt $end_time ]]; do
    remaining=$(( end_time - $(date +%s) ))
    done_pct=$(( (DURATION - remaining) * 100 / DURATION ))
    bar=$(printf '%0.s█' $(seq 1 $(( done_pct / 5 ))))
    pad=$(printf '%0.s░' $(seq 1 $(( 20 - done_pct / 5 ))))
    printf "\r  [%s%s] %3d%% — осталось %ds  " "$bar" "$pad" "$done_pct" "$remaining"
    sleep 1
done

wait "${pids[@]}"

echo
echo
echo "✓ Готово. Теперь в pprof: top или top -cum"

# Cleanup cookies
rm -f /tmp/awg-pprof-cookies-*.txt

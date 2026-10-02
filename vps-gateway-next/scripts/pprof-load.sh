#!/usr/bin/env bash
# Stress-test script for awg-manager during pprof collection.
# Automatically extracts all GET endpoints from swagger.yaml.
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

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SWAGGER="$SCRIPT_DIR/../internal/openapi/swagger.yaml"

if [[ ! -f "$SWAGGER" ]]; then
    echo "Не найден swagger.yaml: $SWAGGER" >&2
    exit 1
fi

# Извлекаем все GET-пути без path-параметров ({id} и т.п.)
mapfile -t ENDPOINTS < <(
    awk '/^  \/[^:]+:$/{path=$0} /^    get:/{gsub(/^  /,"",path); gsub(/:$/,"",path); print path}' "$SWAGGER" \
    | grep -v '{' \
    | sed "s|^|$BASE|"
)

COUNT=${#ENDPOINTS[@]}

if [[ $COUNT -eq 0 ]]; then
    echo "Не удалось извлечь эндпоинты из swagger.yaml" >&2
    exit 1
fi

end_time=$(( $(date +%s) + DURATION ))

echo "► Загружаем $BASE на $DURATION секунд ($WORKERS воркеров, $COUNT GET-эндпоинтов из swagger)"
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

rm -f /tmp/awg-pprof-cookies-*.txt

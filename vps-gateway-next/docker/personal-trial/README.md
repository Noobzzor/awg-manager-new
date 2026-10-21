# Локальная пробная упаковка

Этот отдельный рецепт собирает Linux/amd64 образ из уже проверенных бинарников и имеющегося локального базового образа. Основной Dockerfile проекта для сборки из исходников не заменяется. Бинарники, приватные данные и generated build context в эту папку не копируются.

В контекст сборки допускаются только три бинарника (awg-manager, amneziawg-go, awg), Dockerfile, два entrypoint-скрипта, manifest.json, SHA256SUMS и файлы лицензий/NOTICE. Хэши проверяются до сборки и при обычном запуске. Базовый локальный тег должен точно соответствовать SHA256492d5c2426f257233a9880a44a358a2ec41281c2e4710d1ddb04be17b799ae6f. Сборка выполнялась имеющимся BuildKit без скачивания зависимостей.

Compose публикует панель только на127.0.0.1; для VPS предполагается доступ через SSH-туннель. Внешний AWG UDP порт задаётся явно. Обязателен AWG_GATEWAY_PUBLIC_ENDPOINT в формате адрес:порт. Реальные ключи и WARP-профиль в образ не входят; состояние создаётся в отдельном томе данных.

Новый образ20261021-ui1 прошёл обычный запуск/API/сохранение, реальный браузерный lifecycle и семь фаз UDP/WARP на точных бинарниках. Повтор при384МиБ без swapконтейнера прошёл; Compose использует этот лимит. Переносимый архив и контрольные суммы проверены и хранятся вне исходников; этот рецепт не подменяет полноценную source/multiarch release сборку. VPS/реальное устройство остаются непроверенными. Актуальные доказательства: scripts/m2-local-20261021/codex-personal-trial-66ef0989.

DNS update20261021-dns1: exports include DNS=1.1.1.1 (startup override AWG_GATEWAY_CLIENT_DNS, validated IPv4). Keep existing data volume during upgrades. Actual iPhone DIRECT browsing reported working with manual DNS before update; exact owned VPS updated preserving identity/policy/sites. Device BLOCK/no-bypass and VPS WARP acceptance still open.

# AWG Manager — VPS Gateway: next development

Новая разработка серверного AWG Gateway. Существующая разработка в корне репозитория не заменяется и не удаляется.

## Назначение

Самостоятельный Docker/Linux VPS Gateway без Keenetic/NDMS. Удалённые устройства подключаются по AWG/WireGuard. Управление — через веб-панель на VPS. Сервер выбирает VPN, WARP, DIRECT или BLOCK по правилам доменов, IP/CIDR, портов и протоколов. DIRECT означает выход в интернет с VPS.

Конечный продукт сохраняет отдельных клиентов, ключи, адреса и lifecycle, fail-closed для VPN/WARP, persistence, restart/recovery, backup/restore и безопасное применение конфигурации. Сокращать функциональность без решения пользователя нельзя.

## Границы

- Новую разработку размещать в этой папке; старую разработку сохранять отдельно.
- Не переносить сюда секреты, реальные клиентские конфигурации, ключи, персональные данные, dependency caches, generated build output и несанифицированные diagnostic logs.
- Не изменять VPS, основной локальный checkout, защищённые containers/volumes или host firewall.
- Публикация в GitHub разрешена только для новой папки в подтверждённом репозитории пользователя; это не разрешение публиковать весь dirty worktree.

## Текущее состояние

Новая кодовая база с текущими изменениями подготовлена локально. Пользователь выбрал `Noobzzor/awg-manager-new`; это публичный репозиторий, default branch main, write access проверен.

Первое размещение папки содержит проектную документацию и исходную лицензию. Исходники публикуются только после завершения проверки test fixtures, на которые сработал secret scanner. Старые несанифицированные logs, Git history и реальные конфиги не публикуются. Предыдущий snapshot сохранён.

Не объявлять новую папку функционально готовым Gateway. Оперативное состояние — PROJECT_STATUS.md, цель — PRODUCT_SPEC.md, критерии — ROADMAP.md, задачи — TASKS.md, доказательства — TEST_MATRIX.md, решения — DECISIONS.md.

## Следующий шаг

Завершить security review исходников, публиковать только новую папку с exact commit/tree readback, закрыть slot filename migration и rule-engine regressions по Linux TDD, затем исправить fixture и доказать single-client DIRECT/BLOCK. VPS gate остаётся закрытым для deployment.

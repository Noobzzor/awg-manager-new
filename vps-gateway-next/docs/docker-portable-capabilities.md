# Docker portable capability contract

Статус: текущий Docker-only capability contract и verified implementation boundary.
Дата фиксации: 2026-09-21.

## Граница продукта

Docker-режим является самостоятельным Linux/Sing-box приложением. Он не должен вызывать Keenetic/NDMS, NativeWG, kernel AWG, opkg, router WAN, router firewall или router-specific process/service APIs.

Наличие HTTP-маршрута само по себе не означает поддержку: capability считается поддержанной только если endpoint подключён к portable-зависимости, выполняет реальную read/write-операцию и имеет тест readback.

## Текущий проверенный статус

- Исходное состояние сохранено в `../awg-manager_backup_2026-09-21`.
- Новая рабочая копия: `../awg-manager_docker-full`.
- Local mode запускается как самостоятельный `AWG_MODE=singbox` runtime.
- `/api/capabilities` подтверждает: `singboxStatus`, `awg3`, `singboxConfigEditor`, `subscriptions`, `inbounds`, `dnsRoutes` и `dnsRouteBackend=singbox`.
- Docker-only запреты явно представлены: `proxyInbound`, `logs`, `ndms`, `singboxRouter`, `hydraroute`, `tunnelDiagnostics`, `updates`, `daemonRestart`, `ndmsProxy` равны `false`.
- Реальным runtime smoke подтверждены config preview/slots, config-editor draft/check/discard, inbounds, Clash proxy groups, DNS routes, subscriptions/groups, AWG3 import/list/delete и persistence после restart.
- Startup local mode восстанавливает persisted subscriptions при missing/empty `40-subscriptions.json`, сбрасывает старые router ProxyIndex и ждёт завершения scheduler workers.
- Полный Docker image build прошёл с frontend `check`, полным `npm test` и production build.
- `/api/import/conf` относится к router-era import flow; portable `.conf` должен идти через `/api/awg3-endpoints`.

## Классификация capability

### Portable-core: реализовать и закрепить тестами

- `awg3`: import/list/detail/rename/update/delete, safe DTO, persistence.
- `singboxRuntime`: status, control, generated config, running process.
- `singboxTunnels`: list/detail/rename/share-link and safe connectivity checks.
- `dnsRoutes`: CRUD, enable/disable, refresh and Sing-box reconciliation.
- `settings`: portable settings and API-key handling.
- `events`: manager events without NDMS event source.
- `systemInfo`: Linux/container/Sing-box facts; router fields must be explicit unsupported/null.

### Portable-adapted: expose only after real wiring

- `configEditor`: slots, preview, user draft check/apply/discard/enable with last-known-good rollback.
- `inbounds`: generated Sing-box inbounds and safe manager-level operations.
- `clashAPI`: authenticated manager proxy to loopback-only Sing-box Clash API; no host-public control port.
- `subscriptions`: storage, preview, refresh, members/groups and materialization without NDMS proxy creation.
- `routingPolicy`: Sing-box DNS/route policy only; no claim of kernel/router policy control.
- `backupRestore`: portable `/data` state only, with secret redaction and migration version.
- `logsDiagnostics`: real container/application/Sing-box diagnostics, not fake router snapshots.
- `dashboard`: composite AWG3/Sing-box snapshot using portable IDs and state.

### Router-only: hide or return explicit unsupported response

- NDMS/RCI, KeenDNS, router WAN and native interface discovery.
- NativeWG and kernel AWG module lifecycle.
- opkg/Entware, init.d router services, router process/file manager.
- iptables/policy-tun, router NAT/firewall and external router routing stack.
- Router managed servers and NDMS managed-peer drift.
- Keenetic firmware/memory/kernel metadata.
- Features that require router listener, native binary installation or NDMS network policy.

### Неподдерживаемые функции текущего Docker-релиза

- Keenetic/NDMS: RCI, NDMS hooks, KeenDNS, ProxyN/t2sN, router WAN/interfaces,
  router time/firmware/memory/kernel metadata and NDMS config save.
- Router VPN backends: NativeWG, kernel AWG/`awg_proxy.ko`, router
  `TunnelService` and router-specific tunnel lifecycle.
- Entware/opkg/init.d, router process manager, router file manager and package
  installation/removal.
- Router iptables/policy-tun, NAT, firewall and external router routing stack.
- Managed servers and NDMS managed-peer drift.
- WDTT, FreeTurn, `wg-obfuscator`/Phobos and router proxy-runtime instances,
  including their install/update/server-panel operations.
- Aggregated router diagnostics: NDMS logs, router monitoring matrix and real
  router connection tracking. The portable capability contract reports
  `logs=false`, `proxyInbound=false`, `ndms=false`.
- Full router Sing-box surface: router status/rules/policy/NAT and router-only
  inbound management. `/api/singbox/router/status` returns `404` in local mode.
- Keenetic/Entware credential login. Local mode uses the configured API key and
  returns `501 LOCAL_LOGIN_UNAVAILABLE` for router credential login.

The legacy router-only GET routes `/api/tunnels/all`, `/api/servers/all`,
`/api/managed/drift`, `/api/logs`, `/api/logs/subgroups`,
`/api/monitoring/matrix` and `/api/connections` return HTTP `501` with the
machine-readable code `UNSUPPORTED_DOCKER_CAPABILITY`. They never return an
empty success shape that could be mistaken for a real portable snapshot.

### Частично подключённые или ещё не закрытые функции

- Portable config editor/inbounds/subscriptions работают на API/runtime уровне,
  но browser-level UI smoke для каждого экрана ещё не является release gate.
- Backup/restore `/data` is wired to the portable data volume. Export/import
  and post-restore readiness are covered by Docker smoke; mounted volumes use
  an in-volume staging/rollback path because `/` may be read-only.
- Delay/connectivity/IP/speed tests, geo-data, FakeIP и router Sing-box rules
  намеренно не включаются в Docker UI: portable аналоги ещё не реализованы.
- The portable dashboard skips the legacy composite tunnel poll in local mode;
  portable data is read from Sing-box, AWG3 and subscription stores instead.

## Rules for implementation

1. `capability=true` requires a real handler plus read/write or lifecycle test.
2. Unsupported mutation must not return successful empty data. Use hidden UI, `404`, or `501` with a machine-readable reason.
3. Secrets (`PrivateKey`, `PresharedKey`, subscription credentials, API keys) must not appear in ordinary DTOs, logs, diagnostics or test output.
4. A mutation is complete only after persistence and runtime readback are verified.
5. A config change is complete only after validation, atomic commit, reload and rollback behavior are tested.
6. Portable frontend code must not call `/api/import/conf`.
7. A healthy container proves only runtime startup; it does not prove feature parity.

## Следующий порядок работ

1. Add frontend browser smoke for dashboard, AWG3 detail, config editor,
   inbounds and subscriptions.
2. Remove launcher noise from CI commands and review package-manager inputs.
3. Run final clean build, restart/persistence, secret-redaction and UI review
   before calling the Docker version release-ready.

# Оперативный статус

- Product: полный серверный AWG Manager Gateway; PRODUCT_SPEC.md.
- Milestone: M0 — воспроизводимая основа перед single-client DIRECT/BLOCK gate.
- Current task: GH-001 — новая рабочая область и проверенная публикация; далее POL-001 migration.
- Working directory: vps-gateway-next; код вне этой папки в прежнем snapshot не менять.
- Target: Noobzzor/awg-manager-new, main, public. В репозиторий не попадут реальные ключи, клиентские конфиги и старые raw logs.

## PROVEN

Исходники и vendored зависимости перенесены из текущего dirty snapshot с проверкой SHA256 копий. Исторический no-product control воспроизвёл Docker --internal outside-subnet alias failure. Исторические transport/sniff focused RED и policy package GREEN существуют локально, но повтор в новой копии ещё не принят.

## NOT PROVEN

Новая копия ещё не прошла Linux regression tests. Slot migration не реализована. Живой domain sniff control не выполнен. Исправленная DIRECT fixture и reply path не доказаны. VPN/WARP/multi-client/failure/restart/remote VPS functional gates открыты. Publish на GitHub пока не выполнен.

## Следующий шаг

Secret scan -> scoped publish/readback -> POL-001 RED/GREEN -> policy/merge/forwarding package gates -> POL-002 rule-engine control -> NET-001 fixture control и single-client E2E.

## Чего не делать

Не перезапускать проект с нуля, не сокращать продукт, не менять VPS/host firewall/protected containers/volumes, не вносить SNAT/bind workaround ради стенда, не публиковать грязную старую историю и credentials. Разрешение на publish относится только к новой папке в выбранном repo.

## Review

Независимый review нужен после consolidation POL-001/POL-002 и до новой datapath topology/gate. Не делать формальный review каждой мелочи. REVIEW не означает DONE.

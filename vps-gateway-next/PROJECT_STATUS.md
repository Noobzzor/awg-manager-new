# Оперативный статус

- Product: полный серверный AWG Manager Gateway; PRODUCT_SPEC.md.
- Milestone: M0 — воспроизводимая основа перед single-client DIRECT/BLOCK gate.
- Current task: GH-001 — security review перед публичной загрузкой исходников; POL-001 — independent review после GREEN.
- Working directory: vps-gateway-next; код вне этой папки в прежнем snapshot не менять.
- Target: Noobzzor/awg-manager-new, main, public. В репозиторий не попадут реальные ключи, клиентские конфиги и старые raw logs.

## PROVEN

Исходники и vendored зависимости перенесены из текущего dirty snapshot с проверкой SHA256 копий. В новой Linux-копии проходят orchestrator, configmerge, policy, forwarding и ingress package tests. Migration regression сначала RED, затем GREEN: active/disabled/pending, конфликты и повторный Bootstrap; validator/runtime merge после apply совпадают. Папка на GitHub создана: первый commit 4da0252482c8e4eb26d5d954fd63b6ab5e68c1e5 содержит девять documentation/license files; exact branch/folder/README readback выполнен. Исторический no-product control воспроизвёл Docker --internal outside-subnet alias failure.

## NOT PROVEN

Полная новая кодовая база пока не опубликована: secret scanner дал 80 hits (68 unique locations) в тестовых/mock данных; их синтетичность проверяется независимо, непроверенные значения в public repo не загружаются. Migration ещё проходит independent review; общий gate не закрыт. Живой domain sniff control не выполнен. Исправленная DIRECT fixture и reply path не доказаны. VPN/WARP/multi-client/failure/restart/remote VPS functional gates открыты.

## Следующий шаг

Security review/sanitization -> scoped source publish/readback; migration review/fix wave при необходимости -> POL-002 rule-engine control -> NET-001 fixture control и single-client E2E.

## Чего не делать

Не перезапускать проект с нуля, не сокращать продукт, не менять VPS/host firewall/protected containers/volumes, не вносить SNAT/bind workaround ради стенда, не публиковать грязную старую историю и credentials. Разрешение на publish относится только к новой папке в выбранном repo.

## Review

Независимый review нужен после consolidation POL-001/POL-002 и до новой datapath topology/gate. Не делать формальный review каждой мелочи. REVIEW не означает DONE.

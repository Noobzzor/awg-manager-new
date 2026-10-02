# Оперативный статус

- Product: полный серверный AWG Manager Gateway; PRODUCT_SPEC.md.
- Milestone: M0 — воспроизводимая основа перед single-client DIRECT/BLOCK gate.
- Current task: POL-001 — дополнительные migration acceptance cases; затем POL-002 rule-engine control.
- Working directory: vps-gateway-next; код вне этой папки в прежнем snapshot не менять.
- Target: Noobzzor/awg-manager-new, main, public. В репозиторий не попадут реальные ключи, клиентские конфиги и старые raw logs.

## PROVEN

Исходники и vendored зависимости перенесены из текущего dirty snapshot с проверкой SHA256 копий. В новой Linux-копии проходят orchestrator, configmerge, policy, forwarding и ingress package tests. Migration regression сначала RED, затем GREEN: active/disabled/pending, конфликты и повторный Bootstrap; validator/runtime merge после apply совпадают. Папка на GitHub создана: первый commit 4da0252482c8e4eb26d5d954fd63b6ab5e68c1e5 содержит девять documentation/license files; exact branch/folder/README readback выполнен. Исторический no-product control воспроизвёл Docker --internal outside-subnet alias failure.

Полные исходники новой папки опубликованы source commit 1bd15d4b3737790e44261287e609cd62eb2dacf2: 4067 source/doc/vendor files. Exact remote main/folder inventory и все committed blob hashes прочитаны и сверены; четыре Windows scripts имеют только штатную Git CRLF normalization. Node dependencies, generated build state, реальные .env и local diagnostic logs исключены. 19 unresolved fixture literals заменены с независимой проверкой формулы/полного literal-only diff; targeted originals отсутствуют в payload. Final raw scanner: 78 findings / 66 exact reviewed locations, zero new/unreviewed/changed alerts; blanket exemptions не добавлены. Frontend lockfile unchanged, affected test 30 PASS.

## NOT PROVEN

Migration gate открыт: нужны partial-failure/retry и дополнительные safety/merge cases, а также disk/runtime convergence при пережившем manager restart движке. Живой domain sniff control не выполнен. Исправленная DIRECT fixture и reply path не доказаны. VPN/WARP/multi-client/failure/restart/remote VPS functional gates открыты. Публикация development source не является релизом или доказательством готовности Gateway.

## Последний проверенный срез — POL-001 review fixes

P1: Bootstrap error теперь возвращается из общего core constructor; portable/runtime initialization не выдаёт operator/orchestrator продюсерам, а composition останавливается до дальнейших фаз. P2: config root/disabled/pending должны быть настоящими каталогами, не symlinks; проверка идёт до startup cleanup и миграции. Обе регрессии сначала RED, затем GREEN в Linux. После исправлений проходят шесть package suites (включая cmd/awg-manager), go vet и gofmt check. Локальное versioned evidence: 53-migration-review-fixes. Это startup/filesystem evidence, не fail-closed packet или VPS proof.

## Следующий шаг

Повторный review подтвердил P1 startup fix, но нашёл pre-Bootstrap side effects и cleanup regression. Early preflight теперь выполняется до NewOperator/reconciliation и legacy migrations. Production runtime/local regression с root/disabled/pending symlinks и JSON, требующим URL-миграции: RED -> GREEN, внешние bytes/file set не меняются. Cleanup regression исправлена: ошибка доходит до one-shot/main, недоступный sing-box cleaner не используется, независимые этапы продолжаются и ошибки собираются. Координатор прочитал реализацию и повторил consolidated Linux verification: 13 package suites PASS, internal/cleanup компилируется без standalone tests; vet всех 14 targets PASS. Evidence 55-production-preflight-cleanup. Настоящий uninstall/router cleanup не запускался; при неудачной инициализации sing-box может остаться, теперь это incomplete cleanup, не успех.

Migration extended acceptance/fix wave при необходимости -> POL-002 rule-engine control -> NET-001 fixture control и single-client E2E. Последующие code changes требуют собственной проверки/публикации: текущий public source привязан к проверенному commit выше.

## Чего не делать

Не перезапускать проект с нуля, не сокращать продукт, не менять VPS/host firewall/protected containers/volumes, не вносить SNAT/bind workaround ради стенда, не публиковать грязную старую историю и credentials. Разрешение на publish относится только к новой папке в выбранном repo.

## Review

Независимый review нужен после consolidation POL-001/POL-002 и до новой datapath topology/gate. Не делать формальный review каждой мелочи. REVIEW не означает DONE.

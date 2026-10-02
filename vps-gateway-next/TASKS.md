# Задачи

Статусы: BACKLOG / READY / IN PROGRESS / REVIEW / TESTING / BLOCKED / DONE. DONE — acceptance criteria выполнены и evidence относится к принятой версии.

## GH-001 — новая рабочая область и публикация
- Цель: новая разработка только в Noobzzor/awg-manager-new/vps-gateway-next; прежние checkout и remote не меняются.
- Facts: target repository public, main, write permission подтверждён API; исходники перенесены из текущего snapshot, а не из старого чистого commit.
- Acceptance: исключены .git, caches, приватные конфиги и старые несанифицированные logs; secret scan; publish только новой папки; exact remote commit/tree readback.
- Status: DONE. Source-only payload 4067 files; 19 replacements в 12 tests independently verified against baseline/formula, old targeted literals absent from the entire payload. Final raw scan 78 hits / 66 exact reviewed locations; zero new/unreviewed/changed alerts. No blanket scanner exemptions. Evidence: 54-publication-fixture-sanitization and 56-source-publication.
- Verified slice: шесть affected Linux packages и 30 frontend tests PASS. Frontend package.json explicitly unignored; node_modules остаётся ignored, lockfile unchanged.

## GH-002 — полная публикация новой папки
- Acceptance: только vps-gateway-next в отдельном publication clone; exact remote ref/folder/blob readback.
- Status: DONE. Source commit 1bd15d4b3737790e44261287e609cd62eb2dacf2; все 4067 committed blob hashes verified. Старый dirty snapshot/history и root repo files не публиковались и не менялись. Это source publication, не product readiness.

## POL-001 — миграция имени Gateway slot
- Original defect: KnownSlots использует 18-z-gateway-policy.json; старый Bootstrap не учитывал legacy 19-gateway-policy.json, а runtime MergeDir читал его. Миграция теперь реализована; remaining acceptance ниже.
- Proven: active/disabled/pending layouts, конфликты old/new, идемпотентность успешного Bootstrap, validator/runtime merge regression; отказ startup при migration error и отказ directory symlinks до cleanup.
- Not proven: partial failure/retry, полный non-regular candidate matrix, отдельный conflict MergeDir exclusion test, compatibility guards, disk/runtime convergence при живом пережившем рестарт движке.
- Acceptance: legacy state/content не теряются; current disabled не включается; конфликт сохраняется вне live merge; повторный Bootstrap идемпотентен; после apply нет старого final/duplicate inbound; Linux RED/GREEN/package tests.
- Next: принять independent review или закрыть его конкретные findings; не считать миграцию доказательством packet path.
- Status: REVIEW. Independent review выявил P1 ignored Bootstrap error и P2 directory symlinks. Оба исправлены отдельными RED/GREEN regression tests; шесть связанных Linux packages и vet PASS, formatting clean. Повторный review и перечисленные дополнительные cases нужны до DONE. Evidence: local 53-migration-review-fixes.
- Re-review: P1 startup подтверждён. Bootstrap-only guard не защищал earlier NewOperator/URL migrations: исправлен early non-mutating preflight, production RED/GREEN и два internal package suites/vet PASS (55-production-preflight-cleanup). Cleanup early-return/success-status regression в работе; full consolidated suites и review ещё нужны. Предыдущий six-package PASS не выдавать за проверку последней consolidated версии.
- Consolidation: cleanup regression исправлена и прочитана координатором; mock-only failure/order/one-shot/preflight tests проходят. Actual consolidated verification: 13 suites PASS, internal/cleanup compiles with no standalone tests, vet 14 targets PASS. Не запускался реальный uninstall; migration remaining cases и surviving-engine convergence всё ещё открыты.

## POL-002 — sniff до terminal Gateway policy
- Facts: Gateway renderer теперь содержит scoped sniff перед DNS/domain match и fallback; focused RED и policy package GREEN ранее получены.
- Not proven: реальная HTTP Host/TLS metadata влияет на выбранное действие в текущем engine.
- Acceptance: no-Router merged order сохраняет QoS priority; literal-IP + HTTP Host BLOCK не достигает listener, allowed control до/после отвечает; логи показывают domain rule; никакого prior DNS.
- Next: запустить namespace-local rule-engine control; не называть SOCKS adapter control доказательством AWG/TUN datapath.
- Status: READY. Предыдущая попытка не дошла до запросов из-за missing binary при docker cp.

## POL-003 — transport whitespace
- Facts: Validate принимает trimmed token; compiler теперь trims скопированный Network без изменения исходного Profile. Focused RED/GREEN и policy package GREEN получены до переноса.
- Acceptance: tcp/udp/icmp с whitespace компилируются канонически; input не изменяется; текущий Linux policy package проходит.
- Status: DONE. После переноса Linux policy package повторно прошёл; scope — canonical compiler tokens, не packet/VPS routing.

## NET-001 — исправить fixture и доказать DIRECT/BLOCK
- Facts: независимый sender/sink control без AWG/Sing-box воспроизвёл outside-subnet alias failure на Docker --internal. Inside-subnet alias и bridge address работают. Это fixture confound.
- Not proven: точный deployed DROP rule; корректный non-local DIRECT path в исправленной fixture.
- Hypothesis: disposable namespace-only veth устраняет internal bridge subnet filter; feasibility ещё не принята.
- Acceptance: безопасный owned namespace path без host networking/firewall/protected resources; no-product fixture control; затем .3 client policy/egress/response evidence и BLOCK; exact cleanup.
- Next: минимальная feasibility/review topology, затем control; не повторять прежний outside-alias probe.
- Status: READY. Production SNAT/bind ради старой fixture запрещён.

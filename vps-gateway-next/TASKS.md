# Задачи

Статусы: BACKLOG / READY / IN PROGRESS / REVIEW / TESTING / BLOCKED / DONE. DONE — acceptance criteria выполнены и evidence относится к принятой версии.

## GH-001 — новая рабочая область и публикация
- Цель: новая разработка только в Noobzzor/awg-manager-new/vps-gateway-next; прежние checkout и remote не меняются.
- Facts: target repository public, main, write permission подтверждён API; исходники перенесены из текущего snapshot, а не из старого чистого commit.
- Acceptance: исключены .git, caches, приватные конфиги и старые несанифицированные logs; secret scan; publish только новой папки; exact remote commit/tree readback.
- Status: IN PROGRESS. Copy hashes проверены локально; scan/publish пока не выполнены.

## POL-001 — миграция имени Gateway slot
- Facts: KnownSlots использует 18-z-gateway-policy.json; legacy 19-gateway-policy.json Bootstrap не учитывает, а runtime MergeDir читает.
- Not proven: корректная миграция active/disabled/pending, конфликты old/new, совпадение runtime и validator view.
- Acceptance: legacy state/content не теряются; current disabled не включается; конфликт сохраняется вне live merge; повторный Bootstrap идемпотентен; после apply нет старого final/duplicate inbound; Linux RED/GREEN/package tests.
- Next: выполнить уже добавленный gateway_migration_test.go, затем минимальная migration implementation.
- Status: READY. Предыдущий runner остановился до тестов; migration fix отсутствует.

## POL-002 — sniff до terminal Gateway policy
- Facts: Gateway renderer теперь содержит scoped sniff перед DNS/domain match и fallback; focused RED и policy package GREEN ранее получены.
- Not proven: реальная HTTP Host/TLS metadata влияет на выбранное действие в текущем engine.
- Acceptance: no-Router merged order сохраняет QoS priority; literal-IP + HTTP Host BLOCK не достигает listener, allowed control до/после отвечает; логи показывают domain rule; никакого prior DNS.
- Next: запустить namespace-local rule-engine control; не называть SOCKS adapter control доказательством AWG/TUN datapath.
- Status: READY. Предыдущая попытка не дошла до запросов из-за missing binary при docker cp.

## POL-003 — transport whitespace
- Facts: Validate принимает trimmed token; compiler теперь trims скопированный Network без изменения исходного Profile. Focused RED/GREEN и policy package GREEN получены до переноса.
- Acceptance: tcp/udp/icmp с whitespace компилируются канонически; input не изменяется; текущий Linux policy package проходит.
- Status: TESTING. После переноса требуется проверка точной новой рабочей копии.

## NET-001 — исправить fixture и доказать DIRECT/BLOCK
- Facts: независимый sender/sink control без AWG/Sing-box воспроизвёл outside-subnet alias failure на Docker --internal. Inside-subnet alias и bridge address работают. Это fixture confound.
- Not proven: точный deployed DROP rule; корректный non-local DIRECT path в исправленной fixture.
- Hypothesis: disposable namespace-only veth устраняет internal bridge subnet filter; feasibility ещё не принята.
- Acceptance: безопасный owned namespace path без host networking/firewall/protected resources; no-product fixture control; затем .3 client policy/egress/response evidence и BLOCK; exact cleanup.
- Next: минимальная feasibility/review topology, затем control; не повторять прежний outside-alias probe.
- Status: READY. Production SNAT/bind ради старой fixture запрещён.

# Задачи

Latest checkpoint: **2026-10-21 iPhone DIRECT browsing confirmed; DNS export fixed and owned VPS upgraded**. RED/GREEN87 targeted tests, Linux vet/build0, clean independent review, exact-image entrypoint/API/persistence and7-phase local384MiB UDP/WARP PASS. Same VPS data/phone identity/policy retained, existing sites/database unchanged. Full Linux suite: 8488 passed leaf tests; native exit 1; FAIL: TestByInterface_LoopbackRoot, TestConfDisabled_SuppressedWhenNDMSBouncesBackToRunning. ICMP raw socket refused by cap-drop ALL runner; NDMS settle test failed and is unresolved. Neither package was changed. Full-suite PASS is NOT claimed.. User confirmation after upgrade and controlled device DIRECT/BLOCK/DIRECT, DNS/IPv6/no-bypass/WARP/full product acceptance remain OPEN. Evidence: scripts/m2-local-20261021/codex-client-dns-66ef0989. Earlier checkpoints below are historical.

Latest checkpoint: **2026-10-21 first user-authorized Beget VPS START PASS; iPhone traffic PENDING**. Exact trial image healthy, real AWG interface/API ready, panel loaded via SSH, personal iPhone client/profile created. Original sites/status/content hashes and database state preserved. Current idle53.77MiB, disk~1.31GiB free. No server data/cache cleanup. No external-peer handshake yet; VPS payload/policy/WARP/device acceptance OPEN. Evidence: scripts/m2-local-20261021/codex-beget-trial-66ef0989. Earlier checkpoints below are historical.

Latest checkpoint: **2026-10-21 personal trial UI/image/bundle PASS locally**. Gateway basic visibility fixed; browser create/QR/disable/enable/revoke/reload verified. Fresh exact image, normal entrypoint/API/persistence and seven-phase real AWG/WARP UDP PASS, repeated at384MiB. Portable archive/19OCI blobs/Compose verified; own cleanup/protected9containers33volumes retained. Beget read-only preflight only; first VPS/device acceptance and full product remain OPEN. Evidence: scripts/m2-local-20261021/codex-personal-trial-66ef0989. Earlier checkpoints below are historical.

Latest checkpoint: **2026-10-20 packaged personal-trial image core PASS; browser Gateway visibility fix IN PROGRESS**. Normal entrypoint/API/AWG lifecycle/persistence and packaged seven-phase real UDP/WARP verified. Default basic UI hides Gateway; targeted regression RED witnessed, minimal fix and rebuilt UI pending. No final bundle/VPS acceptance.

Evidence: scripts/m2-local-20261020/codex-personal-trial-66ef0989/awg-personal-trial-core-result-2026-10-20.json. Own containers/volume removed; protected9containers and33priorvolumes retained. UI changes are not yet in the verified image; previous source audit applies historically. No provider/host/VPS/Git changes.

Historical checkpoint: **2026-10-20 real AWG + Cloudflare WARP UDP seven-phase PASS**. DIRECT/BLOCK/DIRECT/WARP-up/WARP-down/DIRECT-control/WARP-restored verified with actual policy TUN transit. HTTPS seven-phase PASS retained. Whole product image, browser workflow and VPS/user-device trial remain open.

Evidence: scripts/m2-local-20261020/codex-udp-tun-66ef0989/awg-udp-tun-result-2026-10-20.json. Fresh Linux23/23, vet/build0, Python14/14, independent final review0 findings. BLOCK reject1/direct0; WARP outage drops3/direct0/no delivery; control and restoration deliver. Exact owned cleanup/protected9. NFQUEUE stays0; separately named acceptance mode requires actual TUNTX all7, positiveRX and correlatedDIRECT events. Legacy queue gate preserved; exact queue-skip cause not localized. No product/provider/host/VPS/Git change. Earlier checkpoints are historical.

Historical checkpoint: **2026-10-20 UDP queue diagnostic complete; full UDP acceptance HOLD, path unexplained**. Real AWG DIRECT STUN response verified, legacy and structured prerouting queue counters both zero. Selector-mismatch hypothesis unconfirmed; full UDP run NOT STARTED. HTTPS seven-phase PASS retained. UDP BLOCK/WARP, whole-image UI and VPS/user-device trial remain open.

Evidence: scripts/m2-local-20261020/codex-udp-counter-66ef0989/awg-udp-counter-result-2026-10-20.json. Fresh Linux Go17/17 and vet0; Python8/8; pre-runtime independent review0 Critical/Important/Minor. Diagnostic native/parent0, exact owned container absent, all9 pre-existing IDs retained. No product code/host/provider/VPS/Git changes. Next: bounded observation of actual UDP route/interception point before full BLOCK/outage/restoration; no assertion relaxation. Earlier checkpoints below retain their historical scope.

Historical checkpoint: **2026-10-13 UDP STUN DIRECT reply observed; full UDP probe HOLD on queue-counter attribution**. HTTPS BLOCK/WARP-outage/restoration PASS retained. UDP BLOCK/WARP/user-device/UI/VPS remain open.

Статусы: BACKLOG / READY / IN PROGRESS / REVIEW / TESTING / BLOCKED / DONE. DONE — acceptance criteria выполнены и evidence относится к принятой версии.

## M2 — real AWG, current development slice

- Current packet task: BLOCKED. HOLD_M2_LOCAL_AWG_DATA_PATH; checkpoint scripts/m2-local-20261008/codex-local-packet-b15343f9-9010-4306-acaa-d21a3a652a2a/RESULT.json. Accepted standalone binary unchanged; real TUN/network-none local probe native1/OOMfalse. All11 AWG fields read back, AWG H1/H2 exchange and client handshake mark observed, but server confirmation0 and no exact ICMP echo/no configured H4 transport; full packet gate FAIL. WG-mode negative and positive2 NOT RUN. Source-backed stale-S4-before-blocking-read hypothesis unproven. Fixture validators/vet/build native0; canonical CAP_NET_ADMIN metadata correction only. Server native join/socket/TUN cleanup and six exact container absences verified; final original8 IDs retained, owned build copies removed. Prior IPC/build ACCEPT retained, M1 unchanged; no manager/UI/VPS/Git/backend changes. Early-blocker report.
- Next narrow diagnostic: record sanitized packet length/header offsets0/25 and reproduce blocked-TUN-read S4 transition; no assertion relaxation or packet-ready claim.

- Current IPC/build task: DONE for bounded local slice. ACCEPT_M2_LOCAL_IPC_BUILD_SLICE; checkpoint scripts/m2-local-20261008/codex-ipc-build-2e7e1740-806f-4843-b7c1-fb7258a958e9/RESULT.json. Confirmed inotify-before-cancel shutdown race fixed by cancellation-first ordering; no ignored errors or weakened assertion. Fresh original behavioral RED2/10, corrected full IPC GREEN10/10 (noop child excluded), descriptor regression100/100, formatting/vet native0. Separate Linux/amd64 corrected binary BUILT, native0/OOMfalse, SHA256 333d0f224ad8bb2645add59e65061ab5b31c67c59e01344ec24d430037447a31; exact source/dependency/image pins and independent read-only review accepted. All10 disposable containers absent by receipts; final original8 IDs retained, owned build copies/caches removed. M2 handshake/traffic/product OPEN; preserved repeated-Close/partial-startup/background-join concerns remain open. M1 retained; no manager/UI/TUN/VPS/Git or service restart.
- Next tasks remain OPEN: true keyed AWG handshake/payload and WG-negative control; manager/panel/product integration. Before accepting broader lifecycle correctness, address once-only Close, partial-startup cleanup, stop-aware sends/background join and surfaced shutdown errors. This checkpoint does not authorize those runtime stages.

### Historical M2 checkpoints — superseded for current IPC/build state
- Current Codex continuation after user Docker start: HOLD_M2_CODEX_DOCKER_PIPE_ACCESS_DENIED; checkpoint scripts/m2-local-20261008/codex-ipc-preflight-f56cb709-8241-4cf1-b95f-7809a6e9c257/ACCESS_BLOCKER.json. Native read-only inventory exit1/permission denied; narrow pipe permission API unsupported. No container create, IPC tests, build or permission bypass; source hashes retained. Prior engine-unavailable stop is historical.
- Current Codex continuation: HOLD_M2_CODEX_DOCKER_ENGINE_UNAVAILABLE. Explicit GO received; read-only selected Docker-engine inventory failed before any create/test/build. Checkpoint scripts/m2-local-20261008/codex-ipc-preflight-f56cb709-8241-4cf1-b95f-7809a6e9c257/RESULT.json. Source preserved, no patched binary, no Docker/service restart or automatic continuation. Previous real IPC failure remains historical unresolved evidence.
- Status: HOLD_M2_REAL_IPC_CLOSE_FAILURE; checkpoint scripts/m2-local-20261008/f1-build-034039a7-9dc5-485f-8c07-e355fa1b9c36/RESULT.json. Early-blocker STOP within authorized two-hour budget. No patched binary, manager/UI/VPS or automatic continuation.
- Source slice: existing AWG profile/registry/export and isolated userspace manager remain not production-wired; selected child runtime overlay remains candidate. Durable prior production/test source and historical binaries unchanged.
- Latest actual execution, 2026-10-08: exact public x/sys v0.36.0 zip/mod prepared in owned scratch and h1-verified against go.sum. Real selected IPC original source with identical regression: native exit1/OOMfalse,2 meaningful cases PASS/8 FAIL, including live foreign-resource deletion. Candidate: native exit1/OOMfalse,9 meaningful cases PASS/1 FAIL; descriptor-handoff Close refuses. No-op child helper excluded. Not genuine keyed AWG/TUN/packet proof.
- Blocker: TestF1UAPIOpenDescriptorHandoffRetainsSocket reports selected IPC listener close failed on both observed versions. Root cause not confirmed. Narrow scratch diagnostic only adds closed/badfd/brokenpipe categories; not executed after pre-create manifest refusal and later interactive approval timeout (200 seconds,zero calls); failures retained, no bypass. Build gate not entered; corrected binary NOT BUILT.
- Evidence: scripts/m2-local-20261008/f1-build-034039a7-9dc5-485f-8c07-e355fa1b9c36/RESULT.json; original/candidate native JSON stdout/stderr receipts; dependency-download-manifest.json, two-file selected-runtime-f1-source.patch, archived diagnostic .go.txt. Fresh readback: two owned containers absent, original eight IDs retained (not service health). Existing F2 historical local regression retained, not rerun.
- Review: not dispatched because real IPC gate FAIL. Next only bounded Close diagnosis and actual package GREEN, then pinned binary build/final checks/one review; no assertion relaxation, no dependency substitution, no new framework. Full product/M1 scope, resource/ownership limits and no commit/push preserved.

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
- Current slice: ACCEPT by consolidated independent review deleg_cae50fe0; coordinator read INDEPENDENT_INVALIDATION_FIX_REVIEW.md. Actual CLI retained RED/fixed GREEN records /readyz200, same surviving engine, direct target-listener control and target sink delta 0 -> 1 through HTTP proxy/mixed inbound, not a SOCKS handshake. Six source hashes and retained binaries verified; Linux safety/retry regressions independently repeated. Four affected suites PASS; vet log exit0 alone does not attest command/targets. Evidence gaps remain explicit: combined migration/validation-failure/fresh-retry, atomic-writer fault injection, immutable full build attestation. No AWG/TUN/VPS claim.
- Acceptance: legacy state/content не теряются; current disabled не включается; конфликт сохраняется вне live merge; повторный Bootstrap идемпотентен; после apply нет старого final/duplicate inbound; Linux RED/GREEN/package tests.
- Next: NET-001 authorized fixture/readback prerequisite; не считать миграцию доказательством packet path. Optional evidence hardening gaps above remain BACKLOG, not exercised PASS.
- Status: DONE for the reviewed filename-migration/adoption seam and pinned local manager runtime comparison. Full Gateway functional acceptance remains OPEN. Historical earlier review checkpoints follow; they do not override this accepted current slice. Evidence: 53/55/57 and 66-post-reboot-build-preflight/INDEPENDENT_INVALIDATION_FIX_REVIEW.md.
- Re-review: P1 startup подтверждён. Bootstrap-only guard не защищал earlier NewOperator/URL migrations: исправлен early non-mutating preflight, production RED/GREEN и два internal package suites/vet PASS (55-production-preflight-cleanup). Cleanup early-return/success-status regression в работе; full consolidated suites и review ещё нужны. Предыдущий six-package PASS не выдавать за проверку последней consolidated версии.
- Consolidation: cleanup regression исправлена и прочитана координатором; mock-only failure/order/one-shot/preflight tests проходят. Actual consolidated verification: 13 suites PASS, internal/cleanup compiles with no standalone tests, vet 14 targets PASS. Не запускался реальный uninstall; migration remaining cases и surviving-engine convergence всё ещё открыты.
- Extended acceptance: 38 leaf cases PASS — partial failure/retry/idempotence, old/new × active/disabled/pending × five nonregular kinds, conflict archive excluded from actual MergeDir, absent/custom registration guards. Координатор прочитал новый test, сверил source/log hashes и независимо повторил все 38 cases; production-код не менялся, behavioral RED не потребовался. Orchestrator/configmerge suites и vet exit 0. Evidence: 57-migration-extended-acceptance/resume-20261002T213745Z. POL-001 остаётся OPEN до surviving-engine convergence.

## POL-002 — sniff до terminal Gateway policy
- Facts: Gateway renderer теперь содержит scoped sniff перед DNS/domain match и fallback; focused RED и policy package GREEN ранее получены.
- Proven: реальный engine получает HTTP Host при literal-IP SOCKS request и выбирает domain reject; prior DNS не используется. TLS metadata и AWG/TUN path этим control не проверены.
- Acceptance: no-Router merged order сохраняет QoS priority; literal-IP + HTTP Host BLOCK не достигает listener, allowed control до/после отвечает; логи показывают domain rule; никакого prior DNS.
- Status: DONE для указанного HTTP rule-engine acceptance. После паузы координатор независимо повторил control в свежем owned namespace: without sniff exit 1 и blocked listener hit 1; with sniff exit 0, allowed-before/after отвечают с верным SHA256, blocked listener hits 0. Trace содержит sniffed HTTP domain и domain_suffix reject. Policy/configmerge packages PASS; no-Router test сохраняет QoS priority. Source/binary/image hashes сверены, runtime удалён по exact ID. Evidence: 61-sniff-coordinator-verification. Это SOCKS adapter control, не AWG/TUN DIRECT/BLOCK/VPS proof.

## POL-003 — transport whitespace
- Facts: Validate принимает trimmed token; compiler теперь trims скопированный Network без изменения исходного Profile. Focused RED/GREEN и policy package GREEN получены до переноса.
- Acceptance: tcp/udp/icmp с whitespace компилируются канонически; input не изменяется; текущий Linux policy package проходит.
- Status: DONE. После переноса Linux policy package повторно прошёл; scope — canonical compiler tokens, не packet/VPS routing.

## NET-001 — исправить fixture и доказать DIRECT/BLOCK
- Final independent M1 review: ACCEPT_M1_WG_ONLY_PACKET_PROOF, no blockers; parent verified report SHA and43/43 artifact +7/7 evidence pins. M1_WG_NATIVE_INDEPENDENT_ACCEPTANCE_20261006.json is final M1 checkpoint. Raw successful tuple/body SHA/full traces not archived; pinned mandatory guards plus native STAGE verified support this retained-evidence review. M1 DONE only; real AWG/Internet/full NET-001/product/production remain unproved, M2 not started. No new remote attempt authorized.
- Latest authoritative M1 checkpoint, 2026-10-06: DONE for minimal WG-only one-client/sink DIRECT/BLOCK/DIRECT packet proof. M1_WG_NATIVE_PACKET_PROOF_ACCEPTANCE_20261006.json; native run 05f36645-75c6-4da7-9e5b-0e442a7af45c check0/runtime0, exact unique Gateway WAN source and client SHA on both DIRECT stages, correlated engine direct/reject/direct, BLOCK healthy-Sink no hit, packet counters and handshake PASS. Independent final exact cleanup/protected fingerprint PASS, PSI0/OOM0; 100 local tests/bash-n and 43 pins PASS. Only fixture native-observed bracketed mapped peer equivalence corrected; no arbitrary normalization/other source/duplicate acceptance. M1 supersedes earlier minimal-path HOLD notes, NOT the full NET-001/true AWG/Internet/product/production acceptance. Next M2 real AWG, not started. Both separately approved single diagnostic/confirmation attempt budgets consumed; no further automatic runtime, host edits or commit/push.
- Authoritative NET-001-A result: local F1/F2 ACCEPT_LOCAL_INERT_FIX_SLICE, full report/hash/exact final sources verified;150 tests PASS, prior originals/C2 preserved. Evidence NET_001_A_FIX1_LOCAL_ACCEPTANCE_20261005.json. User direction: stop universal stand expansion; minimal working AWG DIRECT/BLOCK with real traffic, affected tests during development/full suite at milestones. Site safety/exact cleanup remain required. Runtime admission NOT ACCEPTED, Gateway OPEN; no new implementation delegation or remote operation. Earlier pending/HOLD entries below historical.
- Current NET-001-A: REVIEW. Candidate producer/binding validator implemented in two new scripts; coordinator143 tests PASS,8source hash/AST/compile and33evidence hash/size checks PASS. Independent review deleg_4eafe9e8 pending. No trusted admission/runtime/Linux proof or stage-owner implementation; original108 and accepted C2 preserved. Evidence NET_001_A_COORDINATOR_VERIFIED_REVIEW_PENDING_20261005.json. Earlier implementation/design dispatch entries below superseded.
- Current subtask NET-001-A: IN PROGRESS. Design report fully read/hash verified, ACCEPT_DESIGN for candidate full-baseline producer + independent binding check ONLY. Implementation deleg_c5edf49a: two new scripts, behavioral RED/GREEN, original108-test regression and indexed actual evidence. No accepted controller/artifact changes, CLI unlock or VPS execution. Stage owner/exact-ID recovery contract designed but implementation deferred and still required before mutations. Candidate/receipt structure is not independent reviewer authority/admission. Runtime ordering: admission → network-none → attributed bridge → AWG DIRECT/BLOCK/DIRECT. No generic extensions.
- Final cycle2: ACCEPT_LIMITED_LOCAL_FIX_SLICE. Independent report fully read and report/source hashes verified by coordinator. Independent 108 tests PASS plus 14 inert fault/retry/reopen scenarios; C2.a/C2.b closed, L1-L5/C1 retained, no concrete local fix blockers. Evidence: 70-vps-no-product-control/COORDINATOR_CYCLE2_LIMITED_LOCAL_ACCEPTANCE_20261005.json. NET-001 actual runtime/full-baseline/load/capability/bridge/recover/verify-clean still HOLD / NOT TESTED; Gateway OPEN. Next milestone requires separate concrete runtime producers and independent admission. Older pending entries below superseded; no cycle3, VPS/commit/push.

### История NET-001 — статусы на момент записи, не текущие задания
- Historical final-cycle2 verification, 2026-10-05: coordinator108 tests PASS; independent review was pending then, superseded by final acceptance above. Evidence: COORDINATOR_CYCLE2_VERIFIED_FINAL_REVIEW_PENDING_20261005.json. No VPS/commit/push.
- Latest cycle1 review/final fix, 2026-10-05: IN PROGRESS. Exact independent report read/rehashed: L1-L5/C1 accepted in limited-local scope; C2 remains HOLD due work_failed/B2/final-save uncertainty becoming CLEAN and permitting reuse. Final cycle2 deleg_e7aaaf7c limited to C2 real durable-failure regressions, no adjacent runtime work. Previous 102 PASS is not coverage of those counterexamples; new final tests/review pending. Helper accepted offline unchanged, no VPS actions, commit/push forbidden. Evidence: 70-vps-no-product-control/CYCLE1_PARTIAL_ACCEPT_C2_FINAL_FIX_PENDING_20261005.json.
- Latest cycle1 verification, 2026-10-05: REVIEW. Coordinator repeated 102 tests (51 controller + 34 helper + 17 neutral), exit0, AST/compile six files PASS; final source/test hashes and 27 indexed logs independently match. Accepted helper archive/source unchanged. L1-L5/C1/C2 implementations await independent re-review deleg_6f87b09c; no fixed-slice acceptance or runtime claim yet. Unknown Docker/nft templates and host mappings intentionally HOLD. Evidence: 70-vps-no-product-control/COORDINATOR_CYCLE1_VERIFIED_REREVIEW_PENDING_20261005.json. All VPS load/capability/bridge/recovery gates remain HOLD, no commit/push.
- Latest review/fix checkpoint, 2026-10-05: IN PROGRESS. BusyBox artifact ACCEPT_OFFLINE_HELPER_SLICE; exact archive/report/review hashes independently read back, no runtime acceptance. Controller HOLD L1-L5 plus C1/C2 before runtime wiring; fix cycle1 deleg_be75888e owns only controller/tests and new local evidence. Baseline 80 tests passed but did not detect reviewer counterexamples; final fixed-version tests/re-review pending. Helper source/archive remains frozen, all VPS execution gates HOLD; no commit/push. Evidence: 70-vps-no-product-control/HELPER_ACCEPTANCE_AND_RUNNER_FIX_CYCLE1_20261005.json.
- Latest local checkpoint, 2026-10-05: REVIEW. Explicit soft-pause resume; coordinator repeated all 80 local tests (29 controller + 34 helper + 17 neutral), exit0, AST six files PASS. New actual BusyBox fixture archive SHA/size and unchanged original archives verified; independent artifact/ELF and controller reviews deleg_2ac952fb pending. Concrete read-only observation/limited load decisions exist, but actual load CLI lacks independently admitted full baseline; capability/bridge/recover/verify-clean runtime producers remain unwired/HOLD. No VPS actions or temporary bridge this resume; full DIRECT/BLOCK gate OPEN. Evidence: 70-vps-no-product-control/COORDINATOR_POST_SOFT_PAUSE_RESUME_20261005.json. Older status paragraphs below are historical.
- Current status, 2026-10-04: REVIEW. First shared VPS explicitly authorized for bounded Docker preparation; installation and pinned image checksum-only acceptance verified from 69-vps-docker-install/INSTALLATION_ACCEPTANCE_20261004_210741.json and exact POSTINSTALL_R3 SHA256/fields. Actual host firewall exports now available; no temporary bridge/product traffic tested. 70-vps-no-product-control/PROPOSED_RUNBOOK_20261004_213721.md sent to independent read-only review deleg_34cb9cdc. Fresh capacity/workload/tool gates and tested/reviewed runnable harness precede bridge execution. Temporary-rule cleanup and product DIRECT/BLOCK remain OPEN. Old BLOCKED paragraph below is historical local Docker Desktop evidence; second Hermes VPS/protected resources/manual host rules remain excluded. No commit/push.
- Facts: независимый sender/sink control без AWG/Sing-box воспроизвёл outside-subnet alias failure на Docker --internal. Inside-subnet alias и bridge address работают. Это fixture confound.
- Not proven: точный deployed DROP rule; корректный non-local DIRECT path в исправленной fixture.
- Hypothesis: disposable namespace-only veth устраняет internal bridge subnet filter; feasibility ещё не принята.
- Acceptance: безопасный owned namespace path без host networking/firewall/protected resources; no-product fixture control; затем .3 client policy/egress/response evidence и BLOCK; exact cleanup.
- Next: минимальная feasibility/review topology, затем control; не повторять прежний outside-alias probe.
- Status: BLOCKED on actual Docker-host firewall readback prerequisite. Пользователь разрешил только own temporary network/resources и automatic Docker-managed rules, с доказательством удаления rules; commit/push запретил. Read-only investigation и независимые literal probes подтвердили: доступный WSL namespace показывает лишь lo/eth0, не Docker bridges; /sbin/ip есть, iptables-save/nft отсутствуют. Более ранний shell-variable probe недостоверен и отброшен. Network metadata не заменяет actual rules. Diagnostics/archive не собирались, extra namespace/privileges не добавлялись, temporary network ещё не создавалась. Five working containers full IDs/images/restart/start/mount/health match baseline; Go runner завершился штатно после sleep 1800. Evidence: 63-disposable-network-preflight/FIREWALL-READBACK-BLOCKER.md and COORDINATOR_READBACK_BLOCKER.json. Production SNAT/bind ради fixture запрещён.


## Codex local transport checkpoint 2026-10-08T19:56:32.648852+00:00

PASS_LOCAL_PACKET_WITH_FULL_SUITE_HOLD: first-packet S4 startup bug reproduced and candidate local packet sequence passed. General suite setup blocked by exact test dependency inputs; candidate not promoted. Prior accepted IPC/build retained. Evidence: scripts/m2-local-20261008/codex-local-transport-5a476159-91c9-48b8-843f-193858d1fd10/STATUS.md. No manager/UI/VPS changes.


## Codex local suite checkpoint 2026-10-08T20:08:55.852333+00:00

HOLD_EXTERNAL_OUTLINE_INTEGRATION_TEST: exact dependency archives26, CGO0 device/IPC tests pass but Outline build fails; CGO1 full suite native1. Candidate unchanged and not installed in product. Evidence scripts/m2-local-20261008/codex-local-suite-13ff66d9-1a04-4d13-8e2d-74c0c1c7d168/STATUS.md. No manager/UI/VPS scope.


## Codex local Outline checkpoint 2026-10-08T23:24:10.609829+00:00

HOLD_LOCAL_OUTLINE_FIXTURE_BUILD_TIMEOUT: fixture tests3 passed in partial native log, full pipeline timed out360s; vet/build not confirmed, helper absent, TCP protocol probe NOT_RUN. Warm caches hash-archived; all disposable resources cleaned. Prior AWG packet proof retained. Evidence scripts/m2-local-20261009/codex-local-outline-3045e330-8de4-497f-b424-4103d630a888/STATUS.md. No product/VPS changes.


## Codex local Outline success 2026-10-08T23:44:34.010608+00:00

PASS_LOCAL_OUTLINE_FALLBACK_TCP_OVER_AWG: native vet/build/packet0, exact TCP echo true→false→true, server/wire proof, independent bounded review accepted. Original external Outline test and smart selector/DNS not accepted. Product manager/UI/VPS gate remains open. Evidence scripts/m2-local-20261009/codex-local-outline-success-683e1abe-0c11-429d-875a-445bf4ffe87a/STATUS.md.


## Codex local manager startup 2026-10-09T00:15:19.062427+00:00

PASS_BOUNDED_MANAGER_STARTUP_WITH_BUILD_HOLD: explicit backend/profile admission, owned cleanup before semantic preflight/identity, shared client export profile. Startup RED4/GREEN4, JSON RED/GREEN; earlier regression/vet302 native0, unchanged production; final restored package PASS304 in partial native Go JSON. Combined final tests/vet/build timeout180, manager binary NOT_BUILT; independent bounded source review no remaining important findings. Manager packet/peer-status lifecycle, product image/UI/VPS gates OPEN. Evidence: scripts/m2-local-20261009/codex-manager-startup-66ef0989-4140-4902-beac-2b9f0a3ca7c0/STATUS.md.


## Codex AWG owned client control 2026-10-09T00:47:38.266954+00:00

PASS_OWNED_AWG_REGISTRY_CONTROL_ONLY: manager build resumed native0, peer owner guarded lifecycle and streaming public status; full affected tests316/vet/build0, actual Registry presence true/false/true/false with real AWG/tool, formatted realcontrol+vet0. Independent bounded review accepted. Only exact IP-address/up commands injected: no full-app/HTTP/addressing/routing/packet/UI/VPS acceptance. Exact own cleanup+baseline8 retained. Evidence: scripts/m2-local-20261009/codex-owned-client-control-66ef0989/STATUS.md. Prior manager build HOLD superseded for this local binary; product image gate remains OPEN.


## Codex manager-owned AWG packets — 2026-10-09

PASS_MANAGER_OWNER_REGISTRY_AWG_PACKETS: real address/UP/connected return route with no injected IP commands; fresh encrypted ICMP echo active/disabled/enabled/revoked true/false/true/false; public status/counters and AWG wire observed, actual child/socket/TUN cleanup and baseline8 verified. Formatted two-package tests318 and vet native0; independent bounded review accepted. Fixture GLIBC mismatch corrected without production behavior changes. Two explicitly gated real integration tests added. Full-app HTTP, policy/forwarding/recovery, product image/panel, real device/VPS remain OPEN. Approved personal-first scope: shared rules for own devices; advanced groups/links/expiry deferred, not removed. Evidence scripts/m2-local-20261009/codex-manager-packets-66ef0989/STATUS.md.


## Codex full executable HTTP AWG lifecycle — 2026-10-09

PASS_FULL_APP_HTTP_AWG_LIFECYCLE: Linux manager with embedded existing frontend built native0; actual application/AWG/sing-box readiness and HTTP create/config/disable/enable/revoke, public peers true/false/true/false. Normal actual process restart preserves server identity and revoked device, profile denied409 with exact error and no config. Helper review gaps RED3/GREEN3, helper vet/build0, final full-app native0, mandatory graceful/socket/TUN cleanup and exact disposable-ID removal, baseline8 retained. Production Go code unchanged. Runtime image is fixture only, network-none/WAN lo; GUI, packets through HTTP-created device, shared policy/forwarding, crash recovery/backup, product image/device/VPS gates remain OPEN. Evidence scripts/m2-local-20261009/codex-full-app-http-66ef0989/STATUS.md.


## Codex full-app shared policy TCP — 2026-10-09

PASS_FULL_APP_SHARED_POLICY_TCP: HTTP-created device, actual AWG and sing-box/nft, DIRECT→BLOCK→DIRECT fresh TCP true/false/true and sink1/0/1; exact applied hash/readiness, fresh handshake/RX, connection-correlated engine rejection. Helper review fixes RED/GREEN, final tests4/vet/build0/native0. AWG1/sing-box4 child identities exited, all owned TUN/socket/sink/container cleanup and baseline8 retained. Production Go unchanged. Controlled virtual remote in network-none fixture, not external/device/VPS acceptance. Full remaining gates and deferred native-receipt provenance minor listed in scripts/m2-local-20261009/codex-full-app-policy-66ef0989/STATUS.md.


## Codex target IP/port/TCP/UDP policy — 2026-10-09

PASS_LOCAL_TARGET_TCP_UDP: actual full app/AWG/shared rule, TCP12 + UDP16 requests with two-IP/two-port controls, TCP-rule exclusion on UDP and restoration after disabling. Fresh authenticated handshakes/RX; UDP kernel ingress; exact engine rejection mandatory and retained separately from nft. Independent review Important attribution gap fixed RED/GREEN in one pass, final tests/vet/build/runtime0. Source/helper/artifact pins enforced and included in native receipts. All owned cleanup verified; baseline8 retained. Production unchanged. Evidence: scripts/m2-local-20261009/codex-target-udp-policy-66ef0989/STATUS.md. DNS/domain/fail-closed/VPN/WARP/product image/real-device/VPS remain open.


## Codex DNS interception / domain HTTP Host — 2026-10-09

PASS_LOCAL_DNS_AND_DOMAIN_HTTP: two ordinary/nonstandard-port UDP DNS AWG probes redirected to settings-selected bootstrap resolver; exact nonce response and resolver receipt, fresh authenticated HS/RX/kernel ingress/new hijack log and exact slots/hash/readiness. Six HTTP Host requests at one IP/port: selected suffix BLOCK only while enabled, other suffix passes, disable restores. Same-connection sniffed Host/domain rule/rejected close retained. Test/vet/build/runtime0, independent review and cleanup evidence at scripts/m2-local-20261009/codex-dns-domain-66ef0989. Source/helper/binary provenance enforced; all own cleanup/baseline8. Production unchanged. DoH/DoT/TLS/QUIC/DNS-derived-IP/default-BLOCK fail-closed/VPN/WARP/product image/real-device/VPS remain open.


## Codex hung policy-engine no-bypass — 2026-10-09

PASS_FULL_APP_HUNG_ENGINE_NO_BYPASS: six real-AWG TCP/UDP before/frozen/restored requests; delivery1/1→0/0→1/1. Exact owned PID/start T-state through both outage requests, fresh handshake/RX/kernel ingress, unchanged gateway firewall and independent remote sink alive. Mandatory process/TUN/socket/container cleanup and baseline8. Helper RED/GREEN, test/vet/build/native0, review/provenance at scripts/m2-local-20261009/codex-engine-outage-66ef0989. Production unchanged. Known issue: /readyz reports ready during frozen engine; responsiveness gate next. No crash/selected-exit/general fail-closed acceptance.


## Codex responsive readiness — 2026-10-09

Active readyz now requires bounded valid local engine response; real AWG freeze/resume200→503 unresponsive→200.774 tests/vet/build0; native six TCP/UDP phases1/1→0/0→1/1,2 queued deliveries only after resume. Exactly4 product/test files promoted; source/artifact hashes, final review and cleanup8 in scripts/m2-local-20261009/codex-responsive-readiness-66ef0989. Running supervision and healthz unchanged. No general fail-closed/crash/selected-exit/product-image/device/VPS acceptance. Next crash/removal and VPN/WARP exits, then personal panel/image/device trial.


## Codex killed-engine packet gate — 2026-10-10 Moscow

PASS_FULL_APP_KILLED_ENGINE_NO_BYPASS. Real full-app AWG TCP/UDP healthy1/1→SIGKILL absent0/0. Exact PID/start kill, all-engine absence boundary snapshots,503not_running, fresh HS/RX/kernel ingress, unchanged firewall, independent receiver and synchronized history no outage nonce. Review Important absence-read-error gap RED→GREEN, final6 tests/vet/build0 and matching native rerun; exact owned cleanup/baseline8. Product source unchanged. Evidence scripts/m2-local-20261010/codex-engine-crash-66ef0989. Automatic recovery and selected-exit/general fail-closed acceptance still open.


## Codex automatic policy-engine recovery — 2026-10-10 Moscow

Actual full app/AWG healthy TCP/UDP1/1→SIGKILL absent0/0→stock watchdog replacement1/1. Readiness200→503→200, new PID/start stable, old identity gone and identical applied shared DIRECT hash. Fresh HS/RX/ingress/firewallSHA, synchronized receiver histories, provenance/helper tests/vet/build/native0, exact own cleanup/baseline8. Product unchanged. Evidence scripts/m2-local-20261010/codex-engine-recovery-66ef0989. No selected-exit/whole-app-crash/general fail-closed/device/VPS acceptance. Next VPN/WARP exits.


## Codex selected AWG VPN exit — 2026-10-10 Moscow

Actual full-app HTTP selected AWG endpoint with distinct VPN/DIRECT nonce receivers:10 fresh TCP/UDP phases DIRECT1/1→VPN1/1→exit Down0/0→exit UpVPN1/1→explicitDIRECT1/1. Same selected policy hash across outage, upstream RX/HS, fresh ingress HS/RX/inner ingress, stable engine/nft, full receiver histories no outage nonce. Helper tests/vet/build/native/provenance/final review and own cleanup/protected8. Product unchanged. Evidence scripts/m2-local-20261010/codex-selected-vpn-66ef0989. NOT Cloudflare WARP/external-provider/GUI/product-image/device/VPS acceptance. Current WARP compiler helper remains unwired in product runtime; next WARP provider integration.


## Codex WARP profile/store foundation — 2026-10-11 Moscow

PASS_LOCAL_WARP_PROFILE_ENCRYPTED_STORE. Four new package files promoted after final review. 320 unique Linux leaf tests across warp/storage, vet/build0, CGO1 warp race tests0. Strict profile import, encrypted actual DeviceCipher persistence, safe projection, key loss/corruption refusal, failed-write retention and single-Store concurrency. Cleanup/protected8/source pins verified. NO WARP endpoint generation/orchestrator/runtime/HTTP/Cloudflare/device/VPS acceptance.
 Evidence scripts/m2-local-20261011/codex-warp-foundation-66ef0989.


## Codex WARP runtime — 2026-10-11

PASS_LOCAL_WARP_RUNTIME_STARTUP_CONFIG. Exact13-file delta promoted. Linux551 leaf tests, vet/build0; actual renderer output accepted pinned sing-box1.14.0-awgm.16. Encrypted profile restored at startup; absent/invalid/stale WARP rejected without DIRECT fallback; runtime configs newly written/replaced0600. No HTTP/panel/Cloudflare/actual WARP packet acceptance. Protected8 retained, owned containers removed, owned accidental volume removed.
 Evidence scripts/m2-local-20261011/codex-warp-runtime-66ef0989.


## Codex WARP import — 2026-10-12

PASS_LOCAL_WARP_IMPORT_CONFIG_TRANSACTION. Exact8-file delta promoted; original7-file review Important fixed by executor RED/GREEN (no re-review). Linux955 unique leaf tests, vet/build0. GET /api/gateway/warp secret-free projection; POST /api/gateway/warp/import strict <=64KiB profile, preflight validation, encrypted persistence before validated slot commit, exact-envelope rollback, strict rollback-directory fsync and rollback-failure poison. Existing admin auth configuration; auth-enabled unauthenticated401 before dispatch and valid bearer access verified. Real filesystem/orchestrator rejection preserves slot+encrypted envelope; real backend HTTP import restored by startup helper. No synchronous engine apply/Cloudflare/actual packet/UI/VPS/product E2E/race acceptance. Protected8 retained, owned containers removed; bounded existing cache trimmed, no new cache.
 Evidence scripts/m2-local-20261012/codex-warp-import-66ef0989.


## Controlled WARP packet gate — 2026-10-12

PASS_FULL_APP_CONTROLLED_WARP_NO_DIRECT_FALLBACK. Actual accepted full-app/AWG ingress -> HTTP WARP import -> selected shared WARP policy -> controlled authenticated/decrypted WG-compatible exit.10 fresh TCP/UDP probes DIRECT -> WARP Up -> WARP Down -> restored WARP -> explicit DIRECT; distinct receipts/direct0 on WARP and outage, crypto RX/HS, expected nonzero reserved receive delta/no mismatches. Exact hash/readiness/stable engine/nft and receiver snapshots; no outage nonces through final snapshots. Product source unchanged. No Cloudflare/controlplane/real-provider/infinite-queue/reconnect-SLA/device/UI/VPS/general failclosed acceptance. Reserved adapter explicitly emulates header normalization [0,127,255]; peerkeepalive1s/product25s. Helpertests6/vet/build/native0; source/artifact/client pins and own cleanup/baseline8.
 Evidence scripts/m2-local-20261012/codex-controlled-warp-66ef0989.


## Cloudflare profile preparation — 2026-10-12

READY_FOR_CLOUDFLARE_CONSENT_PREPARATION_ONLY. No real Cloudflare connection/registration or product change. wgcf2.3.1 release SHA verified, network-none help/buildinfo native0; protected8 retained, exact owned containers removed. Operator must obtain explicit user consent before accepting terms/external registration. Need private encrypted credential retention/converter prepared+tested BEFORE external call; one registration max after preparation. Budget next30min, no host/VPS changes.
 Evidence scripts/m2-local-20261012/codex-cloudflare-preparation-66ef0989.


## Codex 2026-10-12 — Cloudflare registration checkpoint

PASS_SINGLE_CLOUDFLARE_WARP_REGISTRATION_PRIVATE_RETENTION: explicit one-account consent, exactly1 attempt, native0, encrypted host readback aftercleanup. Linux8/8 + capture4/4, vet/build0; reviewImportant2 fixed RED→GREEN, Minor1 deferred (bounded ciphertextreadback). Protected8 retained; no product/VPS/host/Git changes. Evidence scripts/m2-local-20261012/codex-cloudflare-registration-66ef0989/awg-cloudflare-registration-result.json. Next reuse existing account (NO new registration), retrieve actual reserved/endpoint/address/peer data and test actualCloudflare WARP packets/fail-closed. Registration PASS; profile/tunnel/Cloudflare packet/VPS NOT TESTED.


## Codex 2026-10-12 — real Cloudflare profile checkpoint

PASS_REAL_CLOUDFLARE_PROFILE_EXTRACTION_MANAGER_VALIDATION_WARP_INACTIVE: one authenticated verifiedTLS GET200, existing account, zero registrations/mutations. Actual reserved/key/IPv4/port extracted, encrypted host readback and real manager DecodeProfile/Store roundtrip validated native0; Python22/Linuxhelper2/vet/build0. Cloudflare warp_enabled=false retained; candidate NOT applied, actualCloudflarepackets/VPS NOT TESTED. Review prefetch0Critical/0Important/2Minor deferred; two postfetch compatibility fixes executorRED/GREEN, no second review. Protected8 retained. Next activation semantics and actualtunnel with existing account, no new registration. Evidence scripts/m2-local-20261012/codex-cloudflare-profile-66ef0989/awg-cloudflare-profile-result.json.


## Codex 2026-10-12 — actual Cloudflare WARP payload checkpoint

PASS_REAL_CLOUDFLARE_WARP_STANDALONE_TLS_PAYLOAD: one realstarted attempt, parent/native0, freshnonce and verifiedTLS direct warp=off / forcedWARP warp=on. Exact product RenderEndpoint+pinnedengine; sameaccount/profile, zero registrations/settingswrites. Historical APIwarp_enabled=false is not tunnelhealth proof, separateactivation not required in this run. Linux6/vet/build0, freshreview0Critical/0Important/2Minor deferred; narrow postreview data-tmpfs admissionfix verifiednative, no secondreview. Exactcontainer/privateTmp/enginecleanup/protected8 and exactnewunusedvolume cleanup verified. WholeAWGmanagerrealCF/UDP/outage/recovery/VPS OPEN. Next wholemanager→AWG→realWARP proof using existingaccount, then noDIRECToutage/recovery. Evidence scripts/m2-local-20261012/codex-cloudflare-tunnel-66ef0989/awg-cloudflare-tunnel-result.json.

## 2026-10-13 — real manager/AWG/Cloudflare WARP HTTPS chain

PASS_FULL_MANAGER_AWG_REAL_CLOUDFLARE_DIRECT_WARP_DIRECT. Exact accepted application binary: HTTP device creation/config export, existing real WARP profile import and exact endpoint slot, shared policy API apply. Actual AWG netstack client verified HTTPS TLS and fresh nonce through DIRECT/off → WARP/on → DIRECT/off. Each phase: fresh server AWG handshake, RX and kernel ingress deltas, stable applied policy/engine/firewall. One bounded runtime, native/parent exit 0; app/children/socket/TUN cleanup and exact owned Docker removal verified, protected 8 retained. No new registration/provider settings mutation. Manager 3/3 + client 2/2 tests, vet/build 0. Fresh review Important3 fixed in one RED/GREEN pass, wrapper 4/4; no second review. Public evidence allowlisted, no raw runtime streams.

Evidence: scripts/m2-local-20261013/codex-real-warp-chain-66ef0989/AWG-REAL-WARP-CHAIN-REPORT-2026-10-13.md. BLOCK, actual provider outage without DIRECT fallback, restoration, UDP and user-device/UI/VPS remain unaccepted. Next bounded task: real-chain BLOCK and WARP outage/restoration. Whole product scope retained; no product source/binary promotion this slice.

## 2026-10-13 — real BLOCK/outage probe early HOLD

HOLD_REAL_AWG_BLOCK_CLIENT_FAILURE_CLASSIFICATION. Prior real HTTPS DIRECT/WARP/DIRECT PASS retained. One new actual runtime: DIRECT TLS/nonce/off verified; BLOCK applied with fresh AWG handshake/RX2036/ingress3 and one target/source/connection-correlated engine rejection, no delivery. Client setup/attempt true but TransportFailed=false; strict negative gate refused, BLOCK not accepted. WARP outage/restoration phases NOT REACHED. Native/parent1/OOMfalse. App child/socket/TUN cleanup, exact Docker absence and protected8 retention verified; no new registration/provider settings writes/host/VPS/Git mutations. Manager9/9, client3/3, wrapper4/4, vet/build0. Review Important2 fixed one RED/GREEN pass; Minor stale private scope description deferred. Public evidence: scripts/m2-local-20261013/codex-real-warp-fail-66ef0989/AWG-REAL-WARP-FAIL-REPORT-2026-10-13.md.

Next: offline diagnose safe client error classification for gVisor wrapped TCP errors (plain-string conversion source-backed hypothesis; actual error unknown); preserve certificate/trace false-failure rejection. Then new bounded BLOCK/outage/restoration probe, no blind retries. Product scope unchanged; product binary/source not promoted by this test-only slice.

## 2026-10-13 — real AWG/WARP HTTPS BLOCK/outage/restoration PASS

PASS_REAL_AWG_WARP_BLOCK_OUTAGE_RESTORE_HTTPS. Seven native phases: DIRECT/off → BLOCK/reset/no delivery → DIRECT/off → WARP/on → WARP-down/timeout/no delivery → DIRECT-control/off under same provider drop → WARP-restored/on. Every phase has live fresh AWG handshake/RX/kernel ingress and stable applied policy/engine/firewall. BLOCK target/source/connection-correlated engine reject1. WARP-down exact provider endpoint UDP drop5, direct TCP443 target packets0; successful DIRECT control proves other Internet availability. Positive phases verify TLS and fresh nonce. Original accepted manager binary/profile HTTP import retained; exact real WARP slot verified.

Prior HOLD closed by known gVisor TCP error classification, reproduced on actual in-memory closed port RED→GREEN. Unknown/TLS errors remain refused; only allowlisted categories retained. Client6/6, manager10/10, wrapper5/5; Linux vet/build0. Fresh independent review Critical0 Important0/no new Minor; historical private scope wording deferred. Single runtime native/parent0/OOMfalse. Actual outage table/app/children/socket/TUN cleanup and exact Docker absence/protected8 verified. No new registration/provider settings mutation/host/VPS/Git changes, no product source/binary promotion. Evidence: scripts/m2-local-20261013/codex-real-warp-class-66ef0989/AWG-REAL-WARP-CLASS-REPORT-2026-10-13.md.

DONE only for bounded single-IPv4 HTTPS/TCP fail-closed proof; not whole M2/product acceptance. NEXT: real-chain UDP payload and refusal/restoration; user-device/full UI/VPS acceptance remain open. Full product scope preserved. Prior HOLD and positive results below/above remain historical evidence.

## 2026-10-13 — real UDP STUN early HOLD on queue-counter attribution

HOLD_REAL_AWG_UDP_QUEUE_COUNTER_ATTRIBUTION. First DIRECT phase produced actual connected UDP STUN reply with correct fresh transaction/strict parsing, fresh AWG handshake/RX308/kernel ingress2, Sent2/direct UDP3478 packets2, stable policy/engine/firewall. QueueDelta0 violates current strict NFQUEUE gate. Native/parent1/OOMfalse; complete UDP acceptance refused. UDP BLOCK/WARP-up/outage/no-DIRECT/restoration NOT REACHED. No product-path failure inference: exact counter applicability/attribution unresolved; helper selects first global queue regex without family/table/chain. Next bounded task: diagnose relevant forwarded AWG UDP counters, reproduce selector/applicability problem, RED/GREEN fix; do not simply drop requirement.

Prior HTTPS BLOCK/WARP-outage/restoration PASS retained. STUN tests3/3 + inherited HTTPS6/6 cached, manager14/14, wrapper4/4, Linux vet/build0; observed RED before implementation. Fresh review Critical0 Important0, Minor stale private scope string deferred. Single runtime, actual monitor table/app/children/socket/TUN cleanup, exact Docker absence/protected8 retained. No registration/settings/host/VPS/Git/product-source/binary changes. Evidence: scripts/m2-local-20261013/codex-real-warp-udp-66ef0989/AWG-REAL-WARP-UDP-REPORT-2026-10-13.md. Public mapped addresses/raw runtime logs not retained. Scope single IPv4 STUN transport reply, not crypto service identity/general UDP/full product/user-device/UI/VPS acceptance. Full product scope unchanged.

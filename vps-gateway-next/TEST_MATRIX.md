# Original editor client profile

The local original preview now uses the existing client registry and profile handlers. Client mutations share the policy owner gate. The browser shows client creation and lifecycle for this mode, while routing remains in the original editor.

The previous binary refused the client flow. The new Linux build passed client creation, protected AWG configuration export, live peer installation, disable, enable, and revoke. The existing server capability tests passed. An AWG client connected using the exported profile and received HTTPS through WARP. The app restarted, reinstalled the persisted peer, exported byte-identical configuration, and the same profile passed again.

The browser created a test device, opened its profile and rendered the QR code. The attempt to observe a browser download timed out. The same configuration endpoint returned HTTP 200 and the exported profile was independently used in the real connection test. The temporary Docker inventories were verified after cleanup. Phone testing remains pending.

# Original editor real WARP verification

The exact Linux manager binary from the original browser DNS stage was exercised in disposable bridge-network containers with real userspace AWG ingress and the existing encrypted provider profile mounted read-only. Only synthetic probe traffic was sent. Profile plaintext stayed in tmpfs and was removed after initialization.

HTTPS trace phases passed DIRECT/off, WARP/on, provider outage/no delivery with zero direct TCP443 packets, DIRECT/off under the same outage, WARP/on after restoration, and WARP/on after application restart. TLS chains and request nonce identity were checked by the existing AWG probe.

DNS phases passed direct resolver response with measured direct UDP53 packets, WARP-detoured response with provider traffic and no direct UDP53 increment, provider outage with no answer and no direct UDP53 increment, restored WARP DNS response, and DNS response after application restart. Every DNS probe required increasing AWG peer RX. Outage counters confirmed an attempted provider packet was dropped. Separate targets avoided reusing a cached positive response.

Both runtime checks exited zero and verified disposable-container cleanup against the baseline container and volume inventories. Source and runtime logs remain local.

A local candidate image awgm-original-trial:20261111 was built from the pinned existing runtime and the tested manager/AWG binaries. Its exact installed manager entrypoint passed original editor read, WARP catalog, DNS configuration and Apply checks on the isolated synthetic profile. It contains no imported provider or client secrets.

Scope remains the fresh-state original composition. The WARP HTTPS check selected the original default route, while the DNS check selected an original domain DNS rule. Browser GitHub preset selection was verified separately. This is not a single iPhone-to-GitHub-on-VPS end-to-end proof. Existing-state migration, online catalog acquisition and broad original editor operations remain open.

The next proposed trial is a separate fresh-state original-editor container on the already authorized Beget host, with bounded memory, private panel access and a separate test profile/port. Existing working deployment migration is a separate gate. The module AGENTS.md forbids VPS deployment without explicit user override, so this candidate requires a new specific deployment authorization.

# Original editor WARP DNS

The opt-in local preview admits the existing DNS requests needed by the rule wizard. The previous binary returned HTTP 501 on DNS server creation. The rebuilt Linux binary accepted the DNS configuration and Apply on the isolated fixture.

Browser verification created GitHub through WARP. The wizard created both resolvers, set DNS globals, added the matching DNS rule, and applied the draft. All six mutation requests returned HTTP 200. The API confirmed geosite-github selects dns-tunnel, dns-tunnel uses detour warp, and hasDraft=false. The browser showed the tunnel resolver. The test container was removed and existing Docker inventory matched.

This verifies local authoring and application on a synthetic WARP profile. Actual DNS and HTTPS traffic through Cloudflare and behavior during outage remain open.

# Original editor WARP choice — 2026-11-09

The local opt-in original editor now lists WARP when its owned endpoint matches its encrypted profile. A fresh original state can bootstrap from a private WARP profile file named by AWG_ORIGINAL_WARP_PROFILE_FILE. Existing state is checked against its owned slot, and a mismatch refuses startup. No profile is included with the build. Only the outbound tag and label are exposed to the browser.

The previous app returned 501 for the outbound catalog. The Linux backend and rebuilt embedded panel passed the isolated acceptance. In the browser the original GitHub template was selected, then Through tunnel and WARP. The rule was saved and applied. The API returned hasDraft=false, geosite-github, and outbound=warp. Existing Docker inventories matched after cleanup.

The wizard's automatic DNS server creation returned 501. This check used a synthetic profile and a private network without internet access. DNS behavior, provider handshake, actual traffic and outage behavior for this original-editor composition remain open. Next, connect the supported DNS path and verify the applied WARP rule with real traffic.

# Original browser Apply feedback — 2026-11-09

The original routing editor now refreshes staging status after a successful Apply. The focused component regression reproduced the missing refresh (1 failed, 5 passed) and passed after the fix (6 passed). The frontend build and Linux embedded app build succeeded.

The disposable browser test opened a prepared GitHub BLOCK above DIRECT draft. A click on Apply returned HTTP 200, the banner disappeared without a page reload, and the API confirmed hasDraft=false and the saved BLOCK, DIRECT order. The exact test container was removed and the existing container and volume inventories matched.

This verifies the one local browser scenario with the pinned GitHub asset. Online rule-set acquisition, additional egress paths and VPS rollout are separate.

# Original browser preview

The opt-in local preview opened the existing sing-box routing editor with 77 templates. GitHub was selected from the catalog, added as BLOCK using the preseeded pinned original asset, dragged above the DIRECT rule, and applied. The move and Apply returned HTTP 200. The server reported hasDraft=false and BLOCK before DIRECT. Reload showed the saved order without the draft banner.

The banner remained stale immediately after Apply. The next targeted change is to refresh the draft status after successful Apply. Online acquisition and additional egress paths remain open. Browser verification used a disposable local container with expert settings. Container and volume inventories matched after cleanup.

# Original GitHub preset — 2026-11-09

The local original application applied the catalog GitHub preset using the pinned original file, changed priority and retained the result after restart. The native matcher recognized github.com. Controlled AWG TCP verified the GitHub Host policy and the nonmatching control. Online acquisition and browser acceptance remain open.

# Original local rule-set files — 2026-11-09

The local original editor now admits inline rule sets compiled by its existing materializer. Native merge output is checked and owned binary assets are copied into the private generation with content hashes reverified at live confirmation. Actual AWG TCP blocked after Apply and restart. Rule-order and journal controls passed again. 2097 passed leaves in 4 packages. The original GitHub asset was acquired and natively decoded. Catalog application and browser acceptance remain open.

# Original local rule priority — 2026-11-09

The experimental local composition admits the original list/add/move handlers. Actual AWG TCP verified conflicting IP/port rules, staged reorder preserving the live decision, explicit Apply changing delivery, and order persistence across clean restart. 1592 passed leaves in three affected packages. Browser and preset assets remain open.

# Original local Apply verification — 2026-11-09

Native exact application plus pinned AWG client/sink: DIRECT delivered, BLOCK withheld, clean restart kept BLOCK, prepared recovery restored DIRECT, accepted recovery restored BLOCK, and subsequent DIRECT delivered. Each phase required fresh AWG handshake and increasing peer RX. Verified owned executable/PID/birth/argv, listener and immutable full config against native merge. Journal crash phases were seeded on disk after clean exit. Four consecutive native passes after bounding retries for just-spawned child identity. 3164 passed leaves in 14 affected packages and vet pass. Browser, assets, DNS and actual interruption/orphan tests remain open.

# Local startup owner WIP — 2026-11-09

Actual local main now holds an exclusive DataDir policy-owner lease before setupCore/settings/auth, through all LIFO cleanup. CLI cleanup and gateway HTTP use the same owner boundary. Fresh original marker claim is implemented only at the low-level API; production original factory remains unwired. Focused23, full Linux 8853 qualified passed leaves/138 packages, 53 explicit skips,3 known raw failures separately NET_RAW pass; fresh vet/embed build and actual-app refusal/panel-lifetime controls pass. Same-DataDir/cooperating-version scope only; external roots, partial initial claim, existing-data handoff, full runtime binding and original Apply/UI remain open. VPS/frozen package unchanged. See docs/original-startup-owner-wip-2026-11-09.md.

# Original recovery WIP — 2026-11-09

Uncommitted Task2 preparation: cold construction/read-only slot discovery plus spawned-child lifecycle identity, pidfd signals, lost-PID duplicate prevention and denied-stop preservation verified. Full Linux 8830 qualified passed leaves/138 packages, 50 explicit skips; only3 known raw permission failures, separately NET_RAW pass. Fresh vet/embed build, native recovery4 and legacy Process controls pass. Actual original factory/HTTP Apply, durable ownership and full generation/asset/controller binding remain unimplemented; editor/VPS/frozen archive unchanged. See docs/original-recovery-wip-2026-11-09.md.

# Legacy owner admission checkpoint — 2026-11-08

Legacy gateway mutations now take the actual app.localConfigGate and check persisted owner before dispatching client/profile/group/subscription/WARP handlers. Unknown/original state refuses local legacy construction before NewOperator reconcile/adoption. Absent owner root preserves existing gateway behavior; reads are preserved. 28 focused RED/GREEN controls pass. Full Linux 8808 qualified passed leaves/138 packages, exits [1, 1, 0] only3 known raw permission tests, separately NET_RAW pass; 46 explicit skips retained. Fresh vet/build and native Process/generation/SRS/raw controls pass. The fresh application binary also refuses original/unknown/unclaimed startup before sing-box tree creation. This is Task1 admission only: no owner is claimed, no interprocess exclusivity/durable handoff or original startup/recovery factory implemented. Original HTTP Apply/runtime binding/UI/AWG/TUN vertical remains open; editor, VPS, frozen package and frontend unchanged. See docs/original-owner-admission-2026-11-08.md.

# Generation library checkpoint — 2026-11-08

Runtime JSON generation preparation and context-bound live witness verification are now implemented in the existing singbox package. Preparation preserves numeric values, rejects ambiguous/reserved/default-changing inputs and appends one unused direct marker. Verification rejects stale/missing/mismatched/competing witnesses, request cancellation/deadline, malformed/trailing/oversized/deep responses. JSON nesting is bounded at128 and HTTP bodies at4MiB. 30 focused controls passed; complete Linux suite 8780 qualified passed leaves in 138 packages, exits [1, 1, 0] only three known raw-socket permission tests, all three separately NET_RAW pass. 46 explicit skips include the newly opt-in native generation test. Fresh vet/build, native generation library DIRECT/BLOCK/DIRECT and stale-file controls, existing actual Process, real SRS compiler/matcher and raw controls pass. No existing launch path enables these primitives yet: full renderer/process/controller/asset binding, exclusive policy ownership, startup recovery/durable root and PortableRuntime/original UI/AWG/TUN acceptance remain open. Editor capability, frontend, frozen archive and VPS unchanged. See docs/original-generation-witness-design-2026-11-08.md.

# Generation witness feasibility — 2026-11-06

After the verified SRS checkpoint, two disposable real-engine controls identified configuration-derived direct/selector tags via Clash API and rejected disk-only/unstarted generations with real loopback DIRECT/BLOCK/DIRECT. Candidate only: no product ConfirmRunning implementation, owner/startup/TUN or original UI integration claim. Product code and the 8750-test SRS verification remain unchanged. See docs/original-runtime-attestation-spike-2026-11-06.md.

# Portable artifact checkpoint — 2026-11-06

Portable source and compiled artifacts plus publication directories sync before acceptance, including cached artifacts and retries. Portable GC retains all versions while known journal recovery is unresolved under the shared writer gate. Nine focused controls pass; complete Linux coverage: 8750 qualified passed leaves in 138 packages, group exits [1, 1, 0] only for three known socket-permission tests; all three pass separately with NET_RAW. 45 explicit skips retained. Fresh Linux vet/build, actual Process/sing-box and inline compiler/matcher controls pass. No physical power-cut or product AWG/TUN/VPS acceptance claim. Startup recovery-before-adoption/GC, durable config-root creation, exclusive owner/runtime attestation and original editor integration remain open. Editor capability, frontend, frozen package and VPS unchanged. See docs/original-srs-durability-design-2026-10-31.md.

# Active local mutation checkpoint — 2026-10-31

Settings Update and root tunnel mutations/live selector/Uninstall now join the shared local writer gate before state changes. Internal tunnel apply does not re-enter admission; settings keep existing persistence-on-apply-error behavior. 15 new controls (RED11 plus4 preserved controls to GREEN15); final full Linux 8741 qualified passes across 138 packages, exits [1, 1, 0] only3 known socket-permission tests; all3 fresh NET_RAW pass, 45 explicit skips retained. Fresh vet/build and actual Process/sing-box control pass. See docs/original-active-mutation-coordination-2026-10-31.md. Ownership/handoff, runtime attestation, pre-adoption recovery/SRS pins and PortableRuntime/original UI/AWG acceptance remain open; original editor capability, VPS, frozen package and frontend unchanged.

# Full writer verification — 2026-10-31

The previously incomplete writer checkpoint full Linux suite is complete across three disjoint sequential groups covering all 138 freshly enumerated packages: 8726 passed leaf tests; group exits [1, 1, 0] only for three known raw-socket permission tests, all three separately pass NET_RAW. 45 explicit skips match the previous completed suite. Fresh actual Process/sing-box proof passes. Runtime/Go/frontend bytes and prior build binary unchanged; no fresh build claimed. See docs/original-writer-full-verification-2026-10-31.md. Original editor capability remains disabled; active tunnel/settings/uninstall, owner/startup/PortableRuntime/UI/AWG work remains open. VPS/frozen package unchanged.

# Writer lifecycle checkpoint — 2026-10-28

Actual local constructor wiring now coordinates ordinary/debounced reload, manual control and watchdog reconciliation with app.localConfigGate. Close cancels queued debounce admission before joining callbacks; transaction-owned RuntimeLease activation does not reacquire the gate. 14 new controls; affected Linux packages 928 passed leaves / three package PASS outcomes (parent timed out later during build); vet/build successful. Fresh real-engine loopback Process proof and three separate NET_RAW controls passed. FULL REPOSITORY VERIFICATION INCOMPLETE: two 600-second timeouts; previous full result belongs to parent 8840bbf. See docs/original-writer-lifecycle-2026-10-28.md. Active tunnel/settings/uninstall, ownership/startup/PortableRuntime and original UI/AWG acceptance remain open. Capability, VPS, frozen archive and frontend unchanged.

# Subscription writer checkpoint — 2026-10-27

Subscription/group service operations and scheduled refresh now join app.localConfigGate, with context-aware HTTP Update admission and existing batch/rollback semantics preserved.24 new controls, full Linux8712passes/exit1 with only the same3 socket permission failures; all3 separately pass NET_RAW; vet/build0. Ordinary reload/manual intent/settings/installer and owner/startup/original editor/UI/AWG acceptance remain open. See docs/original-subscription-coordination-2026-10-27.md. VPS/frozen archive/frontend unchanged.

# Local producer coordination checkpoint — 2026-10-27

DNS service transactions, AWG3 HTTP mutations and expert user-slot mutation/check operations now share one local writer coordinator, retaining existing rollback/store boundaries.24 targeted controls; affected Linux3976/vet/build0. Full8688/exit1 with the same3 socket permission failures, all3 separately pass CAP_NET_RAW only. Subscription batches/groups, manual settings/intent, installer, exclusive owner/startup recovery and actual original editor/runtime/UI/AWG acceptance remain open. See docs/original-producer-coordination-2026-10-27.md. VPS/frozen package/frontend unchanged.

# Local process lifecycle checkpoint — 2026-10-24

Per-Process mutation lease now composes with RuntimeLease and protects direct Operator/watchdog process calls, including adoption's conditional stop. Recovery blocks ordinary starts/reloads; leased recovery and explicit stop remain possible. Affected Linux:3339 leaf passes, vet/build0; full:8664passes/exit1 with3 socket permission failures, all3 separately pass CAP_NET_RAW only. Real Process/sing-box DIRECT/BLOCK/recovery and prior split journal controls pass. Complete producer/manual-intent/installer coordination and original editor startup/UI/AWG gates remain open. See docs/original-process-lifecycle-2026-10-24.md. No capability/VPS/frozen package change.

# Local runtime backend checkpoint — 2026-10-24

Journaled canonical+18/20 SlotApplyBackend, lifecycle lease, required native validation, durable slot metadata and unresolved-recovery reload barrier verified locally. Affected Linux:2897 package-qualified leaf passes, vet/build0. Full command:8649passed/exit1 with3 socket permission failures; all3 separately pass with CAP_NET_RAW only. Actual sing-box loopback adapter/rollback/recovery controls pass; no product AWG/TUN or startup wiring claim. See docs/original-slot-runtime-2026-10-24.md. Full port remains in progress; no capability flip/VPS update.

# Original port progress checkpoint — 2026-10-23

Task1 complete: original 87-preset catalog proven in actual embedded local app startup. Portable editor/runtime boundary and shared original HTTP routes verified. Pure original RouterConfig→gateway projection verified, including real sing-box schema and loopback rule-engine isolation control. Final affected Linux gate: 2,623 leaf passes plus vet exit0. Concrete apply/rollback/recovery, DNS/slot ownership, startup/editor capability, browser/AWG packet acceptance and whole-branch review remain open. No VPS deployment or release. See docs/original-router-projection-2026-10-23.md and docs/original-editor-foundation-2026-10-23.md.

# Local catalog source checkpoint — 2026-10-23

Original catalog HTTP route restored in portable mode. Linux RED→GREEN:7new leaf tests;1319affected leaf passes;vet exit0. Full editor/platform/apply port remains in progress. No live VPS/image update. See docs/original-catalog-port-2026-10-23.md.

# Матрица доказательств

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

PASS / FAIL / NOT TESTED / BLOCKED / STALE / N/A. PASS относится только к зафиксированной версии и acceptance criteria. После изменения соответствующего пути — повторная проверка или STALE. N/A означает неприменимость, не успех.

| Scenario | Config | Unit | Integration | Packet | VPS |
|---|---|---|---|---|---|
| M1 WG-only controlled-Sink DIRECT/BLOCK/DIRECT (accepted, historical) | PASS (pinned runner) | PASS (accepted runner checks) | PASS (bounded native controlled-Sink gate) | PASS (WG-only, not AWG/Internet) | PASS (first authorized shared VPS; exact cleanup/protected fingerprint) |
| M2 F2 primary/cleanup error aggregation and sticky Close | N/A | PASS (preserved in current targeted14/full ingress67/focused cmd25; source pins in parent checkpoint) | PASS (OS/control fixtures and protected marker test only; not real AWG) | NOT TESTED | NOT TESTED |
| M2 F1 selected IPC foreign-resource preservation / normal Close | N/A | PASS (historical control fixture only) | PASS (REAL original RED2/10, corrected IPC10/10, descriptor100/100; pinned Linux binary built and independent bounded review accepted; general lifecycle concerns OPEN) | NOT TESTED | NOT TESTED |
| M2 local standalone AWG binary / ChannelTUN client / real kernel TUN | PASS (11 live fields) | PASS (fixture validators) | PASS (corrected candidate exact ICMP echo and independent local review; original FAIL superseded by retained receipts) | PASS (local controlled payload only; not product policy/Internet) | NOT TESTED |
| Docker installation / pinned image checksum only | N/A | N/A | N/A | N/A | PASS (first authorized shared VPS; protected baseline retained, checksum runtime removed; not Gateway) |
| New no-product bridge / exact temporary-rule cleanup | N/A | PASS (57 local controller tests; final C2 fixes independently ACCEPT_LIMITED_LOCAL_FIX_SLICE, runtime incomplete) | NOT TESTED (actual runtime) | NOT TESTED | NOT TESTED |
| Neutral fixture / BusyBox helper offline artifact | PASS (independent actual graph/config/ELF offline review accepted) | PASS (17 neutral + 34 helper tests at accepted source hashes) | NOT TESTED (daemon/applications) | N/A | NOT TESTED |
| Transport whitespace | NOT TESTED (live engine) | PASS (new Linux copy) | N/A | N/A | N/A |
| Gateway sniff/order | PASS (loopback engine check) | PASS (QoS/no-Router merged order) | PASS (HTTP Host literal-IP SOCKS adapter RED/GREEN) | NOT TESTED (AWG/TUN) | NOT TESTED |
| Slot filename migration | N/A | PASS (RED/GREEN, ten layouts) | PASS (validator/runtime merge, reviewed seam) | N/A | NOT TESTED |
| Migration error startup gate | N/A | PASS (RED/GREEN, portable/runtime initialization) | PASS (related composition tests) | N/A | NOT TESTED |
| Migration state-directory links | N/A | PASS (RED/GREEN, pending/disabled; external bytes preserved) | N/A | N/A | N/A |
| Pre-operator production directory preflight | N/A | PASS (RED/GREEN, local/runtime root/disabled/pending) | PASS (migratable external JSON and file set preserved) | N/A | NOT TESTED |
| Uninstall initialization failure / error status | N/A | PASS (mock-only stages/order/one-shot/preflight) | NOT TESTED (real uninstall) | N/A | NOT TESTED |
| Migration partial retry / nonregular / compatibility | N/A | PASS (38 independently repeated extended cases) | PASS (actual conflict MergeDir exclusion) | N/A | N/A |
| Migration surviving engine convergence | N/A | PASS (invalidation/retry focused Linux; review accepted) | PASS (pinned actual CLI RED/GREEN, HTTP-proxy/mixed-inbound loopback; review accepted) | N/A | NOT TESTED |
| Fixture inside/outside alias control | N/A | N/A | PASS (историческая fixture only) | NOT TESTED (новая topology) | N/A |
| DIRECT single client | NOT TESTED | NOT TESTED | NOT TESTED | BLOCKED (fixture confound) | NOT TESTED |
| BLOCK single client | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED |
| VPN | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED |
| WARP | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED |
| Multi-client / isolation / revoke | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED |
| Safe apply / persistence / restart | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED |
| Backup/restore | NOT TESTED | NOT TESTED | NOT TESTED | N/A | NOT TESTED |
| Browser UI | N/A | NOT TESTED | NOT TESTED | N/A | NOT TESTED |

NOT TESTED здесь означает отсутствие принятого актуального evidence в новой рабочей копии, не отсутствие реализации и не утверждение, что тест раньше никогда не проходил. Исторические results остаются локально в прежнем журнале/review-packs и не публикуются массово.

## Evidence index

Current local packet blocker: HOLD_M2_LOCAL_AWG_DATA_PATH; checkpoint scripts/m2-local-20261008/codex-local-packet-b15343f9-9010-4306-acaa-d21a3a652a2a/RESULT.json. Accepted standalone binary unchanged; real TUN/network-none local probe native1/OOMfalse. All11 AWG fields read back, AWG H1/H2 exchange and client handshake mark observed, but server confirmation0 and no exact ICMP echo/no configured H4 transport; full packet gate FAIL. WG-mode negative and positive2 NOT RUN. Source-backed stale-S4-before-blocking-read hypothesis unproven. Fixture validators/vet/build native0; canonical CAP_NET_ADMIN metadata correction only. Server native join/socket/TUN cleanup and six exact container absences verified; final original8 IDs retained, owned build copies removed. Prior IPC/build ACCEPT retained, M1 unchanged; no manager/UI/VPS/Git/backend changes. Early-blocker report.

Current verified IPC/build slice: ACCEPT_M2_LOCAL_IPC_BUILD_SLICE; checkpoint scripts/m2-local-20261008/codex-ipc-build-2e7e1740-806f-4843-b7c1-fb7258a958e9/RESULT.json. Confirmed inotify-before-cancel shutdown race fixed by cancellation-first ordering; no ignored errors or weakened assertion. Fresh original behavioral RED2/10, corrected full IPC GREEN10/10 (noop child excluded), descriptor regression100/100, formatting/vet native0. Separate Linux/amd64 corrected binary BUILT, native0/OOMfalse, SHA256 333d0f224ad8bb2645add59e65061ab5b31c67c59e01344ec24d430037447a31; exact source/dependency/image pins and independent read-only review accepted. All10 disposable containers absent by receipts; final original8 IDs retained, owned build copies/caches removed. M2 handshake/traffic/product OPEN; preserved repeated-Close/partial-startup/background-join concerns remain open. M1 retained; no manager/UI/TUN/VPS/Git or service restart.

Historical superseded checkpoints follow.
- Current Codex continuation after user Docker start: HOLD_M2_CODEX_DOCKER_PIPE_ACCESS_DENIED; checkpoint scripts/m2-local-20261008/codex-ipc-preflight-f56cb709-8241-4cf1-b95f-7809a6e9c257/ACCESS_BLOCKER.json. Native read-only inventory exit1/permission denied; narrow pipe permission API unsupported. No container create, IPC tests, build or permission bypass; source hashes retained. Prior engine-unavailable stop is historical.

Current Codex continuation: scripts/m2-local-20261008/codex-ipc-preflight-f56cb709-8241-4cf1-b95f-7809a6e9c257/RESULT.json. HOLD_M2_CODEX_DOCKER_ENGINE_UNAVAILABLE before test-container creation; native read-only Docker inventory failed. No new test/build evidence or packet claim. Historical real IPC FAIL remains unresolved; durable source hashes unchanged.

Current work: scripts/m2-local-20261008/f1-build-034039a7-9dc5-485f-8c07-e355fa1b9c36/RESULT.json. Exact x/sys v0.36.0 source zip/mod h1 verified after explicit authorization; real selected IPC original behavioral RED now observed, but candidate GREEN failed:9 meaningful cases PASS/1 descriptor-handoff Close FAIL, native exit1/OOMfalse; child helper excluded. No patched binary, full ./ipc suite/vet/build/re-review or AWG packet/VPS proof. Actual source delta patch exported, fresh exact absence of two disposable IDs/original8 inventory retained. Close-error categories prepared only in scratch; diagnostic request timed out without execution after retained pre-create source-pin refusal. Early-blocker STOP, not missing-source setup HOLD. M1 unchanged/accepted WG-only, full product scope retained. Prior checkpoint M2_F1_PARENT_VERIFIED_DEPENDENCY_HOLD_20261007.json is historical and superseded only for current state.

70-vps-no-product-control/COORDINATOR_CYCLE2_LIMITED_LOCAL_ACCEPTANCE_20261005.json: final C2.a/C2.b limited-local acceptance, full independent report read and exact report/source hashes verified. Independent108 tests PASS plus14 inert scenarios (not additional unittest methods). Earlier pending checkpoints superseded. Runtime/load/full baseline/capability/bridge/recover/verify-clean HOLD, Gateway OPEN; no remote commissioning acceptance.

70-vps-no-product-control/COORDINATOR_CYCLE2_VERIFIED_FINAL_REVIEW_PENDING_20261005.json: actual coordinator108 tests PASS, exit0 (57 controller + 34 helper + 17 neutral); six AST/compile and exact source/15 indexed log hashes verified, accepted helper frozen. Final independent review deleg_42d32d34 pending for C2 sticky UNKNOWN/CLEAN/reuse root fixes; previous L1-L5/C1 acceptance remains limited-local. Runtime/remote/actual packet/cleanup acceptance not claimed; retained ledger.next and persistence impossibility explicitly reported, no universal power-loss guarantee.

70-vps-no-product-control/CYCLE1_PARTIAL_ACCEPT_C2_FINAL_FIX_PENDING_20261005.json: cycle1 independent re-review report read fully and rehashed; L1-L5/C1 accepted only within local scope. C2 still has actual inert counterexamples after work_failed/final CLEAN durable-write failure: uncertainty is not sticky, CLEAN/reuse can return. Previous 51/102 tests passed but lacked those seams. Final cycle2 C2-only behavioral fix deleg_e7aaaf7c pending; no runtime/remote acceptance or helper changes.

70-vps-no-product-control/COORDINATOR_CYCLE1_VERIFIED_REREVIEW_PENDING_20261005.json: actual coordinator cycle1 regression 102 tests, exit0 (51 controller + 34 helper + 17 neutral); six source/test AST/compile checks PASS, exact final controller/test hashes and all27 indexed logs match. Behavioral RED/GREEN content and seven fixes await independent re-review deleg_6f87b09c; no final local acceptance yet. Conservative C1 does not authorize unknown real Docker/nft rule templates or unverified host mappings. Accepted helper archive actual bytes unchanged; all remote/runtime/packet/cleanup gates remain HOLD/NOT TESTED.

70-vps-no-product-control/HELPER_ACCEPTANCE_AND_RUNNER_FIX_CYCLE1_20261005.json: independent BusyBox helper ACCEPT_OFFLINE_HELPER_SLICE and controller HOLD reports read fully; exact report hashes and archive/report bytes reverified. Accepted offline graph/config/layers/ELF/provenance does not prove runtime identity, applets/logging/reaping/traffic or cleanup. Controller L1-L5 and C1/C2 are concrete independent local counterexamples despite prior29/80 unit PASS. Fix cycle1 deleg_be75888e pending; old controller tests are not fixed-version evidence. No real CLI/Docker/SSH/VPS actions or POL-001/POL-002 reopening.

70-vps-no-product-control/COORDINATOR_POST_SOFT_PAUSE_RESUME_20261005.json: actual coordinator rerun 80 tests, exit0 (29 controller + 34 helper + 17 neutral); final previously paused test edit included. Six source/test AST checks PASS; three actual archive hashes/sizes and accepted transformer/test hashes match pins. Limited static scan found no matching secret/shell=True/eval/exec/pickle locations, not a comprehensive security proof. Independent reviews deleg_2ac952fb pending for actual helper archive/ELF and current controller source. Executed load/full baseline admission and capability/bridge/recovery/verify-clean CLI remain HOLD. No remote/runtime/packet/cleanup proof and no changes to accepted POL-001/POL-002 scope. Older evidence below remains historical.

69-vps-docker-install: INSTALLATION_ACCEPTANCE_20261004_210741.json and exact POSTINSTALL_R3 evidence hash verified after resume. Service identities/config hashes/root responses equal preinstall; network-none binary checks exit0, exact runtime removed and final inventory empty. Persistent authorized Docker baseline remains, not fixture residue. No Gateway/readiness/AWG/bridge acceptance. 70-vps-no-product-control: proposed bounded contract under independent read-only review deleg_34cb9cdc; plan is NOT runtime evidence. Local Docker Desktop read-only inspect failed (engine unavailable), no restart performed. Fresh host/tool gates required before subsequent execution.

66-post-reboot-build-preflight: FINAL_FIX_SOURCE_SHA256.json matches six current source files; retained baseline/fixed/helper binaries match runtime result hashes. Same-helper old-manager RED/fixed GREEN: /readyz200, distinct managers, surviving engine identity unchanged, direct target-listener control, desired merge excludes legacy, proxied target sink delta 0 -> 1. This helper uses HTTP proxy/mixed inbound, not SOCKS handshake. Four affected package suites PASS; final-vet.log preserves exit0 without command/targets. Coordinator focused regression repeat exit0; COORDINATOR_RESUME_READBACK_V2.json confirms protected baseline/owned-runtime cleanup at its observation time. INDEPENDENT_INVALIDATION_FIX_REVIEW.md ACCEPT read by coordinator; POL-001 reviewed seam accepted, no AWG/TUN/VPS claim. Combined validation-failure retry, atomic-writer fault injection and immutable full-build provenance remain NOT TESTED; tmpfs/process-crash atomicity is not a power-loss transaction guarantee.

Новые результаты записывать со source hash/commit, runner/image ID, командой, exit code и точным scope. Sensitive/raw logs держать локально; в public repository публиковать только проверенный redacted summary.

Текущий локальный пакет 52-next-development-bootstrap: migration RED exit 1; migration GREEN exit 0; orchestrator/configmerge/policy/forwarding/ingress package tests exit 0. SHA256 baseline copy manifest сохранён локально. Публичный baseline пока содержит только documentation/license files, не эти исходники и не raw logs.

Пакет 53-migration-review-fixes: symlink RED/GREEN; production-bootstrap RED/GREEN; cmd/awg-manager плюс пять связанных packages PASS; go vet PASS; final focused tests и gofmt clean. SOURCE_AND_RUNNER.json фиксирует hashes изменённых файлов и pinned image. Истёкший disposable runner был поднят снова по exact owned ID; protected containers readback — running/healthy. Не считать harness failure «container not running» production test failure или GREEN.

Пакет 55-production-preflight-cleanup (preflight-*): actual production symlink regression RED/GREEN до/после переноса проверки перед NewOperator; singbox/orchestrator suites и vet PASS, formatting clean. Cleanup-* результаты и consolidated cmd suite требуют отдельного принятия после worker completion; этот preflight slice не закрывает cleanup или packet gate.

55 cleanup/consolidation принято координатором: actual cmd/awg-manager + related suites повторены, 13 test-bearing packages PASS; internal/cleanup has no standalone tests. Vet всех 14 targets PASS. Cleanup tests не выполняют реальное удаление ресурсов. Migration extended cases, real engine convergence и packet/VPS proof не закрыты этим результатом.

61-sniff-coordinator-verification: actual independent engine RED/GREEN, only sniff rule differs; prior DNS отсутствует, allowed-before/after response hashes match, blocked listener count 1 -> 0, actual domain sniff/reject trace verified. Runtime/image/source hashes verified, exact owned runtime removed. This is HTTP rule-engine SOCKS adapter evidence, not AWG/TUN/VPS or TLS proof.

57-migration-extended-acceptance/resume-20261002T213745Z: 38 focused leaf cases, package suites and vet PASS; coordinator source/log hashes verified and focused 38 cases independently repeated. No behavioral RED or production edits because existing behavior passed. Filesystem/actual MergeDir only; surviving-engine convergence stays OPEN.


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

## 2026-10-23 — original editor apply/recovery foundation (partial port)

Verified local foundation: original editor drafts share an explicit apply coordinator; projected staging delegates to portable runtime. Portable inline SRS artifacts are content-addressed, so editing a draft cannot replace the applied binary; GC retains applied/draft paths. Added private checksummed prepared/accepted journal, cancellation-safe rollback, recovery, explicit activation AND confirmation contract, and strict directory-sync error propagation.

Linux affected suites: 2,645 leaf tests passed; vet exit0. Separate real sing-box controls passed: DIRECT baseline, invalid-candidate refusal, activation-failure rollback, prepared recovery, accepted recovery BLOCK, DIRECT restoration; actual SRS compile/match preserved old and new content. These use loopback mixed ingress and simulated interruption/fresh backend and engine instances. They are NOT product orchestrator/AWG/TUN, full-manager power-loss, per-client DNS isolation or full-suite acceptance. Historical two full-suite failures remain unclosed.

Task1 catalog complete (87 original presets); Tasks2/3 remain partial. Concrete runtime/orchestrator adapter, single rule ownership, DNS/other-slot priority, all-writer coordination, recovery-before-GC, startup/status/browser and exact-image real AWG packet gates remain OPEN. Capability stays disabled. No live VPS, frozen ZIP or original frontend changes. Installer deferred; full product scope and attribution preserved. Whole-branch fresh review remains pending until the full plan is implemented.

Evidence: `apply-durability-final-05a02487-76ab-4ab6-8685-6c5f41981152.json`, `confirmed-apply-native-603c5148-a75c-4144-936d-34d0afba50ce.json`, `confirmed-inline-native-e4998a74-b036-4e89-834a-bd80ff41f27e.json`. RED/GREEN history retained in local execution ledger.

## 2026-10-23 — split runtime projection and canonical editor (partial port)

Implemented18 ingress/sniff/DNS prefix plus20 scoped original rules/resources/fallback, preserving19 DNS-route precedence. New read-only CheckMergedBatch validates joint candidate slots and untouched active siblings, without pruning, commit/toggle/reload. Canonical editor applied/draft documents now live in a separate private store; original CRUD/discard cannot overwrite derived engine20. Portable external rename delegates to runtime transaction before legacy side effects.

Affected Linux suites: 2862 package-qualified leaf passes; vet/build exit0. Full repository command exit1: 8615 package-qualified leaf passes; failed: TestByInterface_LoopbackRoot, TestOpenPacketSocketsFailsAtomically, TestReadPacketStatisticsOnLoopbackSocket. All three failed socket tests passed separately with only CAP_NET_RAW added in a network-none owned container. This closes the identified permission mismatch for those tests; the original full command remains exit1, not a single full-suite PASS. Historical failures and raw local evidence retained.

Real sing-box control: split TUN schema accepted; joint CheckMergedBatch accepts projected slots and refuses invalid outbound type without changing live20; five loopback mixed-input order phases preserve19-before20, first-match card ordering and unrelated-input access. This is NOT AWG/TUN product traffic, actual DNS resolution/isolation, whole-manager power-loss, image/browser/VPS acceptance. Native controls are separate from normally skipped env-gated tests.

Task1 catalog complete; Tasks2/3 still partial. Concrete transactional runtime adapter, all-writer/reload coordination, owner handoff, running-state confirmation, recovery-before-GC, outbound catalog wiring and startup/browser/exact-image AWG gates remain OPEN. Capability stays disabled. No frontend, live VPS, frozen archive or installer changes. Whole-branch fresh review remains pending.

Evidence: `owner-final-gate-570ecd1a-529c-4d12-8f62-e41225bbd986.json`, `owner-full-suite-f98f184d-11a0-4bce-8237-43cd95e809a9.json`, `confirmed-owner-native-75ee29c2-458e-4bfa-a072-d9f963f56c43.json`, `owner-raw-controls-db60b039-e7b2-47ac-aabf-419af0412eb8.json`. Count audit uses (package,test) identity; previous runner's name-only totals may undercount duplicate names across packages.

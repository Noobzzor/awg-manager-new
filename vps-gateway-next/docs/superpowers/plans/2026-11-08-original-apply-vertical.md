# Original Editor Apply Vertical Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans; use the existing parent Task3 workspace/ledger. User explicitly said GO after the Hermes discussion.

**Goal:** A real original-editor HTTP Apply through ownership, safe startup/recovery, owned runtime confirmation and local exact-image packet checks.

**Architecture:** Reuse original ServiceImpl/HTTP, PortableRuntime boundary, SlotApplyBackend/journal and existing writer/process leases. Start on an explicitly selected empty local state. Refuse legacy mixing; migration remains mandatory before the working VPS update.

**Spec:** PRODUCT_SPEC.md, existing original-editor design and outputs/AWG-NEXT-PLAN-AFTER-HERMES-2026-11-08.md (local discussion record). Hermes reviewed the description, not source code.

## Task1: production legacy admission boundary

- [x] RED tests for actual gateway HTTP create/transition/profile/group/subscription/WARP entry and local buildSingboxCore: reject original/unknown ownership before store writes or runtime reconcile/adoption; reads and absent-owner legacy behavior preserved.
- [x] Implement read-only owner inspection at <dataDir>/original-router/owner.json. Absent state root means current gateway owner; an unclaimed/nonprivate/linked/malformed state root/file is an error. Version1 with gateway/original modes only; a gateway marker plus original apply-journal presence is an error. Inspection never creates/claims state.
- [x] Add context-bound outer admission in gatewayClientHTTP.ServeHTTP for mutations, sharing app.localConfigGate through actual local wiring; check owner after taking the gate and before all delegated stateful handlers. Preserve read paths. Saver callbacks schedule reload; do not re-enter the gate.
- [x] Local legacy buildSingboxCore must check ownership before NewOperator migrations/reconcile/adoption. It must refuse original state until Task2's factory can recover it safely.
- [x] GREEN, affected/full Linux, vet/build/native/raw controls and exact local commit. This task closes a boundary, not the whole vertical goal.

## Task2: original owner/startup and live apply

- [ ] Define durable owner claim/handoff plus cold construction and recovery before normal adoption/GC/producers. Inspect existing editor journal/documents without creating them before ownership decisions. On fresh opt-in state initialize durably; unknown/legacy state refuses original start, no automatic conversion.
- [ ] Bind actual rendered generation and immutable referenced assets to the owned Process/controller and real VerifyOwnership/ConfirmRunning callbacks under existing leases; respect manual-stop and avoid lock re-entry. No success callbacks returning nil unconditionally.
- [ ] Implement concrete PortableRuntime.ApplyStaging and startup factory, retaining original CRUD/catalog/routes. Exercise real HTTP Apply, successful activation, refusal/rollback, manager restart and prepared/accepted journal recovery in local exact image.

## Task3: original UI and acceptance

- [ ] Original cards/87 presets/drag order browser flow in exact image, including persistence/restart.
- [ ] Actual local AWG/TUN/DNS DIRECT/BLOCK/DIRECT and fail-closed checks.
- [ ] Separate existing-data migration and crash-state compatibility gate before a user-authorized working VPS update; clean-state success cannot close this gate.

No VPS/frozen archive/frontend rewrite or scope removal. New explicit-GO stage has its own recorded one-hour budget, old completed generation stage retained. Whole-parent fresh review remains required before shipping.

Task1 verified and committed; Task2/Task3 and whole-parent review remain open.

## Direction review and executable goal — 2026-11-09

User explicitly requested another Hermes direction check. Two text-only consultations support the architecture but warn against further standalone prerequisites; their source-access limit and exact replies are saved outside this module. Source tracing confirms main still chooses gateway, and NewPortableService/NewSlotApplyBackend/NewApplier/PrepareRuntimeGeneration have no production original-mode caller.

The next work is anchored to `TestNativeOriginalApplyThroughApplication` in `cmd/awg-manager/original_apply_acceptance_test.go`, an opt-in goal using the actual compiled main and original APIs with a real AWG/TUN namespace. Initial RED: main remains gateway and GET original presets returns404. `AWG_ROUTER_POLICY_MODE=original` is the proposed selector exercised by this goal, NOT an implemented or released setting. Do not document it as an available user feature.

The test then requires original87 catalog, route-final staging/Apply/status, finalized drafts and independently observed owned engine/controller socket, private immutable full config, real18TUN ingress/scoped fallback and fresh live witness across DIRECT/BLOCK/DIRECT. Its later binding checks have NOT run while it stops at the first missing wiring. A passing HTTP goal still cannot close packet, rollback, restart/recovery or browser gates.

First supported experimental candidates may be asset-free within fresh normalized private local roots; external assets/legacy/orphan/shared roots must refuse safely, not receive fake success. All agreed broader features remain open. All process/log/cache/config roots must be owned: the current legacy Process log default is outside DataDir, so the original composition must explicitly contain it. Capture/recheck the selected draft in the writer transaction and do not discard a newer concurrent draft after Apply.

Do not accept another independent helper as completion of this goal. The next production change must connect a concrete original startup/runtime composition to main and the existing HTTP handlers, with actual ownership/generation checks; do not expose the editor or modify the working VPS before the acceptance gates.

## Executed original composition checkpoint — 2026-11-09

The missing main-to-Apply connection is implemented for a fresh, private local original owner. Actual AWG TCP DIRECT/BLOCK/DIRECT, clean restart and both seeded journal phases passed. The API exposes 87 catalog entries overall and 77 router-applicable entries. The initial goal had conflated these counts. Unsupported preset mutations refuse before dispatch. The local prototype still needs full browser, asset, DNS, abnormal process interruption and migration verification before promotion.

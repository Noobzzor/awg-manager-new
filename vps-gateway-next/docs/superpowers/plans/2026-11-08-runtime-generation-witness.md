# Runtime Generation Witness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans, using the existing parent Task3 workspace and ledger.

**Goal:** Move deterministic runtime JSON generation preparation and a bounded live engine witness from the disposable prototype into the existing singbox package.

**Architecture:** Pure JSON preparation appends one direct witness; the existing ClashClient supplies a separate context-bound verification method. No automatic runtime activation or owner callback is fabricated.

**Tech Stack:** Existing Go/vendor, pinned Linux Go and real sing-box Docker images.

**Spec:** docs/original-generation-witness-design-2026-11-08.md

## Single bounded Task3 extension

- [x] Add runtime_generation_test.go and clash_generation_test.go; compile-only behavioral placeholders delegate to current generic parsing/GetProxies behavior.
- [x] Observe behavioral RED: deterministic preparation rejects absent safeguards; generic proxy existence wrongly accepts mismatched or competing witnesses and ignores cancelled context.
- [x] Implement runtime_generation.go (strict deterministic JSON, namespace/default protection) and clash_generation.go (bounded context-bound exact direct witness).
- [x] GREEN, then add runtime_generation_native_test.go using the actual library functions and real engine DIRECT/BLOCK/DIRECT; no transport-error-as-BLOCK acceptance.
- [x] Format, affected tests, full split Linux/vet/build and native generation/Process/SRS/raw controls on exact source. Record skips and known raw permission controls explicitly.
- [x] Secret review, local commit, exact Git readback, report/resume/Obsidian. Leave production ownership/process binding/startup/UI/TUN requirements open.

## Review focus

Witness cannot become an implicit default or target a supplied reserved reference. Digest includes the entire supplied JSON and preserves numeric values. JSON duplicate/trailing inputs are rejected. Cancellation and body limits must govern the actual HTTP request. Matching an unrelated healthy process/API or mutable referenced assets is not proven by this API; do not enable the editor or claim production attestation.

Same fresh 8 November one-hour stage, including waits. No clock reset, VPS, frozen archive or frontend change. Parent whole-branch review remains due after the complete parent plan.

Child primitive implementation verified and saved; parent Task3 production binding/ownership/startup and whole-parent review remain due.

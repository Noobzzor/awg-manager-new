# Portable SRS Durability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. This is a bounded extension of parent Task3 and uses its existing workspace/ledger.

**Goal:** Refuse acceptance of portable artifacts after sync failure and retain artifacts while recovery is unresolved.

**Architecture:** Reuse immutable materialization, the common writer gate and existing recovery barrier. Avoid a second pin registry; skip all portable GC while recovery is required.

**Tech Stack:** Existing Go module, pinned Linux Go/vendor, real pinned sing-box compiler, existing filesystem helpers.

**Spec:** docs/original-srs-durability-design-2026-10-31.md

## Global Constraints

- Same 31 October bounded stage clock; no silent extension.
- No VPS, capability, frozen package, frontend or legacy behavior change.
- Accepted behavioral RED precedes runtime implementation; compile-only injection seam is not durability implementation.
- No power-cut, startup-recovery-complete or AWG traffic claim.

## Review Focus

- Cache hit after failed directory sync must not bypass persistence verification.
- Error after publication may leave unreferenced files, but must not return an accepted reference.
- GC must retain all artifacts while recovery is required; normal GC still removes unreferenced files afterwards.
- Sync the newly created publication hierarchy within the existing config root; durable root creation remains a startup obligation.
- Never acquire the portable writer gate recursively from an already-owned apply callback.

## Task: materialization durability and recovery-aware GC

- [x] Add focused tests in router/ruleset_durability_test.go and router/portable_gc_recovery_test.go; introduce only an unused per-materializer sync injection field for compilation.
- [x] Observe file, compiled output, directory and cache-hit failures; legacy control remains passing; recovery GC witness must fail on existing code.
- [x] Implement portable file/publication syncing, read-only orchestrator recovery accessor, and portable GC barrier check under its existing gate.
- [x] Run GREEN, format, full split Linux suite, vet/build and native real compiler/matcher plus process/raw controls on exact source.
- [x] Save sanitized evidence/status, verify Git blobs and clean tree, update resume/Obsidian. Keep startup/owner/runtime attestation pending.

Verification resumed on 6 November with a fresh bounded one-hour stage after the user-requested pause; the expired 31 October admission was not reused. This child extension is verified; parent Task3/startup/ownership and whole-parent review remain open.

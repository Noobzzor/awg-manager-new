# AGENTS — AWG Manager VPS Gateway next

## Product and source of truth
Read PRODUCT_SPEC.md, PROJECT_STATUS.md, TASKS.md and TEST_MATRIX.md before continuing. Preserve the whole agreed product: server AWG/WireGuard, web UI, clients/lifecycle, VPN/WARP/DIRECT/BLOCK, domain/IP/port/protocol policy, fail-closed, persistence/recovery and backup/restore. Product scope changes need user approval.

## Working boundary
Work only in this `vps-gateway-next` module. Do not edit the legacy checkout/snapshot source outside this directory. The public destination is `Noobzzor/awg-manager-new/vps-gateway-next`. Publication permission does not extend to old dirty Git history, real keys, customer configs or raw evidence logs. Keep original LICENSE and attribution.

Do not deploy to VPS, modify VPN/host firewall or touch protected containers/volumes. Use exact owned disposable Docker IDs for cleanup. Do not restart Docker/services to clear a blocker. No force-push/history rewrite. Commits/push may contain only the agreed new-folder changes after secret review and exact readback.

## Verification
Strict RED/GREEN for behavior fixes. Run Go tests in Linux with the pinned Go image/vendor dependencies; Windows compilation does not prove Linux compatibility. Never claim packet success from generated config or unit tests. Separate fixture/network control, policy/rule-engine control and product E2E. No production SNAT/bind workaround to compensate for Docker --internal fixture filtering.

Report milestones briefly. Update PROJECT_STATUS.md and task evidence after verified slices. Public evidence must be sanitized; historical/raw local evidence stays outside this folder. A paused/interrupted worker does not imply a completed fix.

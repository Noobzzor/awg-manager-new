# Portable original editor foundation — 2026-10-23

NewPortableService reuses the original rule, preset, DNS, outbound, order and draft operations. Its dependency contract excludes NDMS, iptables, OpkgTun and Keenetic policy providers. Status, settings, reconcile, mode changes and activation require an explicit PortableRuntime. Missing dependencies fail construction; runtime errors do not select the legacy router path.

The original router endpoint registration is now shared between deployment modes, using the existing SetSingboxRouterHandler and authentication guard. The original endpoint block is unchanged after newline normalization. The portable app does not yet supply a runtime/handler or advertise router capability.

Linux behavioral RED/GREEN proved settings/runtime isolation and the missing local HTTP seam. Final targeted gate:2561leaf tests pass across router/server/api/main, followed by go vet exit0. Original service preset projection, DNS edits/order, rule reorder and draft discard were exercised. Exact owned test containers cleaned; protected inventory retained.

This is partial Task2. Real gateway rule ownership, projected staging validation, atomic apply/recovery, app wiring, frontend workflow and exact-image packet gates remain open. A stub or saved draft must not be presented as successful routing. Working VPS/phone/frozen package remain unchanged.

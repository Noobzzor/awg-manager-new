# Original router to gateway projection — 2026-10-23

A pure routerpolicy.Render adapter now reuses the original RouterConfig: rule order, logical predicates/inversion, narrowing conditions, actions, DNS, outbounds, rule sets, HTTP clients and experimental settings. It does not write or activate configuration.

The proven gateway ingress is retained as raw JSON: the original editor Inbound type does not contain dns_mode/include_interface and must not be used to round-trip those constraints. Original router platform inbounds are refused. Mandatory gateway sniff/DNS rules precede original user rules; each original matcher is enclosed in an AND with gateway inbound, preserving nested OR/invert and existing inbound conditions. The editor final becomes a scoped terminal route/reject, with no global route.final.

RED reproduced missing user rules/resources and acceptance of foreign inbounds. An initial DNS comparison used Go slice identity (nil versus omitted empty); the corrected comparison checks the serialized DNS contract without weakening resource/action assertions. Final Linux gate: 2,623 affected leaf tests pass, followed by go vet exit0.

The exact emitted TUN configuration passed sing-box check using accepted image2178524bc069. A loopback control replaces only the TUN inbound with mixed listeners, retaining generated rules. Gateway HTTP is rejected502; a separate unrelated inbound reaches the fixture200. The earlier order control proved200→502→200 and a negative verifier rejected intentionally wrong order. These are schema and routing-engine controls, not AWG/TUN product packet acceptance or DNS isolation proof.

Not wired into application startup or VPS. Activation still needs one authoritative rule owner, coordination with existing DNS-route/gateway-profile writers, projected staging validation, durable transaction/rollback/recovery and honest runtime status. Engine-wide DNS/global fields retain original semantics and need cross-slot tests; this adapter does not promise per-client DNS isolation. Fresh whole-branch review remains ahead of release.

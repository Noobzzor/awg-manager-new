# AWG Manager VPS Gateway — Stage 0 contract

Статус: утверждённый технический baseline для реализации server ingress.
Дата: 2026-09-22

## 1. Runtime boundary

- Deployment: Docker Compose, Linux VPS.
- Supported architecture: `amd64`, `arm64`.
- MVP dataplane: IPv4-only. IPv6 fields may be stored for future use, but IPv6 forwarding is disabled until a separate contract and tests exist.
- Router/Keenetic/NDMS is not a dependency of the VPS gateway.
- `awg-manager` and `sing-box` run as UID/GID `10001`.
- Runtime root filesystem remains read-only; mutable state is only `/data`.

## 2. Network contract

- Server AWG/WireGuard ingress: UDP `51820` on the VPS public interface.
- Client address pool: `10.66.0.0/24`.
- Server tunnel address: `10.66.0.1/24`.
- Maximum active peers in MVP: `250`.
- Client source addresses are allocated as `/32` addresses from the pool and are never reused while a peer is active.
- Admin HTTP remains bound to localhost inside the container contract (`0.0.0.0:2222` container port, localhost-only host publication in the supplied compose file). Public admin exposure requires an explicitly configured reverse proxy/TLS layer; it is not opened by the gateway dataplane.
- The optional public client-profile route is only `/s/{opaque-token}` behind HTTPS and requires `AWG_GATEWAY_SUBSCRIPTION_BASE_URL`; `/api/gateway/` remains authenticated. Reverse proxies must suppress access-log paths for `/s/` and rate-limit that location.
- The local mixed proxy remains a localhost/admin-plane facility, not a client ingress.

## 3. Packet flow

```text
remote AWG peer
  -> UDP :51820
  -> server AWG/WireGuard interface
  -> source-address identity (10.66.0.x/32)
  -> sing-box route/DNS policy
      -> DIRECT: VPS WAN bind + nftables MASQUERADE
      -> VPN: managed AWG3 upstream outbound
      -> WARP: managed WARP outbound
      -> BLOCK: explicit reject/drop, counters only
  -> return traffic through established/related nftables state
```

Admin traffic is a separate plane:

```text
localhost/reverse-proxy -> HTTP :2222 -> admin API/UI
remote AWG client        -X-> admin HTTP/API
```

## 4. Policy semantics

- Rules use lower numeric `priority` first; equal priority preserves declaration order.
- First matching enabled rule wins.
- `defaultAction` is mandatory and applies when no rule matches.
- `BLOCK` is terminal: it emits reject/drop and has no outbound and no fallback.
- VPN/WARP failure policy is fail-closed for traffic explicitly assigned to that action. It must not silently become DIRECT.
- DIRECT is an explicit action, not an implicit recovery path.
- DNS queries from clients follow the same policy identity. A DNS server/detour that bypasses the selected action is invalid.
- Client and group source selectors resolve to concrete client `/32` addresses or an explicit pool; unresolved references block apply.

## 5. Firewall ownership

- nftables is the only gateway firewall/NAT backend.
- One reconciler owns the gateway chains and uses an ownership marker/comment.
- Reconcile is idempotent: repeated apply does not duplicate rules.
- Managed chains must contain:
  - established/related accept;
  - client-pool anti-spoofing;
  - forwarding from AWG ingress to approved egress;
  - MASQUERADE for DIRECT/VPN/WARP egress as required;
  - explicit admin-plane isolation;
  - explicit reject/drop path for BLOCK.
- Rules outside the ownership marker are not deleted by reconcile.

## 6. Persistence and backup

- Versioned stores will be added for clients, peers, policies, subscriptions and gateway state.
- Atomic write plus interrupted-write recovery is mandatory.
- Default backup is redacted: no private keys, preshared keys, WARP credentials or opaque tokens.
- Encrypted backup is an explicit user-selected mode and requires an external encryption key; credentials are never printed to logs or list APIs.
- Restore is staged and rollback-safe inside mounted `/data`; the mount point and read-only root are not renamed.

## 7. Release gates for this contract

Before calling the Stage 0 contract implemented:

- server ingress accepts a synthetic peer handshake;
- revoked/disabled peer no longer passes traffic;
- DIRECT returns the VPS public IP;
- admin HTTP is unreachable through the client ingress;
- nftables reconcile is idempotent after restart;
- persistence and redacted/encrypted backup behavior are read back independently.

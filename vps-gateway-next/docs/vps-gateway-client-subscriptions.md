# Gateway client profile subscriptions

Дата: 2026-09-23

Client subscriptions are capability URLs for refreshing one AWG/WireGuard client profile. They are separate from upstream sing-box subscriptions.

## Configuration

Set `AWG_GATEWAY_SUBSCRIPTION_BASE_URL` to the public HTTPS origin served by the reverse proxy, for example `https://vpn.example.net`. The value must be an HTTPS origin only: no credentials, path, query, or fragment. It is distinct from `AWG_GATEWAY_PUBLIC_ENDPOINT`, which is the WireGuard endpoint in `host:port` form.

If the subscription base URL is empty, subscription creation fails closed and no token is issued. Profile links are limited to 1–365 days. A client must be active and its live peer/profile must be available before a subscription is created.

## API and token behavior

- `GET /api/gateway/subscriptions` — authenticated metadata list; token hashes are never included.
- `POST /api/gateway/subscriptions/create` — authenticated JSON body `{"clientId":"phone","expiresInDays":30}`. The response includes the metadata and the absolute HTTPS URL once. The URL is not persisted in the response history by this service and must be copied immediately.
- `POST /api/gateway/subscriptions/{id}/revoke` — authenticated, idempotent revocation.
- `GET /s/{token}` — public capability URL; returns the current `.conf` only while the token is unexpired, unrevoked, and its client is active. Invalid, expired, revoked, or unavailable profiles fail closed. Responses use `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, and `X-Content-Type-Options: nosniff`.

Tokens are 256-bit opaque values (43 base64url characters). Persistent state stores only a SHA-256 token hash, with versioned atomic file replacement and mode `0600`. The raw token is returned only at issuance; it is not returned by list/get operations. Treat the URL like a password: anyone who has it can download that client's private profile until expiry or revocation.

The application redacts `/s/{token}` paths from its panic and slow-request logs. Reverse proxies and CDN/WAF access logs must also omit subscription paths; backend redaction cannot remove a token already recorded upstream. The application limiter allows 60 requests/minute per valid subscription and 60/minute for invalid/expired token lookups. Also apply a per-source limit at the public reverse proxy so repeated invalid guesses from one client cannot consume the shared invalid-token budget.

Example Nginx configuration (declare the zone in the `http` block; the `location` belongs in the HTTPS server):

```nginx
limit_req_zone $binary_remote_addr zone=awg_gateway_subscriptions:10m rate=30r/m;

location ~ ^/s/[A-Za-z0-9_-]{43}$ {
    access_log off;
    limit_req zone=awg_gateway_subscriptions burst=5 nodelay;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_pass http://awg-manager;
}
```

Configure the proxy's upstream and TLS normally. Keep `/api/gateway/` behind the existing authenticated admin route; only `/s/{token}` is intentionally public.

## Verification boundary

Unit/component tests cover token issue/list/revoke, profile response headers, and active-client gating. The local gateway packet path has separate synthetic-peer evidence. An external device fetching this URL, connecting the downloaded profile, reaching the assigned policy, and stopping after revocation remains a required release gate.

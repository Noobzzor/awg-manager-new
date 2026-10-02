# Publication fixture provenance

This sanitization changes test literals only: 17 key occurrences (six distinct
values) and two password occurrences (one value), in 11 Go tests and one TS test.
These publicly reproducible synthetic fixtures must NEVER be used on a live
connection, account, or deployment. They are not secret or secure credentials.

## Deterministic generation (v1)

Let `H` be the lowercase hexadecimal SHA256 of the exact original literal
(ASCII text; for keys, hash the base64 text, not decoded bytes). No original
credential material is retained here. Use ASCII concatenation with no newline.

- Key: standard padded base64 of the 32-byte SHA256 digest of
  `awg-manager-new/publication-fixture/v1/key/` + `H`. No clamping is applied;
  these are parser/serialization fixtures, including header-protection keys.
- Password: `fixture-only-` + the first 24 lowercase hexadecimal characters of
  SHA256 of `awg-manager-new/publication-fixture/v1/password/` + `H`.

The same old hash always maps to the same replacement across files and
assertions. Existing peer/public-key literals are unchanged; the affected tests
do not assert a derived private-to-public key relationship. All non-target
literals and test behavior remain unchanged.

## Exact literal hash mapping

| Original literal SHA256 | Replacement literal SHA256 | Occurrences |
|---|---|---:|
| `5c4c9af3010dba7ca469b8c27b0a84f4e98b2bf5d9a2fa8eb85db604ca0dce2d` | `42e3e0540707ea220c2118959822c6fc3abb6db943dc546410eeb6ea194f1c74` | 3 |
| `59ab54243ae515e7a9065862a12f2c252292d81249a2f895902a5d05d2abd6c7` | `42ba7ac162af67085f1a35aec3c0c84a2999d3ddb7bc4847d49c60ce7d5f0285` | 1 |
| `e16af6835b9be1c5a11e75913ade763511751e53ab9a14f65e44f236aa6705e8` | `779b0850790a72df062dc51d70ea3a27f85c813305c488949b8a50df507a9d62` | 3 |
| `dd12fafacf193777b7eb6cbb131a2f17d970baf4bb155c2e76649be4dc7aeefc` | `ae247d01d49c2f092c8ee95d9ad6f5ed327678e0443152f99146c86c3890d581` | 3 |
| `10acb38718ebc975ee6e03bded79b1ed7b1eebdcc38a9dc115fec8c4f56e95f8` | `adcc3ba24d86ffd755a5092e9ee73b63f85ccfefaa5f6a314d8b6fa00e2b8adb` | 2 |
| `3a901703cadf99b725b51d9efe1aa4ca7aec1d39d3b4736e9d546e654d9e767b` | `18fd33224327be8a2682a9b4b12b0e957cba892eb2f32abb0819fb0d4f00e087` | 5 |
| `1acbaab53ae998ad0e1d14df0599f11a6cb4e62be20886d1c4a0c56b2e516468` | `7bd7bdfcac3e8bd928eee13f7ca9e178835526e6457a7ad0a17bb0d36692f217` | 2 |

## Changed tests

- `internal/api/import_obfuscator_test.go`
- `internal/obfuscator/instance_test.go`
- `internal/awg3endpoint/conf_test.go`
- `internal/tunnel/config/file_test.go`
- `internal/tunnel/service/nativewg_conf_test.go`
- `internal/tunnel/service/import_awg31_test.go`
- `internal/tunnel/nwg/import_awg3_test.go`
- `internal/tunnel/nwg/keepalive_test.go`
- `internal/tunnel/nwg/signature_normalize_test.go`
- `internal/tunnel/config/validate_awg3_test.go`
- `internal/tunnel/config/validate_keepalive_test.go`
- `frontend/src/lib/components/proxy/shareWizard.logic.test.ts`

## Verification

The six affected Linux package suites passed before and after replacement
(`go test -count=1`, exit 0), using the pinned owned offline runner. A read-only
`gofmt -l` check returned no files, exit 0. Exact literal-only readback confirms
19 replacements and preserves all other base64 literals in these files.

The affected frontend test was attempted before and after replacement through
`npm test -- src/lib/components/proxy/shareWizard.logic.test.ts`, exit 1 both
times: local dependencies are absent and `vitest` is unavailable. The frontend
test did not execute; no dependencies were installed or fetched.

The coordinator subsequently verified all 19 literal-only replacements against
the preserved baseline and their documented generation formula, and found zero
targeted original values in the non-generated source tree. The six affected
Linux packages were independently rerun with exit 0. Frontend dependencies were
installed from the unchanged lockfile with lifecycle scripts disabled, then
SvelteKit sync and the affected Vitest file executed with exit 0: 30 tests passed.
Generated dependencies/build state are not part of the publication payload.

## Gate boundary

The credential-sanitization slice changed no production code or scanner exclusions. This note is provenance, not
a broad tests/mocks allowlist or a completed source security gate. The
coordinator must independently verify exact replacements, classify remaining
scanner findings, rescan current source and the publication payload, then verify
exact remote readback if publication is separately approved.

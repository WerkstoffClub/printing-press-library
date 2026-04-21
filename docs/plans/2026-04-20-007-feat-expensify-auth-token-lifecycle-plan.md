---
title: "feat: Expensify auth token lifecycle — headless mint, auto-retry, staleness visibility"
type: feat
status: active
date: 2026-04-20
---

# feat: Expensify auth token lifecycle — headless mint, auto-retry, staleness visibility

**Target repo:** printing-press-library (branch `feat/expensify` or a new branch off it; user's call at execution time)

## Overview

Expensify session authTokens are fragile in the current CLI:

- They expire silently in roughly 2-3 hours — no refresh, no warning, just jsonCode 407 on the next API call
- They don't live in cookies — the current capture path requires a headed browser + a fetch interceptor to scrape the token from form-body traffic
- Recovery requires a full interactive browser round trip, which means the CLI is unusable for CI, cron, long-lived MCP servers, or any unattended flow
- `doctor` catches expiry but every other command dies with the same opaque error; no auto-refresh, no actionable hint beyond "run auth login"

This plan adds three capabilities that together make auth survivable:

1. **Headless credential-based login** — `auth store-credentials` saves email+password to the OS keychain, `auth login --headless` mints a fresh token via Expensify's internal authenticate endpoint without a browser
2. **Client-level auto-retry on 407** — when a command returns a session-expired jsonCode, the client attempts one transparent re-auth (only when headless creds are available) and retries
3. **Staleness visibility** — `auth status` shows token age and approximate TTL, `doctor` warns proactively when the token is close to expiry, and `auth login --headless` is offered as the fix

The headed-browser `auth login` path stays as the default for first-time setup and for accounts with 2FA/SSO where password-only auth won't work.

## Problem Frame

Mid-flight in our dogfood session, the authToken expired. Every subsequent command returned `{"jsonCode":407,"message":"Your session has expired. Please sign in again."}`. The user had to manually re-launch a headed browser, log in, wait for the fetch interceptor to capture a new token, and re-configure the CLI. The user's (valid) reaction: "why do these tokens suck so much."

This is the bottom of Expensify's auth UX: the web app refreshes silently because it owns the whole browser session. A third-party CLI that borrows the token has no equivalent mechanism unless we build one. The best fix uses what Expensify's login page already does — POST email+password (and optional 2FA code) to an /api/Authenticate-style endpoint — and caches the result with rotation.

This plan does NOT attempt:
- SSO / OAuth flows (Expensify uses its own magic-link and SSO primarily for enterprise accounts; those paths are out of scope for v1)
- TOTP/2FA collection UX (when the account has 2FA, the plan falls back to the existing headed browser flow with a clear message)
- Partner-key (Integration Server) expansion to cover filing commands (the Integration Server doesn't wire up /RequestMoney or /CreateReport; it's a separate API surface)

## Requirements Trace

- R1. `expensify-pp-cli auth store-credentials` saves an email+password pair to the OS keychain (macOS Keychain / Linux libsecret / Windows Credential Manager), never to the TOML config
- R2. `expensify-pp-cli auth login --headless` mints a fresh authToken from stored credentials via Expensify's internal authenticate endpoint, with no browser involvement; falls back with a clear error when 2FA is detected
- R3. Every `client.Post` and `client.Search` call that returns a session-expired response (jsonCode 407 or equivalent) attempts one transparent re-auth using headless credentials (when available) and retries the original request before surfacing the error
- R4. `auth status` shows token age, approximate expiry (if known), and whether headless credentials are configured
- R5. `doctor` prints a WARN line when the token is older than a threshold (default 60 minutes) and actionably recommends `auth login --headless`; ERROR when expired
- R6. The existing headed-browser `auth login` path continues to work unchanged for first-time setup and for accounts where headless auth isn't possible

## Scope Boundaries

- No SSO or OAuth implementations
- No TOTP/2FA prompting — when 2FA is detected, fail with a clear message telling the user to use the existing headed `auth login`
- No background daemon that refreshes proactively without user action — the auto-retry fires on-demand when a request hits 407
- No migration of the Integration Server partner-key path; it already works for admin commands
- No change to write-path request bodies (consistent with the expensify live-data plan's constraint)

### Deferred to Separate Tasks

- **2FA / TOTP support**: future plan; needs a code-entry UX for interactive shells and a recovery-code input for CI
- **Automatic credential rotation alerts** when an Expensify login is changed externally: future plan; requires listening on a different endpoint
- **Encryption of the TOML config file itself**: not in scope; credentials live in keychain specifically to avoid ever touching the TOML

## Context & Research

### Relevant Code and Patterns

- `library/productivity/expensify/internal/config/config.go` — existing token persistence; this plan adds an optional `expensify_email` field (the email is non-secret; password stays in keychain). Mirror the existing `SaveSessionToken` + env-override pattern
- `library/productivity/expensify/internal/client/client.go` — the `do()` method already has adaptive rate limiting and retry logic for 429 and 5xx. Adding a 407 branch follows the same structure; the retry triggers a callback into the auth layer rather than sleeping
- `library/productivity/expensify/internal/cli/auth.go` — existing `auth login`, `auth set-token`, `auth status`, `auth logout`, `auth set-keys`. New subcommands extend this tree
- `library/productivity/expensify/internal/cli/doctor.go` — existing WARN/OK/FAIL output style; the staleness warning slots in cleanly
- Other CLIs in the library use keychain for secret storage (e.g., browser-auth flows in `pp-linear`) — mirror whichever keychain library is already adopted; if none is, adopt `github.com/zalando/go-keyring` which is the de facto standard for cross-platform Go keychain access
- `library/productivity/expensify/internal/cli/sync.go` — `client.Search` wrapper and error classification through `classifyAPIError`; this is the path that needs the auto-retry hook

### Institutional Learnings

- Prior sniff work in this repo (flightgoat, espn, recipe-goat) established the pattern of reverse-engineering endpoint shapes from a logged-in browser session via agent-browser fetch interception. The authenticate-endpoint discovery in Unit 2 follows this pattern
- The expensify live-data plan (`2026-04-20-006-feat-expensify-live-data-and-consolidate-plan.md`) introduced the client.Search typed helper and the pattern of wrapping all POSTs through a single code path — making auto-retry a one-location change
- AGENTS.md for `printing-press-library`: "Never commit changes that store secrets in source files; use the keychain for credentials, TOML for non-secret config"

### External References

- No external research in this pass. Expensify's internal auth endpoint isn't publicly documented; the discovery happens via the Unit 2 sniff task and is recorded in the plan's execution artifacts. If a future maintainer needs to re-verify, the capture procedure is straightforward: open new.expensify.com in agent-browser, install a fetch interceptor, log in, observe the POST body

## Key Technical Decisions

- **Credentials in keychain, email mirror in TOML**: The email address is non-secret and useful for `auth status` display; it goes in `~/.config/expensify-pp-cli/config.toml` under `expensify_email`. The password lives only in the OS keychain. Rationale: minimizes blast radius if the TOML leaks; no secrets in diffs, backups, or `cat`
- **Keychain library: `go-keyring`**: Cross-platform (macOS Keychain via Security.framework, Linux Secret Service via dbus, Windows Credential Manager), minimal API surface, widely adopted. Rationale: follows the institutional pattern, avoids writing our own CGO bindings
- **Auto-retry is opt-in at command level, opt-out at client level**: The default is to auto-retry when headless creds are available. A `--no-auto-retry` persistent flag disables it for callers who want deterministic failures. Rationale: user's primary gripe is the silent failure + manual recovery; auto-retry directly addresses that. The opt-out exists for CI flows that prefer to fail fast and log
- **Only ONE auto-retry per request**: If the re-auth mints a fresh token and the second attempt still fails, surface the original error. No exponential retry loops. Rationale: avoids a runaway loop when credentials are wrong or the account is locked
- **Expiry detection uses jsonCode 407 primarily, with an HTTP status fallback**: Expensify's dispatcher returns HTTP 200 with `jsonCode 407` in the body (not a real 401). Client inspects the parsed response. If the body can't be parsed, fall back to treating actual HTTP 401/403 as expired. Rationale: matches observed behavior; the fallback handles future API shape changes
- **Staleness threshold: 60 minutes default, configurable via env var**: `EXPENSIFY_TOKEN_STALE_AFTER=<minutes>` overrides. Doctor prints WARN when age > threshold, ERROR when the last known auth-verification returned 407. Rationale: 60 minutes leaves headroom before the real expiry of ~2-3 hours; tight enough to prompt a refresh without being annoying
- **Headless auth endpoint: `/api/Authenticate` (or whichever the sniff reveals)**: Unit 2 captures the exact endpoint name and body shape; the plan does not pin an exact name because Expensify may call it `SignIn`, `Authenticate`, or something else. The client helper accepts the shape as a typed struct regardless of endpoint name
- **2FA detection: check the authenticate response for a 2FA-required marker**: If the response returns a jsonCode or message indicating 2FA is needed, fail with: `"This account requires 2FA. Use \`expensify-pp-cli auth login\` (headed browser) to complete the 2FA prompt."` Rationale: 2FA UX is deferred; detection keeps the headless path safe and directs users to the working fallback

## Open Questions

### Resolved During Planning

- **Store the password in keychain or in a file?** → Keychain only. Files-on-disk for passwords are a common audit finding
- **Should auto-retry also attempt headed-browser login when headless creds aren't configured?** → No. Auto-retry only kicks in when headless is viable. When headless creds aren't configured, the 407 error surfaces normally with a hint to run `auth store-credentials` + `auth login --headless`
- **Should `auth login --headless` work when partner creds are configured but session creds aren't?** → Yes. Partner creds and session creds are independent paths. Headless auth uses the password path regardless of partner-key state

### Deferred to Implementation

- **Exact /Authenticate endpoint name and body shape**: Unit 2's first task is to sniff this from a real login. Until then, the plan describes the pattern without pinning a specific endpoint string
- **Whether Expensify returns a TTL in the authenticate response**: if yes, we persist it and make `auth status` show real expiry rather than a heuristic estimate. If no, the age-since-mint heuristic is the only signal available
- **Keychain library choice on Linux specifically**: `go-keyring` uses Secret Service which requires a running desktop session. For server/CI Linux flows, a fallback to an encrypted file (e.g., `~/.config/expensify-pp-cli/.credentials.enc` with a master key derived from a user-supplied passphrase) may be needed. Decide based on the implementer's context at build time; the default is keychain
- **Whether `auto-retry` should also handle concurrent requests racing to re-auth**: in single-command CLI usage this doesn't matter; for a future `mcp` server or `watch` daemon it does. Sketch a mutex around the re-auth routine but defer the full concurrency test coverage

## Implementation Units

- [ ] **Unit 1: Keychain-backed credential store + `auth store-credentials` command**

**Goal:** Store an email+password pair in the OS keychain. `auth status` reports whether credentials are configured. The email (non-secret) mirrors to TOML.

**Requirements:** R1, R4

**Dependencies:** None

**Files:**
- Create: `library/productivity/expensify/internal/credentials/credentials.go`
- Create: `library/productivity/expensify/internal/credentials/credentials_test.go`
- Modify: `library/productivity/expensify/internal/config/config.go` (add `ExpensifyEmail string` field, toml tag `expensify_email`, env override `EXPENSIFY_EMAIL`)
- Modify: `library/productivity/expensify/internal/cli/auth.go` (register `newAuthStoreCredentialsCmd(flags)`, update `auth status` to surface credential presence)
- Modify: `library/productivity/expensify/go.mod` / `go.sum` (add `github.com/zalando/go-keyring` dependency)

**Approach:**
- `credentials` package: thin wrapper around `go-keyring` exposing `Set(email, password string) error`, `Get(email string) (password string, err error)`, `Delete(email string) error`, `Has(email string) bool`
- Service key for the keychain entry: constant string like `"expensify-pp-cli"`; account key is the email
- `auth store-credentials`: prompts for email (unless `--email X`) and password (always via stdin without echo; use `golang.org/x/term` for no-echo read). When `--no-input`, require both `--email` and `--password` (document that this is dangerous for shell history — recommend `EXPENSIFY_EMAIL` + `EXPENSIFY_PASSWORD` env vars and avoid putting `--password` on the command line)
- On success: persist email to TOML config, write password to keychain, print "Credentials stored for <email>." with a next-step hint: "Run `expensify-pp-cli auth login --headless` to mint a session token."
- `auth status` gains lines: `Email: <email>` (or `Email: not configured`), `Headless credentials: configured` / `not configured`

**Patterns to follow:**
- Existing `auth set-token` command for the cobra subcommand skeleton and validation style
- Config env-override block for `EXPENSIFY_AUTH_TOKEN`
- Other library CLIs' keychain usage (search the repo for `go-keyring` imports)

**Test scenarios:**
- Happy path: `Set("a@b.com", "pw1")` then `Get("a@b.com")` returns "pw1"
- Happy path: `Delete` removes the entry; subsequent `Has` returns false
- Edge case: `Get` for unknown email returns `keyring.ErrNotFound` (or equivalent) — callers use this to detect "not configured"
- Edge case: empty email or password — return usage error before calling keyring
- Integration: `auth store-credentials --email a@b.com --password pw` (in a test harness) persists to keychain AND writes `expensify_email = "a@b.com"` to a temp TOML
- Integration: `auth status` after storage shows `Email: a@b.com` and `Headless credentials: configured`
- Error path: keychain unavailable (mocked) → clear error message suggesting "keychain access requires a graphical session or unlock"

**Verification:**
- `go vet ./internal/credentials/...` clean, `go test ./internal/credentials/...` passes
- Manual: `auth store-credentials` + `auth status` round-trips a credential on macOS (at minimum)

---

- [ ] **Unit 2: Discover & implement headless authenticate endpoint + `auth login --headless`**

**Goal:** Extend `auth login` to accept `--headless` which posts stored credentials to Expensify's internal authenticate endpoint and saves the returned authToken. Direct, no browser.

**Requirements:** R2

**Dependencies:** Unit 1 (needs the credentials store)

**Execution note:** The first task is a short discovery step: open agent-browser, install a fetch interceptor, log in to new.expensify.com with a test or real account, and capture the POST body + response shape of the login call. Record the endpoint name, field names, and response shape in a short comment block at the top of the new client helper. Do NOT hard-code credentials into test fixtures.

**Files:**
- Create: `library/productivity/expensify/internal/client/authenticate.go`
- Create: `library/productivity/expensify/internal/client/authenticate_test.go` (tests use canned response bodies, never live credentials)
- Modify: `library/productivity/expensify/internal/cli/auth.go` (extend `newAuthLoginCmd` to branch on `--headless`)

**Approach:**
- `authenticate.go` exposes `Authenticate(ctx context.Context, email, password string) (token string, expiresAt time.Time, err error)`. Internals:
  1. Marshal a form body with the fields discovered in the sniff (likely something like `partnerName`, `partnerPassword`, `twoFactorAuthCode`, `useExpensifyLogin` — exact fields TBD from sniff)
  2. POST to the discovered endpoint with the standard form-body headers used elsewhere in the client
  3. Parse the response; on success, extract the authToken from the response body. If the response includes an expiry hint, return it; otherwise zero-value `expiresAt` (caller falls back to heuristic)
  4. 2FA detection: if the response contains an indicator that a 2FA code is required (e.g., jsonCode 402 with message mentioning 2FA, or a field like `requiresTwoFactorAuth: true`), return a typed error `ErrTwoFactorRequired` that the CLI layer translates to the user-facing "use headed login" message
  5. Invalid-credentials detection: specific jsonCode (likely 403 or a dedicated code) → typed `ErrInvalidCredentials`, user-facing message: "Email or password rejected by Expensify. Re-run `auth store-credentials` or log in via browser to reset your password."
- `auth login --headless`:
  1. Read email from config (error clearly if unset)
  2. Read password from keychain via the Unit 1 credentials package (error clearly if unset, suggesting `auth store-credentials`)
  3. Call `client.Authenticate(...)`. On `ErrTwoFactorRequired`, print the fallback message and exit 2 (usage). On `ErrInvalidCredentials`, exit 4 (auth). On other errors, exit 5 (API)
  4. On success, call `cfg.SaveSessionToken(token, email)` (existing method from the live-data plan) and also persist `LastLoginAt = time.Now().UTC()` to a new config field for staleness display
  5. Print: "Session token minted via headless login. Valid for ~2-3h. Run `expensify-pp-cli doctor` to verify."

**Patterns to follow:**
- `internal/client/client.go` `buildNewExpensifyRequest` for the form-body shape
- `internal/cli/auth.go`'s existing subcommands for the RunE skeleton
- Typed errors pattern from `expensifysearch.SearchError` (Unit 1 of the live-data plan)

**Test scenarios:**
- Happy path: canned 200 response with a valid token → `Authenticate` returns token, zero-value `expiresAt` (if response doesn't include TTL) or parsed TTL (if it does)
- Error path: canned 2FA-required response → returns `ErrTwoFactorRequired`
- Error path: canned invalid-credentials response → returns `ErrInvalidCredentials`
- Error path: network timeout → returns the underlying timeout error
- Integration: `auth login --headless` with no email configured → exit 2, message mentions `auth store-credentials`
- Integration: `auth login --headless` with email but no keychain password → exit 2, same hint
- Integration: successful headless login (using a spy on the Authenticate function) writes the new token via `cfg.SaveSessionToken` and sets `LastLoginAt`
- Edge case: response body is valid JSON but missing the authToken field → typed error "authenticate response missing token"

**Verification:**
- `go vet` clean; tests pass
- Manual dogfood: `auth store-credentials` + `auth login --headless` succeeds against a real account (no 2FA) and the minted token works for a subsequent `me get`
- The existing headed `auth login` path still works with no regressions

---

- [ ] **Unit 3: Client auto-retry on jsonCode 407**

**Goal:** Every request the CLI makes survives one silent re-auth + retry when it hits session-expired, provided headless credentials are configured. No retry loop; one attempt and out.

**Requirements:** R3

**Dependencies:** Unit 2 (needs `client.Authenticate` and the updated `auth login --headless` path)

**Files:**
- Modify: `library/productivity/expensify/internal/client/client.go` (wrap the existing `do()` method's response-parsing path with an expiry check; add `autoRetryOnExpired bool` field on the Client; add a retry callback)
- Modify: `library/productivity/expensify/internal/cli/root.go` (register a persistent `--no-auto-retry` flag; when set, client disables the retry behavior)
- Test: `library/productivity/expensify/internal/client/client_test.go` (create if missing)

**Approach:**
- Client gets a new method-level hook `refreshAuth func(ctx) error` injected at construction (or left nil). When not nil, the client uses it to re-auth on 407
- In `do()`: after parsing the response body, check for `jsonCode == 407` (or the precise indicator the sniff confirms). If present AND `autoRetryOnExpired` is true AND `refreshAuth` is non-nil AND this is the first attempt in the current `do()` call, invoke `refreshAuth` and re-issue the request with the freshly loaded authToken. Return the second attempt's result regardless of outcome
- `root.go` constructs the default `refreshAuth` as a closure that: (1) checks `cfg.ExpensifyEmail != ""` and keychain has the password, (2) calls `client.Authenticate`, (3) saves the new token via `cfg.SaveSessionToken`. When email/password aren't available, the closure returns `ErrHeadlessNotConfigured` and the client falls through to surface the original 407 with an improved hint mentioning `auth store-credentials`
- Add a one-request mutex around the re-auth closure so concurrent callers in a future MCP/watch mode don't stampede
- `--no-auto-retry`: when set, the client's `autoRetryOnExpired` is false regardless of credentials

**Patterns to follow:**
- Existing 429 retry branch in `client.go` `do()` for the retry-counter discipline
- Existing classifyAPIError in `internal/cli/helpers.go` for the post-failure hint format

**Test scenarios:**
- Happy path: first response returns jsonCode 407, refreshAuth succeeds, second response returns jsonCode 200 with payload → caller receives the 200 payload, sees no error
- Error path: first response returns jsonCode 407, refreshAuth fails (e.g., ErrInvalidCredentials) → caller receives a clear error that includes BOTH the original expiry cause AND the re-auth failure reason
- Error path: first response returns jsonCode 407, refreshAuth succeeds, second response ALSO returns jsonCode 407 → caller receives the second error; NO third attempt
- Error path: refreshAuth is nil (no headless creds) → client returns the 407 error with an improved hint
- Edge case: `--no-auto-retry` → client skips the retry branch entirely; 407 surfaces immediately
- Integration: three concurrent requests hit 407 simultaneously → only one refreshAuth call fires (mutex), all three retry with the fresh token
- Edge case: 429 rate-limit AND 407 expiry in the same attempt counter → 429 handling wins first; only after the 429 retry settles does the 407 branch apply
- Edge case: `auto-retry` enabled but body isn't JSON (e.g., Cloudflare HTML error page) → falls back to the existing error path; does NOT attempt re-auth

**Verification:**
- Tests pass against the updated do() logic
- Manual: start a session, let the token expire (or manually invalidate it), run `expense list --live`; command succeeds silently after transparent re-auth

---

- [ ] **Unit 4: `auth status` staleness display + doctor warning**

**Goal:** Give the user visibility into token age before the next command dies from expiry. Doctor prints a WARN when the token is approaching staleness and ERROR when the last verification hit 407.

**Requirements:** R4, R5

**Dependencies:** Unit 2 (needs `LastLoginAt` config field); Unit 1 helpful for the "headless credentials present?" line but not strictly required

**Files:**
- Modify: `library/productivity/expensify/internal/config/config.go` (add `LastLoginAt time.Time` field; env override not needed)
- Modify: `library/productivity/expensify/internal/cli/auth.go` (extend `newAuthStatusCmd` output)
- Modify: `library/productivity/expensify/internal/cli/doctor.go` (add staleness branch; flag 407 explicitly)

**Approach:**
- `auth status` output gains:
  - `Token age: <duration>` — computed from `LastLoginAt`
  - `Token status: fresh | stale (>60m) | possibly expired (>120m)` — based on age thresholds
  - `Email: <email or "not configured">`
  - `Headless credentials: configured | not configured`
- Staleness threshold: default 60 minutes; overridable via `EXPENSIFY_TOKEN_STALE_AFTER` env var (minutes int)
- Doctor gains a new line:
  - OK when age < threshold
  - WARN when age >= threshold AND headless creds are available: "Token is <age> old. Run `auth login --headless` or let auto-retry refresh on next call."
  - WARN when age >= threshold AND headless creds NOT available: "Token is <age> old. Run `auth login` (headed) or configure headless creds via `auth store-credentials` + `auth login --headless`."
  - ERROR (from existing 407 validation path): same as today but mention `auth login --headless` in the hint when applicable
- `auth status --json` includes `token_age_seconds`, `stale_threshold_seconds`, `is_stale`, `email_configured`, `headless_credentials_configured`

**Patterns to follow:**
- Existing `auth status` output formatting
- Existing doctor WARN/ERROR/OK rendering
- Existing `--json` branch pattern in other CLI commands

**Test scenarios:**
- Happy path: `LastLoginAt` is 10 minutes ago → `auth status` prints "Token age: 10m, fresh"
- Edge case: `LastLoginAt` is 90 minutes ago → "Token age: 1h30m, stale"
- Edge case: `LastLoginAt` is zero-value (never set) → "Token age: unknown"
- Edge case: `EXPENSIFY_TOKEN_STALE_AFTER=30` env var → threshold becomes 30 minutes; 45-minute-old token renders stale
- Integration: `doctor` with a 90-minute-old token AND configured headless creds → WARN line mentioning `auth login --headless`
- Integration: `doctor` with a 90-minute-old token AND no headless creds → WARN line mentioning both `auth login` and `auth store-credentials`
- Integration: `doctor` with session-validation returning 407 → ERROR line (existing behavior) but the hint now includes `auth login --headless` when credentials are configured
- JSON output: `auth status --json` parses to an object with every documented field

**Verification:**
- Tests pass
- Manual: walk through each state (fresh / stale / expired / no-creds) and confirm output matches the spec

## System-Wide Impact

- **Interaction graph:** Every CLI command that calls `client.Post` or `client.Search` now goes through the auto-retry path. Read-only commands (`expense list`, `me get`, etc.) are safe to retry transparently; write-side commands (`expense create`, `report submit`) are ALSO safe because the mutation hasn't committed by the time a 407 surfaces (Expensify rejects the whole request when the token is invalid). No special handling needed per verb
- **Error propagation:** 407 stops being a terminal error when headless creds are configured. When re-auth fails, the wrapped error surfaces with both the original expiry and the re-auth cause so users can diagnose "am I locked out or is my password wrong?"
- **State lifecycle risks:**
  - Partial-write on retry: a request that mutates state (e.g., `AddExpensesToReport`) that hits 407 before the mutation applies → zero risk (Expensify rejected the whole request). A request that returns 407 AFTER partial mutation → effectively impossible with Expensify's dispatcher architecture, but flagged as a known assumption
  - Concurrent re-auth stampede: the plan adds a mutex; a future MCP/daemon mode benefits directly. Single-command CLI usage is not affected
  - Keychain availability on headless Linux: falls back to a clear error; does not silently write secrets to disk
- **API surface parity:** No other CLI interface exposes the new commands. If MCP unstubs in a future plan, it inherits `auth login --headless`, `auth store-credentials`, `auth status` trivially via the same cobra tree
- **Integration coverage:** Auto-retry needs a real end-to-end test (spy on client.Authenticate + spy on client.Post, wire them together, simulate the 407→re-auth→200 sequence). Mocks alone don't prove the flow
- **Unchanged invariants:**
  - The existing headed `auth login` flow works unchanged — no user loses their current path
  - `auth set-token` and `auth set-keys` unchanged
  - Partner-key (Integration Server) flow unchanged
  - TOML config file format backwards compatible (new fields are optional)
  - Exit-code table unchanged; new paths reuse existing typed exits

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| The internal `/Authenticate` endpoint changes shape | Unit 2's discovery step is repeatable; a future sniff + update to `authenticate.go` fixes it. Typed error classes (ErrTwoFactorRequired, ErrInvalidCredentials) are resilient to field-name changes |
| Expensify detects automated login and rate-limits or blocks the account | Unit 2 reuses the existing `buildNewExpensifyRequest` headers (User-Agent: `expensify-pp-cli/x.y`, referer: `ecash`) which are already accepted by `/ReconnectApp` and `/Search`. If Expensify adds captcha or device fingerprinting, the plan falls back to headed login cleanly. Include rate limiting on the retry path (mutex + jitter) to avoid stampede detection |
| Keychain unavailable on Linux CI | Document the fallback path (re-run `auth store-credentials` on each CI node, or pass `EXPENSIFY_EMAIL` + `EXPENSIFY_PASSWORD` env vars explicitly in that session). Add a keychain-less fallback in a future plan if users report friction |
| Password in env vars is risky | `auth store-credentials` prefers stdin with no-echo read. The `--password` flag exists but is documented as CI-only with a warning about shell history. The env-var path reads from the environment where it's already protected by whatever ran the process |
| Auto-retry masks real auth problems | Errors from the re-auth step are wrapped, not swallowed. When the second attempt also fails, the user sees both causes. `--no-auto-retry` gives deterministic failures for diagnosis |
| 2FA-enabled accounts can't use headless | Detection is explicit; 2FA users get a clear message to use the existing headed path. Future plan can add TOTP support |
| Token expiry detection false positives (non-407 errors misclassified) | Unit 2's sniff should capture the EXACT jsonCode for expiry; the plan uses that specific code, not a range. HTTP 401/403 fallback is defensive only |
| Retry invalidates rate-limit discipline | Re-auth fires a single extra request per 407; it shares the same limiter. No escalation of request rate |

## Documentation / Operational Notes

- Update `library/productivity/expensify/README.md` Quick Start to mention `auth store-credentials` + `auth login --headless` as the recommended flow for repeat use
- Update `library/productivity/expensify/SKILL.md` Auth Setup section to add the headless path as the default recommendation alongside the browser flow; keep the browser flow for first-time and 2FA users
- Add a troubleshoots entry: "Token expired" → "Run `auth login --headless` or let auto-retry refresh on next call"
- No monitoring changes — CLI is local-first
- Rollout: land as one PR stacked on top of PR #104 (or a separate PR if #104 is merged first). No feature flags; the new commands are additive

## Sources & References

- Session transcript: live evidence of 407 mid-flow, fetch-interceptor auth capture, headed-browser re-login pain
- Related plan: `docs/plans/2026-04-20-006-feat-expensify-live-data-and-consolidate-plan.md` (Unit 1 established `client.Search` as the single wrapped call site; auto-retry plugs in here)
- Related code:
  - `library/productivity/expensify/internal/client/client.go` — the `do()` method and existing retry discipline
  - `library/productivity/expensify/internal/cli/auth.go` — the existing auth subcommand tree
  - `library/productivity/expensify/internal/cli/doctor.go` — session validation and output formatting
  - `library/productivity/expensify/internal/config/config.go` — persistence + env-override pattern
- External reference: `github.com/zalando/go-keyring` (prospective dependency; not yet in the repo's go.sum)

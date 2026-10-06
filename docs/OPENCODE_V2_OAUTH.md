# OpenCode Go v2: OAuth login plan

Status: proposal. Nothing in this document is implemented yet.

## Problem

`occb` authenticates against OpenCode Go with a static API key:

```
Authorization: Bearer <api_key>      -> https://opencode.ai/zen/go/v1/chat/completions
x-api-key: <api_key>                 -> https://opencode.ai/zen/go/v1/messages
```

With the OpenCode v2 Console this no longer works for new Go subscriptions:

- The [Go docs](https://opencode.ai/docs/go/) still say "subscribe, copy your API key, run `/connect`", but users with an active
  subscription report that the v2 Console offers no way to create, view or copy a Go API key
  ([#52227](https://github.com/anomalyco/opencode/issues/52227), [#50885](https://github.com/anomalyco/opencode/issues/50885),
  [#50932](https://github.com/anomalyco/opencode/issues/50932)).
- Keys that can be created have the Zen prefix `oc_sk_` instead of the Go prefix `sk-`, and the CLI answers
  "Invalid credential" ([#52243](https://github.com/anomalyco/opencode/issues/52243)).
- The supported path is the Console device login (`opencode console login` in V1, `opencode auth login opencode` in V2).

Result: without a Go API key, occb has nothing to send, and its PR #1 (mixed Claude/OpenCode mode) can route to OpenCode models
but cannot authenticate the upstream call. The model catalog (`GET /zen/go/v1/models`) is public, so `/model` discovery still works.

## What is known

Gathered from the open-source clients that already implement the flow. These details are **reverse-engineered, not documented by
OpenCode**, and have to be confirmed against the opencode source before implementing.

| Topic | What we know | Source |
| --- | --- | --- |
| Grant type | OAuth 2.0 device authorization grant (RFC 8628) | [pi-provider-opencode-console](https://github.com/DevJake/pi-provider-opencode-console) |
| Client | Public client `opencode-cli` (the one the opencode binary signs in with) | [oh-my-pi #12325](https://github.com/can1357/oh-my-pi/pull/12325) |
| Device code | `expires_in: 900`, `interval: 5`, relative verification URIs such as `/console/device?user_code=...` | oh-my-pi #12325 |
| Token use | `Authorization: Bearer <access_token>` plus `x-opencode-org-id` | [burnrate #130](https://github.com/RichardODonoghue/burnrate/pull/130) |
| Inference lane | Console tokens use `https://opencode.ai/inference/go/...`, not `/zen/go/v1/...` (e.g. `/inference/go/v1/usage`) | burnrate #130 |
| Client identity | `User-Agent` starting with `opencode/<version>` and a well-formed `x-opencode-session` (`ses_<12 hex><14 alnum>`) appear to be required | oh-my-pi #12325, [pi-provider #7](https://github.com/grikomsn/pi-provider-opencode-console/pull/7) |
| Storage in opencode | Token lives in opencode's SQLite `account` table (`access_token`, `account_state.active_org_id`), not in `auth.json` | burnrate #130 |
| Refresh | Access token is refreshed shortly before expiry; refresh tokens rotate | pi-provider-opencode-console |
| Catalog | Public `GET /zen/go/v1/models`, no auth | pi-provider #7 |

Unknown and to be resolved from the opencode source: exact device-authorization and token endpoint URLs, the client id value,
scopes, token lifetime, how the org id is obtained (`/api/orgs`, `/api/config` are mentioned), and the exact inference paths for
`/chat/completions` and `/messages` on the console lane.

## Decision

Implement the device login natively in occb (option 2 of the review). occb will own its own credential instead of depending on a
static key or on opencode's database.

Why not the alternatives:

- **Static `sk-` key**: not obtainable for affected accounts; kept as a supported fallback when it exists.
- **Read opencode's SQLite token (option 3)**: depends on an internal schema, and sharing a rotating refresh token with a running
  opencode would invalidate one side.
- **`opencode serve` as backend**: it runs its own sessions and tools, which does not fit Claude Code's tool loop.

## Design

### Commands

```
occb login      # device flow: print code + URL, poll, store credential
occb logout     # delete the stored credential
occb status     # also shows auth state: api key / oauth (expires in ...) / none
```

### Credential storage

- File `credentials.json` next to `config.yaml` (`~/.config/occb/`), mode `0600`, written atomically.
- Contents: access token, refresh token, expiry, org id, console base URL. Never logged.
- `api_key` in config / `OCB_API_KEY` keeps working and takes precedence, so existing setups do not change.

### Auth abstraction

Replace the raw `apiKey string` in `internal/client` with an interface:

```go
type Authenticator interface {
    // Apply sets credentials and any required identity headers on the upstream request.
    Apply(ctx context.Context, req *http.Request) error
    // BaseURLs returns the endpoints to use for this credential type.
    Endpoints() Endpoints
}
```

- `APIKeyAuth`: current behaviour (`/zen/go/v1/...`).
- `OAuthAuth`: refreshes the token when it is within 5 minutes of expiry (single-flight, mutex-protected), sets
  `Authorization`, `x-opencode-org-id`, `x-opencode-session` and the `opencode/<ver>` User-Agent, and points at the console lane.
- On a `401`, refresh once and retry before surfacing the error. If the refresh token is rejected, tell the user to run `occb login`.

### Endpoints

`opencode_go` config keeps the key-based URLs. The console-lane URLs are derived from the credential (and overridable in config)
so a change on OpenCode's side does not need a release.

### Packages

- `internal/auth/oauth` – device flow, token refresh, credential store (no dependency on the proxy).
- `internal/client` – takes an `Authenticator`.
- `internal/cli/login.go` – `login` / `logout`, status output.

### Testing

- Fake OAuth server (`httptest`) for: device flow with `authorization_pending` / `slow_down`, expiry, refresh, rotated refresh
  token, concurrent refresh (single-flight), 401-then-refresh retry, rejected refresh.
- Credential file permissions and atomic write.
- Client tests asserting the headers and base URL for each authenticator.
- Manual check with a real Go subscription (cannot be automated).

## Risks

- **Undocumented surface.** The flow and headers are inferred from third-party clients and can change without notice. Endpoint
  and header values are therefore configurable and isolated in one package.
- **Terms of use.** Presenting an `opencode/` User-Agent and session header from a different client may be against OpenCode's
  terms or anti-abuse rules. Before shipping this, check the Go terms and ask OpenCode whether third-party clients are allowed;
  if they are not, this feature should not ship.
- **Account safety.** Tokens are stored locally with `0600`; they are as sensitive as the API key was.
- **Windows.** File mode `0600` is not enforced by NTFS; document it and keep the file under the user profile.

## Plan

1. Read the opencode source for the exact device-flow endpoints, client id, scopes and inference paths; update the table above.
2. `internal/auth/oauth` with tests.
3. `Authenticator` in `internal/client`, keeping the API-key path unchanged.
4. `occb login` / `logout` / `status`.
5. Docs (`README`, `docs/CONFIGURATION.md`, `docs/TROUBLESHOOTING.md`).
6. Manual verification against a real account, then un-draft.

Depends on PR #1 (mixed mode) for the proxy changes; the auth work itself is independent of it.

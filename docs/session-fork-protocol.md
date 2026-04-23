# Session Fork Protocol

The Proton API uses a session fork protocol to create child sessions for
service-specific apps (Lumo, Drive, Pass, etc.) from a parent account
session. This is how the web client navigates between `account.proton.me`
and service apps like `lumo.proton.me`.

## Overview

The fork protocol has two phases:

1. **Push** — The parent (account) app creates a fork selector on its host.
2. **Pull** — The child (service) app consumes the selector on its own host.

The selector is a shared credential that bridges the two hosts. The Proton
backend correlates push and pull via the `Session-Id` cookie (domain
`.proton.me`, shared across all subdomains).

## Push Phase

The parent app sends an authenticated POST to create the fork:

```
POST /api/auth/v4/sessions/forks
Host: account.proton.me
Cookie: AUTH-<uid>=<token>; Session-Id=<sid>; ...
x-pm-uid: <uid>
x-pm-appversion: web-account@5.0.367.1
Content-Type: application/json

{
  "Payload": "<encrypted-fork-blob>",
  "ChildClientID": "web-lumo",
  "Independent": 0
}
```

Key details:
- Auth is **cookie-based** (`AUTH-<uid>=<token>` cookie), not Bearer token.
- `x-pm-uid` header is present.
- No `Authorization: Bearer` header.
- `ChildClientID` determines which scopes the child session receives.
- `Independent: 0` means the child session is linked to the parent.
- `Payload` contains an encrypted blob with the key password.

Response:
```json
{"Code": 1000, "Selector": "<selector>"}
```

## Pull Phase

The child app consumes the selector with an unauthenticated GET:

```
GET /api/auth/v4/sessions/forks/<selector>
Host: lumo.proton.me
Cookie: Session-Id=<sid>; iaas=...; Tag=default
x-pm-appversion: web-lumo@1.3.3.4
```

Key details:
- **No auth headers** — no `x-pm-uid`, no `Authorization`, no `AUTH-*` cookie.
- Only session cookies: `Session-Id`, `iaas`, `Tag`, `zId`, `aId`.
- `x-pm-appversion` must match the target service.
- The `Session-Id` cookie correlates the pull with the push.

Response:
```json
{
  "Code": 1000,
  "Payload": "<encrypted-fork-blob>",
  "LocalID": 1,
  "ExpiresIn": 86399,
  "TokenType": "Bearer",
  "Scope": "full locked self organization payments keys parent user loggedin paid nondelinquent drive docs verified settings lumo",
  "Scopes": ["full", "locked", "self", "organization", "payments", "keys", "parent", "user", "loggedin", "paid", "nondelinquent", "drive", "docs", "verified", "settings", "lumo"],
  "UID": "<child-uid>",
  "UserID": "<user-id>",
  "AccessToken": "<child-access-token>",
  "RefreshToken": "<child-refresh-token>"
}
```

## Post-Fork Setup

After the pull, the child app establishes its session:

1. `GET /api/core/v4/users` — fetch user data
2. `PUT /api/auth/v4/sessions/local/key` — set local session key
3. `POST /api/core/v4/auth/cookies` — establish cookie-based auth
4. `GET /api/core/v4/addresses` — fetch addresses for key unlock

All post-fork calls use `x-pm-appversion` matching the service host
(e.g., `web-lumo@1.3.3.4` for `lumo.proton.me`).

## Scope Assignment

The `lumo` scope (and other service-specific scopes) is granted only
when the push uses **cookie-based auth**. Bearer token auth on the push
results in a restricted scope set that excludes service-specific scopes.

This is the critical difference between the browser flow and a CLI client:
the browser naturally uses cookies, while API clients typically use Bearer
tokens.

## Cookie Requirements

| Cookie | Domain | Push | Pull | Purpose |
|---|---|---|---|---|
| `AUTH-<uid>` | `.proton.me` | Yes | No | Parent session auth |
| `Session-Id` | `.proton.me` | Yes | Yes | Correlates push/pull |
| `iaas` | `.proton.me` | Yes | Yes | Session metadata |
| `Tag` | `.proton.me` | Yes | Yes | Session metadata |

## CLI Implementation Notes

For a CLI client to use the fork protocol:

1. The login flow must capture and persist cookies (especially `AUTH-*`
   and `Session-Id`) from the Proton API responses.
2. The fork push must send the `AUTH-<uid>=<token>` cookie instead of
   (or in addition to) the `Authorization: Bearer` header.
3. The fork pull must use a clean cookie jar with only `Session-Id`
   (no `AUTH-*` cookies).
4. Each request must use the `x-pm-appversion` matching the target host.

## References

- `WebClients.git/packages/shared/lib/authentication/fork/produce.ts` — push
- `WebClients.git/packages/shared/lib/authentication/fork/consume.ts` — pull
- `WebClients.git/packages/shared/lib/api/auth.ts` — `pushForkSession`, `pullForkSession`
- `tmp/fork-debug-findings.md` — raw debug session notes

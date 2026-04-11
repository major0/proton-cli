# Authentication / Session Management

Proton API authentication uses the SRP (Secure Remote Password) protocol for initial
login, followed by token-based session management with access/refresh token pairs.

## Authentication Flow

1. **SRP handshake** — The client performs an SRP exchange with `/auth` to prove
   knowledge of the password without transmitting it.
2. **2FA (optional)** — If the account has TOTP enabled, the client submits the code
   via `/auth/v4/2fa`.
3. **Session tokens** — On success the API returns an `AccessToken` and `RefreshToken`.
   The access token is short-lived; the refresh token is used to obtain new access
   tokens via `/auth/refresh`.

## Session Lifetime

Sessions expire relatively quickly compared to typical OAuth flows. Third-party
clients must implement automatic token refresh or risk mid-operation failures.
The rclone Proton Drive backend has encountered issues where sessions expire during
long sync operations.

## Known Issues

- **2FA timing** — TOTP codes are time-sensitive. Clock skew between the client
  machine and Proton's servers causes repeated 2FA failures. NTP sync is essential.
- **Special characters in passwords** — Some characters (particularly URL-unsafe ones)
  may not be handled correctly by all client implementations during SRP.
- **Missing signature errors** — Observed during 2FA flows in rclone; may indicate
  a mismatch between the SRP proof and the 2FA submission.

## References

- [rclone #7381 — Proton Drive session expires too quickly](https://github.com/rclone/rclone/issues/7381)
- [rclone forum — 2FA for proton drive keeps failing](https://forum.rclone.org/t/2fa-for-proton-drive-keeps-failing/52895)
- [rclone forum — Protond Drive 2fa code doesn't work](https://forum.rclone.org/t/protond-drive-2fa-code-doesnt-work/47312)
- [rclone forum — Some characters seem not to be handled in passwords](https://forum.rclone.org/t/some-characters-seem-not-to-be-handled-in-passwords/42760)
- [hydroxide auth.go](https://github.com/emersion/hydroxide/blob/master/protonmail/auth.go)

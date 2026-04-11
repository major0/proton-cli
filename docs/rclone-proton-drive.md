# rclone Proton Drive Documentation

rclone includes a Proton Drive backend (`protondrive`) that serves as the most mature
third-party integration with the Proton Drive API. Its documentation and issue tracker
are valuable sources of practical API knowledge.

## Configuration

rclone's Proton Drive backend is configured via `rclone config` and requires:

1. Proton account username and password.
2. Optional 2FA code if TOTP is enabled.
3. The account must have logged into Proton Drive via the web client at least once
   (to generate the required keyring).

## Known Limitations

- **CAPTCHA during config** — The `rclone config` flow may trigger Proton's CAPTCHA,
  which cannot be solved in a headless terminal. Workarounds involve authenticating
  via browser first or retrying.
- **Session expiry** — Long-running operations may fail if the session expires
  mid-sync. rclone handles token refresh but edge cases remain.
- **Block upload tokens** — API changes have required per-block verification tokens,
  breaking older rclone versions.

## References

- [rclone.org/protondrive](https://rclone.org/protondrive/)

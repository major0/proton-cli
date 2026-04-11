# Drive API

The Proton Drive API provides encrypted file storage and sync. It is the least
publicly documented of Proton's APIs, and most knowledge comes from reverse-engineering
official clients and the rclone Proton Drive backend.

## Core Concepts

- **Shares** — A Drive account has one or more shares. The main share is the root of
  the user's file tree.
- **Links** — Files and folders are represented as links within a share. Each link has
  encrypted metadata (name, timestamps) and, for files, encrypted block content.
- **Keyrings** — Encryption keys are organized in a hierarchy. The user's address key
  unlocks the share key, which unlocks individual link keys. A keyring must be
  generated (typically on first web login) before any API client can operate.
- **Blocks** — File content is split into blocks, each encrypted separately. Upload
  requires obtaining per-block verification tokens from the API.

## Known Issues and API Behavior

- **HTTP 422 / 400 errors** — Various operations can return these during upload or
  metadata operations, often due to stale revision IDs or missing verification tokens.
- **Block upload changes** — The API has evolved to require per-block verification
  tokens, breaking older client implementations that assumed a single upload token.
- **Keyring generation** — If the user has never logged into Proton Drive via the web
  client, no keyring exists and the API returns errors. The keyring must be generated
  through the official web interface first.
- **Sync failures** — Reported in rclone, possibly due to API changes in link
  listing or revision handling.

## References

- [rclone #7266 — Error 422's & 400's](https://github.com/rclone/rclone/issues/7266)
- [rclone forum — ProtonDrive: fix takes 100 lines](https://forum.rclone.org/t/protondrive-fix-takes-100-lines/53519)
- [rclone forum — ProtonDrive error: no keyring is generated](https://forum.rclone.org/t/protondrive-error-no-keyring-is-generated/47611)
- [rclone #8870 — Can't sync with Proton Drive](https://github.com/rclone/rclone/issues/8870)
- [rclone forum — How feasible is Proton Drive support?](https://forum.rclone.org/t/how-feasible-is-proton-drive-support/39860)
- [Proton-API-Bridge #1 — What about a README to use this project?](https://github.com/henrybear327/Proton-API-Bridge/issues/1)

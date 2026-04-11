# Official Proton Repositories

Open-source repositories maintained by Proton. These are the authoritative references
for API behavior, encryption, and client implementation patterns.

## Core Libraries

- **[ProtonMail/go-proton-api](https://github.com/ProtonMail/go-proton-api)** —
  Official Go API client library. Covers authentication, SRP, session management,
  and API error codes. The foundation for proton-bridge and rclone's Proton Drive
  backend.

- **[ProtonMail/gopenpgp](https://github.com/ProtonMail/gopenpgp)** —
  OpenPGP library used across Proton products for encryption and key management.
  Wraps the lower-level `go-crypto` library with a higher-level API.

- **[ProtonMail/proton-python-client](https://github.com/ProtonMail/proton-python-client)** —
  Official Python client. Covers authentication and basic API access.

## Applications

- **[ProtonMail/proton-bridge](https://github.com/ProtonMail/proton-bridge)** —
  Official Mail Bridge (Go). The most complete reference for human verification,
  authentication, session handling, and key management in a desktop context.

- **[ProtonMail/WebClients](https://github.com/ProtonMail/WebClients)** —
  Web client monorepo (TypeScript/React). Useful for understanding API endpoints,
  request/response shapes, and UI flows.

- **[ProtonDriveApps/android-drive](https://github.com/ProtonDriveApps/android-drive)** —
  Official Android Drive client (open source, Kotlin). Reference for Drive API
  usage on mobile.

- **[protonpass/pass-cli](https://github.com/protonpass/pass-cli)** —
  Official Proton Pass CLI. Documentation only — the binary is closed source.

## References

- [ProtonMail/go-proton-api](https://github.com/ProtonMail/go-proton-api)
- [ProtonMail/proton-bridge](https://github.com/ProtonMail/proton-bridge)
- [ProtonMail/proton-python-client](https://github.com/ProtonMail/proton-python-client)
- [ProtonMail/WebClients](https://github.com/ProtonMail/WebClients)
- [ProtonMail/gopenpgp](https://github.com/ProtonMail/gopenpgp)
- [ProtonDriveApps/android-drive](https://github.com/ProtonDriveApps/android-drive)
- [protonpass/pass-cli](https://github.com/protonpass/pass-cli)

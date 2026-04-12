# App Version String

Every request to the Proton API must include a valid application version string in the
`x-pm-appversion` header. The API enforces this strictly — requests with a missing or
malformed version are rejected.

## Error Codes

| Code | Name              | Meaning                                    |
|------|-------------------|--------------------------------------------|
| 5001 | AppVersionMissing | The `x-pm-appversion` header was not sent. |
| 5002 | AppVersionInvalid | The header value is not a recognized app.  |
| 5003 | AppVersionBad     | The header value is present but malformed. |

## Format

The version string follows the pattern:

```
<platform>-<product>@<semver>
```

Where:
- `<platform>` is the OS: `windows`, `macos`, `linux`, `android`, `ios`, `web`
- `<product>` is the Proton product: `bridge`, `drive`, `mail`, `account`, `pass`
- `<semver>` is a semantic version, optionally with pre-release/build metadata

Examples from official clients:
- `linux-bridge@3.24.1` — Proton Mail Bridge
- `windows-drive@1.20.2` — Proton Drive for Windows
- `web-account@5.0.363.1` — Proton web client (account)

## Token Scope and API Endpoints

Access tokens are scoped based on the `x-pm-appversion` header and the API endpoint
used during authentication. A token obtained with a `bridge` app version against
`mail-api.proton.me` will not have permission to access Drive endpoints, and vice versa.

| Product  | API Host                    | App Version Pattern        | Token Scope |
|----------|-----------------------------|----------------------------|-------------|
| Mail     | `mail-api.proton.me`        | `<os>-bridge@<version>`    | Mail        |
| Drive    | `drive-api.proton.me`       | `<os>-drive@<version>`     | Drive       |
| Account  | `account-api.proton.me`     | `web-account@<version>`    | Account     |

Attempting to use a token outside its scope returns error code **9100**
("Access token does not have sufficient scope").

Each API host serves all Proton API paths (`/auth/`, `/core/`, `/drive/`, `/mail/`,
etc.) but scopes the resulting session to the product identified by the app version
and host combination.

## CAPTCHA Behavior

The app version also determines whether the API requires human verification (CAPTCHA)
during authentication:

- Recognized official versions (e.g. `linux-bridge@3.24.1`) are whitelisted and
  bypass CAPTCHA entirely.
- Unrecognized or third-party versions trigger CAPTCHA (error code 9001) on
  every login attempt.
- Invalid platform-product combinations (e.g. `linux-drive@1.0.0`) are rejected
  outright with error code 5002.

## Practical Notes

- The `go-proton-api` library defines these error codes and handles version header
  injection automatically via `WithAppVersion()`.
- The version string must match a registered product. Arbitrary strings like
  `myapp@1.0.0` will be rejected with 5002.
- Pre-release and build metadata in the semver (e.g. `+proton-cli`) are accepted
  by some products but may trigger CAPTCHA.

## References

- [ProtonMail/proton-version](https://github.com/ProtonMail/proton-version)
- [go-proton-api response.go error codes](https://github.com/ProtonMail/go-proton-api)
- [ProtonDriveApps/windows-drive config](https://github.com/ProtonDriveApps/windows-drive) — `ProtonDrive.config.json`
- [ProtonMail/proton-bridge constants](https://github.com/ProtonMail/proton-bridge) — `internal/constants/`

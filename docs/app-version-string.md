# App Version String

Every request to the Proton API must include a valid application version string in the
`x-pm-appversion` header. The API enforces this strictly — requests with a missing or
malformed version are rejected.

## Error Codes

| Code | Name              | Meaning                                    |
|------|-------------------|--------------------------------------------|
| 5001 | AppVersionMissing | The `x-pm-appversion` header was not sent. |
| 5003 | AppVersionBad     | The header value is present but invalid.   |

## Format

The version string follows the pattern used by official Proton clients. The
`ProtonMail/proton-version` repository contains utilities for generating and
validating these strings. Third-party clients typically adopt a format like:

```
<ClientName>/<Major>.<Minor>.<Patch>
```

The exact format requirements are not publicly documented beyond what can be inferred
from official client source code and the `proton-version` library.

## Practical Notes

- Using a version string that mimics an official client (e.g. the web client) may
  reduce CAPTCHA triggers, but risks breakage if Proton enforces version-specific
  behavior.
- The `go-proton-api` library defines these error codes and handles version header
  injection automatically.

## References

- [ProtonMail/proton-version](https://github.com/ProtonMail/proton-version)
- [go-proton-api response.go error codes](https://github.com/ProtonMail/go-proton-api)

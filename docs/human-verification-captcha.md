# Human Verification / CAPTCHA

Proton uses a custom CAPTCHA system (replacing the earlier Google reCAPTCHA) to gate
certain API operations — most notably authentication from third-party clients.
When the API returns error code **9001**, the client must complete a human-verification
challenge before retrying the request.

## How It Works

1. The API responds with HTTP 422 and a `HumanVerification` JSON body containing
   supported verification methods (e.g. `captcha`, `sms`, `email`).
2. For the `captcha` method the client must open a browser-based flow hosted at
   `verify.proton.me` (or the equivalent for the API entry point).
3. The user solves the challenge; the page returns a verification token.
4. The client retries the original request with the token in the
   `x-pm-human-verification-token` header and the method in
   `x-pm-human-verification-token-type`.

The official Go reference implementation lives in the **proton-bridge** `internal/hv`
package.

## Key Observations

- Third-party clients (rclone, proton-cli) frequently hit CAPTCHA during initial
  login because Proton's anti-abuse system flags non-browser user agents.
- Encryption keys must already exist on the account (i.e. the user must have logged
  in via the web client at least once) before a headless client can authenticate.
- Proton's own CAPTCHA image format is documented in the `gravilk/protonmail-documented`
  repository, though the format may have changed since July 2023.

## References

- [rclone #7967 — proton-drive remote fails due to captcha](https://github.com/rclone/rclone/issues/7967)
- [rclone forum — Captcha error when using Proton Drive](https://forum.rclone.org/t/captcha-error-when-using-proton-drive/47879)
- [rclone forum — Rclone can't connect to Proton Drive (CAPTCHA errors)](https://forum.rclone.org/t/rclone-can-t-connect-to-proton-drive-captcha-errors/52302)
- [proton-bridge `internal/hv` package](https://pkg.go.dev/github.com/ProtonMail/proton-bridge/v3/internal/hv)
- [ElectronMail #419 — Human Verification broken captcha workflow](https://github.com/vladimiry/ElectronMail/issues/419)
- [gravilk/protonmail-documented — Proton CAPTCHA documented and solved](https://github.com/gravilk/protonmail-documented)
- [gravilk — ProtonMail captcha solving function (gist)](https://gist.github.com/gravilk/1299aefa9324e33d1d84e0ad23b6f3ad)
- [Proton blog — Introducing Proton CAPTCHA](https://www.proton.me/blog/proton-captcha)
- [WebClients #242 — ProtonMail includes Google Recaptcha for Login](https://github.com/ProtonMail/WebClients/issues/242)
- [HN discussion — ProtonMail includes Google Recaptcha for login](https://news.ycombinator.com/item?id=27326243)

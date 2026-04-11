# Human Verification / CAPTCHA

Proton uses a custom CAPTCHA system (replacing the earlier Google reCAPTCHA) to gate
certain API operations — most notably authentication from third-party clients.
When the API returns error code **9001**, the client must complete a human-verification
challenge before retrying the request.

## How It Works

1. The API responds with HTTP 422 and a JSON body containing error code 9001 and
   `HumanVerificationMethods` / `HumanVerificationToken` in the `Details` field.
2. The response also includes a `WebUrl` field:
   `https://verify.proton.me/?methods=captcha&token=<HumanVerificationToken>`.
3. The client opens this URL in a browser. The user solves the CAPTCHA challenge
   on Proton's servers.
4. Proton's backend records that the challenge token has been solved server-side.
5. The client retries the original request, passing the **same** `APIHVDetails`
   (challenge token + methods) via `NewClientWithLoginWithHVToken`. The backend
   recognizes the token as solved and allows the request through.

The client does **not** need to capture or intercept the solved CAPTCHA response.
The `HumanVerificationToken` is a challenge identifier, not a one-time credential.
Solving the CAPTCHA on `verify.proton.me` marks the token as verified server-side.
The retry succeeds because the backend checks the token's verification status.

This is confirmed by the proton-bridge CLI implementation (`internal/hv/hv.go` and
`internal/frontend/cli/accounts.go`), which simply prints the `verify.proton.me` URL,
waits for the user to press ENTER, and retries with the original `hvDetails` unchanged.

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

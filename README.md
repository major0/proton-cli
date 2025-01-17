# ProtonDrive CLI

_**NOTE! This project is currently in its infancy. While it is actively being worked
on, it is not remotely ready for any sort of use by "users". If you are
interested in assisting with development, or fixing any of the hideous code
decisions being made early on then, by all means, patches are welcome!**_

The [ProtonDrive CLI](https://github.com/major0/protondrive-cli) is a command
line interface for [ProtonDrive][], a secure storage solution hosted by
[ProtonMail][].

This tool is a basic CLI wrapper around the [go-proton-api][] library. While it
is intended to be useful in and of itself, it is also intended to act as a an
example of interacting with the [ProtonDrive][] API as well as a useful tool
for aiding in debugging other application which also leverage the API.

## Installation

Run `go build` and copy the compiled binary into your `$PATH`.

## Contributing

See the [CONTRIBUTING.md](CONTRIBUTING.md) file for details.

[//]: # (References)

[ProtonDrive]: https://proton.com/drive
[ProtonMail]: https://proton.com/mail
[go-proton-api]: https://github.com/ProtonMail/go-proton-api

# Proton CLI

A command line interface for [Proton][] services. Currently supports
[Proton Drive][ProtonDrive] with file management, directory operations,
and share listing.

> **Status:** Early development. The core Drive operations work but the
> tool is not yet stable. Contributions welcome.

## Features

- **Multiple accounts** — switch between Proton accounts with `--account`
- **Drive file management** — ls, find, mkdir, rmdir, rm, mv/rename
- **Volume usage** — df-style disk usage per volume
- **Share management** — list shares with type, creator, and permissions
- **CAPTCHA handling** — automatic browser-based CAPTCHA solving via chromedp
- **Encrypted-first** — all data stays encrypted in memory; lazy decryption only when displaying
- **Concurrent operations** — parallel directory traversal with rate-limit throttling

## Installation

### From source

Requires Go 1.22+ and Chrome/Chromium (for CAPTCHA).

```sh
git clone https://github.com/major0/proton-cli.git
cd proton-cli
make build
```

### Dependencies

| Platform | Packages |
|----------|----------|
| Ubuntu/Debian | `libsecret-1-dev` (for keyring) |
| Fedora/RHEL | `libsecret-devel` |
| macOS | None (uses Keychain) |

Chrome or Chromium must be installed for CAPTCHA solving during login.

## Usage

### Login

```sh
proton account login -u <username>
```

The tool uses the `windows-drive` app version for Proton Drive API access.
If CAPTCHA is triggered, a Chrome window opens automatically for solving.

### Drive commands

```sh
# List files (supports -l, -a, -F, --color, --trash, -R, -1, -x, -C)
proton drive ls
proton drive ls -lF proton://My\ files/

# Find files (Unix find compatible, single-hyphen flags)
proton drive find proton://root/ -type f -iname '*.pdf'
proton drive find -maxdepth 2 -type d -name 'src'

# Volume usage
proton drive df

# Create directories
proton drive mkdir proton://My\ files/new-folder
proton drive mkdir -p proton://My\ files/a/b/c

# Move / rename
proton drive mv proton://My\ files/old proton://My\ files/new
proton drive mv proton://src1 proton://src2 proton://dest-dir/

# Remove (moves to trash by default)
proton drive rm proton://My\ files/unwanted.txt
proton drive rm -r proton://My\ files/old-dir/
proton drive rm --permanent proton://My\ files/secret.txt

# Remove empty directories
proton drive rmdir proton://My\ files/empty-dir/

# Empty trash
proton drive empty-trash
```

### Share commands

```sh
# List all shares
proton share ls
proton share ls -F --color
```

### Account management

```sh
proton account login -u <username>
proton account logout
proton account info
proton account list
```

## Path format

Proton Drive paths use the `proton://` prefix:

```
proton://My files/Documents/report.pdf
proton://Photos/2024/
```

The first path component is the share name (e.g. "My files", "Photos").
Paths support `.` and `..` resolution.

## Architecture

```
proton-cli/
├── api/           # Core library — encrypted-first Proton API wrapper
│   ├── links.go   # Link type with lazy decryption (sync.Once)
│   ├── readdir.go  # Concurrent directory reading
│   ├── walk.go    # Tree traversal with SkipDir
│   ├── mkdir.go   # Folder creation with encryption
│   ├── move.go    # Move/rename with key re-encryption
│   ├── rm.go      # Trash and permanent delete
│   ├── stat.go    # Concurrent link resolution
│   ├── crypto.go  # Node key generation, keyring unlock
│   ├── throttle.go # Shared 429 rate-limit coordination
│   ├── session.go # Authentication, session management
│   └── share.go   # Share resolution
├── cmd/           # CLI commands (thin wrappers over api/)
│   ├── drive/     # ls, find, df, mkdir, rmdir, rm, mv
│   ├── account/   # login, logout, info, list
│   └── share/     # ls, info (stubs: invite, revoke)
├── internal/      # Session storage (keyring + index file)
└── docs/          # API documentation and research notes
```

## Design principles

- **Data stays encrypted** — raw API objects are the canonical in-memory
  representation. Decryption is lazy and last-second.
- **Index by ID** — all lookups use encrypted object IDs, never decrypted names.
- **POSIX-like API** — the `api/` package mirrors libc: stat, readdir, mkdir,
  rmdir, walk, etc.
- **Concurrent by default** — directory reads fan out decryption across workers;
  find walks sibling directories in parallel.
- **Throttle-aware** — shared rate limiter pauses all workers on 429.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

See [LICENSE](LICENSE).

[Proton]: https://proton.me
[ProtonDrive]: https://proton.me/drive
[ProtonMail]: https://proton.me/mail
[go-proton-api]: https://github.com/ProtonMail/go-proton-api

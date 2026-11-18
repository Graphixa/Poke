# Installation

[← Poke](../README.md) · [Quick start](../README.md#quick-start) · [Troubleshooting](TROUBLESHOOTING.md)

Install a prebuilt release for your system. You do not need Go unless you are building Poke from source.

## Supported release builds

| System | Architecture | Archive suffix |
| --- | --- | --- |
| Windows | x64 | `windows_amd64.zip` |
| macOS | Apple silicon | `darwin_arm64.tar.gz` |
| macOS | Intel | `darwin_amd64.tar.gz` |
| Linux | x64 | `linux_amd64.tar.gz` |
| Linux | ARM64 | `linux_arm64.tar.gz` |

See [Releases](https://github.com/Graphixa/Poke/releases/latest) for available downloads. First launch needs an internet connection to download Pokémon data.

## Windows

Run in PowerShell:

```powershell
irm https://raw.githubusercontent.com/Graphixa/Poke/main/scripts/install.ps1 | iex
```

The installer downloads the latest release, verifies the archive checksum and installs `poke.exe` in `%LocalAppData%\Programs\Poke`, plus a `pokedex.exe` alias. It adds this directory to your user `PATH`; administrator access is not required for the default location.

Open a new terminal if the command is not recognised, then check:

```powershell
poke version
```

## macOS and Linux

```bash
curl -fsSL https://raw.githubusercontent.com/Graphixa/Poke/main/scripts/install.sh | sh
```

The installer downloads the latest release, verifies its checksum and places `poke` in `~/.local/bin` by default, plus a `pokedex` symlink alias. Follow its instructions if that directory is not on your `PATH`.

For the current Bash or Zsh session, you can add the default directory with:

```bash
export PATH="$HOME/.local/bin:$PATH"
poke version
```

Add the same export to your shell configuration if you want it to persist. If you use another shell or a custom installation directory, follow that shell’s syntax and use the actual directory.

### Homebrew on macOS

```bash
brew tap Graphixa/homebrew-tap
brew install --cask poke
```

The [tap’s Poke cask](https://github.com/Graphixa/homebrew-tap/blob/main/Casks/poke.rb) installs both executable names. Use Homebrew to manage updates for this installation.

## Install manually

1. Download the matching archive and `checksums.txt` from the [same release](https://github.com/Graphixa/Poke/releases/latest).
2. Calculate the archive’s SHA-256 hash and compare it with its entry in `checksums.txt`.
3. Extract the archive and place `poke` (or `poke.exe`) in a directory on your `PATH`. Optionally add a `pokedex` symlink/hardlink to the same binary.
4. Run `poke version`, then `poke`.

Hash commands are `Get-FileHash -Algorithm SHA256 <archive>` in PowerShell, `shasum -a 256 <archive>` on macOS and `sha256sum <archive>` on Linux. Replace `<archive>` with the downloaded file’s path.

## Installer options

Download the script first if you want to inspect it or set installation options explicitly.

### macOS and Linux

```bash
curl -fsSL https://raw.githubusercontent.com/Graphixa/Poke/main/scripts/install.sh -o install-poke.sh
sh install-poke.sh --dry-run
```

`--dry-run` prints the plan without installing. For a specific release, choose an existing version from Releases and set `POKE_VERSION` for the shell that executes the script. For example:

```bash
POKE_VERSION=0.1.0 sh install-poke.sh
```

| Variable | Purpose |
| --- | --- |
| `POKE_VERSION` | Release number or `latest` (default) |
| `POKE_INSTALL_DIR` | Destination directory; defaults to `~/.local/bin` |
| `POKE_NONINTERACTIVE=1` | Skip the installer’s confirmation prompt |
| `POKE_DRY_RUN=1` | Print the plan without installing |

### Windows

Set the version before invoking the installer:

```powershell
$env:POKE_VERSION = "0.1.0"
irm https://raw.githubusercontent.com/Graphixa/Poke/main/scripts/install.ps1 | iex
Remove-Item Env:POKE_VERSION
```

Replace `0.1.0` with the release you want. `POKE_NONINTERACTIVE=1` also skips the installer’s confirmation prompt. The Windows installer uses the fixed user-local directory described above.

## Updates

For script or manual installations:

```bash
poke update --check
poke update
```

The first command checks only. The second asks before installing a newer release and verifies its checksum. For more options, see [Commands](COMMANDS.md#update-the-app).

For Homebrew installations:

```bash
brew update
brew upgrade --cask poke
```

To refresh Pokémon data and official artwork, use `poke refresh`. This is separate from updating the app.

## Build from source

Requires Go 1.26 or newer.

```bash
git clone https://github.com/Graphixa/Poke.git
cd Poke
go build -o poke ./cmd/poke
```

Run `./poke` on macOS/Linux. On Windows, build with `go build -o poke.exe ./cmd/poke` and run `.\poke.exe`.

If you want the optional alias, symlink or hardlink the executable as `pokedex` (or `pokedex.exe`). See [Contributing](../CONTRIBUTING.md) for development checks.

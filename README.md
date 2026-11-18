<p align="center">
  <img src="assets/poke-banner.png" alt="poke" width="720"/>
</p>

<p align="center">
  <a href="https://github.com/Graphixa/Poke/actions/workflows/ci.yml"><img src="https://github.com/Graphixa/Poke/actions/workflows/ci.yml/badge.svg" alt="CI"/></a>
  <a href="https://github.com/Graphixa/Poke/releases/latest"><img src="https://img.shields.io/github/v/release/Graphixa/Poke?label=release" alt="Release"/></a>
  <a href="https://github.com/Graphixa/Poke/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-GPL--3.0-blue.svg" alt="License: GPL-3.0"/></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-1.26+-00ADD8.svg" alt="Go 1.26+"/></a>
</p>

<p align="center">
  <strong>A terminal Pokédex.</strong> Browse the National Dex with a keyboard-first TUI, peek at stats from the CLI, and keep everything offline after the first refresh.
</p>

The canonical command is **`poke`**. **`pokedex`** is the same binary under another name.

## Install

### One-liner (recommended)

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/Graphixa/Poke/main/scripts/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/Graphixa/Poke/main/scripts/install.ps1 | iex
```

Pin a version with `POKE_VERSION=1.0.0` (shell) or `$env:POKE_VERSION="1.0.0"` (PowerShell). Scripts download the GitHub Release archive, verify `checksums.txt`, and install both `poke` and `pokedex`.

### Winget (Windows)

```powershell
winget install --id Graphixa.Poke
```

### Homebrew (macOS)

```bash
brew tap Graphixa/homebrew-tap
brew install poke
```

### Chocolatey (Windows)

Coming soon (`choco install poke`). Package scaffolding is in-repo; publish follows the first Chocolatey.org approval.

### From a GitHub Release

Download the archive for your OS/architecture from the [Releases](https://github.com/Graphixa/Poke/releases) page, extract it, and put the binaries on your `PATH`.

Each archive includes both names (`poke` / `pokedex`, or `.exe` on Windows). Verify with `checksums.txt` from the same release.

### From source

Requires Go 1.26+.

```bash
git clone https://github.com/Graphixa/Poke.git
cd Poke
go build -o poke ./cmd/poke
cp poke pokedex   # or: ln -s poke pokedex
```

Scoop is not supported (out of scope for v1).

## Quick start

```bash
# First run: setup wizard, then metadata bootstrap before browse opens
poke

# Or bootstrap metadata + artwork explicitly
poke refresh

# Update the poke binary from GitHub Releases
poke update
poke update --check

# Open browse with a selection / search prefill
poke pikachu

# Inline lookups (no TUI)
poke pikachu --info
poke pikachu --stats
poke pikachu --evolution
poke search char
poke random --stats

# Cache / config helpers
poke cache path
poke cache clear
poke config path
poke config init
poke config edit

# Colour themes (interactive selector with live preview)
poke theme
```

All of the above work with `pokedex` instead of `poke`.

**`poke refresh`** rebuilds the local species dataset and syncs official artwork.  
**`poke update`** installs a newer `poke` binary from GitHub Releases (checksum-verified). Use `--check` to inspect only, `-y` to skip confirmation, or `--version 1.2.3` for a specific release.

Default theme is **Dracula**. Bundled themes include Pokédex, Nord, Catppuccin, and Solarized Light; `system` uses the terminal defaults. Custom YAML themes can live next to the config file under `themes/` (FontGet-compatible keys: `primary`, `secondary`, `components`, `placeholders`, `base`, `warning`, `error`, `success`). Set `theme.name` in `config.yaml` or pick one with `poke theme`.

## Features

- **Keyboard browse** — type-to-filter list, glance pane, and sprite art in one screen
- **F2 / F3 / F4** — filters & sort, side-by-side compare, favourites
- **Dossier** — overview, stats, evolution, moves, encounters, descriptions, biology
- **Offline-first** — after `poke refresh`, browse and inline lookups work from a local snapshot
- **Sprite smarts** — Kitty / iTerm2 / Sixel when the terminal can; otherwise half-block cell art
- **Self-update** — `poke update` pulls the latest GitHub Release

## Browse keyboard reference

Typing always filters the list (including letters like `q` and `?`). There is no separate help overlay — the footer shows the live bindings.

| Key | Action |
| --- | --- |
| Type | Filter the list within the active view options |
| Backspace | Delete from the search query |
| ↑ / ↓ | Move selection |
| Enter | Open the Pokémon dossier |
| Tab | Cycle browse view |
| F2 | Filter by collection / type; change sorting |
| F3 | Pin selected Pokémon, then compare another (same Pokémon unpins) |
| F4 | Add/remove selected Pokémon from favourites |
| Esc | Clear search/filters, or quit when the query is empty |
| Ctrl+C | Always quit |

### In the dossier

| Key | Action |
| --- | --- |
| ← / → | Change section |
| ↑ / ↓ | Scroll (or move in lists) |
| PgUp / PgDn | Page scroll |
| Home / End | Jump |
| Enter | Open move detail (on Moves) |
| Esc | Close dossier (or back / cancel a picker) |
| g | Game picker (where offered) |

Sprites need roughly **110×40** terminal cells for the fixed sprite frame; smaller terminals hide the sprite and keep browse usable. Below about **40×12**, the UI shows a “terminal too small” message.

## Config and cache paths

Resolved via the OS user config/cache directories (`os.UserConfigDir` / `os.UserCacheDir`), under an application folder named `pokedex`:

| | Typical location |
| --- | --- |
| Config file | `$XDG_CONFIG_HOME/pokedex/config.yaml` on Linux; `~/Library/Application Support/pokedex/config.yaml` on macOS; `%AppData%\pokedex\config.yaml` on Windows |
| Cache directory | `$XDG_CACHE_HOME/pokedex` on Linux; `~/Library/Caches/pokedex` on macOS; `%LocalAppData%\pokedex` on Windows |

```bash
poke config path
poke cache path
```

`poke config init` writes defaults without overwriting an existing file. `poke config edit` opens the file with `$VISUAL` or `$EDITOR`.

## Offline behaviour

After a successful first-run **metadata bootstrap**, browse and inline commands work from a local dataset snapshot. Ordinary navigation does not hit the network for metadata.

Official artwork downloads in the background after the TUI opens (and can resume on later launches). Derived sixel/chafa/halfblock frames are generated once per render size and cached.

| Situation | Behaviour |
| --- | --- |
| Dataset present | TUI starts immediately; no metadata network required |
| Empty cache + online | First run / `poke refresh` builds a complete local dataset, then activates it atomically |
| Empty cache + offline | Hard failure with a clear remediation (`poke refresh` while online) |
| Failed refresh | Previous snapshot stays active |
| Artwork still downloading | List and text stay usable; selected missing art is prioritised |

`poke refresh` rebuilds a new dataset snapshot beside the active one, switches `active.json` only if the snapshot is complete, and syncs official artwork.

Renderer fallback: Kitty / iTerm2 native → Sixel → Chafa (optional binary, else in-process) → half-block → placeholder.

## Troubleshooting

| Problem | What to try |
| --- | --- |
| Empty cache / offline error | Connect to the network and run `poke refresh` |
| Stale or odd data | `poke refresh`, or `poke cache clear` then `poke refresh` |
| Corrupt cache entry | App quarantines bad files under `corrupt/` and rebuilds; clear cache if needed |
| Rate limited (HTTP 429) | Wait and retry; client backs off automatically |
| Not found (inline flags) | Check spelling/slug/number; inline mode exits non-zero |
| No sprite | Expected while art is still downloading, sprites are disabled, the terminal is small, or the image failed — browse still works |
| Terminal too small | Resize to at least 40×12 |
| Binary update failed | `poke update --check`; retry `poke update -y`; or reinstall via the install script / Winget |
| Debug diagnostics | `poke --debug` (optional path) writes a log file; path is printed on stderr, not into the TUI |

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup. Short version:

```bash
make check          # fmt-check, vet, staticcheck, gosec, govulncheck, race, cross-build
make release-check  # goreleaser check + snapshot + archive name verification
```

On Windows without `bash` on `PATH`:

```powershell
pwsh -File scripts/verify-release-archives.ps1
```

CI runs format, vet, staticcheck, gosec, govulncheck, race tests, cross-build smoke, and a Goreleaser snapshot archive check on every push/PR. Pushing a version tag (`v*`) runs GoReleaser, publishes GitHub Release assets with checksums, updates the Homebrew tap, and opens a Winget PR. Chocolatey packs from a separate Windows workflow after the release.

Release notes are grouped from conventional commit subjects (`feat`, `fix`, `docs`, …).

## Credits

Powered by [PokeAPI](https://pokeapi.co/). Official artwork is downloaded into your local cache on demand — it is not redistributed inside the `poke` binary.

Pokémon and Pokédex are trademarks of Nintendo / Creatures Inc. / GAME FREAK. This project is unofficial and not affiliated with them.

Terminal fireworks animation adapted from [erik-adelbert/firework](https://github.com/erik-adelbert/firework) (MIT). See [`internal/fireworks/LICENSE.MIT`](internal/fireworks/LICENSE.MIT).

Licensed under [GPL-3.0](LICENSE).

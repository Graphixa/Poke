# Pokedex (`poke`)

Cross-platform terminal Pokédex powered by [PokeAPI](https://pokeapi.co/). Browse with a keyboard-first TUI, or look up stats and evolution chains from the command line.

The canonical command is **`poke`**. **`pokedex`** is an equivalent alias (same flags and subcommands).

## Install

### From a GitHub Release

Download the archive for your OS/architecture from the [Releases](https://github.com/Graphixa/Pokedex/releases) page, extract it, and put the binaries on your `PATH`.

Each archive includes both names:

| Platform | Executables |
| --- | --- |
| Linux / macOS | `poke` and `pokedex` (identical binaries; a Homebrew tap may later install `pokedex` as a symlink) |
| Windows | `poke.exe` and `pokedex.exe` |

Verify the download with `checksums.txt` from the same release.

```bash
# Example after extracting a Linux/macOS archive
./poke version
./pokedex version
```

### From source

Requires Go 1.25+.

```bash
git clone https://github.com/Graphixa/Pokedex.git
cd Pokedex
go build -o poke ./cmd/poke
# optional alias name
cp poke pokedex   # or: ln -s poke pokedex
```

### Homebrew / Scoop

- **Homebrew tap:** planned immediately after the first tagged GitHub Release. Not part of the v1 packaging gate.
- **Scoop:** not supported (out of scope for v1).

## Quick start

```bash
# First run: bootstrap the local species index (needs network)
poke update

# Browse TUI
poke

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
poke cache refresh   # same as: poke update
poke cache clear
poke config path
poke config init
poke config edit

# Colour themes (interactive selector with live preview)
poke theme
```

All of the above work with `pokedex` instead of `poke`.

Default theme is **Catppuccin**. Bundled themes include Nord, Dracula, and Solarized Light; `system` uses the terminal defaults. Custom YAML themes can be placed next to the config file under `themes/` (FontGet-compatible keys: `primary`, `secondary`, `components`, `placeholders`, `base`, `warning`, `error`, `success`). Set `theme.name` in `config.yaml` or pick one with `poke theme`.

## Browse keyboard reference

Typing always filters the list (including the letter `q`). Press `?` in browse mode for the in-app help card.

| Key | Action |
| --- | --- |
| Type | Filter the list |
| Backspace | Delete from search |
| Esc | Clear search (or close help) |
| ↑ / ↓ | Move selection |
| Tab / Shift+Tab | Cycle detail tabs |
| PgUp / PgDn | Scroll the detail panel |
| Home / End | Jump in the detail panel |
| q | Quit **only when search is empty** |
| Ctrl+C | Always quit |
| ? | Toggle help |

Unbound in v1: Enter, `r`, `/`.

Sprites need roughly **100×30** terminal cells; smaller terminals hide the sprite and keep browse usable. Below about **40×12**, the UI shows a “terminal too small” message.

## Config and cache paths

Resolved via the OS user config/cache directories (Go `os.UserConfigDir` / `os.UserCacheDir`), under an application folder named `pokedex`:

| | Typical location |
| --- | --- |
| Config file | `$XDG_CONFIG_HOME/pokedex/config.yaml` on Linux; `~/Library/Application Support/pokedex/config.yaml` on macOS; `%AppData%\pokedex\config.yaml` on Windows |
| Cache directory | `$XDG_CACHE_HOME/pokedex` on Linux; `~/Library/Caches/pokedex` on macOS; `%LocalAppData%\pokedex` on Windows |

Print the resolved paths:

```bash
poke config path
poke cache path
```

`poke config init` writes defaults without overwriting an existing file. `poke config edit` opens the file with `$VISUAL` or `$EDITOR`.

## Offline behaviour

Data is **cache-first**:

| Situation | Behaviour |
| --- | --- |
| Fresh / warm cache | Browse and inline commands work immediately; no network required for cached data |
| Stale cache | Usable offline; may background-refresh when online and configured |
| Empty cache + online | Bootstrap via `poke update` (or first launch) |
| Empty cache + offline | Hard failure with a clear remediation (`poke update` while online) |
| Tab never fetched while offline | Clear unavailable/offline message — not a panic |

`poke update` / `poke cache refresh` refresh the species index and enrichment; they do not download every encounter or sprite up front. Detail and sprites load on demand and are cached locally.

## Troubleshooting

| Problem | What to try |
| --- | --- |
| Empty cache / offline error | Connect to the network and run `poke update` |
| Stale or odd data | `poke cache refresh`, or `poke cache clear` then `poke update` |
| Corrupt cache entry | App quarantines bad files under `corrupt/` and rebuilds; clear cache if needed |
| Rate limited (HTTP 429) | Wait and retry; client backs off automatically |
| Not found (inline flags) | Check spelling/slug/number; inline mode exits non-zero |
| No sprite / `[ Name ]` placeholder | Expected when sprites are disabled, terminal is small, colour is limited, or the image failed — browse still works |
| Terminal too small | Resize to at least 40×12 |
| Debug diagnostics | `poke --debug` (optional path) writes a log file; path is printed on stderr, not into the TUI |

## Screenshots

Representative browse screenshots are not checked into the repository yet. Until they are added, use `tui-example.md` for the canonical wireframe of browse mode, or run `poke` locally after `poke update`.

## Development

```bash
make check          # fmt-check, vet, race tests, cross-build
make release-check  # goreleaser check + snapshot + archive name verification
```

On Windows without `bash` on `PATH`, archive verification can use:

```powershell
pwsh -File scripts/verify-release-archives.ps1
```

CI runs format, vet, race tests, cross-build smoke, and a Goreleaser snapshot archive check on every push/PR. Pushing a version tag (`v*`) runs GoReleaser and publishes GitHub Release assets with checksums.

Release notes are grouped from conventional commit subjects (`feat`, `fix`, `docs`, …).

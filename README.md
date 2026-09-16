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

Requires Go 1.26+.

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
# First run: setup wizard, then metadata bootstrap completes before browse opens
poke

# Or bootstrap metadata + artwork explicitly first
poke update

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

# Clear cache and config to re-run first-run setup
poke reset

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
| Tab / Shift+Tab | Cycle detail dialog sections (Enter/i) |
| PgUp / PgDn | Scroll the detail panel |
| Home / End | Jump in the detail panel |
| Enter / i | Open Pokémon details dialog |
| q | Quit **only when search is empty** |
| Ctrl+C | Always quit |
| ? | Toggle help |

Sprites need roughly **110×40** terminal cells for the fixed 24×24 sprite frame; smaller terminals hide the sprite and keep browse usable. Below about **40×12**, the UI shows a “terminal too small” message. Capable terminals (Kitty / iTerm2 / Sixel, including Windows Terminal) may show the PNG natively; others use coloured half-block cell art.

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

After a successful first-run **metadata bootstrap**, browse and inline commands work from a local dataset snapshot. Ordinary navigation does not refresh metadata over the network.

Official artwork downloads in the background after the TUI opens (and can resume on later launches). Derived sixel/chafa/halfblock frames are generated once per render size and cached.

| Situation | Behaviour |
| --- | --- |
| Dataset present | TUI starts immediately; no metadata network required |
| Empty cache + online | First run / `poke update` builds a complete local dataset (blocking), then activates it atomically before browse |
| Empty cache + offline | Hard failure with a clear remediation (`poke update` while online) |
| Failed update | Previous snapshot stays active |
| Artwork still downloading | List and text stay usable; selected missing art is prioritised |

`poke update` / `poke cache refresh` rebuild a new dataset snapshot beside the active one, then switch `active.json` only if the snapshot is complete. They also sync official artwork.

### Renderer fallback

```
Kitty / iTerm2 native → Sixel → Chafa (optional binary, else in-process) → half-block → placeholder
```

Sixel and chafa output are cached. A warm sixel/chafa paint reads the derived artefact without decoding the original PNG.

Braille and ASCII modes are no longer in the production path; old config values map to the text/half-block fallback.

## Troubleshooting

| Problem | What to try |
| --- | --- |
| Empty cache / offline error | Connect to the network and run `poke update` |
| Stale or odd data | `poke cache refresh`, or `poke cache clear` then `poke update` |
| Corrupt cache entry | App quarantines bad files under `corrupt/` and rebuilds; clear cache if needed |
| Rate limited (HTTP 429) | Wait and retry; client backs off automatically |
| Not found (inline flags) | Check spelling/slug/number; inline mode exits non-zero |
| No sprite | Expected while the image pane is blank during scroll debounce, when sprites are disabled, the terminal is small, or the image failed — browse still works |
| Terminal too small | Resize to at least 40×12 |
| Debug diagnostics | `poke --debug` (optional path) writes a log file; path is printed on stderr, not into the TUI |

## Screenshots

Representative browse screenshots are not checked into the repository yet. Until they are added, use `tui-example.md` for the canonical wireframe of browse mode, or run `poke` locally after `poke update`.

## Development

```bash
make check          # fmt-check, vet, staticcheck, gosec, race, cross-build
make release-check  # goreleaser check + snapshot + archive name verification
```

On Windows without `bash` on `PATH`, archive verification can use:

```powershell
pwsh -File scripts/verify-release-archives.ps1
```

CI runs format, vet, staticcheck, gosec, race tests, cross-build smoke, and a Goreleaser snapshot archive check on every push/PR. Pushing a version tag (`v*`) runs GoReleaser and publishes GitHub Release assets with checksums.

Release notes are grouped from conventional commit subjects (`feat`, `fix`, `docs`, …).

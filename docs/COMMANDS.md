# Commands

[← Poke](../README.md) · [User guide](USER_GUIDE.md) · [Configuration](CONFIGURATION.md) · [Troubleshooting](TROUBLESHOOTING.md)

The main command is `poke`. Installers also create a `pokedex` alias (same program) on your `PATH`.

Run `poke --help` for built-in help, or add `--help` to a command, such as `poke update --help`.

## Browse

| Command | What it does |
| --- | --- |
| `poke` | Open the interactive Pokédex |
| `poke --setup` | Run first-run setup again, then open browse |
| `poke pikachu` | Open browse with Pikachu selected |
| `poke 25` | Select a Pokémon by National Dex number |
| `poke char` | Use text without an exact match as a browse search filter |
| `poke random` | Open browse with a randomly selected Pokémon |

See the [user guide](USER_GUIDE.md#keyboard-reference) for keyboard controls.

## Look up a Pokémon

These commands print information and exit without opening the interactive browser:

```bash
poke pikachu --info
poke pikachu --stats
poke eevee --evolution
```

| Flag | Output |
| --- | --- |
| `--info` | Compact Pokémon information |
| `--stats` | Base stats |
| `--evolution` | Evolution tree |

Use one of these flags at a time. A name or number is required, except when using `random`:

```bash
poke 25 --stats
poke random --info
poke random --stats
poke random --evolution
```

## Search

```bash
poke search char
```

Print matching entries from the species index. The command takes one query; quote it if it contains spaces.

## Themes

```bash
poke theme
```

Open the theme selector with live preview. Use **↑ / ↓** to preview, **Enter** to save and **Esc** to cancel. See [Themes](THEMES.md) for custom palettes.

## Refresh Pokémon data

```bash
poke refresh
poke refresh --pokeapi
```

`poke refresh` downloads the dataset snapshot from this repository when it has changed, then syncs official artwork. A matching snapshot skips the data download. If the download fails, the current snapshot stays.

`--pokeapi` rebuilds the dataset from PokeAPI. That is the slow path. A failed rebuild leaves the previous complete snapshot in place.

This command updates Pokémon data; it does not install a newer app version. Artwork rendering caches are prepared separately while browsing.

## Update the app

For installations managed by Homebrew, use [Homebrew’s update command](INSTALLATION.md#updates). For script or manual installations:

| Command | What it does |
| --- | --- |
| `poke update` | Check for a newer release and ask before installing |
| `poke update --check` | Check without installing; also accepts `-c` |
| `poke update --yes` | Install a newer release without the usual confirmation; also accepts `-y` |
| `poke update --version <version>` | Install a specific published version; replace `<version>` with a release number |

Downloads are checksum-verified. A downgrade still requires confirmation, even with `--yes`.

Use `--check` on its own: combining it with `--version` selects the version-installation path, rather than a check-only operation.

## Configuration

| Command | What it does |
| --- | --- |
| `poke config path` | Print the configuration file’s location |
| `poke config init` | Create a default configuration if one does not exist |
| `poke config edit` | Create the file if needed, then open it in your editor |

The editor is selected from `VISUAL`, then `EDITOR`, then the platform default. See [Configuration](CONFIGURATION.md) for settings and examples.

## Cache

| Command | What it does |
| --- | --- |
| `poke cache path` | Print the cache directory’s location |
| `poke cache clear` | Delete Poke’s application cache |

Clearing the cache removes downloaded data and artwork. The next `poke` needs an internet connection and downloads them again. Try `poke refresh` first when the data looks stale; that updates the dataset without deleting the cache.

## Version and diagnostics

```bash
poke version
poke --version
```

Both print the build version. For diagnostic logging:

```bash
poke --debug
poke --debug=./poke-debug.log
```

Without a path, Poke uses its default temporary log location and prints the location to standard error. Use `--debug=<path>` to provide an explicit path. The flag is available on subcommands too:

```bash
poke refresh --debug=./poke-refresh.log
```

See [Troubleshooting](TROUBLESHOOTING.md#report-a-problem) for what to include in a bug report.

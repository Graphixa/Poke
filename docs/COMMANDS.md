# Commands

[← Poke](../README.md) · [User guide](USER_GUIDE.md) · [Configuration](CONFIGURATION.md) · [Troubleshooting](TROUBLESHOOTING.md)

The main command is `poke`. Release archives also include `pokedex`, an equivalent alias with the same commands and flags.

Run `poke --help` for built-in help, or add `--help` to a command, such as `poke update --help`.

## Browse

| Command | What it does |
| --- | --- |
| `poke` | Open the interactive Pokédex |
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
```

Rebuild the local Pokémon dataset and sync official artwork. This needs an internet connection and may take time on the first run. If the dataset rebuild fails, the previous complete snapshot remains available.

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

Clearing the cache removes downloaded data and artwork. You will need an internet connection and `poke refresh` before using that data offline again. Try `poke refresh` first when resolving stale data.

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

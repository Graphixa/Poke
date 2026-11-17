# Configuration

[← Poke](../README.md) · [User guide](USER_GUIDE.md) · [Commands](COMMANDS.md) · [Themes](THEMES.md)

Poke works with its defaults. Use `poke theme` for a quick change of appearance, or edit `config.yaml` for more control.

## Open your settings

```bash
poke config edit
```

This creates a default file if needed and opens it in your editor. Poke uses `VISUAL`, then `EDITOR`; the fallback is Notepad on Windows and `vi` on macOS/Linux.

To locate the file or create it without opening an editor:

```bash
poke config path
poke config init
```

`init` never overwrites an existing file. Save your edits and restart Poke to load them. YAML uses spaces for indentation, and setting names are case-sensitive.

## Everyday settings

Change the matching entries in your existing file. This example selects Nord and keeps automatic artwork rendering:

```yaml
theme:
  name: nord

render:
  sprite_mode: auto
  sprite_source: artwork
  color: auto
```

Missing settings use defaults. Avoid duplicating a section such as `render:` when editing an existing file.

| Setting | Default | Options or purpose |
| --- | --- | --- |
| `theme.name` | `dracula` | `dracula`, `pokedex`, `nord`, `catppuccin`, `solarized-light`, `system`, or a custom theme filename without `.yaml` |
| `render.sprite_mode` | `auto` | `auto`, `image`, `chafa`, `off`; keep `auto` for terminal detection |
| `render.sprite_source` | `artwork` | `artwork` or `pixel` |
| `render.color` | `auto` | `auto`, `truecolor`, `256`, `mono` |
| `render.sixel_colors` | `64` | Sixel palette size: `64`, `128` or `256` |

Set `render.sprite_mode: off` to hide Pokémon images. If native images are not displaying correctly, try `chafa` for text-based rendering. Available image output still depends on your terminal’s capabilities.

Custom theme files live in a `themes` folder beside `config.yaml`. See [Themes](THEMES.md#create-a-custom-theme) for a complete example.

## Artwork downloads

The first-run wizard records your artwork choice in the `setup` section. To allow artwork to download in the background after setup, update the existing entries to:

```yaml
setup:
  completed: true
  artwork_consent: accepted
  artwork_mode: background
```

| Setting | Allowed values | Meaning |
| --- | --- | --- |
| `setup.completed` | `true`, `false` | Whether the first-run wizard has finished |
| `setup.artwork_consent` | `none`, `accepted`, `declined` | The choice recorded by setup |
| `setup.artwork_mode` | `off`, `blocking`, `background` | Skip bulk artwork download, download during setup, or download while browsing |

To repeat setup, set `setup.completed: false` and run `poke`. For a manual dataset and artwork sync, run `poke refresh`. That explicit command syncs official artwork regardless of the automatic background-download setting.

These settings control bulk artwork downloading. Use `render.sprite_mode: off` if your goal is to hide images.

## Network settings

The defaults suit most connections. If downloads repeatedly time out, try increasing the timeout before changing other settings:

```yaml
network:
  timeout: 30s
  retries: 3
  max_concurrency: 8
  user_agent: poke-cli
```

| Setting | Default | Requirements |
| --- | --- | --- |
| `network.timeout` | `10s` | Positive duration, such as `30s` or `1m` |
| `network.retries` | `3` | Zero or greater |
| `network.max_concurrency` | `8` | Greater than zero; lower values reduce simultaneous requests |
| `network.user_agent` | `poke-cli` | Non-empty text |

## Background preparation

Poke prepares nearby data and rendered images to make navigation smoother.

| Setting | Default | Allowed values |
| --- | --- | --- |
| `cache.dossier_prewarm` | `window` | `window`, `all`, `off` |
| `render.sprite_prewarm` | `window` | `window`, `all_pixel`, `off` |

Keep `window` for the normal balance between responsiveness and background work. Use `off` to disable the corresponding preparation.

Generated configurations also contain `cache.auto_refresh` (default `true`) and `cache.index_ttl` (default `24h`). These are cache-policy fields, not a promise that ordinary browsing refreshes the active dataset every day. Use `poke refresh` when you want to refresh Pokémon data. `cache.index_ttl` must be a positive duration.

## App update settings

The update section uses capitalised keys:

```yaml
Update:
  CheckForUpdates: false
  UpdateCheckInterval: 24
  LastUpdateCheck: ""
  NextUpdateCheck: ""
```

Set `CheckForUpdates: true` to enable background release checks at startup. A newer version produces a notice; it is not installed automatically. `UpdateCheckInterval` is in hours and must not be negative. The timestamp fields are maintained by Poke; normally leave them alone. Manual `poke update --check` and `poke update` are available independently of automatic-check preferences. These settings concern app releases, not Pokémon data.

## File locations

Use `poke config path` and `poke cache path` for the exact paths on your machine. Poke keeps the directory name `pokedex` for compatibility.

| Platform | Configuration | Cache |
| --- | --- | --- |
| Windows | `%AppData%\pokedex\config.yaml` | `%LocalAppData%\pokedex` |
| macOS | `~/Library/Application Support/pokedex/config.yaml` | `~/Library/Caches/pokedex` |
| Linux | `$XDG_CONFIG_HOME/pokedex/config.yaml`, or `~/.config/pokedex/config.yaml` | `$XDG_CACHE_HOME/pokedex`, or `~/.cache/pokedex` |

## Fix an invalid configuration

If Poke reports a YAML or validation error, check indentation, spelling and the allowed values above. Unknown setting names are ignored, so a typo may have no effect.

To start again, close Poke, make a backup of `config.yaml`, move the original aside and run `poke config init`. This recreates defaults and resets the setup state. It does not require clearing the cache.

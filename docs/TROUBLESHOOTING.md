# Troubleshooting

[← Poke](../README.md) · [Installation](INSTALLATION.md) · [Commands](COMMANDS.md) · [Configuration](CONFIGURATION.md)

Start with the symptom below. Most data problems can be resolved with a refresh; clearing the cache is a later step.

## Poke is not recognised after installation

Open a new terminal and run `poke version`.

If it is still not found, confirm the installation directory is on your `PATH`. The script defaults are `%LocalAppData%\Programs\Poke` on Windows and `~/.local/bin` on macOS/Linux. Homebrew uses its own installation paths.

See the [installation guide](INSTALLATION.md) for instructions. You can also run the executable directly from its installation directory.

## First launch fails or the dataset is missing

Connect to the internet, then run `poke`. The first launch downloads the Pokémon data and artwork. `poke refresh` does the same download when you want to rebuild the dataset without the setup questions.

An empty cache cannot provide offline browsing. Once the dataset is present, ordinary browsing and inline lookups use the local snapshot.

If a refresh fails, the previous complete dataset remains available. Try again when your connection is stable.

## Pokémon data looks outdated or incomplete

Run `poke refresh` first. This rebuilds Pokémon data and syncs official artwork.

If refresh does not resolve a persistent cache problem, close other running Poke sessions and rebuild the cache:

```bash
poke cache clear
poke refresh
```

Clearing the cache deletes downloaded data and artwork. Do this while online; Poke will need to download them again before offline use. It does not repair an invalid configuration file.

## Artwork is missing

- Enlarge the terminal to roughly **110 columns × 40 rows**. Smaller windows hide the artwork panel.
- Check that `render.sprite_mode` is not `off` in your [configuration](CONFIGURATION.md#everyday-settings).
- Wait for the artwork download from setup or `poke refresh` to finish.
- Run `poke refresh` while online to sync official artwork, especially before going offline.

Brief gaps while moving quickly through the list can occur while a new image is being prepared. If an image is unavailable, browsing text should still work.

## Images or colours look wrong

Keep `render.sprite_mode: auto` and `render.color: auto` for normal use. Poke chooses output based on what your terminal supports, including native graphics or text-based fallbacks.

If native images are not working, try this in the existing `render` section of `config.yaml`:

```yaml
render:
  sprite_mode: chafa
  color: auto
```

Save, then restart Poke. The `chafa` path can use an installed Chafa executable or an in-process fallback. Native graphics support varies by terminal; a text-rendered image is a supported fallback.

For colour problems, try `poke theme` and select System, or try `render.color: "256"`. See [Themes](THEMES.md) for custom palette problems.

## The terminal is too small

Resize to at least approximately **40 columns × 12 rows**. For browsing with artwork, aim for **110 × 40** or larger. Reducing your terminal’s font size can make more rows and columns available.

## Function keys do not work

Try holding **Fn** with **F2**, **F3** or **F4**. Also check whether your terminal or operating system reserves those shortcuts.

In browse mode, letters are search input. `q` does not quit, and `?` does not open a help overlay. The footer shows the current controls; use **Esc** to clear search/filters and then exit, or **Ctrl+C** to quit.

## A Pokémon is not found

Check the name or try its National Dex number:

```bash
poke search pika
poke 25 --info
```

Use `poke search` to find the spelling used by the dataset. If expected entries are missing, refresh the dataset. Inline lookups return an error when the target cannot be resolved.

## Downloads time out or return HTTP 429

For HTTP 429, wait before trying again; the service is rate limiting requests. Poke retries requests, but repeated manual retries can add to the load.

For recurring timeouts, check your connection and try increasing `network.timeout`, for example to `30s`. You can also lower `network.max_concurrency` to reduce simultaneous requests. See [Network settings](CONFIGURATION.md#network-settings).

## An app update fails

Run `poke update --check` to see whether Poke can reach the release service. Close other Poke sessions before retrying the update.

If the executable cannot be replaced, reinstall using the original installation method. For a Homebrew installation, use `brew update` and `brew upgrade --cask poke`.

If checksum verification fails, retry the download or use the files and checksums from the same GitHub release. Do not bypass the checksum check. `--yes` skips confirmation; it does not fix download, permission or verification errors.

## Configuration will not load

Check YAML indentation and setting values. To restore defaults while retaining your old settings for reference, follow [Fix an invalid configuration](CONFIGURATION.md#fix-an-invalid-configuration).

## Report a problem

If you can reproduce the issue, [open a GitHub issue](https://github.com/Graphixa/Poke/issues/new) with:

- The output of `poke version`.
- Your operating system and terminal application.
- The command or keyboard steps that reproduce it.
- What you expected and what happened, including the exact error message.

For additional diagnostics:

```bash
poke --debug=./poke-debug.log
```

For a refresh problem, use `poke refresh --debug=./poke-refresh.log`. Review logs for personal paths or other private details before sharing them. A screenshot is useful for layout or rendering problems.

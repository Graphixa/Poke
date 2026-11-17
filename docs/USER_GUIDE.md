# User guide

[← Poke](../README.md) · [Commands](COMMANDS.md) · [Configuration](CONFIGURATION.md) · [Troubleshooting](TROUBLESHOOTING.md)

Poke brings the National Dex to your terminal. Browse with your keyboard, open detailed profiles and compare Pokémon without leaving the app.

## First launch

After [installing Poke](INSTALLATION.md), run:

```bash
poke
```

The first launch needs an internet connection to build the local Pokémon dataset. Wait for this step to finish before browsing. The setup wizard then offers an optional download of official artwork; for larger downloads, it offers a background option so you can start browsing sooner.

You can also prepare the dataset and sync official artwork explicitly:

```bash
poke refresh
```

Once the dataset is ready, ordinary browsing and command-line lookups work offline. Artwork that has not yet downloaded will still need a connection. If you plan to go offline, finish downloading the artwork you want first.

## Find a Pokémon

Start typing to filter the list. Use **↑ / ↓** to select a Pokémon and **Enter** to open its detailed profile, called the dossier.

You can also start with a name or National Dex number:

```bash
poke pikachu
poke 25
```

An exact match selects that Pokémon. Other text becomes a search filter. Browse search is always active: typing `q` or `?` searches rather than quitting or opening help.

## Explore the dossier

Use **← / →** to move through sections including the overview, stats, evolution, moves, encounters, descriptions and biology. Use **↑ / ↓** to scroll or move through a list.

In Moves, **Enter** opens the selected move’s details. Where a game picker is offered, press **g** to choose the game context. Available information depends on the Pokémon and the underlying data.

Press **Esc** to go back or close the dossier. Follow the on-screen controls when a picker or detail view is open.

## Filter and sort

Press **F2** in browse mode to open view options. Filter by collection or Pokémon type and choose how to sort the list. Typing continues to search within the active view options.

Press **Esc** in browse mode to clear the search and active filters. With those already clear, **Esc** exits Poke.

## Compare Pokémon

1. Select the first Pokémon and press **F3** to pin it.
2. Move to another Pokémon to compare the pair.
3. Press **F3** on the pinned Pokémon again to unpin it.

The comparison view lets you inspect two Pokémon side by side.

## Keep favourites

Press **F4** to add or remove the selected Pokémon from your favourites. Open **F2** and use the favourites filter to browse that collection.

## Keyboard reference

### Browse

| Key | Action |
| --- | --- |
| Type | Search within the active view options |
| Backspace | Remove a character from the search |
| ↑ / ↓ | Move through the list |
| Enter | Open the selected Pokémon’s dossier |
| Tab | Cycle browse view |
| F2 | Open filters and sorting |
| F3 | Pin a Pokémon for comparison; press on the same Pokémon to unpin |
| F4 | Toggle favourite |
| Esc | Clear search and filters; quit if they are already clear |
| Ctrl+C | Quit |

### Dossier

| Key | Action |
| --- | --- |
| ← / → | Change section |
| ↑ / ↓ | Scroll or move through a list |
| PgUp / PgDn | Scroll a page |
| Home / End | Jump through scrollable content |
| Enter | Open the selected move’s details in Moves |
| g | Open the game picker where available |
| Esc | Close the dossier, go back or cancel a picker |

The footer shows the controls available in your current view. On keyboards with media keys, you may need **Fn** with the function keys.

## Make it yours

Run `poke theme` to preview and choose a colour theme. See [Themes](THEMES.md) for the bundled palettes and custom themes, or [Configuration](CONFIGURATION.md) for artwork and display preferences.

For comfortable browsing with artwork, aim for a terminal around **110 columns × 40 rows** or larger. Smaller windows hide the artwork to keep browsing usable. The minimum is approximately **40 × 12**.

## Keep Poke current

Use `poke refresh` to refresh Pokémon data and sync official artwork. Use `poke update` to update the app installed by the install script or a manual download. For a Homebrew installation, use Homebrew to update it.

See [Installation and updates](INSTALLATION.md#updates) for details, or [Troubleshooting](TROUBLESHOOTING.md) if something is not working.

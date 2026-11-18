<p align="center">
  <img src="assets/poke-banner.png" alt="Poke — a terminal Pokédex" width="600">
</p>

<h3 align="center">Your Pokédex. In your terminal.</h3>

<p align="center">
  Explore Pokémon, compare stats and keep your favourites close.<br>
  All from your keyboard. On Windows, macOS and Linux.
</p>

<p align="center">
  <a href="https://github.com/Graphixa/Poke/releases/latest"><img src="https://img.shields.io/github/v/release/Graphixa/Poke?style=flat&amp;label=release&amp;color=ff4061" alt="Latest release"></a>
  <a href="https://github.com/Graphixa/Poke/actions/workflows/ci.yml"><img src="https://github.com/Graphixa/Poke/actions/workflows/ci.yml/badge.svg" alt="Build status"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/licence-GPL--3.0-8be9fd?style=flat" alt="Licence: GPL-3.0"></a>
</p>

<p align="center">
  <a href="#installation">Install</a> ·
  <a href="#quick-start">Get started</a> ·
  <a href="docs/USER_GUIDE.md">User guide</a> ·
  <a href="https://github.com/Graphixa/Poke/releases">Release notes</a>
</p>

## Features

- **Explore the National Dex.** Search as you type, filter by type and open detailed Pokémon profiles.
- **Compare your next pick.** See Pokémon side by side, with stats, moves and evolution information at hand.
- **Keep your favourites.** Save Pokémon you want to return to and browse your own collection.
- **Take it offline.** Browse and look up Pokémon from a local dataset after the first download. Artwork is available offline once cached.

<p align="center">
  <img src="assets/screenshots/main-screen.png" alt="Poke browse screen showing Bulbasaur with stats and abilities" width="900">
</p>

## Installation

Install Poke with one of the commands below. Poke is a single binary with no external dependencies.

#### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/Graphixa/Poke/main/scripts/install.ps1 | iex
```

#### macOS & Linux (Shell)

```bash
curl -fsSL https://raw.githubusercontent.com/Graphixa/Poke/main/scripts/install.sh | sh
```

The installers download the latest release and verify its checksum. Follow any `PATH` instructions shown when installation finishes.

More options are in the [installation guide](docs/INSTALLATION.md).

## Installation via Package Manager

### Windows

**[WinGet](https://learn.microsoft.com/windows/package-manager/winget/)**

```powershell
winget install --id Graphixa.Poke
```

### macOS

**[Homebrew](https://brew.sh/)**

```bash
brew tap Graphixa/homebrew-tap
brew install --cask poke
```

## Quick start

```bash
poke
```

On first launch, pick a theme and whether to check for updates. Poke then downloads Pokémon data and artwork (internet required once).

Type to search, **↑ / ↓** to move, **Enter** for details. The footer always shows the keys for your current view.

### Lookups

Skip the browser when you just need a quick answer:

```bash
poke pikachu --stats
poke eevee --evolution
```

### Themes

<p align="center">
  <img src="assets/theme-palettes.svg" alt="Bundled colour palettes: Dracula, Pokédex, Nord, Catppuccin and Solarized Light" width="920">
</p>

```bash
poke theme
```

Preview with **↑ / ↓**, then **Enter** to apply. Dracula is the default; System uses your terminal colours.

[User guide](docs/USER_GUIDE.md) · [Themes](docs/THEMES.md)

## Documentation

| Guide | Find what you need |
| --- | --- |
| [User guide](docs/USER_GUIDE.md) | Browsing, comparisons, favourites and keyboard controls |
| [Commands](docs/COMMANDS.md) | Lookups, search, updates and everyday shortcuts |
| [Configuration](docs/CONFIGURATION.md) | Settings, artwork preferences and file locations |
| [Troubleshooting](docs/TROUBLESHOOTING.md) | Help with installation, data, artwork and updates |
| [Contributing](CONTRIBUTING.md) | Build Poke and contribute improvements |

## Credits & licence

Powered by [PokeAPI](https://pokeapi.co/). Licensed under [GPL-3.0](LICENSE).

Pokémon and Pokédex are trademarks of Nintendo / Creatures Inc. / GAME FREAK. Poke is an unofficial project and is not affiliated with them. Official artwork is downloaded to your local cache, not bundled with the app.

Terminal fireworks adapted from [erik-adelbert/firework](https://github.com/erik-adelbert/firework) ([MIT](internal/fireworks/LICENSE.MIT)).

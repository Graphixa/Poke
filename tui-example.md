# Browse TUI wireframe

Canonical visual contract for Pokedex browse mode. Product rules, CLI, and phases live in [plans/SPEC.md](plans/SPEC.md) (see section 5). If this layout changes, update the spec in the same change.

Implementing agents: match regions, proportions, and information density. Do not invent extra panes, a second chrome bar, or a different tab set.

```
Pokedex — Browse Mode

╭─ Pokedex — Browse Mode ──────────────┬──────────────────────────────────────────────────────────────╮
│ > Search Pokémon…                    │ Charmander #004                     │                        │
│──────────────────────────────────────│ Type: Fire                          │      sprite art        │
│ > Charmander      #004  Fire         │ Species: Lizard Pokémon             │      (no border)       │
│   Charmeleon      #005  Fire         │ Height: 0.6 m    Weight: 8.5 kg     │                        │
│   Charizard       #006  Fire/Flying  │ Ability: Blaze                      │                        │
│   Chikorita       #152  Grass        │ Hidden: Solar Power                 │                        │
│   Chimchar        #390  Fire         │ Base Exp: 62                        │                        │
│   Chingling       #433  Psychic      │                                     │                        │
│   Chinchou        #170  Water/Elec   │ Summary                             │                        │
│   Cubone          #104  Ground       │ Obviously prefers hot places. When  │                        │
│   Cyndaquil       #155  Fire         │ it rains, steam is said to spout.   │                        │
│   Cacturne        #332  Grass/Dark   │──────────────────────────────────────────────────────────────│
│   Camerupt        #323  Fire/Ground  │ [Stats] [Evolution] [Locations] [Moves] [Flavour] [Info]   │
│   Carbink         #703  Rock/Fairy   │──────────────────────────────────────────────────────────────│
│   Carnivine       #455  Grass        │ ╭─  Evolution  ───────────────────────────────────────────╮ │
│   Carracosta      #565  Water/Rock   │ │ Charmander ── Level 16 ──► Charmeleon                   │ │
│   Cascoon         #268  Bug          │ │ Charmeleon ── Level 36 ──► Charizard                    │ │
│   Castform        #351  Normal       │ │                                                         │ │
│   Celebi          #251  Psychic/Gr…  │ │ Stage: Basic                                            │ │
│   …                                  │ │ Final evolution: Charizard                              │ │
│                                      │ │ Trigger source: evolution-chain                         │ │
│                                      │ ╰────────────────────────────────────────────────────────╯ │
╰──────────────────────────────────────┴──────────────────────────────────────────────────────────────╯

 Type to filter • ↑↓ move • Tab tabs • i info • PgUp/PgDn scroll • Esc clear • q quit
```

Notes:

- Left ~1/3 (search + species list only), right ~2/3.
- Right identity band is **two columns**: (main info stacked above Summary) | unbordered sprite. Separator is `│` before the sprite. Summary is not a third side-by-side column.
- Press `i` (with empty search) for a Pokémon info modal; `i` / Esc closes it. Typed `i` while filtering goes into the search box.
- This snapshot is Charmander on the Evolution tab after a `c`-prefix filter. Other tabs replace only the inner detail card.
- `q` quits only when the search query is empty; typed `q` otherwise goes into the filter. `Ctrl+C` always quits.

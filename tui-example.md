# Browse TUI wireframe

Canonical visual contract for Pokedex browse mode. Product rules, CLI, and phases live in [plans/SPEC.md](plans/SPEC.md) (see section 5). If this layout changes, update the spec in the same change.

Implementing agents: match regions, proportions, and information density. Do not invent extra panes, a second chrome bar, or a different tab set.

```
Pokedex — Browse Mode

┌──────────────────────────────────────┬──────────────────────────────────────────────────────────────┐
│ > Search Pokémon…                    │ Charmander #004                                              │
│──────────────────────────────────────│──────────────────────────────────────────────────────────────│
│ > Charmander      #004  Fire         │ Type: Fire                                                   │
│   Charmeleon      #005  Fire         │ Species: Lizard Pokémon                                      │
│   Charizard       #006  Fire/Flying  │ Height: 0.6 m    Weight: 8.5 kg                              │
│   Chikorita       #152  Grass        │ Ability: Blaze                                               │
│   Chimchar        #390  Fire         │ Hidden: Solar Power                                          │
│   Chingling       #433  Psychic      │ Base Exp: 62                                                 │
│   Chinchou        #170  Water/Elec   │                                                              │
│   Cubone          #104  Ground       │ Summary                                                      │
│   Cyndaquil       #155  Fire         │ Obviously prefers hot places. When it rains, steam is      │
│   Cacturne        #332  Grass/Dark   │ said to spout from the tip of its tail.                     │
│   Camerupt        #323  Fire/Ground  │                                                              │
│   Carbink         #703  Rock/Fairy   │──────────────────────────────────────────────────────────────│
│   Carnivine       #455  Grass        │ [Stats] [Evolution] [Locations] [Moves] [Flavour] [Info]   │
│   Carracosta      #565  Water/Rock   │──────────────────────────────────────────────────────────────│
│   Cascoon         #268  Bug          │ ╭─  Evolution  ───────────────────────────────────────────╮ │
│   Castform        #351  Normal       │ │ Charmander ── Level 16 ──► Charmeleon                  │ │
│──────────────────────────────────────│ │ Charmeleon ── Level 36 ──► Charizard                   │ │
│ ╭──────────────────────────────────╮ │ │                                                        │ │
│ │                                  │ │ │ Stage: Basic                                           │ │
│ │            sprite                │ │ │ Final evolution: Charizard                             │ │
│ │           1:1 square             │ │ │ Trigger source: evolution-chain                        │ │
│ │                                  │ │ │                                                        │ │
│ ╰──────────────────────────────────╯ │ ╰────────────────────────────────────────────────────────╯ │
└──────────────────────────────────────┴──────────────────────────────────────────────────────────────┘

 Type to filter • ↑↓ move • Tab next panel • PgUp/PgDn scroll details • Esc clear • q quit
```

Notes:

- Left ~1/3, right ~2/3.
- This snapshot is Charmander on the Evolution tab after a `c`-prefix filter. Other tabs replace only the inner detail card.
- Sprite slot is a 1:1 square; v1 fills it with half-block `front_default` art (placeholder until that phase).
- `q` quits only when the search query is empty; typed `q` otherwise goes into the filter. `Ctrl+C` always quits.

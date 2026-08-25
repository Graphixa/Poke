# TUI layout concept — sprite-first browse

Status: **accepted** (implementation contract).  
Canonical short wireframe: [plans/tui-example.md](plans/tui-example.md).

Inspiration: [FontGet](https://github.com/Graphixa/FontGet) dialog overlays, and Charm Lip Gloss v2 `Layer` / `Compositor` (already available via `charm.land/lipgloss/v2` in this repo).

## Problem with the current layout

Browse tries to show **list + identity + summary + tabs + detail card + sprite** in one frame. On a typical ~120×30 terminal that forces:

- a cramped or hidden sprite
- thin letterboxing that wastes cells on theme-coloured padding
- low-fidelity mosaics because the frame shrinks below a useful size
- detail tabs that compete with the art for vertical space

The sprite is the emotional centre of a Pokédex. Everything else should support it, not squeeze it.

---

## Design goals

1. **Sprite first** — large, square (1:1), high-fidelity Chafa/native art is the default browse view.
2. **Minimum 40×40 cells** whenever the sprite view is shown — never shrink below that.
3. **Tight crop** — little or no theme padding around the Pokémon; fill the square with the subject.
4. **Browse views via Tab** — the right pane cycles Sprite → Summary → Stats → Evolution; the switcher sits **under** the view body.
5. **Full-screen detail dialog** — `Enter` / `i` opens a floating overlay that nearly fills the terminal (with margin), for richer API fields.
6. **Two depths, clear jobs** — browse = find & glance; dialog = inspect the rest of the PokéAPI surface.

---

## Mental model

```
┌─────────────────────────────────────────────────────────────┐
│  BROWSE                                                     │
│  left: search + list                                        │
│  right: view body + bottom switcher                         │
│         [ Sprite ] · Summary · Stats · Evolution            │
│                                                             │
│            Enter / i                                        │
│               ▼                                             │
│  FULL-SCREEN DIALOG (overlay, padded from edges)            │
│  floats over the whole browse frame                         │
│  deeper API: moves, locations, abilities, breeding, …       │
└─────────────────────────────────────────────────────────────┘
```

| Surface | Shows | Does not show |
| --- | --- | --- |
| Browse Sprite | Large portrait only (+ header) | Vitals, flavour, stats tables |
| Browse Summary | Height, weight, abilities, short flavour | Long move / encounter lists |
| Browse Stats / Evolution | Glance bars / short chain | Dense API extras |
| Detail dialog | Moves, locations, full ability text, breeding, … | Does not replace the browse hero sprite |

---

## Browse mode

Two columns. Right column stack (top → bottom):

1. **Header** — name, dex #, types, genus  
2. **View body** — sprite art or text for the active view  
3. **View switcher** — always under the body  

### View switcher (always bottom of the right pane)

Active view uses bracket emphasis; inactive views stay muted with dots:

```text
[ Sprite ] · Summary · Stats · Evolution
  Sprite · [ Summary ] · Stats · Evolution
  Sprite · Summary · [ Stats ] · Evolution
  Sprite · Summary · Stats · [ Evolution ]
```

`Tab` / `Shift+Tab` cycle forward / back. Selection changes always reset to **Sprite**.

### View: Sprite (default)

```
╭─ Pokedex ────────────────────────┬──────────────────────────────────────────────╮
│ > Search Pokémon…                │ Charmander                           #004    │
│──────────────────────────────────│ Fire · Lizard Pokémon                        │
│ > Charmander      #004  Fire     │                                              │
│   Charmeleon      #005  Fire     │           ┌──────────────────────┐           │
│   Charizard       #006  Fire/Fly │           │                      │           │
│   Chikorita       #152  Grass    │           │     SPRITE 40×40     │           │
│   Chimchar        #390  Fire     │           │     (square, tight)  │           │
│   Chingling       #433  Psychic  │           │                      │           │
│   Chinchou        #170  Water    │           │                      │           │
│   Cubone          #104  Ground   │           └──────────────────────┘           │
│   Cyndaquil       #155  Fire     │                                              │
│   …                              │──────────────────────────────────────────────│
│                                  │  [ Sprite ] · Summary · Stats · Evolution    │   < CENTRE ALIGNED JUST LIKE SPRITE
╰──────────────────────────────────┴──────────────────────────────────────────────╯
 ↑↓ move · Tab view · Enter/i details · Esc clear · ? help · q quit
```

### View: Summary

Same reserved body height as the sprite frame; switcher stays on the bottom row.
Carries the vitals that used to sit under the sprite (height, weight, ability) so Sprite view stays art-only.

```
│ Charmander                           #004    │
│ Fire · Lizard Pokémon                        │
│                                              │
│ Height  0.6 m          Weight  8.5 kg        │
│ Ability Blaze          Hidden  Solar Power   │
│                                              │
│ Obviously prefers hot places. When it rains, │
│ steam is said to spout from the tip of its   │
│ tail.                                        │
│                                              │
│ Generation I · Kanto                         │
│                                              │
│ Sprite · [ Summary ] · Stats · Evolution     │
```

### View: Stats

```
│ Charmander                           #004    │
│ Fire · Lizard Pokémon                        │
│                                              │
│ HP      ████████░░░░  39                     │
│ Attack  ██████████░░  52                     │
│ Defense ████░░░░░░░░  43                     │
│ Sp.Atk  ████████░░░░  60                     │
│ Sp.Def  ██████░░░░░░  50                     │
│ Speed   ████████░░░░  65                     │
│ ─────────────────────                        │
│ Total                 311                    │
│                                              │
│ Sprite · Summary · [ Stats ] · Evolution     │
```

### View: Evolution

```
│ Charmander                           #004    │
│ Fire · Lizard Pokémon                        │
│                                              │
│ Charmander                                   │
│    └─ Lv 16 → Charmeleon                     │
│         └─ Lv 36 → Charizard                 │
│                                              │
│ Stage: Basic · Final: Charizard              │
│                                              │
│                                              │
│ Sprite · Summary · Stats · [ Evolution ]     │
```

### Browse regions

| Region | Content | Notes |
| --- | --- | --- |
| Left | Search + species list | ~⅓ width |
| Right header | Name, dex #, types, genus | Always visible |
| Right body | Active browse view | Same footprint as ≥40×40 sprite |
| **Right bottom** | **`[ Sprite ] · Summary · Stats · Evolution`** | **Always under the body** |
| App footer | Browse keys | Mentions Tab for views |

### Browse view rules

- **Default view = Sprite** when the selected species changes.
- Switcher is **never** above the sprite / body — it anchors the bottom of the right pane.
- Views share one reserved body rectangle so the layout does not jump when Tabbing.
- Non-sprite views fill that rectangle with text; only Sprite must stay square art.
- Search is insert-only; **Tab always cycles browse views** (v1).

### What browse deliberately drops from the old layout

- Always-on detail card under the identity band
- Six-way tab strip fighting the sprite for height
- Sub-40 sprite fallbacks
- View labels above the art
- Height / weight / ability teaser under the sprite (those live on **Summary**, with fuller ability text in the dialog)

If the terminal cannot fit a **40×40** sprite region plus list + chrome + switcher row, show a clear size cue — do **not** silently render a tiny mosaic.

---

## Sprite rendering rules (Sprite view)

| Rule | Detail |
| --- | --- |
| Shape | Square **1:1** cell grid |
| Minimum | **40×40** cells always when Sprite view is shown |
| Preferred | 40×40; grow to 44 / 48 only when width and height clearly allow (bucketed) |
| Crop | Preserve PNG AR, but **minimise letterbox** — prefer tighter fill over large theme margins |
| Background | Composite transparency onto theme base; avoid empty multi-cell rings around the subject |
| Pipeline | Native image if supported → else Chafa mosaic → else named placeholder |
| Quality | Mosaic quality suited to ≥40 cells |

Mental model: a **portrait frame**, not a padded thumbnail.

---

## Detail dialog (`Enter` / `i`) — full-screen overlay

`Enter` and `i` open the **same** dialog. It floats over the **entire** browse TUI (both columns + frame), not only the right pane.

### Overlay approach (implementation hint)

Follow FontGet-style modal composition and Charm’s Lip Gloss v2 layering:

- Render browse as the **background layer** (optionally dimmed / faded).
- Render the dialog as a **foreground layer** with higher Z.
- Composite with `lipgloss.NewLayer` + `lipgloss.NewCompositor` (this project already depends on Lip Gloss v2), or an equivalent Bubble Tea overlay helper.
- Trap focus while open: browse keys idle; dialog owns Tab / scroll / Esc.

### Sizing — nearly full terminal, with padding

The dialog is **not** a small card in the middle. It uses most of the screen:

| Dimension | Rule |
| --- | --- |
| Width | `termW - 2×padX` (suggested `padX = 2`–`4` cells) |
| Height | `termH - 2×padY` (suggested `padY = 1`–`2` rows) |
| Position | Centred (equal remaining margin on each side) |
| Border | Rounded theme border; title integrated on the top edge (FontGet dialog pattern) |
| Body | Viewport / scrollable content for long lists |

This gives real estate for moves, locations, and multi-section API data without crowding the browse portrait.

### Wireframe

```
╭─ Pokedex ────┬──────────────────────────────────────────────────╮
│ (dimmed)     │                                                  │
│ browse list  │     ╭─ Charmander #004 ─────────────────────╮   │
│ + sprite     │     │ [ Moves ] · Locations · Abilities · … │   │
│ still drawn  │     │───────────────────────────────────────│   │
│ underneath   │     │ Lv  1  Scratch                        │   │
│              │     │ Lv  1  Growl                          │   │
│              │     │ Lv  7  Ember                          │   │
│              │     │ Lv 13  Smokescreen                    │   │
│              │     │ Lv 19  Dragon Rage                    │   │
│              │     │ …                                     │   │
│              │     │                                       │   │
│              │     │                                       │   │
│              │     │ Tab section · ↑↓ scroll · Esc close   │   │
│              │     ╰───────────────────────────────────────╯   │
│              │         ↑ pad from terminal edges               │
╰──────────────┴──────────────────────────────────────────────────╯
```

Section switcher inside the dialog mirrors browse style:

```text
[ Moves ] · Locations · Abilities · Flavour · Breeding · Training · Forms · Meta
```

### Why both Enter and i

One dialog, two familiar keys. Footer: `Enter/i details`.  
`i` still respects today’s typing rule: with a non-empty search query, `i` inserts into the filter; with an empty query, `i` opens the dialog. `Enter` always opens the dialog for the selection.

### Dialog sections (deeper API surface)

Browse already covers portrait / summary / base stats / evolution glance. The dialog focuses on the rest:

| Section | Source ideas | Content |
| --- | --- | --- |
| **Moves** | Pokémon learnset | Level-up (optionally machine/tutor) |
| **Locations** | Encounters | Area / method / level ranges |
| **Abilities** | Pokémon + ability | Slot, hidden flag, effect text (fuller than Summary) |
| **Flavour** | Species flavour | Versioned Pokédex entries |
| **Breeding** | Species | Egg groups, hatch steps, gender rate |
| **Training** | Species / Pokémon | EVs, catch rate, friendship, growth |
| **Forms** | Species | Varieties / default form |
| **Meta** | Pokémon | Height/weight again if useful, held items, game indices, etc. |

Empty sections show a quiet “No data” rather than disappearing from the strip.

### Dialog behaviour

| Input | Action |
| --- | --- |
| `Enter` / `i` | Open dialog (from browse, when allowed) |
| `Tab` / `Shift+Tab` | Cycle dialog sections |
| `←` `→` | Optional alternate section cycle |
| `↑` `↓` / PgUp / PgDn | Scroll section body |
| `Esc` | Close → browse (selection + browse view kept) |
| `q` | Close dialog first; from browse with empty search, quit |

Optional: small **24×24** sprite chip in the dialog title bar for orientation — never a substitute for browse Sprite view.

---

## Keybindings

| Key | Browse | Dialog open |
| --- | --- | --- |
| `Tab` | Cycle **browse views** | Cycle **dialog sections** |
| `Enter` | Open dialog | no-op (unless a future inner confirm) |
| `i` | Open dialog (empty search) | ignored / same as browse rule |
| `↑` `↓` | Move list selection | Scroll dialog body |
| `Esc` | Clear search | Close dialog |
| `q` | Quit if search empty | Close dialog |

---

## Minimum terminal sizes

| Mode | Suggested minimum | Reason |
| --- | --- | --- |
| Browse + Sprite view | **≈ 100×50** | 40×40 sprite + list + header + **bottom switcher** + chrome |
| Browse + text views | Same body footprint as Sprite | Shared reservation |
| Detail dialog | Same as terminal; content = term − padding | Full-screen overlay |

Below Sprite minimum: keep list usable; show a size hint in the body — no sub-40 art.

---

## Alternate density (optional)

If width ≥ ~140 and height ≥ ~54, Sprite body may grow to **48×48**. Switcher stays under it. Still square. Still ≥40.

---

## Migration from current layout

| Today | Concept |
| --- | --- |
| Sprite squeezed beside summary + always-on tabs | Sprite is default browse view at ≥40×40 |
| Adaptive frames down to 16 | Floor at **40**; hide rather than degrade |
| Tabs under identity / above detail | **Bottom** switcher: `[ Sprite ] · Summary · …` |
| Vitals under / beside the sprite | Vitals on **Summary** (+ dialog Abilities / Meta) |
| Detail card always mounted | Long fields in **full-screen Enter/i** overlay |
| Small `i` info popup | One large padded dialog over the whole TUI |

---

## Acceptance sketch

- [ ] Selecting a Pokémon lands on Sprite view; art is recognisable immediately
- [ ] Sprite is square and ≥40 wide whenever Sprite view is shown
- [ ] View switcher reads `[ Sprite ] · Summary · Stats · Evolution` and sits **under** the body
- [ ] Tab cycles views without jumping the right-pane layout
- [ ] Changing list selection resets to Sprite view
- [ ] Enter / i opens a dialog that covers nearly the full terminal with edge padding
- [ ] Browse remains visible (dimmed) underneath the overlay
- [ ] Esc closes the dialog and restores browse focus cleanly

---

## Open questions

1. **Crop policy** — slight zoom-to-fill vs strict full-sprite letterbox with minimal pad?
2. **Exact dialog padding** — lock `padX=3`, `padY=1`, or scale with terminal size?
3. **Background dim** — fade browse with a lipgloss style, or leave full brightness under the overlay?
4. **Browse view set** — lock four views, or allow a later fifth glance?
5. **Dialog section set** — which endpoints are v1 vs later?
6. **Tiny terminals** — list-only until resize, or allow text browse views without sprite?

---

## Recommendation

Adopt **bottom-anchored browse view switcher + full-screen padded detail dialog**.

Browse stays visual and fast; Tab peeks at Summary / Stats / Evolution under a fixed portrait slot; Enter/i opens a FontGet/Charm-style floating dialog with real room for API depth. Once accepted, update [plans/tui-example.md](plans/tui-example.md) / SPEC and implement against this contract.

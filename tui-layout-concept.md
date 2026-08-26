# TUI layout concept — list + sprite browse, Enter detail

Status: **accepted** (implementation contract). Supersedes browse Tab text views
(Summary / Stats / Evolution on the right pane).  
Canonical short wireframe: [plans/tui-example.md](plans/tui-example.md) (update
when implementing). Cache / dossier policy: [plans/DECISIONS.md](plans/DECISIONS.md).

Inspiration: [FontGet](https://github.com/Graphixa/FontGet) dialog overlays, and
Charm Lip Gloss v2 `Layer` / `Compositor`.

## Problem with the previous layout

Browse tried to show **list + identity + Tab text views + sprite** in one frame.
That forced cramped art, competing Tab strips, and **highlight-driven detail
loads** (dossier / API) on every scroll.

The sprite is the emotional centre while browsing. Full PokéAPI text belongs on
an explicit **Enter** surface, not on cursor moves.

---

## Design goals

1. **Sprite first on browse** — large, square (1:1), high-fidelity Chafa/native
   art beside the list.
2. **Minimum 40×40 cells** whenever the sprite panel is shown.
3. **Tight crop** — little theme padding; fill the square with the subject.
4. **Browse = find** — search + list + thin header + hero sprite only.
5. **Enter / `i` = inspect** — full-screen overlay loads **one dossier** and
   shows all text detail (summary, stats, evolution, moves, locations, …).
6. **Scroll ≠ GetDossier** — highlight paints from the species **index** and
   local sprite path only.
7. **Tab on browse** — reserved for future **sprite variant** cycling (not
   text views). Dialog Tab cycles detail sections.

---

## Mental model

```
┌─────────────────────────────────────────────────────────────┐
│  BROWSE                                                     │
│  left: search + list (index)                                │
│  right: header (from index) + large sprite                  │
│                                                             │
│            Enter / i  →  GetDossier                         │
│               ▼                                             │
│  FULL-SCREEN DIALOG (overlay)                               │
│  all text API: Summary · Stats · Evolution · Moves · …      │
└─────────────────────────────────────────────────────────────┘
```

| Surface | Shows | Does not show |
| --- | --- | --- |
| Browse | List, name/dex/types from index, large sprite | Stats tables, flavour, moves, evolution text |
| Detail dialog | Full dossier sections | Does not replace browse as the hero portrait home |

---

## Browse mode

Two columns:

1. **Left** — search + species list (~⅓ width)
2. **Right** — header (name, dex #, types; genus only if already on index) +
   sprite body (≥40×40). **No** bottom `[ Sprite ] · Summary · Stats · Evolution`
   switcher.

```
╭─ Pokedex ────────────────────────┬──────────────────────────────────────────────╮
│ > Search Pokémon…                │ Charmander                           #004    │
│──────────────────────────────────│ Fire                                         │
│ > Charmander      #004  Fire     │                                              │
│   Charmeleon      #005  Fire     │           ┌──────────────────────┐           │
│   Charizard       #006  Fire/Fly │           │                      │           │
│   …                              │           │     SPRITE 40×40     │           │
│                                  │           │                      │           │
│                                  │           └──────────────────────┘           │
╰──────────────────────────────────┴──────────────────────────────────────────────╯
 ↑↓ move · Enter/i details · Esc clear · ? help · q quit
```

### Browse data rules

- List and header: **`index/species.json`** only.
- Sprite: named path `sprites/<id>-<slug>/front_default.png` (or placeholder).
- Cursor change: update highlight + sprite prioritize/warm; **do not** call
  `GetDossier` / `DetailService.Load`.
- Missing sprite file: named placeholder; warm may fill in background.

### What browse deliberately drops

- Summary / Stats / Evolution as browse Tab views
- Always-on detail card under the identity band
- Highlight-driven core/dossier network loads
- Sub-40 sprite fallbacks (hide or size-cue instead)

If the terminal cannot fit a **40×40** sprite region plus list + chrome, show a
clear size cue — do **not** silently render a tiny mosaic.

---

## Sprite rendering rules (browse)

| Rule | Detail |
| --- | --- |
| Shape | Square **1:1** cell grid |
| Minimum | **40×40** cells when the sprite panel is shown |
| Preferred | 40×40; grow to 44 / 48 when width/height clearly allow |
| Crop | Preserve PNG AR; minimise letterbox |
| Background | Composite transparency onto theme base |
| Pipeline | Native image if supported → else Chafa mosaic → else named placeholder |

Mental model: a **portrait frame**, not a padded thumbnail.

### Tab → sprite variants (later)

Not required for this pass. When assets exist:

- Browse `Tab` / `Shift+Tab` may cycle pixel ↔ official artwork (and later
  variants), using files under `sprites/` and `official-artwork/`.
- Still **no** dossier load — paths only.
- Until then, Tab on browse may be unbound or a no-op; do not restore text views.

---

## Detail dialog (`Enter` / `i`) — full-screen overlay

`Enter` and `i` open the **same** dialog and trigger **`GetDossier`** for the
selected species (disk-first after dossier warm).

### Overlay approach

- Browse = background layer (optionally dimmed).
- Dialog = foreground layer (Lip Gloss v2 `Layer` / `Compositor` or equivalent).
- Trap focus while open: dialog owns Tab / scroll / Esc.

### Sizing

| Dimension | Rule |
| --- | --- |
| Width | `termW - 2×padX` (suggested `padX = 2`–`4`) |
| Height | `termH - 2×padY` (suggested `padY = 1`–`2`) |
| Position | Centred |
| Body | Scrollable viewport |

### Sections (full dossier surface)

Former browse glance content **moves here**. Suggested strip:

```text
[ Summary ] · Stats · Evolution · Moves · Locations · Abilities · Flavour · Breeding · Training · Forms · Meta
```

| Section | Content |
| --- | --- |
| **Summary** | Height, weight, abilities teaser, short flavour, generation |
| **Stats** | Base stat bars + total |
| **Evolution** | Chain |
| **Moves** | Level-up learnset (from dossier `level_up_moves`) |
| **Locations** | Encounters |
| **Abilities** | Fuller ability text |
| **Flavour** | Versioned Pokédex entries |
| **Breeding** | Egg groups, gender, hatch |
| **Training** | Catch rate, growth, EVs, … |
| **Forms** | Varieties |
| **Meta** | Extra ids / held items / indices as useful |

Empty sections: quiet “No data”, keep strip entry.

Loading: only if dossier miss / fetch in flight when dialog opens. After warm,
open should be near-instant from disk.

Optional: small **24×24** sprite chip in the dialog title bar.

### Dialog behaviour

| Input | Action |
| --- | --- |
| `Enter` / `i` | Open dialog + load dossier (from browse, when allowed) |
| `Tab` / `Shift+Tab` | Cycle dialog sections |
| `↑` `↓` / PgUp / PgDn | Scroll section body |
| `Esc` | Close → browse |
| `q` | Close dialog first; from browse with empty search, quit |

`i` with non-empty search still inserts into the filter; with empty search, opens
detail. `Enter` always opens detail for the selection.

---

## Keybindings

| Key | Browse | Dialog open |
| --- | --- | --- |
| `Tab` | Later: sprite variants (noop until then) | Cycle **dialog sections** |
| `Enter` | Open detail (dossier) | no-op unless future inner confirm |
| `i` | Open detail (empty search) | per browse typing rule |
| `↑` `↓` | Move list (index only) | Scroll dialog body |
| `Esc` | Clear search | Close dialog |
| `q` | Quit if search empty | Close dialog |

---

## Minimum terminal sizes

| Mode | Suggested minimum | Reason |
| --- | --- | --- |
| Browse + sprite | **≈ 100×50** | 40×40 sprite + list + header + chrome |
| Detail dialog | Terminal − padding | Full-screen overlay |

Below sprite minimum: keep list usable; size hint in the body — no sub-40 art.

---

## Alternate density (optional)

If width ≥ ~140 and height ≥ ~54, sprite body may grow to **48×48**. Still square.
Still ≥40.

---

## Migration from prior concept

| Prior concept | Now |
| --- | --- |
| Tab: Sprite · Summary · Stats · Evolution | Browse = sprite only; those text views live in Enter dialog |
| Highlight loads core detail | Highlight = index + sprite path; Enter = dossier |
| Dialog = “deeper API only” | Dialog = **all** text detail |
| Tab on browse = text views | Tab on browse = future sprite variants |

---

## Acceptance sketch

- [ ] List scroll does not fetch dossiers / show detail Loading
- [ ] Browse shows list + ≥40×40 sprite when space allows
- [ ] No Summary/Stats/Evolution switcher under the browse sprite
- [ ] Enter / i opens padded full-terminal dialog and loads one dossier
- [ ] Dialog includes summary, stats, evolution, moves, locations, …
- [ ] After dossier warm, Enter is disk-fast when file exists
- [ ] Esc closes dialog and restores browse focus

---

## Open questions

1. **Crop policy** — slight zoom-to-fill vs strict full-sprite letterbox?
2. **Exact dialog padding** — fixed vs scale with terminal size?
3. **Background dim** — fade browse under overlay or leave bright?
4. **Dialog section order / v1 subset** — ship all strips or start smaller?
5. **Tiny terminals** — list-only until resize?

---

## Recommendation

Adopt **list + sprite browse** and **Enter-only dossier detail**. Keep sprite
warm on cursor for art snappiness; warm dossiers in the background so Enter
stays fast. Implement against this contract and
[plans/DECISIONS.md](plans/DECISIONS.md).

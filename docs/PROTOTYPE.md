# Filter, compare and favourites prototype

Branch: `prototype/filters-compare-favourites`

This is an implementation in the existing Bubble Tea TUI. It keeps the artwork downloader, full image caching, warmers and renderers. New dialogs use text cells and temporarily cover native graphics with the existing clear/restore mechanism.

## Run it

From your existing clone, with Go 1.26 installed:

```sh
git fetch origin
git switch --track origin/prototype/filters-compare-favourites
go run ./cmd/poke
```

If you already have the branch locally, use `git switch prototype/filters-compare-favourites`. Your existing configuration and artwork cache are reused. On a fresh installation, normal setup still runs. For a standalone executable, use `go build -o poke-prototype ./cmd/poke` (Windows: `poke-prototype.exe`).

Use at least **40 columns × 18 rows** for the new dialogs; 100 × 30 is more comfortable for browsing. Some keyboards require Fn with F2–F4.

## Three-minute walkthrough

1. **Favourite:** type `pikachu`, press **F4**, then Esc to clear the search. A starred quick-link appears at the top; Pikachu also stays in dex order with a star. Cursor does not jump. Favourite Eevee too. The divider separates quick-links from the full list.
2. **Filter:** press **F2**. Use ↑/↓ to choose Collection, Type or Sort; ←/→ changes the value. Choose *Favourites only*, then **Enter**. Reopen F2, press **Backspace**, then Enter to reset view options. Search text is preserved.
3. **Compare:** search `char`, select Charmander and press **F3** to pin it. Select Charizard and press F3 again. **Esc** returns to the same selection and search, retaining the pin. **Backspace** inside comparison closes it and clears the pin. Pressing F3 on the pinned species also unpins it.
4. **Check persistence:** exit and reopen. Favourites remain; filters and the comparison pin reset for a new session.

Letters still type into search. Enter still opens details, Tab switches Glance/Art, and `?` opens help. Esc closes a dialog; in the browse screen it clears a nonempty search and otherwise exits.

## View options

The count previews the draft filter against the current search. Enter applies; Esc discards the draft. Backspace resets only the draft view options, then Enter applies that reset.

```text
   ╭────────────────────────────────────────────────────────╮
   │VIEW OPTIONS                                            │
   │                                                        │
   │  Collection   All Pokémon                              │
   │> Type         Fire                                     │
   │  Sort         Default                                  │
   │                                                        │
   │2 matches · keeps your search                           │
   │                                                        │
   │↑/↓ choose   ←/→ change                                 │
   │Enter apply  Esc cancel                                 │
   │Backspace reset view options                            │
   ╰────────────────────────────────────────────────────────╯
```

- **Collection:** all Pokémon or favourites only.
- **Type:** any one of the 18 standard types; dual-type Pokémon match either type.
- **Sort:** Default (existing search relevance, or National Dex order without a search), name, speed descending or base-stat total descending. Favourites remain the first group under each sort. Missing cached stats sort last within their group.
- These are **session view options**, not persistent game/version settings. Search, type and collection intersect. An empty result is valid; use F2 to adjust/reset filters.

## Favourites

Actual list-panel render using a small test fixture (the real app uses your complete dataset):

```text
╭─ Pokedex ─────[ 4/4 ]─╮
│ > Search Pokémon…     │
│───────────────────────│
│ Favourites · 2        │
│   ★ Pikachu     #025  │
│   ★ Eevee       #133  │
│ ───────────────────── │
│ >   Charmander  #004  │
│     Charizard   #006  │
│   ★ Pikachu     #025  │
│   ★ Eevee       #133  │
╰───────────────────────╯
```

Favourites are **quick-links**: a starred copy is prepended at the top, and the same species stays in its normal list position (also starred). F4 does not move the cursor or jump the viewport. While a **search** is active, F4 only stars the matching row (no quick-link block). Favourites-only (F2) shows each favourite once.

**F4 toggles without a confirmation dialog.** The change becomes visible after its small background save succeeds. Another F4 is ignored while that write is in flight. A normal exit waits for the outstanding save.

The quick-link group scrolls with the list; it does not permanently consume rows while browsing further down. It disappears when no favourites match the active search/filters. In a favourites-only view, removing the final entry leaves an empty list.

Choices live in `favourites.json` beside the file reported by `poke config path`, independently of artwork/render caches. Both `poke cache clear` and the current `poke reset` preserve this file. A failed save leaves the list unchanged and shows an error; malformed or unsupported preference files are preserved and writes are disabled until repaired and the app restarted.

## Compare

Actual dialog render with Charmander pinned on the left and Charizard selected on the right:

```text
   ╭────────────────────────────────────────────────────────╮
   │COMPARE · BASE STATS                                    │
   │          Charmander              Charizard             │
   │          #004                    #006                  │
   │          Fire                    Fire                  │
   │                                  Flying                │
   │HP        39                      78 +                  │
   │Attack    52                      84 +                  │
   │Defence   43                      78 +                  │
   │Sp. Atk   60                      109 +                 │
   │Sp. Def   50                      85 +                  │
   │Speed     65                      100 +                 │
   │Total     309                     534 +                 │
   │+ higher stat · — unavailable                           │
   │Esc back · Backspace unpin                              │
   ╰────────────────────────────────────────────────────────╯
```

The title bar in browse mode identifies the pinned Pokémon. Comparison shows names, Dex numbers, types, six base stats and totals. `+` means the larger value for that row, not a judgment that one Pokémon is better. Missing stats display `—`. This uses the existing in-memory browse data (or an already-loaded dossier); it does not fetch a second image or introduce another artwork cache.

## Prototype limits and feedback

- One type filter and two-species comparison. No game-specific stat history, generation selection, moves/abilities comparison or team builder in this pass.
- Favourites are species-level choices. Run one instance when changing them; cross-process preference merging is not implemented.
- Filter choices and the comparison pin are intentionally session-only. Favourites persist across sessions.
- Native overlay guards have automated coverage. Please try the dialogs in your actual Sixel/Kitty/iTerm terminal to assess graphics restoration and flicker; automated text tests cannot verify a terminal emulator's pixels.

The main UX questions to try: does the favourites group feel useful without taking over the list; are F2/F3/F4 comfortable; and is pin-then-compare clear enough without a separate picker?

## Validation

Focused tests cover apply/cancel/reset, search intersections, favourite grouping and scrolling, persistence and corrupt files, failed writes and graceful exit, stat sorting, comparison state, missing stats, small-terminal dimensions and native-overlay suppression.

```sh
go vet ./...
go test -race ./... -skip '^TestSixelPipelineProfile$'
go build ./cmd/poke
```

The skip excludes the existing network-backed Sixel profiling test, which performs a live artwork download and timing measurements; it does not skip renderer unit tests or the new feature tests. The repository's normal CI still runs its existing full test command.

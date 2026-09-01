# Native cache acceptance checklist

Automated coverage lives in `internal/render/sprite/nativewarm/warm_test.go`, `internal/render/sprite/persistqueue_test.go`, `internal/render/sprite/coldsched/selection_test.go`, `internal/tui/e2e_acceptance_test.go`, and `internal/cache/cache_lifecycle_test.go`.

## Expected behaviour

- Background native warming persists `.sixel.gz` for the **active bucket only** (24/32/40).
- Traversal is progressive through the full dex (N+1..max, N-1..1), not limited to prefetch radius.
- Footer shows `Image Cache done/total · bucket` while incomplete; `Done` counts **persisted/readable** frames only.
- Foreground cold scheduler keeps one active + one latest pending; ordinary selection changes do not cancel active encodes.
- Stale frames may persist but never display.
- Clean browse exit drains the persistence worker (`engine.ClosePersist()`).

## Manual verification (Windows Terminal + sixel)

### First launch / cold cache
- [ ] Open browse on a species with no native frame
- [ ] Text/UI appears immediately
- [ ] Spinner only for selected cold image
- [ ] Selected image appears quickly
- [ ] Gzip cache file appears after frame-ready
- [ ] Background warms current bucket only; file count grows while stationary

### Reading dossier
- [ ] Leave cursor stationary; warmer gradually fills current bucket beyond nearest neighbours
- [ ] Image Cache progress bar visible and increasing
- [ ] CPU remains reasonable; TUI responsive

### Mouse wheel (~10–20 Pokémon/sec)
- [ ] Cursor/text tracks immediately
- [ ] Cached frames appear immediately (no spinner)
- [ ] Cold frames do not FIFO-backlog
- [ ] Final image settles quickly after stop

### Resize
- [ ] Warm at bucket 32, resize to 40: current uses 40, background targets 40, progress resets for @40
- [ ] Previously generated 32 frames remain and reuse on resize back

### Palette
- [ ] Default 64; 128/256 distinct cache identity; invalid value (e.g. 254) falls back to 64

### Restart
- [ ] Cached active-size frames load from gzip; no full-dex rebuild required

### Corruption
- [ ] Corrupt one `.gz`; browse regenerates without fatal TUI error

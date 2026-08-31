# Native cache acceptance checklist

Automated coverage lives in `internal/tui/e2e_acceptance_test.go`, `internal/cache/cache_lifecycle_test.go`, and `internal/render/sprite/nativewarm/warm_test.go`.

## Manual verification (Windows Terminal + sixel)

### First launch / cold cache
- [ ] Open browse on a species with no native frame
- [ ] Text/UI appears immediately
- [ ] Spinner only for selected cold image
- [ ] Selected image appears quickly
- [ ] Gzip cache file appears after frame-ready
- [ ] Background warms current bucket only

### Reading dossier
- [ ] Leave cursor stationary; warmer gradually fills current bucket
- [ ] CPU remains reasonable; TUI responsive

### Mouse wheel (~10–20 Pokémon/sec)
- [ ] Cursor/text tracks immediately
- [ ] Cached frames appear immediately
- [ ] Cold frames do not FIFO-backlog
- [ ] Final image settles quickly after stop

### Resize
- [ ] Warm at bucket 32, resize to 40: current uses 40, background targets 40
- [ ] Previously generated 32 frames remain and reuse on resize back

### Palette
- [ ] Default 64; 128/256 distinct cache identity; invalid value (e.g. 254) falls back to 64

### Restart
- [ ] Cached active-size frames load from gzip; no full-dex rebuild required

### Corruption
- [ ] Corrupt one `.gz`; browse regenerates without fatal TUI error

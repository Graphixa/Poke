# Pokedex Go codebase

Repository architecture for module `github.com/Graphixa/Poke` (Go 1.26). Not a copy of `plans/pokedex-backend-refactor-plan.md`.

**Public binary:** `poke` (`cmd/poke`). Alias name `pokedex` is the same program.

**Supported GOOS/GOARCH (Makefile + CI):** `linux/amd64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`. CI also builds natively on ubuntu, macos, and windows hosts. Windows exes embed `assets/icons/icon.ico` via `go-winres` v0.3.3 (`make winres` / GoReleaser before hook → `cmd/poke/rsrc_windows_*.syso`, gitignored). `poke` and `pokedex` share that package, so both get the icon.

**Formatting / lint:** `gofmt` (Makefile `fmt` / `fmt-check`; CI prints offenders). `go vet ./...`. `staticcheck ./...` (`2026.2.1`). `gosec ./...` (`v2.29.0`; excludes G104/G115/G301/G304/G306/G404). `govulncheck ./...` (`v1.10.0`). No golangci-lint. Race: `CGO_ENABLED=1 go test -race ./...` — Windows **can** run this (gcc/CGO required; recorded PASS ~197s on this tree). Linux CI (`ubuntu-latest`, `go test -race ./...`) is the merge gate.

---

## Architecture (layers)

```
cmd/poke          → os.Exit(app.Main)
internal/cli      → cobra command surface; no product logic
internal/app      → orchestration: services, browse wiring, setup, dataset bootstrap, theme
internal/domain   → transport- and presentation-independent models
internal/dataset  → compact on-disk snapshot models + validate
internal/pokeapi  → HTTP client; DTOs never leave the package
internal/cache    → file store, dataset snapshots, artwork PNG, native gzip frames
internal/search   → in-memory species index
internal/render/sprite (+ subpackages) → PNG resolve, encode, mem/disk artefact cache, warmers
internal/tui      → Bubble Tea browse; presentation only (via injected interfaces)
internal/output   → CLI table/info/stats/evolution printers
internal/config   → YAML config + OS path resolution
internal/theme + themeui + tui/style → palettes
pkg/version       → linker-stamped version
```

**Intended ownership**

| Concern | Owner |
| --- | --- |
| CLI flags / routing | `internal/cli` |
| Wire services, start warmers, first-run / bootstrap | `internal/app` |
| Domain types | `internal/domain` |
| Metadata snapshot + PNG + native frame files | `internal/cache` |
| HTTP | `internal/pokeapi` |
| Encode / mem LRU / persist queue | `internal/render/sprite` |
| Background encode / download queues | `prewarm` (kinds: chafa, native, artwork, dossier) |
| Presentation, keys, overlay, footer | `internal/tui` |

**Internal-dependency contract**

Target import rules. `app` is allowed to assemble services and translate progress. Forbidden edges are locked by `internal/app/architecture_test.go`.

| Layer | May | Must not |
| --- | --- | --- |
| `internal/tui` (browse, setup) | presentation types (`WarmStage`, setup `Progress`) | import `internal/cache` |
| `prewarm` | one `Progress` type (with `Kind`); encode/download mechanics via per-kind hooks | no `cache` import (hooks injected by `app`) |

| `cache`, `dataset` | storage, snapshots, dataset bootstrap | import `app` or `tui` |
| `internal/app` | construct services, map subsystem progress → TUI/CLI | dump presentation terminology into renderer packages |
| renderer / warmer packages | mechanical progress for their work units | know TUI footer/setup copy or `tui.WarmStage` |

`cache` owns **dataset** progress (`DatasetStage` / `DatasetProgress`). It is not a global event-types package. Native/artwork/dossier/chafa stages belong to the producing subsystem; `app` classifies them into `tui.WarmStage`. Architecture/import tests in `internal/app/architecture_test.go` lock the forbidden edges.

**Progress ownership (current)**

- `cache.DatasetProgress` / `DatasetStage` — dataset bootstrap and enrich only. No native/artwork/dossier/chafa work uses this type.
- `prewarm.Progress` — one shared type; `app` classifies it into `tui.WarmStage*` per service. Prewarm does not import `cache`.
- `app` maps warmer progress → `tui.WarmProgress` / `tui.WarmStage`. A terminal dataset download (first run, cache clear, `poke refresh`) goes through one weighted bar into `setup.Progress`. A non-terminal uses `formatBootstrapProgress`.
- `tui.WarmProgress.Stage` is `tui.WarmStage`. Browse and setup do not import `cache`. The TUI stage sentence is `setup.StageLabel`; CLI copy stays in `formatBootstrapProgress`.
- All four warmers are one `prewarm.Service` (heap of P0/P1 hints, latest-first, plus an optional background scan outward from the browse anchor; phased size buckets; retry backoff). Only the per-kind `Warm` hook differs.

---

## End-to-end flows

### 1. Startup

1. `cmd/poke/main.go` calls `app.Main(ctx, app.OSRuntime())`.
2. `app.Execute` resolves OS config/cache paths (`config.DirectoryResolver`), builds a `sync.Once` lazy `LoadServices`, applies theme, constructs cobra via `cli.NewRoot`.
3. `LoadServices` → `config.Load` → `cache.NewStore` + `cache.EnsureCompatibleCache` → `pokeapi.NewClient` → `cache.NewService`.
4. Root with no inline flags → `Handler.RunRoot`.
5. `openBrowse` → `runSetupIfNeeded` (flow 2) → `EnsureIndex` → capability detect → `sprite.Engine` + optional warmers → `tui.Run`. Missing Pokédex or pictures are downloaded inside setup, not by a separate bootstrap before it.
6. Inline `--info/--stats/--evolution` skips TUI; `DetailService` + `output`.
7. `--debug` enables `debuglog` to a file (stderr prints the path only).

### 2. Dataset download (setup preloader)

One screen, `tui/setup`. The caller turns steps on or off.

| Command | Questions | Border title |
| --- | --- | --- |
| First run, `poke --setup` | theme, update check, intro, then OK | `Poke - Initial Setup` |
| `poke refresh`, cache clear | none | `Refreshing Pokédex` |

The download frame is the same everywhere: a 5-line spinner (`preload.SpinEvery`, `preload.SpinDots`), a shuffled joke (`preload.Lines`, `preload.LineEvery`), one weighted bar (`preload.Bar`), and one grey `setup.StageLabel` line. No count and no species name. Empty filter in browse is unrelated: the list says "No matches" and the right pane is blank (no "Select a Pokémon", and the last native picture is cleared).

1. First run / cache clear: `runSetupIfNeeded` → `prepareWork` when the dataset or pictures are missing. `Ask` is true only for first run and `--setup`.
2. `poke refresh`: `cacheUpdateProgress`. Terminal stderr runs `setup.Run` with `Ask: false` via `setupUpdateRun` (dataset rebuild, then `SyncArtwork` on the same bar). Otherwise throttled CLI lines (`formatBootstrapProgress`).
3. `Service.BootstrapDataset` is single-flight (`group.Do("dataset-bootstrap")`).
4. Stages (`cache.DatasetStage`, plus `ability` / `form` / `artwork`): `index` → parallel `records` → `evolution` → `moves` → catalog → abilities → forms → validate → write aggregates → activate. Artwork is its own bar part.
5. Bar floors in `downloadReserve` hold space until a stage reports a real total. A non-positive total drops that part.
6. Staging snapshot under `datasets/<id>/`. `ActivateDataset` writes `active.json` only if complete. Failed run leaves the previous snapshot.
7. Workers: HTTP semaphore + `datasetWorkers()` goroutines, result channels, cancel via context. `Ctrl+C` and `Esc` cancel the setup screen.

### 3. Artwork download

Two production paths:

- **CLI / refresh:** `Service.SyncArtwork` walks the index, compares CDN URL + SHA, downloads official-artwork PNGs, and invalidates native frames when bytes change. It emits `ArtworkSyncProgress`; app owns the composite dataset-refresh → artwork-sync presentation. (`poke update` is binary self-update via `internal/update`, not artwork.)
- **Browse / setup:** `prewarm.NewArtwork` (selected / neighbors / background scan). Resolves via `sprite.SourceResolver` (store then HTTP). Stage `"artwork"`. Started on `OnUIReady` or blocking (`Wait`) in setup wizard.

Consent: `config.Setup.ArtworkMode` (`off|blocking|background`). Browse starts the warmer when mode is `background` (or setup asked for background).

### 4. Native image-cache construction

1. `openBrowse` detects native image (`spriteimage` + `DetectCapabilities`). If native and sprites on, constructs `prewarm.NewNative`.
2. TUI `notifyWarmer` calls `SetBuckets(BrowseWarmBuckets())` then `EnsureStarted(ctx, focusCells)`. Browse warm phases are **glance then hero** (`16` then `40`). Intermediate buckets `24|32` remain valid encode sizes for stepped Art layout / dossier helpers.
3. Single worker: disk audit (`HasNativeFile`) then opportunistic encode of the **active phase** bucket. `SetBuckets` sets phase order; `SetCells` sets **focus** (Prioritize prefers focus, then other phase sizes) without abandoning the scan phase. Old gzip files stay on disk.
4. Encode: `Engine.NativeWarmRender` → mem LRU (decompressed) → async `persistQueue` gzip to `cache/native/<key>.sixel.gz`. Keys include pixel size from `SixelCellPxW/H` (10×20) × frame cells, so 16×8 and 40×20 are distinct artefacts.
5. Progress: `prewarm.Progress` (Done/Total/Cells; message `"Building image cache"`). Phase completion emits `Done>=Total` for the finished bucket before advancing (so the footer can clear or reset); the next phase does **not** emit a sticky `0/N` when that bucket is already warm. `app` maps progress to `tui.WarmStageNative`. Footer shows only native.
6. Completeness is reconstructed from content-addressed `.sixel.gz` files (`HasNativeFile` / prewarm disk audit). There is no aggregate image-completion coordinator or Pokémon-level completion manifest. `native/manifest.json` (`NativeManifest`) is a small version/protocol stamp used by compatibility and acceptance-audit tooling — not a Done/Total authority. `scripts/acceptance_audit.go` reads the stamp, then counts exact files on disk.

Read path for a paint: mem LRU → gunzip disk → cold encode.

### 5. Sprite rendering

**Native paint (cells-then-DCS):** Glance/Art keep cell-buffer truth for chafa/halfblock. Native (Sixel/Kitty/iTerm) reserves blank themed cells; `compositorWriter` on `tea.WithOutput` appends CUP+DCS **after** each completed cell frame so shared art/stats rows cannot punch mid-sprite. Warm progress stays in the View footer.

**Manual RC (Windows Terminal):** `poke` — scroll warm + cold list; Image Cache progress visible; open dialog and toggle Art; mid-band through sprite must stay gone.

1. Selection change → `bumpSelection` blanks image VM, `ClearNative()`, clears paint identity (`nativePaintANSI` / species) but **keeps** `nativePaintCells` as a geometry token, marks FollowUp dirty. `glanceHoldID` may keep previous **text/stats** until the new selection is commit-ready (sprite ready **and** About can fill — or no snapshot / new core settled with error); it does **not** sticky-preserve prior graphics, and it is cleared when the new id is 0 (empty search). About uses a **last-good snapshot** (`glanceAbout*`) so cold fast scroll survives hold-ID churn and `resources` LRU eviction — not only `resources[glanceHoldID].HasCore`. List cursor moves immediately; art shows themed blank reservation (no browse spinner). Stats stay put via `browseStats` / held core; About must not blank between two species that had/have core.
2. Not-ready dirty → `emitNativeClear` (themed space-clear art rect, zeros geometry) — **only when a frame was actually painted** (skipped on startup / already-clear), and **spaces only**: no solid-Sixel `ClearNativeRegion` fill (encoding an opaque rect per uncached selection was pure lag; a cell write already erases Sixel on WT). Same cell bucket may keep the last frame while a **selected** species is still loading. An empty list always clears, even when the bucket matches. Ready dirty → immediate `emitNativeOverlay`: **same cell bucket** overwrites in place (DCS only); **first paint / size change / post-wipe** space-clears then DCS. View-changing dirties set `nativeNeedsFollowUp` so the first paint also arms one ~60ms insurance re-paint — not a defer-first 16ms wait.
   - **Browse dossier-core arrival** repaints the overlay **once, in place** via `markNativeOverlayDirty` (no `FollowUp`) and only while a native sprite is active. Core adds genus/About cells outside the art box; the Sixel payload is unchanged, so it must not arm a second, delayed ~60ms redraw (that late re-paint per newly-browsed species read as "much slower"). `origin/main` did not repaint on core at all; this is the minimal safe version.
3. Stable-frame gate (`nativeOverlayReady`): native image ready for the selected species and sprite shown. Text hold does **not** block Raw — sixel may paint while held About remains; text commits when sprite+About are ready together.
4. `ensureSpriteLoad` / `runColdSelection`: `LookupCached` at `currentSpriteCells()` (Glance `16` or Art `40`). Hit applies immediately; cell art appears in View on apply; native marks FollowUp for immediate Raw + insurance.
5. Miss → `tui.coldRunner` latest-selection-wins. Right-pane text may stay on the held species until sprite **and** About commit policy allow (no spinner).
6. Fallback chain: Kitty/iTerm/Sixel (`image`) → chafa → halfblock → placeholder. Cell art in View is buffer-safe; native frames use reservation + Raw.
7. Native paint seq: DECSC → (optional theme space-clear + slack) → CUP + `?8452h` + DCS + `?8452l` → DECRC. Space-clear only on geometry/identity wipe; same-geometry paints overwrite. Help/dialog dismiss invalidates geometry so restore clears. Footer progress uses `tea.Raw` (`footer_raw.go`) while native graphics are on screen for the selection.

**Manual RC (Windows Terminal):** scroll a warm list — same-bucket overwrite (no pre-DCS space-clear flash); immediate Raw + ~60ms insurance. Cold neighbor: previous glance **text** held until sprite+About commit; **About** keeps last-good snapshot through cold fast scroll (10+ Downs through uncached species) until the selection gains core — must not blank after leaving the last warm mon; sixel may paint while text lags. Dossier/help dismiss still space-clears then restores art. No spinner tick churn.
8. Chafa-only terminals may run `prewarm.NewChafa` with the same `[16,40]` phase policy. Native terminals do not start chafa warm (CPU contention).

### 6. TUI event processing

`Model.Update` → `update` → `scheduleNativeOverlay`.

Message kinds: `tea.WindowSizeMsg`, keys, `spriteMsg`, `coldFrameMsg`, dossier core/extras, `warmProgressMsg`, `nativeOverlayMsg`, `tea.RawMsg` (ack after overlay write).

Warm channel: `listenWarmProgress` is a recursive tea.Cmd on `<-chan WarmProgress`. Closed channel clears footer.

Keys: typing filters; `v` cycles Glance/Art browse views; `q` quits only when search empty; `Ctrl+C` always quits; Enter/`i` opens dialog (dossier load).

### 7. Shutdown

1. Quit key → `m.quitting` + `tea.Quit`.
2. `tui.Run` after `p.Run()` calls `Model.shutdown()` → `coldRunner.Stop`.
3. `openBrowse` defers: `engine.ClosePersist()`, `nativeWarmer.Stop()`, `artworkWarmer.Stop()`, `spriteWarmer.Stop()`, `dossierWarmer.Stop()`, `cancel()` on `runCtx`.
4. Warmers cancel worker ctx and `WaitGroup.Wait`. Persist queue drains/closes.
5. Dataset bootstrap / setup cancel via their own context + `Ctrl+C`.
6. Process exit: no explicit HTTP close; `pokeapi.Client` uses standard `http.Client`.

---

## Build tags and extra packages

| Tag / file | Effect |
| --- | --- |
| `//go:build rendertest` | `internal/cli/rendertest_cmd.go`, `internal/app/rendertest.go` — `poke test` diagnostic |
| `//go:build !rendertest` | stubs: `registerTestCommand` no-op; `RunRenderTest` unavailable |
| `//go:build ignore` | `scripts/acceptance_audit.go`, `scripts/capture_browse_view.go`, `scripts/gen_typeseed.go` — not in `./...` |
| `scripts/diag_sixel.go` | `package main`, no ignore tag — **is** a second main in `scripts/` if `go test ./scripts` is ever run; normally not in `./...` because Makefile uses `./...` from module root which includes `scripts` if it is a package. `diag_sixel.go` is `package main` without ignore — `go test ./...` may try to build it as a main package. |

No `GOOS`/`GOARCH` build-tagged source files. Windows path handling is runtime (`filepath`, `cli.InvocationName` strips `.exe`).

Embedded: `internal/theme/themes/*.yaml`, `internal/cache/typeseed/types.json`.

---

## Package and file inventory

Dependencies listed are **internal** imports unless noted. Callers are production (non-test) users.

---

### `cmd/poke` — process entry

#### `cmd/poke/main.go`

- **Responsibility:** `main`; `os.Exit(app.Main(context.Background(), app.OSRuntime()))`.
- **Types / funcs:** none exported.
- **Deps:** `internal/app`.
- **Callers:** OS / `go build ./cmd/poke`.
- **Concurrency:** none.
- **Tests:** none (covered via `internal/app`, `internal/cli`).
- **Ownership:** thin; all product logic below.

---

### `pkg/version`

#### `pkg/version/version.go`

- **Responsibility:** linker-stamped `Version`, `Commit`, `Date`.
- **Types / funcs:** `Info`, `Current()`, `Info.String()`.
- **Deps:** none internal.
- **Callers:** `internal/cli` (`version` command + cobra Version), `internal/pokeapi` (User-Agent fallback).
- **Concurrency:** none (immutable vars).
- **Tests:** none.

---

### `internal/app` — orchestration

**Package deps:** `cli`, `config`, `cache`, `debuglog`, `domain`, `output`, `pokeapi`, `search`, `theme`, `themeui`, `tui`, `tui/preload`, `tui/setup`, `tui/style`, `usererr`, `render/sprite` + `chafa`, `halfblock`, `image`, `placeholder`, `prewarm`.

**Callers:** `cmd/poke`, tests.

**Concurrency:** `lazyServices` `sync.Once`; browse `runCtx` cancel; `sendWarmProgress` non-blocking channel send; `throttledProgress` mutex.

#### `app.go`

- **Responsibility:** `Runtime`, `Execute`, `Main`, exit codes, lazy services, theme apply before TUI.
- **Types / funcs:** `Runtime`, `OSRuntime`, `Execute`, `Main`, `ExitCode`, `ApplyTheme`, `lazyServices`.
- **Tests:** `app_test.go` (execute/exit).

#### `contracts.go`

- **Responsibility:** `Clock` / `SystemClock` for tests and timestamps. One-implementation `Cache` / `PokeAPI` interfaces were removed; orchestration uses `*cache.Service` and `*pokeapi.Client`.

#### `bootstrap.go`

- **Responsibility:** `Services`, `BuildServices`, `LoadServices`, CLI cache commands (`Clear`, `Update`).
- **Types / funcs:** `Services`, `cacheCommands`, `FixedClock`.
- **Concurrency:** none in this file (delegates to cache).
- **Tests:** `bootstrap_test.go`.

#### `dataset_bootstrap.go`

- **Responsibility:** terminal vs CLI progress for `poke refresh` (dataset rebuild plus artwork sync).
- **Types / funcs:** `cacheUpdateProgress`, `setupUpdateRun`, `throttledProgress`, `formatBootstrapProgress`.
- **Ownership:** terminal path is `setup.Run` (`Ask: false`) and `preload.Bar`. CLI strings stay in `formatBootstrapProgress` and are not the TUI stage sentences.
- **Tests:** `dataset_bootstrap_test.go`.

#### `handler.go`

- **Responsibility:** `RunRoot`, `Search`, `Random`, inline print, `ensureIndex` with typed `DatasetStage` stderr progress.
- **Types / funcs:** `Handler`, `BrowseRequest`, `IntnRand`, `FixedRand`.
- **Tests:** `handler_test.go` (external `app_test`).

#### `browse.go`

- **Responsibility:** wires engine, warmers, progress channel, `tui.Run`. Adapters `tuiDetails`, `tuiSprites`, `warmPokemonLoader`. `sendWarmProgress` plus `nativeWarmToTUI` / `assetWarmToTUI` / `spriteWarmToTUI` / `dossierWarmToTUI` classify subsystem progress → `tui.WarmProgress`. Does not import `cache`.
- **Tests:** `browse_test.go` (`sendWarmProgress` completion must not drop).

#### `setup.go`

- **Responsibility:** first-run questions and any missing Pokédex/pictures, including after cache clear. `prepareWork` feeds `preload.Bar` (`downloadReserve` floors). `Ask` selects the question steps. Blocking artwork warm via `prewarm.NewArtwork`; `SaveConfig` only when setup asked.
- **Tests:** none dedicated (screen tests in `tui/setup`).

#### `details.go`

- **Responsibility:** `DetailService` — cache-backed dossier/core/extras/inline detail. Marks interactive priority on ctx.
- **Types / funcs:** `PokemonDetail`, `LevelUpMove`, `NewDetailService`, `Load`, `LoadDossier*`, `PokemonForSprite`, `WarmCore`, `Evolution`.
- **Concurrency:** none itself.

#### `theme.go`

- **Responsibility:** `Handler.Theme` launches `themeui.Run` and persists `theme.name`.

#### `errors.go`

- **Responsibility:** `NotFoundError` with `ExitCode() int`.

#### `rendertest.go` (`//go:build rendertest`)

- **Responsibility:** `RunRenderTest` visual/timing compare.

#### `rendertest_stub.go` (`//go:build !rendertest`)

- **Responsibility:** stub `RunRenderTest` error.

#### `rendertest_types.go`

- **Responsibility:** `RenderTestRequest` shared by stub and real impl.

---

### `internal/cli` — command surface

**Deps:** `config`, `pkg/version`, cobra. No cache/render imports in production files.

#### `root.go`

- **Responsibility:** cobra tree: root, version, config, cache (`path`/`clear`), refresh (dataset+artwork), update (GitHub Releases self-update), search, random, theme; `--debug`; `InlineMode` typed enum.
- **Types / funcs:** `InlineMode`, `RootRequest`, `RootHandler`, `Options`, `CacheCommands`, `NewRoot`, `InvocationName`.
- **Tests:** `root_test.go`, `cache_test.go`, `search_random_test.go`.

#### `handlers.go`

- **Responsibility:** `SearchHandler`, `RandomHandler` interfaces + func adapters.

#### `rendertest_cmd.go` / `rendertest_stub.go`

- **Responsibility:** register `poke test` only with `-tags rendertest`.

---

### `internal/config`

#### `config.go`

- **Responsibility:** YAML schema, defaults, validation, `EffectiveSixelColors` / `EffectiveSpriteMode` / artwork + prewarm helpers, `Load`/`Save`/`Initialize`/`Edit`, `DirectoryResolver` (`os.UserConfigDir` / `UserCacheDir` + `pokedex`).
- **Types / funcs:** `Config`, `Setup`, `Cache`, `Render`, `TUI`, `Theme`, `Network`, `Duration`, `Paths`, `DirectoryResolver`, `CommandRunner`, `ExecRunner`.
- **Concurrency:** none.
- **Tests:** `config_test.go`, `setup_test.go`.
- **Ownership:** artwork consent/mode are typed constants; sprite prewarm/dossier prewarm remain strings (`all_pixel|window|off`, `all|window|off`).

---

### `internal/domain`

#### `models.go`

- **Responsibility:** `Pokemon`, `Species`, `BrowseEntry`, `SpeciesSummary`, `Dossier`, evolution/move/encounter types. No I/O.
- **Callers:** almost every internal package.
- **Tests:** none in-package (mapped in dataset/pokeapi/output tests).

---

### `internal/dataset`

**Deps:** `domain`. **Callers:** `cache` (bootstrap + runtime load).

#### `layout.go`

- Aggregate filenames + `SchemaVersion`.

#### `models.go`

- Compact JSON structs + `Builder`.

#### `map.go`

- Domain ↔ dataset mapping.

#### `learnset.go`

- Level-up learnset extraction (latest version group).

#### `validate.go`

- Completeness checks before activate.

**Tests:** `map_test.go`, `learnset_test.go`, `validate_test.go`.

---

### `internal/pokeapi`

**Deps:** `domain`, `debuglog`, `pkg/version`.

#### `client.go`

- **Responsibility:** REST v2 client, retries, 429 backoff, list pagination, sprite GET. Idle pool sized for artwork warm.
- **Concurrency:** `http.Client` concurrent-safe; sleeper injectable.

#### `dto.go`

- Unexported API DTOs.

#### `map.go`

- DTO → domain.

#### `errors.go`

- `RateLimitError`, `StatusError`, `ErrOffline`, `ErrNotFound`.

**Tests:** `client_test.go` (httptest).

---

### `internal/cache` — persistence and metadata orchestration

**Deps:** `config`, `dataset`, `debuglog`, `domain`, `pokeapi` (bootstrap only), `render/sprite` (keys, CDN URL, PNG validate), `typeseed`.

**Callers:** `app`, `usererr`. Rendering warmers do not import `cache`. TUI browse and setup do **not** import `cache`.

**Concurrency:** `Service.sema` HTTP slots; `singleFlight`; `inFlight` atomic; `localOnly` atomic; `Store` RWMutex (active id + snapshot); bootstrap worker pools; `BackgroundRefresh`/`BackgroundEnrich` goroutines; `prewarm` workers.

#### `service.go`

- **Responsibility:** cache-first reads, `EnsureIndex`, `Refresh`, `EnrichTypes`, `BootstrapDataset` entry, concurrency budget, `DatasetProgress` / `DatasetProgressFunc`.
- **Ownership:** dataset/enrich/bootstrap progress only. Warmers own their own `Progress`. TUI does not import this type.

#### `store.go`

- File-backed store: index, sprites, artefacts, native gzip, active dataset pointer, in-memory snapshot cache.

#### `dataset.go` / `dataset_runtime.go`

- Snapshot layout, activate, load browse/aggregates into `loadedSnapshot`.

#### `bootstrap.go` / `moves_bootstrap.go` / `completeness.go`

- Parallel fetch + `DatasetBootstrapReport`. Stages `"index"|"records"|"evolution"|"moves"|"dataset"`.

#### `artwork_sync.go`

- Selective official-artwork sync for `poke refresh`.

#### `asset.go` / `sprites.go` / `sprite_meta.go` / `paths.go`

- Named PNG object store (`artwork/objects`, mappings), legacy URL sprites, slug sanitization.

#### `native_frame.go` / `native_manifest.go`

- `.sixel.gz` gzip, legacy `.frame` purge. `NativeManifest` (`native/manifest.json`) is a small version/protocol stamp for compatibility and acceptance-audit tooling. It is **not** a completeness/Done-Total index; `scripts/acceptance_audit.go` uses its protocol only while counting exact filesystem artefacts. Runtime hits and prewarm native progress use exact content-addressed file existence.

#### `dossier.go` / `detail.go` / `aggregate.go`

- Dossier assembly from snapshot; species/pokemon/evolution/move/encounter reads.

#### `atomic.go`

- Temp+rename writes; schema version comments.

#### `errors.go` / `priority.go`

- Sentinel errors; `WithInteractivePriority` so warmers yield HTTP slots.

#### `cache/typeseed/typeseed.go`

- Embedded national-dex type hints. `Apply` fills missing types. `sync.Once` load.

#### `render/sprite/prewarm`

- See `prewarm/prewarm.go` (core) and `kinds.go` (dossier hook: stage `"dossier"`).

**Tests (selected):** `cache_test.go`, `bootstrap_test.go`, `bootstrap_cancel_test.go`, `dataset_test.go`, `dataset_corruption_test.go`, `completeness_test.go`, `artwork` via `sprites_test.go` / lifecycle, `native_gzip_test.go`, `native_manifest_test.go`, `race_test.go`, `dossier_test.go`, `moves_bootstrap_test.go`, `schema_reset_test.go`, `acceptance_spec_test.go`, `refactor_fix_test.go`, `typeseed/typeseed_test.go`.

---

### `internal/search`

#### `index.go` / `normalize.go` / `rank.go`

- In-memory index, slug normalize, rank. No concurrency.
- **Callers:** `app` (handler + browse), `tui.Model`.
- **Tests:** `search_test.go`.

---

### `internal/render/sprite` — encode + artefact cache

**Subpackages:** `sixel`, `image`, `chafa`, `halfblock`, `placeholder`, `prewarm`. Sibling: `internal/render/termimg`.

**Callers:** `app/browse`, `app/setup`, TUI (via `SpriteRenderer` + `coldRunner`), cache (CDN helpers, keys).

#### `engine.go`

- Orchestrates resolve → mode → mem/disk → cold encode. Native warming uses phased browse buckets (`SetBuckets` / active phase + focus `SetCells`) through `NativeWarmRender`.
- **Concurrency:** `sync.Once` mem caches; persist queue goroutine.

#### `native_cache.go` / `memcache.go` / `cache.go` / `persistqueue.go`

- Native key + mem LRU (decompressed bytes) + artefact helpers + async gzip writer (`nativePersistQueueCap=2`).

#### `source.go` / `asset.go` / `cdn.go` / `resolver.go` / `options.go` / `frame.go` / `resize.go` / `imageprep.go`

- PNG resolve (named store → HTTP), asset refs, CDN URLs, mode fallback, typed `RenderMode`/`ColorMode`/`SpriteSource`, frame buckets `{16,24,32,40}` (browse warm phases `16` then `40`), resize quality.

#### `capabilities.go` / `activity.go` / `metrics.go` / `renderer.go`

- Env capability detect; `ActivityGate` atomics; process counters; `Renderer`/`Frame` interfaces.

**Tests:** `engine_test.go`, `engine_batch_test.go`, `engine_bench_test.go`, `memcache_test.go`, `persistqueue_test.go`, `frame_test.go`, `source_resolve_test.go`, `imageprep_test.go`, `resize_quality_test.go`, `capabilities_test.go`.

#### `sixel/sixel.go`

- Sixel encode (mattn/go-sixel + quant). Palette 64/128/256.
- **Tests:** `sixel_test.go`.

#### `image/renderer.go`

- Native protocol renderer (termimg + sixel). Session disable on fail.
- **Tests:** `renderer_test.go`, `sixel_profile_test.go`.

#### `chafa/` — `detect.go`, `renderer.go`

- External `chafa` binary or in-process via `uiimage`.
- **Tests:** `renderer_test.go`.

#### `halfblock/renderer.go` / `placeholder/renderer.go`

- Cell-art fallback and label placeholder.
- **Tests:** `halfblock/renderer_test.go`, benches.

#### `termimg/termimg.go`

- Isolates `go-termimg` protocol detect.

#### `prewarm/prewarm.go` + `kinds.go`

- One `Service` per `Kind` (`ChafaArtefact`, `NativeFrame`, `ArtworkAsset`, `Dossier`): hint heap + workers + `ActivityGate`, phased buckets, optional disk audit (`Has`), retry backoff. Constructors `NewChafa/NewNative/NewArtwork/NewDossier` supply the `Warm` hook. App classifies progress to `tui.WarmStage*`. `FallbackBytesPerArtwork` drives setup's HD size estimate (no HEAD sampling).
- **Tests:** `warm_test.go`, `integration_test.go`, `bench_test.go`.




- **Tests:** `warm_test.go`.




- **Tests:** `warm_test.go`.

---

### `internal/tui` — browse presentation

**Deps:** bubbles/bubbletea/lipgloss, `domain`, `search`, `debuglog`, `render/sprite`, `tui/style`, `tui/components`, `tui/views`. **Does not import** `cache` (browse or setup). Contract: TUI presentation has no metadata-cache dependency.

**Concurrency:** tea commands (goroutines); `coldFrameCh`; `warmEvents` channel; overlay follow-up timer; no mutex on `Model` (single-threaded tea loop). Warmers run off-loop.

#### `model.go`

- Browse state machine. `WarmProgress.Stage` is `WarmStage`. Footer/reload policy switches on typed constants.

#### `deps.go`

- Injected interfaces: `DetailSource`, `PokemonSource`, `DossierWarmer`, `SpriteRenderer`.

#### `update.go`

- Event switch. Footer vs sprite-refresh policy uses `tui.WarmStage` constants (`WarmStageNative`, `WarmStageArtwork`).

#### `async.go`

- Tea cmds for dossier/sprite; `listenWarmProgress`.

#### `fgrender.go` / `coldbridge.go`

- Foreground cold + scheduler bridge; `shutdown`.

#### `native_overlay.go` / `footer_raw.go` / `warm_footer.go`

- Snappy immediate Raw when ready; same-geometry overwrite (no space-clear); clear+DCS on geometry/identity wipe; `nativeNeedsFollowUp` → one ~60ms insurance. Not-ready → immediate clear. No stdout wrap. Footer **only** renders native stage via Raw while graphics on screen.

#### `image.go` / `resources.go` / `content.go` / `view.go` / `layout.go` / `frame.go` / `keys.go` / `tabs.go` / `run.go`

- Image pane state, dossier cache LRU, dialog text, view assembly, breakpoints, panel split, key map, browse/dialog enums, `Run`.

**Tests:** `model_test.go`, `layout_test.go`, `sprite_test.go`, `artwork_ready_test.go`, `ui_ready_test.go`, `spinner_ux_test.go`, `native_overlay_test.go`, `warm_footer_test.go`, `warm_footer_internal_test.go`, `e2e_acceptance_test.go`, `export_test.go` (test helpers).

#### `tui/preload`

- Shared download pieces: `Lines` / `LineEvery` (jokes), `SpinEvery` / `SpinDots` (ring above the joke; the moving dot is an unstyled `●`), `Bar` (one fraction, larger parts weigh more). No `cache` import.
- **Tests:** `lines_test.go`.

#### `tui/setup/model.go`

- First-run steps (theme, update check, intro) and the download frame. `Ask: false` skips the questions and titles the frame `Refreshing Pokédex`. `Progress` is already summed; `Stage` picks `StageLabel`. No `cache` import.
- **Tests:** `setup_test.go`, `progress_test.go`.

#### `tui/style/colors.go`

- `Palette` + current theme colors. **Tests:** `colors_test.go`.

#### `tui/components/panels.go` / `dialog.go`

- List/search/footer chrome, dialog switcher. `FooterRightWidth` used by raw footer.

#### `tui/views/browse.go`

- Browse layout from `BrowseData` (no Model import).

---

### `internal/output`

- `format.go` — dex/type/slug helpers.
- `table.go` — search table.
- `info.go` / `evolution.go` — inline printers.
- **Callers:** `app/handler`.
- **Tests:** `golden_test.go`.
- **Concurrency:** none.

---

### `internal/theme` / `internal/themeui`

- `theme/theme.go` — load embed + user YAML, `Apply` into `tui/style`.
- `themeui/model.go` — interactive selector.
- **Callers:** `app.ApplyTheme`, `app.Theme`.
- **Concurrency:** none (tea loop).

---

### `internal/debuglog`

- `logger.go` — file logger, `sync.Mutex`, process-global.
- `startup.go` — ordered startup marks for `--debug`.
- `trace.go` — scoped timed traces.
- **Tests:** `logger_test.go`, `startup_test.go`.

---

### `internal/usererr`

- Maps `cache` / `pokeapi` / not-found to user copy.
- **Callers:** `app.Main`.
- **Tests:** `usererr_test.go`.

---

### `internal/textwidth`

- ANSI-aware width/truncate (lipgloss).
- **Tests:** `width_test.go`.

---

### `internal/uiimage`

- Chafa pixel buffer + ANSI generate. Used by `sprite/chafa`, not TUI directly.
- **Tests:** `loader_test.go`.

---

### `scripts/` (tooling, not product)

| File | Role |
| --- | --- |
| `acceptance_audit.go` | `//go:build ignore` — cache/native audit CLI |
| `capture_browse_view.go` | `//go:build ignore` — screenshot helper |
| `gen_typeseed.go` | `//go:build ignore` — regenerate `types.json` |
| `diag_sixel.go` | sixel smoke `package main` (no ignore) |

---

## Known ownership concerns

1. **Duplicate stage copy:** `app.formatBootstrapProgress` (CLI) vs `setup.StageLabel` (TUI) vs `Handler.ensureIndex` stderr. Keep separate; do not unify.
2. **One warm service:** all kinds share `prewarm.Service`; kind differences live only in the `Warm`/`Has` hooks and config flags (`Background`, `Workers`, `ThrottleOnActivity`).
3. **Native protocol stamp:** `NativeManifest` is retained for compatibility and acceptance-audit tooling. It is not a completeness authority and its protocol/version requires separate investigation before any change.
4. **Test-only diagnostics:** rendering diagnostics may retain test-specific call sites; audit them separately rather than widening production ownership.

---

## Test map (by package)

| Package | Test files |
| --- | --- |
| `app` | `app_test.go`, `bootstrap_test.go`, `browse_test.go`, `dataset_bootstrap_test.go`, `handler_test.go` |
| `cli` | `root_test.go`, `cache_test.go`, `search_random_test.go` |
| `config` | `config_test.go`, `setup_test.go` |
| `cache` | many (see cache section); plus `typeseed_test.go` |
| `dataset` | `map_test.go`, `learnset_test.go`, `validate_test.go` |
| `pokeapi` | `client_test.go` |
| `search` | `search_test.go` |
| `sprite` + subs | engine/mem/persist/frame/source/imageprep/resize/capabilities; sixel; image; chafa; halfblock; prewarm |
| `tui` + subs | model/layout/sprite/prefetch/artwork/ui_ready/spinner/overlay/footer/e2e/export; preload; setup; style |
| `output` | `golden_test.go` |
| `debuglog` | `logger_test.go`, `startup_test.go` |
| `usererr` | `usererr_test.go` |
| `textwidth` | `width_test.go` |
| `uiimage` | `loader_test.go` |

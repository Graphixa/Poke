# Pokedex Refactor Plan

## Document purpose

This document defines the implementation plan for refactoring **Pokedex** from a network-driven prototype into a fast, low-footprint, cache-first CLI/TUI application.

The target user experience is:

- the application performs a deliberate first-run data bootstrap;
- all TUI-visible Pokémon metadata is available locally after bootstrap;
- official artwork begins downloading in the background as soon as the TUI can open;
- native image protocols are used where supported;
- sixel and chafa output are generated once per relevant render size and cached;
- future image rendering is a direct cache read and terminal write wherever possible;
- scrolling through the Pokédex does not perform unnecessary network requests, PNG decoding, image conversion, or unbounded memory accumulation;
- expensive work is cancellable, deduplicated, and moved off the hot navigation path.

This is a refactor, not a rewrite. The existing package separation, cache foundations, Bubble Tea UI, content-addressed sprite storage, atomic writes, and renderer abstraction should be retained where they remain useful.

---

# 1. Executive summary

The current prototype has several architectural bottlenecks:

1. **First-run cache population performs excessive writes.**  
   The species index is rewritten repeatedly during type enrichment, creating O(N × full-index-size) disk work and repeated fsyncs.

2. **Derived render cache hits occur too late.**  
   The current render pipeline reads and validates the PNG, then fully decodes the image, before checking whether a cached text/sixel artefact already exists.

3. **Official artwork is not actually being selected.**  
   The current `artwork` source mode only alters image interpolation. It still uses `sprites.front_default`.

4. **Normal detail access remains network-aware.**  
   Individual resources use TTL-based stale checks and may trigger network requests during ordinary browsing.

5. **The TUI retains resources for every visited Pokémon.**  
   This allows memory usage to grow for the lifetime of the session.

6. **Rapid navigation can start work that will later be discarded.**  
   Sprite work is cancellable, but core/detail selection work is not consistently cancelled or debounced.

7. **Renderer responsibilities are blurred.**  
   Backend detection, protocol selection, asset loading, conversion, cache lookup, and terminal emission need clearer boundaries.

8. **Prototype artefacts remain in production code.**  
   Temporary assessment commands and now-obsolete render modes should be moved or removed.

The refactor should establish three distinct layers:

```text
NETWORK / UPDATE LAYER
    ↓
LOCAL DATASET + ORIGINAL ARTWORK CACHE
    ↓
DERIVED RENDER CACHE
    ↓
TUI HOT PATH
```

Normal navigation must operate almost entirely in the bottom two layers.

---

# 2. Architectural principles

The implementation should follow these principles throughout the refactor.

## 2.1 Network activity is dataset installation/update work

The application should not treat ordinary TUI reads as opportunities to refresh arbitrary resources.

After a successful bootstrap:

- the main TUI dataset is local;
- ordinary navigation does not require network access;
- explicit update/refresh operations own network synchronisation;
- background artwork download is the only expected post-TUI-start network activity until bootstrap completes.

This produces predictable behaviour and removes network latency from selection changes.

## 2.2 Original assets and derived assets are separate concerns

Original artwork:

```text
PokeAPI / CDN
    ↓
official artwork PNG
    ↓
content-addressed original asset cache
```

Derived rendering:

```text
original artwork
    ↓
backend-specific conversion
    ↓
cached sixel / chafa / other derived artefact
```

Do not mix these concerns in one cache API.

## 2.3 Cache lookup must precede expensive work

For cacheable derived formats, the rendering pipeline must determine the cache key before:

- reading the original PNG;
- validating the PNG;
- decoding the image;
- resizing;
- quantising;
- dithering;
- invoking a sixel encoder;
- invoking chafa.

A warm sixel render should ideally be:

```text
selection
  → derive key
  → os.ReadFile(cached .six)
  → terminal write
```

## 2.4 The TUI hot path must remain bounded

Scrolling should never create unbounded:

- goroutines;
- network requests;
- cache entries retained in memory;
- image decode work;
- renderer conversions.

The in-memory resource cache should be deliberately bounded.

## 2.5 Keep `go-termimg`, but isolate it

`go-termimg` should remain in the project because the current sixel output works reasonably well in Windows Terminal and it provides useful protocol support.

However, application architecture must not depend deeply on the library.

Treat `go-termimg` as a protocol/encoding dependency behind a narrow adapter.

The application owns:

- backend detection policy;
- session-level backend selection;
- cache keys;
- artefact paths;
- render lifecycle;
- cancellation;
- debounce;
- fallback order;
- TUI state.

`go-termimg` owns only protocol-specific output generation.

This makes future replacement possible without another application-wide refactor.

---

# 3. Target user experience

## 3.1 First run

On first run:

```text
start application
    ↓
cache/dataset missing
    ↓
bootstrap required
    ↓
download all required metadata
    ↓
build complete local dataset
    ↓
atomically activate dataset
    ↓
enter TUI
    ↓
start background official-artwork downloader
```

The metadata bootstrap must complete before the normal TUI starts.

The artwork bootstrap does not need to block TUI entry.

## 3.2 Subsequent launches

On later launches:

```text
start application
    ↓
load active dataset manifest
    ↓
load compact browse index
    ↓
detect renderer backend once
    ↓
enter TUI immediately
    ↓
resume missing artwork download if necessary
```

No normal selection should require a metadata network request.

## 3.3 Scrolling

During scrolling:

```text
selection changes
    ↓
text metadata updates immediately
    ↓
image pane becomes blank
    ↓
cancel prior selection image job
    ↓
start short debounce
    ↓
selection stable?
   / \
 no   yes
 |     |
restart derive render key
       ↓
   cache hit?
    /    \
  yes     no
  |        |
emit      begin work
          ↓
      show spinner only if
      work remains visibly slow
          ↓
      generate / cache
          ↓
         emit
```

### Required UX behaviour

- The image pane must be **blank during debounce**.
- Do not display temporary text such as `[Eevee]`.
- Do not flash a spinner for work that completes almost immediately.
- If real work exceeds the spinner threshold, show a Bubble Tea spinner in the image region.
- Textual metadata and list navigation remain responsive while image work proceeds.
- When the user moves again, stale image work is cancelled or its result is ignored by generation ID.

---

# 4. Target renderer chain

The production renderer fallback order should be:

```text
Kitty / iTerm2 native
        ↓
      Sixel
        ↓
      Chafa
        ↓
 built-in half-block
        ↓
   placeholder / none
```

Braille and ASCII should be removed from the normal production path unless a concrete compatibility requirement emerges.

## 4.1 Native Kitty / iTerm2

For native protocols:

- retain `go-termimg` initially;
- detect support once per TUI session;
- avoid repeated feature-cache clearing;
- avoid repeated environment mutation on every render;
- prefer passing cached original PNG content/path without unnecessary preprocessing where possible;
- benchmark before adding a separate derived native artefact cache.

Native protocol generation is expected to be inexpensive relative to sixel.

## 4.2 Sixel

Sixel is a primary supported backend.

Requirements:

- use `go-termimg`/its sixel path initially;
- cache generated sixel output;
- use a maximum 256-colour palette unless profiling/compatibility testing justifies another setting;
- make derived cache keys pixel-dimension aware;
- include all encoder-affecting settings in the cache key;
- write sixel output atomically;
- on cache hit, do not touch the PNG.

## 4.3 Chafa

Chafa is an external fallback backend.

Requirements:

- resolve `chafa` availability once at application/TUI startup using `exec.LookPath`;
- do not execute it repeatedly for capability probing;
- run chafa only on derived-cache miss;
- cache chafa ANSI output;
- include relevant chafa arguments/version identifier in the artefact cache key;
- cancel the process if the current selection becomes stale;
- fall back cleanly if chafa is unavailable.

## 4.4 Half-block

Retain one small built-in text renderer as the no-external-dependency fallback.

Half-block is sufficient.

Its responsibilities should be deliberately narrow:

- no protocol detection;
- no asset downloading;
- no cache policy;
- only convert an already supplied image into ANSI output.

---

# 5. Metadata dataset redesign

## 5.1 Goal

All metadata required for normal TUI browsing should be available offline after bootstrap.

The bootstrap should perform network work once, deduplicate dependent resources, write efficiently, then activate the completed dataset atomically.

## 5.2 Define “all data” precisely

For this application, “all data” should mean **all data visible through the current TUI without an explicit online-only feature**.

At minimum:

- species index;
- Pokémon core/default-form records;
- typing;
- base stats;
- dimensions;
- abilities if displayed;
- species text shown by the application;
- evolution chains;
- level-up move metadata required by the current tabs;
- encounter data if the encounters tab is intended to remain fully offline.

Do not indiscriminately mirror every PokeAPI object.

The bootstrap should fetch only the dependency graph the application actually uses.

## 5.3 Bootstrap dependency graph

Recommended sequence:

### Phase A — species discovery

Fetch the species index once.

Build the canonical list of Pokémon/species IDs used by the application.

### Phase B — core records

Fetch concurrently, with a bounded worker pool:

- Pokémon default-form resource;
- species resource.

As records arrive, collect unique referenced IDs for:

- evolution chains;
- moves;
- any other dependent resource required by the TUI.

### Phase C — deduplicated dependent resources

Fetch each unique:

- evolution chain exactly once;
- move exactly once;
- encounter dataset once per relevant Pokémon if encounters are part of the offline contract.

Use maps/sets keyed by resource ID or canonical URL before scheduling.

### Phase D — build browse summary

Produce a compact structure containing fields required on every list selection.

Example conceptual model:

```go
type BrowseEntry struct {
    ID        int
    Name      string
    Types     []TypeID
    HP        int
    Attack    int
    Defence   int
    SpAttack  int
    SpDefence int
    Speed     int
    Artwork   AssetRef
}
```

Exact fields should reflect current TUI requirements.

This browse summary should be inexpensive to load wholly into memory.

### Phase E — commit dataset

Only after all required metadata is complete:

1. finalise manifest;
2. fsync required files;
3. atomically switch the active dataset pointer/manifest;
4. leave the previous dataset untouched until activation succeeds.

---

# 6. Dataset snapshot model

## 6.1 Do not mutate the live dataset during refresh

Build updates beside the active dataset.

Example:

```text
cache/
  datasets/
    2026-08-29T001/
    2026-09-14T001/
  active.json
```

`active.json` points to one completed snapshot.

Update flow:

```text
active snapshot A
      ↓
build snapshot B in temp/staging
      ↓
validate B
      ↓
atomic active manifest replacement
      ↓
snapshot B becomes active
```

If refresh fails, snapshot A remains usable.

## 6.2 Prefer simple storage first

Do not introduce SQLite as part of this refactor unless profiling proves that file-level access is a meaningful bottleneck after the hot-path fixes.

Reasons:

- additional binary footprint;
- additional complexity;
- migration overhead;
- unnecessary for a dataset of this size.

A compact JSON or small packed-file snapshot is sufficient initially.

If per-resource files are retained:

- avoid repeatedly reading the same resource solely to obtain timestamps;
- keep refresh metadata in the dataset manifest;
- avoid per-read TTL logic;
- use compact JSON rather than indented JSON for cache storage.

## 6.3 Manifest

The active snapshot manifest should contain enough information to validate compatibility.

Example:

```json
{
  "schema_version": 2,
  "dataset_version": "2026-08-29T001",
  "created_at": "2026-08-29T12:34:56Z",
  "pokeapi_base": "https://pokeapi.co/api/v2",
  "pokemon_count": 1025,
  "app_data_version": 3
}
```

The exact fields can vary.

Important requirements:

- explicit schema version;
- creation timestamp;
- complete/incomplete state is unambiguous;
- no partially written snapshot can become active.

---

# 7. Fix first-run write amplification

The current enrichment path rewrites the full species index during each worker completion.

This must be removed.

## 7.1 Required behaviour

Workers should update an in-memory structure.

Pseudo-flow:

```go
index := loadSpeciesIndex()

results := make(chan enrichment)

workers:
    fetch Pokémon
    send enrichment result

collector:
    apply result to in-memory index

after all workers:
    StoreSpeciesIndex(index) // once
```

At most:

- periodic progress metadata may be written if truly necessary;
- the full index should not be serialised and fsynced once per Pokémon.

## 7.2 Atomic write helper

Retain atomic writes.

Change cache payload encoding from `json.MarshalIndent` to compact `json.Marshal` unless human-readable cache files are an explicit requirement.

Pretty-printing is unnecessary for runtime cache state and wastes disk space and write bandwidth.

---

# 8. Official artwork support

## 8.1 Current problem

The existing source model exposes only `sprites.front_default`.

`SourceArtwork` therefore does not select official artwork; it only changes interpolation behaviour.

This must be corrected.

## 8.2 DTO changes

Extend the PokeAPI Pokémon DTO to read:

```text
sprites
  └── other
      └── official-artwork
          └── front_default
```

## 8.3 Domain model changes

Replace or extend the sprite model.

Example:

```go
type Images struct {
    PixelFrontDefault  string
    OfficialArtwork    string
}
```

Prefer terminology that distinguishes source assets from rendered sprites.

## 8.4 Mapping changes

Map both URLs into the domain model.

`SourceArtwork` must refer to `OfficialArtwork`.

`SourcePixel` must refer to the pixel sprite.

## 8.5 Missing artwork

If official artwork is missing:

1. fall back to pixel sprite;
2. record source type in the resolved asset metadata;
3. ensure cache keys differ between pixel/artwork assets.

---

# 9. Artwork downloader

## 9.1 Start after metadata activation

The TUI may open once metadata is complete.

Immediately start a bounded background artwork downloader.

Recommended initial concurrency:

```text
2–4 concurrent downloads
```

Make this configurable only if there is a real operational requirement.

## 9.2 Priority model

The selected Pokémon must take priority over bulk background work.

Required behaviour:

```text
background queue:
  #001
  #002
  #003
  ...

user selects #658
        ↓
priority request #658
        ↓
join existing request if already downloading
        OR
place ahead of bulk work
```

## 9.3 Singleflight / deduplication

There must never be two concurrent downloads of the same URL/content request.

Use either:

- `singleflight.Group`, or
- a small internal in-flight registry keyed by asset identity.

## 9.4 Storage

Retain content-addressed storage.

Recommended layout:

```text
cache/
  artwork/
    objects/
      ab/
        abcdef....png
    mappings/
      pokemon/
        0001.json
        0002.json
```

The exact directory hierarchy is flexible.

Key requirement:

- original bytes stored once by content hash;
- Pokémon/source mapping does not require scanning the object store;
- resolving asset metadata should not require reading the PNG.

---

# 10. Asset metadata API

The current asset resolver returns PNG bytes too early.

Introduce a metadata-first API.

Example:

```go
type AssetRef struct {
    PokemonID int
    Source    SourceKind
    Hash      string
    Path      string
    URL       string
    Ready     bool
}
```

A resolver call should be able to answer:

- whether an original asset exists;
- its hash;
- its filesystem path;
- source type;

without reading the image bytes.

Possible interface:

```go
type AssetStore interface {
    ResolveAsset(ctx context.Context, pokemonID int, source SourceKind) (AssetRef, error)
    ReadAsset(ctx context.Context, ref AssetRef) ([]byte, error)
}
```

Derived-cache lookup should use the hash from `AssetRef`.

---

# 11. Derived render cache redesign

## 11.1 Generic terminology

Rename current `Halfblock` cache APIs and directories to generic render artefact terminology.

For example:

```go
RenderArtifact(key RenderKey) ([]byte, bool, error)
StoreRenderArtifact(key RenderKey, data []byte) error
```

Directory:

```text
cache/
  rendered/
    sixel/
    chafa/
    halfblock/
```

## 11.2 Render key

The render key must identify all inputs that affect output.

Conceptual structure:

```go
type RenderKey struct {
    AssetHash       string
    Source          SourceKind
    Backend         Backend
    PixelWidth      int
    PixelHeight     int
    ColourMode      string
    PaletteSize     int
    BackgroundMode  string
    DitherMode      string
    EncoderVersion  string
    AlgorithmVer    int
}
```

Not every backend needs every field, but the serialised key must be deterministic.

## 11.3 Sixel geometry

Sixel is pixel-oriented.

Do not rely only on terminal cell dimensions.

The cache key must include the actual target pixel dimensions used for conversion.

If terminal pixel geometry cannot be determined reliably, choose a stable fixed pixel target per TUI layout and encode that explicit size in the key.

## 11.4 Cache hit contract

A sixel cache hit must satisfy:

```text
0 network requests
0 original PNG reads
0 PNG validations
0 image decodes
0 resizes
0 quantisation
0 dithering
0 go-termimg conversion
1 derived artefact read
1 terminal emission
```

This is a core acceptance criterion.

---

# 12. Renderer engine refactor

## 12.1 Current problem

The engine currently resolves and decodes source image data before checking the derived artefact cache.

Reverse that dependency.

## 12.2 Target pipeline

```text
selection
   ↓
resolve AssetRef metadata
   ↓
select backend
   ↓
determine target geometry
   ↓
build RenderKey
   ↓
lookup derived cache
   ↓
hit ───────────────→ emit
   ↓ miss
read original asset
   ↓
decode / prepare
   ↓
backend encode
   ↓
atomic artefact store
   ↓
emit
```

## 12.3 Backend interface

Recommended conceptual interface:

```go
type BackendRenderer interface {
    Backend() Backend
    Cacheable() bool
    Render(
        ctx context.Context,
        asset AssetRef,
        geometry Geometry,
        options RenderOptions,
    ) ([]byte, error)
}
```

The engine owns cache policy around the renderer.

Alternative interfaces are acceptable if the separation is preserved.

## 12.4 Backend selection

Detect the preferred backend once.

Store it in session/app state.

Do not repeatedly mutate:

- terminal capability environment variables;
- `TERMIMG_BYPASS_DETECTION`;
- go-termimg feature caches.

If a one-time compatibility workaround is still required, isolate it in the backend initialiser and document why.

---

# 13. `go-termimg` integration

## 13.1 Keep the dependency

Do not remove `go-termimg` during the primary refactor.

It is currently useful for:

- Kitty;
- iTerm2;
- sixel;
- platform/protocol compatibility already validated by the project.

## 13.2 Narrow adapter

Create an adapter package, for example:

```text
internal/render/termimg/
```

or:

```text
internal/render/backend/termimg/
```

Only this package should import `github.com/blacktop/go-termimg`.

The rest of the application should depend on internal interfaces/types.

## 13.3 Detection

Perform protocol/capability detection at TUI/application startup.

Persist the result for the session.

Expose a simple application-owned enum:

```go
type Backend int

const (
    BackendNone Backend = iota
    BackendKitty
    BackendITerm
    BackendSixel
    BackendChafa
    BackendHalfblock
)
```

## 13.4 Sixel conversion

On cache miss:

1. read original asset;
2. decode;
3. resize to target pixels;
4. convert using go-termimg/sixel;
5. cache bytes;
6. emit.

On cache hit, go-termimg must not be called.

## 13.5 Future removal

After the refactor is complete, benchmark:

- binary size;
- startup latency;
- sixel cold-render time;
- Kitty/iTerm cold-render time.

Only then decide whether direct protocol emitters are worth implementing.

Dependency removal is not a primary goal if it regresses compatibility.

---

# 14. Chafa backend

## 14.1 Detection

At startup:

```go
path, err := exec.LookPath("chafa")
```

Cache the result.

Do not shell out merely to test availability on every selection.

## 14.2 Rendering

On cache miss:

```text
original artwork
    ↓
chafa subprocess
    ↓
ANSI output
    ↓
atomic derived-cache write
```

Capture stdout directly.

Do not pass through shell parsing.

Use `exec.CommandContext`.

## 14.3 Cancellation

If the user moves selection while chafa is generating:

- cancel context;
- terminate process;
- do not commit partial output;
- discard stale result.

## 14.4 Cache key

Include:

- original asset hash;
- dimensions;
- selected chafa symbol/format mode;
- colour mode;
- background handling;
- an internal chafa-render version.

Optionally include detected chafa major/minor version if output compatibility proves unstable between releases.

---

# 15. TUI data model redesign

## 15.1 Browse summary in memory

Load the compact browse summary once at startup.

Fields used on every selection should not require asynchronous disk reads.

The following should update synchronously from memory:

- Pokémon ID;
- name;
- type;
- primary stats shown in the main pane;
- any other always-visible summary data.

## 15.2 Heavy details remain lazy/local

Keep larger resources on disk and load them when needed:

- encounters;
- long flavour text;
- evolution detail if not visible by default;
- move detail lists;
- other tab-specific content.

These reads are local-only.

## 15.3 Bounded resource cache

Replace the unbounded:

```go
map[int]*resourceCache
```

behaviour with an LRU or similar bounded cache.

Recommended initial capacity:

```text
16 entries
```

Acceptable range:

```text
8–32
```

Choose a default and benchmark.

The cache should retain recently viewed heavy details, not every Pokémon visited during the session.

## 15.4 Image data should not accumulate in the resource cache

Do not keep large rendered strings indefinitely per Pokémon.

The disk derived cache is the source of truth.

The in-memory image pane only needs:

- current selection render;
- perhaps the previous/current small working set if profiling supports it.

---

# 16. Selection debounce and cancellation

## 16.1 Behaviour

Every selection change should:

1. synchronously update text from browse summary;
2. set image pane to blank;
3. increment selection generation;
4. cancel prior selection image context;
5. start/reset debounce timer.

Recommended initial debounce:

```text
50 ms
```

This should be a constant initially, not user configuration.

## 16.2 After debounce

When the debounce fires:

1. confirm generation/selection is still current;
2. resolve asset metadata;
3. derive backend/render key;
4. perform cache lookup;
5. emit immediately on hit;
6. otherwise begin expensive work.

## 16.3 Spinner threshold

Do not start spinner animation during the debounce itself.

The image pane remains blank.

If actual work continues long enough to become visually meaningful, begin spinner animation.

Recommended initial threshold:

```text
100 ms after expensive work begins
```

A range of roughly 80–120 ms is acceptable.

This threshold prevents spinner flicker on quick cache misses.

## 16.4 Image state model

Use an explicit state machine.

Recommended:

```go
type ImageState int

const (
    ImageBlank ImageState = iota
    ImageLoading
    ImageReady
    ImageUnavailable
)
```

Internally, debounce may remain separate from the visual state.

Alternatively:

```go
type ImageState int

const (
    ImageIdle ImageState = iota
    ImageDebouncing
    ImageLoadingCache
    ImageGenerating
    ImageReady
    ImageUnavailable
)
```

The rendered view rules must still be:

- debounce → blank;
- short loading → blank;
- slow loading → spinner;
- ready → image;
- unsupported/error → fallback/placeholder.

## 16.5 Cancellation scope

Selection-specific context must cover all work triggered solely by the current selection:

- asset priority download;
- cache lookup where appropriate;
- original file read;
- PNG decode;
- resize;
- sixel conversion where context support permits;
- chafa subprocess;
- heavy tab detail load where it is no longer relevant after selection changes.

Generation IDs should remain as a second line of defence against stale results.

---

# 17. Bubble Tea integration

## 17.1 Spinner

Use Bubble Tea's spinner component in the image pane.

Only tick the spinner while:

```text
ImageState == ImageLoading
```

Avoid permanent spinner subscriptions when no loading state exists.

## 17.2 Messages

Suggested message types:

```go
type selectionDebounceMsg struct {
    Generation int
    PokemonID  int
}

type spinnerDelayMsg struct {
    Generation int
    PokemonID  int
}

type imageReadyMsg struct {
    Generation int
    PokemonID  int
    Content    string
    Backend    Backend
}

type imageErrorMsg struct {
    Generation int
    PokemonID  int
    Err        error
}
```

Exact naming may follow project conventions.

Every asynchronous result must contain enough identity to reject stale work.

## 17.3 Rendering

The image pane render switch should remain simple:

```go
switch m.image.State {
case ImageBlank:
    return blankImageRegion()
case ImageLoading:
    return centredSpinner()
case ImageReady:
    return m.image.Content
case ImageUnavailable:
    return fallbackContent()
}
```

Blank region sizing must be stable so the layout does not jump.

---

# 18. Optional adjacent prewarming

Do not implement this in the first pass.

After the core architecture is benchmarked, consider low-priority prewarming of:

```text
selected - 2
selected - 1
selected + 1
selected + 2
```

Only do this when:

- the selected image is ready;
- no foreground render is pending;
- no high-priority artwork download is pending.

Prewarming must be cancellable and lower priority than interactive work.

If the base derived-cache path is already fast enough, omit this feature entirely.

---

# 19. Cache directory proposal

The exact paths may be adjusted, but responsibilities should be evident from layout.

```text
<UserCacheDir>/pokedex/
  active.json

  datasets/
    <dataset-id>/
      manifest.json
      browse.json
      pokemon/
      species/
      evolution/
      moves/
      encounters/

  artwork/
    objects/
      <content-hash>.png
    mappings/
      <pokemon-id>-artwork.json
      <pokemon-id>-pixel.json

  rendered/
    sixel/
      <render-key>.six
    chafa/
      <render-key>.ansi
    halfblock/
      <render-key>.ansi
```

The current OS-specific `UserCacheDir`/`UserConfigDir` handling should be retained.

On Windows this correctly maps into the expected application cache area.

---

# 20. File-by-file implementation plan

The filenames below reflect the current project structure. Programmers may introduce new files where cleaner.

## `internal/domain/models.go`

### Change

- replace/extend `Sprites` with explicit source-image metadata;
- add official artwork URL;
- keep pixel sprite URL where needed;
- add compact browse model if domain ownership is appropriate.

### Acceptance criteria

- official artwork and pixel sprite are distinct domain fields;
- `SourceArtwork` cannot accidentally resolve to `FrontDefault`.

---

## `internal/pokeapi/dto.go`

### Change

Add DTO fields for:

```text
sprites.other.official-artwork.front_default
```

### Acceptance criteria

Fixture/test verifies mapping for a Pokémon with official artwork.

---

## `internal/pokeapi/map.go`

### Change

Map the new image source fields into domain objects.

### Acceptance criteria

Unit test covers official-artwork URL mapping.

---

## `internal/cache/service.go`

### Change

Refactor bootstrap/update workflow.

Remove full species-index persistence from individual enrichment jobs.

Separate:

- dataset bootstrap/update;
- normal local reads;
- artwork download lifecycle.

### Acceptance criteria

- bootstrap performs O(N) resource writes;
- species/browse index is finalised once rather than once per Pokémon;
- normal TUI reads do not trigger metadata refresh.

---

## `internal/cache/atomic.go`

### Change

- retain atomic temp-write/rename behaviour;
- switch runtime cache JSON to compact encoding;
- keep fsync semantics where required for snapshot activation.

### Acceptance criteria

No pretty-printed runtime cache unless intentionally requested.

---

## `internal/cache/store.go`

### Change

Separate APIs into:

- dataset snapshot access;
- original asset access;
- derived artefact access.

Remove per-resource TTL dependence from normal read flow.

### Acceptance criteria

Reading an active local resource does not require a second file read for freshness metadata.

---

## `internal/cache/detail.go`

### Change

Convert detail access to local-first/local-only under the active dataset.

Network fetches belong to bootstrap/update services, not ordinary detail getters.

### Acceptance criteria

With metadata bootstrap complete and network disabled, every TUI tab intended to be offline still functions.

---

## `internal/cache/sprites.go`

### Change

Rename/generalise.

Introduce:

- original asset metadata lookup;
- original asset byte read;
- generic rendered artefact load/store.

Avoid reading PNG when only content hash/path is required.

### Acceptance criteria

A warm sixel render can reach rendered artefact storage without opening the source PNG.

---

## `internal/app/bootstrap.go`

### Change

Wire dataset bootstrap/update services and artwork manager.

### Acceptance criteria

Application can determine:

- no dataset → bootstrap;
- valid dataset → proceed immediately;
- explicit update → construct and activate replacement snapshot.

---

## `internal/app/browse.go`

### Change

- load active browse summary;
- detect renderer backend once;
- initialise render engine;
- start artwork background manager;
- pass compact local data into TUI.

### Acceptance criteria

No renderer capability detection occurs during ordinary selection changes.

---

## `internal/app/details.go`

### Change

Use active local dataset only for normal detail loads.

Avoid resource calls that can silently reach network.

### Acceptance criteria

Offline mode is deterministic after bootstrap.

---

## `internal/render/sprite/source.go`

### Change

Replace byte-returning eager resolution with metadata-first asset resolution.

### Acceptance criteria

Resolving an already-downloaded artwork asset returns hash/path without loading image contents.

---

## `internal/render/sprite/cache.go`

### Change

- replace halfblock-specific naming with generic render artefact key/storage;
- include backend and pixel geometry;
- increment algorithm/cache version.

### Acceptance criteria

Old incompatible rendered artefacts are naturally invalidated through versioning.

---

## `internal/render/sprite/engine.go`

### Change

Major pipeline inversion:

```text
metadata
→ backend
→ render key
→ derived cache
→ source read only on miss
```

### Acceptance criteria

Warm sixel/chafa renders do not invoke image decoding.

---

## `internal/render/sprite/frame.go`

### Change

Review fixed 24×24 frame assumptions.

Decide one of:

1. retain fixed frame and remove unnecessary size-bucket abstraction; or
2. implement actual responsive sizes and make geometry explicit.

Do not keep a complex bucket system that does not affect real TUI dimensions.

### Acceptance criteria

Render cache geometry reflects real renderer output dimensions.

---

## `internal/render/sprite/imageprep.go`

### Change

Retain only cold-path image preparation.

Ensure no code path calls it on a derived cache hit.

### Acceptance criteria

Benchmarks confirm zero calls during warm sixel render.

---

## `internal/render/sprite/image/renderer.go`

### Change

Refactor into a narrow `go-termimg` adapter.

Remove per-render:

- feature-cache clearing;
- environment mutation where avoidable;
- repeated protocol detection.

### Acceptance criteria

Protocol/backend is initialised once per session.

---

## New: `internal/render/sprite/sixel/`

### Responsibility

Backend-specific sixel cache miss generation.

If `go-termimg` already encapsulates this cleanly, the package may be a small adapter.

### Acceptance criteria

- context-aware generation where possible;
- ≤256 colours by default;
- output returned as bytes for cache/store/emission.

---

## New: `internal/render/sprite/chafa/`

### Responsibility

External chafa process adapter.

### Acceptance criteria

- one-time availability detection;
- `exec.CommandContext`;
- stdout capture;
- clean cancellation;
- no shell execution.

---

## `internal/render/sprite/halfblock/renderer.go`

### Change

Keep as the single internal textual fallback.

Ensure it has no unrelated cache/source responsibilities.

---

## `internal/render/sprite/braille/`

### Change

Remove from production fallback path.

Delete package if no longer required by tests or supported CLI options.

---

## `internal/render/sprite/ascii/`

### Change

Remove from production fallback path.

Delete package if no longer required.

---

## `internal/render/sprite/resolver.go`

### Change

Replace fallback order with:

```text
native Kitty/iTerm
→ sixel
→ chafa
→ halfblock
→ placeholder
```

Prefer session-selected backend rather than recomputing a full chain on every render.

---

## `internal/render/sprite/options.go`

### Change

Ensure `SourceArtwork` now has actual source-selection semantics.

Remove obsolete mode values after compatibility migration.

---

## `internal/tui/model.go`

### Change

Add:

- compact browse summary;
- bounded detail LRU;
- image state;
- current selection generation;
- selection-scoped cancellation;
- spinner state.

### Acceptance criteria

Memory remains bounded after browsing the full Pokédex.

---

## `internal/tui/resources.go`

### Change

Split persistent/compact browse state from lazy detail state.

Do not store rendered output for every Pokémon indefinitely.

---

## `internal/tui/async.go`

### Change

Implement:

- debounce command;
- spinner-delay command;
- selection-scoped render command;
- cancellable lazy detail commands.

### Acceptance criteria

Rapid selection movement does not produce a long tail of obsolete work.

---

## `internal/tui/update.go`

### Change

On selection:

1. update text immediately;
2. blank image;
3. cancel old context;
4. bump generation;
5. reset debounce;
6. launch image workflow after debounce.

Reject every stale asynchronous message.

---

## `internal/config/config.go`

### Change

Simplify renderer options.

Recommended public modes:

```text
auto
kitty
iterm
sixel
chafa
halfblock
off
```

If forcing specific modes is not required, expose fewer values:

```text
auto
image
text
off
```

and keep backend specifics internal.

Do not retain misleading or dead modes.

### Source options

```text
artwork
pixel
```

with `artwork` as the preferred/default source if that is the desired product direction.

---

## `internal/app/sprite_test_cmd.go`

### Change

Remove from the production application package.

Move to one of:

- `_test.go`;
- a developer-only helper command;
- a build-tagged diagnostic utility.

### Acceptance criteria

Temporary assessment code is not linked into the production application path.

---

## `README.md`

### Change

Document:

- first-run metadata bootstrap;
- background artwork population;
- offline guarantees;
- renderer fallback order;
- sixel support;
- optional chafa dependency;
- cache location;
- explicit update command/behaviour.

Do not describe the application as downloading sprites only on demand if that is no longer true.

---

## `go.mod`

### Change

After renderer cleanup:

- run `go mod tidy`;
- inspect `go mod why` for large/transitive dependencies;
- remove dependencies only when no longer used;
- keep `go-termimg` for this refactor.

Do not optimise dependency count before the rendering hot path is corrected.

---

## `.github/workflows/ci.yml`, `Makefile`, build entrypoint

### Change

Resolve current repository/build drift:

- repository `main` must contain the build entrypoint referenced by CI/README/Makefile;
- align Go version across `go.mod`, CI, README, and release workflow.

### Acceptance criteria

A clean clone of `main` builds using documented commands without relying on an uncommitted local file.

---

# 21. Remove normal runtime TTL checks

The active dataset should be treated as a coherent snapshot.

Do not perform:

```text
read resource
→ read resource again for fetched_at
→ decide whether to fetch network
```

during normal application use.

Freshness is a property of the dataset manifest.

If update policy is required, evaluate it at application/update level:

```text
dataset created_at
→ update recommended?
```

The user can still choose to refresh.

A stale but internally complete snapshot is preferable to random individual resources silently updating underneath the TUI.

---

# 22. Concurrency model

Use bounded, role-specific concurrency.

## Metadata bootstrap

```text
worker pool
```

Recommended initial concurrency:

```text
4–8
```

Respect PokeAPI/CDN behaviour and benchmark.

## Artwork background queue

```text
2–4
```

Lower priority than interactive selection.

## Interactive image render

At most one foreground render for the active selection.

New selection cancels previous foreground work.

## Chafa

At most one foreground chafa process per active selection.

Prewarming, if later added, must have separate lower-priority capacity.

---

# 23. Error handling

## 23.1 Bootstrap failure

If no previous active dataset exists:

- show a clear bootstrap error;
- do not enter a partially functional TUI.

If a previous dataset exists and update fails:

- retain previous active snapshot;
- surface non-fatal update failure;
- continue using existing data.

## 23.2 Artwork failure

Artwork failure must not break metadata browsing.

Fallback:

```text
official artwork failure
→ pixel sprite if available
→ renderer fallback / placeholder
```

Background downloader may retry later according to a bounded policy.

## 23.3 Derived artefact corruption

If a cached rendered artefact is unreadable or invalid:

1. remove/quarantine it;
2. regenerate from original asset;
3. atomically replace.

Do not invalidate the original artwork merely because a derived artefact is bad.

---

# 24. Cache invalidation

Use explicit versioning.

## Dataset schema

Increment when local metadata representation changes.

## Render algorithm version

Increment when output-affecting logic changes, including:

- sixel encoder settings;
- resize algorithm;
- palette logic;
- transparency/background behaviour;
- chafa argument set;
- frame geometry.

Do not attempt clever migration of rendered artefacts.

Old derived output is disposable.

---

# 25. Performance budgets

These are target budgets, not guaranteed absolute limits across every machine.

## Warm application launch

After bootstrap:

- no required network request;
- no dataset rebuild;
- browse summary loaded once;
- backend detection once.

## Warm selection, cached sixel/chafa

Required operations:

```text
derive key
read one small rendered artefact
emit
```

Target:

- cache lookup/read comfortably within one frame budget;
- ideally <1 ms filesystem/cache overhead on warm OS cache, excluding terminal output.

## Cold sixel render

One-time work:

```text
original asset read
decode
resize
quantise/dither
sixel encode
atomic cache write
emit
```

This may take tens of milliseconds or more.

It must never repeat for the same cache key unless invalidated.

## Memory

After browsing every Pokémon:

- application memory must plateau;
- detail cache remains at configured LRU capacity;
- no per-visited-Pokémon rendered string retention.

---

# 26. Benchmarks to add

Add targeted Go benchmarks where practical.

## Cache

- load browse summary;
- resolve asset metadata;
- read derived artefact;
- write derived artefact.

## Rendering

- sixel cold render;
- sixel warm render;
- halfblock cold render;
- chafa process timing may be an integration benchmark;
- native protocol cold render if deterministic enough.

## TUI/resource memory

Add a test or profiling scenario that simulates visiting all Pokémon.

Verify:

- LRU entry count remains bounded;
- goroutine count settles;
- retained heap does not grow linearly with number of selections.

---

# 27. Profiling requirements

Before and after the refactor, capture:

```text
go test -bench
pprof CPU
pprof heap
binary size
first-run bootstrap duration
first-run bytes written
warm launch duration
cold sixel generation
warm sixel render
memory after 100 / 500 / all Pokémon selections
```

Do not optimise search, Cobra, YAML, or theme code until profiling shows they are significant.

The current linear Pokémon search over approximately one Pokédex worth of entries is not expected to be the primary bottleneck.

---

# 28. Binary footprint

## Immediate actions

Use normal production linker settings:

```text
-trimpath
-ldflags="-s -w"
```

where appropriate in release builds.

## Do not

- use UPX as the primary optimisation strategy;
- remove useful tests/documentation because they do not affect linked binary size;
- replace working image libraries before measuring their actual binary cost;
- introduce a large embedded database merely to reduce file-open count.

## Later measurement

After the architectural work:

```text
go tool nm
go version -m
go mod why
binary comparison by commit
```

Then decide whether any dependency deserves replacement.

---

# 29. Configuration migration

If existing users have old render values such as:

```text
braille
ascii
```

handle them deliberately.

Options:

1. accept deprecated values for one release and map to `halfblock`;
2. emit a clear config migration warning;
3. automatically rewrite config only if the project already has safe migration infrastructure.

Do not silently behave unpredictably.

---

# 30. Testing plan

## Unit tests

Add/modify tests for:

- PokeAPI official artwork mapping;
- snapshot manifest activation;
- failed snapshot does not replace active snapshot;
- content-addressed artwork mapping;
- render key determinism;
- cache-key invalidation by algorithm version;
- sixel cache hit avoids source asset read;
- chafa cache hit avoids subprocess;
- selection generation rejects stale result;
- LRU eviction;
- source fallback artwork → pixel;
- config migration.

## Integration tests

### Offline TUI service test

Bootstrap fixture data, disable network, confirm:

- browse data loads;
- details load;
- evolution data loads;
- moves load;
- encounters load if contractually offline.

### Renderer test

With fixture PNG:

```text
first render:
  source read occurs
  artefact write occurs

second render:
  artefact read occurs
  source read does not occur
```

### Cancellation test

Start a deliberately slow render.

Change selection.

Verify:

- old result does not replace current image;
- cancellable process/job exits.

## Windows Terminal manual test

Maintain a manual compatibility checklist for:

- Windows Terminal sixel;
- resizing/repeated renders;
- rapid scrolling;
- cache-hit behaviour;
- transparency/background;
- 256-colour output;
- app restart and warm cache.

Also manually test at least one Kitty-compatible terminal and one iTerm2-compatible terminal if available.

---

# 31. Observability and debug logging

Debug logs should make cache behaviour obvious without flooding normal output.

Useful debug events:

```text
dataset.bootstrap.start
dataset.bootstrap.complete
dataset.activate
artwork.download.start
artwork.download.hit
artwork.download.complete
render.backend.selected
render.cache.hit
render.cache.miss
render.generate.start
render.generate.complete
render.cancelled
detail.cache.hit
detail.cache.miss
```

Avoid logging per-pixel/per-cell work.

Include timing values in debug mode where useful.

---

# 32. Implementation phases

The project should be refactored in ordered phases so performance and correctness can be measured after each one.

## Phase 0 — Repository/build hygiene

Tasks:

- restore/confirm the actual `cmd/poke` entrypoint on `main`;
- align Go version in `go.mod`, CI, README, release;
- confirm a clean clone builds/tests.

Exit criteria:

```text
go test ./...
go build ./cmd/poke
```

pass from a clean checkout.

---

## Phase 1 — Dataset write amplification fix

Tasks:

- remove per-Pokémon full-index writes;
- compact cache JSON;
- preserve atomic writes;
- add bootstrap write-count benchmark/instrumentation.

Exit criteria:

- index written once/few times per bootstrap, not once per enrichment;
- substantial reduction in bytes written and fsync count.

---

## Phase 2 — Snapshot-based offline metadata

Tasks:

- introduce active dataset manifest;
- build refresh beside active snapshot;
- fetch/deduplicate required metadata graph;
- generate browse summary;
- make detail reads local-only.

Exit criteria:

- network disabled after bootstrap does not break intended TUI metadata/detail functionality;
- failed refresh leaves previous snapshot intact.

---

## Phase 3 — Official artwork model

Tasks:

- DTO/domain/map changes;
- artwork/pixel source semantics;
- content-addressed original artwork storage;
- metadata-first `AssetRef`.

Exit criteria:

- `SourceArtwork` resolves actual official artwork;
- resolving cached artwork does not read PNG bytes.

---

## Phase 4 — Background artwork manager

Tasks:

- bounded background queue;
- priority selected-Pokémon requests;
- deduplicated downloads/singleflight;
- resume missing assets on later launches.

Exit criteria:

- TUI starts after metadata without waiting for complete artwork set;
- selected missing artwork is prioritised;
- no duplicate download for same asset.

---

## Phase 5 — Render-cache pipeline inversion

Tasks:

- generic render artefact cache;
- deterministic render keys;
- cache lookup before image read/decode;
- version invalidation.

Exit criteria:

- warm derived render performs zero PNG reads/decodes.

This is one of the most important performance milestones.

---

## Phase 6 — Sixel production backend

Tasks:

- isolate `go-termimg`;
- one-time backend detection;
- pixel-dimension-aware cache key;
- ≤256-colour sixel;
- atomic `.six` cache.

Exit criteria:

- Windows Terminal renders correctly;
- first render generates;
- subsequent render directly reads cached sixel;
- restart preserves warm render.

---

## Phase 7 — Chafa + renderer simplification

Tasks:

- add chafa adapter;
- detect once;
- cache ANSI output;
- cancel stale process;
- simplify fallback chain;
- remove Braille/ASCII production modes.

Exit criteria:

```text
native → sixel → chafa → halfblock → placeholder
```

works predictably.

---

## Phase 8 — TUI debounce, blank pane, spinner, cancellation

Tasks:

- selection-scoped context;
- 50 ms debounce;
- blank image pane during debounce;
- spinner only after slow-work threshold;
- generation guards;
- bounded LRU resource cache.

Exit criteria:

- rapid scrolling stays responsive;
- no image placeholder text appears during debounce;
- spinner does not visibly flash for quick cache hits;
- obsolete image work does not update the UI;
- memory plateaus.

---

## Phase 9 — Prototype cleanup

Tasks:

- move/remove `sprite_test_cmd.go`;
- rename legacy halfblock cache APIs;
- remove obsolete render packages/config;
- tidy dependencies;
- update docs.

Exit criteria:

- no temporary production commands;
- no misleading `artwork` semantics;
- no dead renderer modes.

---

## Phase 10 — Profiling and optional refinements

Tasks:

- compare before/after benchmarks;
- inspect binary;
- consider adjacent prewarming only if needed;
- consider direct Kitty/iTerm emitters only if go-termimg remains a measured problem;
- consider alternative storage only if filesystem profiling warrants it.

Exit criteria:

No speculative optimisation remains in the critical plan.

---

# 33. Definition of done

The refactor is complete when all of the following are true.

## Data

- [ ] First run constructs one complete local metadata dataset.
- [ ] Metadata refresh uses snapshot activation.
- [ ] Failed refresh cannot corrupt the active dataset.
- [ ] Normal TUI metadata/detail reads require no network.
- [ ] Bootstrap no longer rewrites the whole species index per Pokémon.

## Artwork

- [ ] Official artwork URL is sourced correctly from PokeAPI.
- [ ] Original artwork downloads in the background after metadata bootstrap.
- [ ] Selected missing artwork receives priority.
- [ ] Duplicate concurrent downloads are prevented.
- [ ] Original assets are content-addressed.

## Rendering

- [ ] Backend detected once per TUI session.
- [ ] `go-termimg` is isolated behind an internal adapter.
- [ ] Windows Terminal sixel remains supported.
- [ ] Sixel output is cached.
- [ ] Sixel cache key includes actual pixel geometry.
- [ ] Sixel defaults to a compatibility-safe ≤256-colour output.
- [ ] Chafa output is cached.
- [ ] Chafa execution is cancellable.
- [ ] Half-block remains as the minimal built-in fallback.
- [ ] Braille/ASCII are removed from the production fallback chain.
- [ ] Warm sixel/chafa render does not read or decode the PNG.

## TUI

- [ ] Text selection updates synchronously from memory.
- [ ] Image pane is blank while debounce occurs.
- [ ] Spinner appears only when actual work remains slow enough to warrant it.
- [ ] Rapid selection cancels stale image work.
- [ ] Stale asynchronous results are rejected.
- [ ] Resource memory cache is bounded.
- [ ] Browsing the entire Pokédex does not cause linear retained-memory growth.

## Repository

- [ ] Clean clone builds from documented entrypoint.
- [ ] Go version is consistent across project metadata and CI.
- [ ] Temporary sprite assessment command is removed from production.
- [ ] README reflects actual bootstrap/cache/render behaviour.
- [ ] `go mod tidy` succeeds.
- [ ] Tests pass.

---

# 34. Non-goals

The following are explicitly outside the first refactor unless profiling demonstrates necessity:

- replacing Bubble Tea;
- replacing Cobra;
- replacing YAML configuration;
- introducing SQLite;
- implementing a custom sixel encoder;
- implementing custom Kitty/iTerm2 protocols;
- aggressive adjacent-image prewarming;
- rewriting search indexing;
- deleting tests or documentation for binary-size reasons;
- adding complex user-facing tuning knobs for worker counts/debounce.

The goal is architectural efficiency, not novelty.

---

# 35. Final target architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                         PokeAPI                             │
└────────────────────────────┬────────────────────────────────┘
                             │
                    explicit bootstrap/update
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    Dataset Builder                          │
│  species → pokemon → dependent resource deduplication       │
└────────────────────────────┬────────────────────────────────┘
                             │
                     atomic activation
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                   Active Local Dataset                      │
│  browse summary + detail resources + dataset manifest       │
└────────────────────────────┬────────────────────────────────┘
                             │
                    TUI starts immediately
                             │
                ┌────────────┴────────────┐
                │                         │
                ▼                         ▼
┌───────────────────────┐       ┌────────────────────────────┐
│  In-memory Browse     │       │ Background Artwork Manager │
│  Summary              │       │ bounded + priority queue   │
└───────────┬───────────┘       └────────────┬───────────────┘
            │                                │
            │                                ▼
            │                    ┌────────────────────────────┐
            │                    │ Original Artwork Cache     │
            │                    │ content addressed          │
            │                    └────────────┬───────────────┘
            │                                 │
            ▼                                 │
┌─────────────────────────────────────────────────────────────┐
│                         TUI                                 │
│                                                             │
│ selection                                                   │
│   → synchronous text update                                 │
│   → blank image pane                                        │
│   → cancel old render                                       │
│   → 50 ms debounce                                          │
│   → derive backend/render key                               │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                 Derived Render Cache                        │
│            sixel / chafa / halfblock                        │
└──────────────────────┬──────────────────────────────────────┘
                       │
            hit ───────┴─────── miss
             │                    │
             ▼                    ▼
      direct file read      original artwork
             │                    │
             │              cold conversion only
             │                    │
             │           ┌────────┼──────────┐
             │           │        │          │
             │         Sixel    Chafa    Halfblock
             │           │        │          │
             │           └────────┴──────────┘
             │                    │
             │             atomic cache write
             │                    │
             └──────────────┬─────┘
                            ▼
                         terminal
```

---

# 36. Expected outcome

After this refactor, the application should behave less like a live PokeAPI browser and more like a locally installed Pokédex dataset with asynchronous asset enrichment.

The critical performance property is:

> **Work that can be done once must not be repeated during scrolling.**

Metadata network access is performed during dataset installation/update.

Artwork network access is performed once per original asset.

Sixel/chafa conversion is performed once per render key.

Warm navigation is reduced to:

```text
memory lookup
+
small local file read
+
terminal output
```

That is the architecture most likely to deliver near-instant scrolling while preserving Windows Terminal sixel support, cross-terminal compatibility, and a manageable implementation footprint.

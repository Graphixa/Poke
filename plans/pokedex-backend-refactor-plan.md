# Pokedex Backend Refactor Plan â€” Sixel Rendering, Lazy Cache, and Opportunistic Warming

> **Target repository:** `Graphixa/Pokedex`  
> **Recommended branch:** `refactor/lazy-sixel-cache`  
> **Base branch:** `main`  
> **Planning baseline:** current `main` around `e08956a6b1a1e5aa2bd85c21f87abfd85f99a006`  
> **Experiment branch:** preserve `experiment/render-cache-lab` as benchmark evidence; do **not** merge it wholesale into production.

---

## Agent operating instructions

This document is intended to be executed by coding agents in order.

**Every task uses a Markdown checkbox. Change `[ ]` to `[x]` immediately when the task is completed and verified. Do not mark a task complete merely because code was written; the listed success criteria must also pass.**

Rules for this refactor:

1. Work phase-by-phase. Do not jump ahead unless a dependency explicitly permits it.
2. Keep commits small and aligned to phases.
3. Do not bring benchmark-only CLI commands, render-lab packages, experimental scripts, or temporary instrumentation into production unless explicitly called for below.
4. Do not merge the experiment branch wholesale. Port only the production decisions established by measurement.
5. Run tests after each phase.
6. Run `go test ./...` before each commit.
7. Run `go vet ./...` before the final PR.
8. Run race tests on a supported host/CI before merge.
9. On Windows PowerShell, format Go recursively with:

```powershell
$files = Get-ChildItem -Recurse -Filter *.go | ForEach-Object FullName
gofmt -w $files
```

Do **not** rely on `gofmt -w .`.

---

# 1. Final architecture decisions

The refactor must implement the following measured decisions.

## 1.1 Sixel render buckets

Supported sixel terminal-cell buckets are exactly:

```text
24
32
40
```

Selection rule:

```text
available < 24  -> no native image / fallback
24..31          -> 24
32..39          -> 32
40+             -> 40
```

Do not restore the historical six-bucket sequence:

```text
20 / 24 / 28 / 32 / 36 / 40
```

Only the **currently required bucket** is proactively warmed in the background.

If the terminal bucket changes, the warmer switches to the newly active bucket. Previously generated buckets remain on disk and may be reused later.

## 1.2 Sixel palette

Production default:

```yaml
render:
  sixel_colors: 64
```

Supported values:

```text
64
128
256
```

Effective-value policy:

```text
missing / 0 -> 64
64          -> 64
128         -> 128
256         -> 256
anything else -> 64
```

Examples:

```text
254 -> 64
65  -> 64
512 -> 64
-1  -> 64
```

An unsupported value must **not** fail application startup, be rounded, be clamped to the nearest supported value, or silently become 256. It falls directly back to 64. A debug/config warning is acceptable, but fallback must be non-fatal.

## 1.3 Persistent sixel storage

Sixel native frames should be stored as **gzip-compressed persistent artefacts**.

Measured #001â€“150 result:

```text
450 frames
raw   ~17.4 MB
gzip   ~9.3 MB
saving ~46.5%
```

Measured gzip disk p95 is around 1 ms and runtime LRU performance is effectively unchanged once warm. Use the straightforward gzip implementation unless profiling demonstrates otherwise. The experiment did not justify a more complicated reusable/pool implementation.

## 1.4 Memory cache

Keep/use a process-local LRU containing renderer-ready, **decompressed** native frame bytes.

Read path:

```text
memory LRU
    -> hit: immediate frame

disk gzip
    -> hit: read + gunzip
    -> put decompressed bytes into memory LRU
    -> immediate frame

true miss
    -> cold render
```

Do not gzip/decompress memory-LRU entries.

## 1.5 Foreground rendering contract

The TUI selection path is non-negotiable:

```text
selection changes
-> text/list/dossier selection updates immediately
-> compute exact native render identity
-> memory cache lookup
-> persistent native cache lookup
-> if hit: display immediately
-> if true miss: schedule cold work
```

For cold work:

```text
maximum active foreground cold renders = 1
maximum latest pending foreground request = 1
FIFO backlog = forbidden
```

When a new cold selection arrives while a render is active, replace the pending request with the newest selection. Do not append to a queue.

When active work finishes:

- if still current, display it;
- if stale, never display it;
- a valid stale artefact may still be persisted/cached because the CPU work has already been paid;
- run the latest pending foreground request next.

Do not add debounce, fast-scroll mode, mouse-wheel special mode, adaptive foreground worker count, or large request queues.

## 1.6 Foreground display and persistence ordering

Persistence must not block first display.

Preferred cold path:

```text
PNG read
-> decode
-> resize
-> sixel encode
-> frame ready
-> frame may be displayed
-> put into process memory cache
-> asynchronously gzip + atomically persist
```

Do not require:

```text
encode -> gzip -> write -> reopen -> display
```

If the application exits before persistence completes, re-rendering the frame next launch is acceptable.

## 1.7 Opportunistic background warming

We **do** want background warming as a UX enhancement. It is no longer required for correctness or baseline responsiveness.

Purpose:

> While the user is reading/browsing a dossier and no foreground cold render is waiting, use otherwise-free time to populate the currently active terminal-size bucket so subsequent scrolling is increasingly likely to hit cache immediately.

Constraints:

```text
only current active bucket
only one simple background stream
foreground has absolute priority
no 4-worker bulk renderer
no adaptive worker pool
no whole-dex x all-sizes generation
```

The background warmer must not proactively generate 24/32/40 together.

Example at bucket 32:

```text
foreground current Pokemon @32
background gradually warms other Pokemon @32
```

After resize to bucket 40:

```text
foreground current Pokemon @40
background switches to warming @40
existing @32 artefacts remain cached
```

## 1.8 Background scheduling

Prefer a simple foreground-priority arbitration model.

The safest implementation is conceptually one expensive-encode lane:

```text
if foreground cold work is waiting:
    run foreground
else:
    run one background warm item
```

If current architecture instead uses separate goroutines, it must still guarantee that background work does not meaningfully compete with foreground work. Measure before accepting that design.

A background item already encoding does not need complex cancellation. It may finish, persist its valid result, then foreground must be selected next.

Do not build explicit â€œuser idle for N millisecondsâ€ heuristics unless proven necessary.

## 1.9 Background warm order

Keep scheduling simple and locally useful.

Recommended initial order after current selection `N`:

```text
N+1, N+2, ... max
then
N-1, N-2, ... 1
```

Alternative acceptable implementation:

1. finish missing entries in the visible list/window first;
2. continue sequentially through the rest of the Dex.

Do not introduce weighted priority graphs or complicated prediction logic.

## 1.10 No speculative neighbour renderer

Do not maintain a separate `Â±1` speculative foreground prewarm mechanism. The opportunistic active-bucket background warmer already serves that purpose without introducing a second scheduling system.

## 1.11 Cache identity

PNG SHA256 remains the canonical source identity.

Native sixel cache identity must include all values capable of changing frame bytes, including at minimum:

```text
source PNG SHA256
protocol = sixel
pixel dimensions / effective bucket
theme/background identity
render/algorithm version
effective sixel palette = 64 | 128 | 256
```

Changing 64 -> 128 must never reuse a 64-colour sixel artefact. Do not reintroduce named-key native identity as canonical storage.

## 1.12 Schema policy

This application is still unreleased/prototyping.

> **Do not implement cache/schema migrations.**

If the native artefact/cache schema changes incompatibly:

- bump the appropriate compatibility/version marker;
- invalidate/delete incompatible derived cache state;
- rebuild cleanly;
- preserve independent user config unless that config itself is invalid.

Do not create v2â†’v3 migration code, dual native-cache readers, legacy compatibility converters, or long-lived fallback file layouts. Destructive cache reset is explicitly preferred over migration complexity.

---

# 2. Performance evidence to preserve

Production code does not need benchmark CLI features, but the engineering decision should remain documented.

Measured reference points:

```text
64-colour sixel:
median encode approximately 25 ms/frame in the palette experiment

warm gzip:
~46.5% storage reduction
~1.06 ms disk p95
runtime LRU effectively unchanged

cold wheel-20, size 32:
selection->frame p50 ~36-39 ms
selection->frame p95 ~44-48 ms
final settle ~29-37 ms, one ~58 ms outlier
average CPU ~1.7%
peak CPU ~14.4%
stale encode CPU median ~13%

size 40 at wheel-20:
cold encode roughly matches 50 ms scroll interval
high stale-work percentage is expected
final selected frame still settles quickly
```

These results justify one bounded foreground renderer, gzip persistence, 64 colours by default, no foreground debounce, and opportunistic background warming rather than aggressive multi-worker bulk generation.

---

# 3. Phase 0 â€” Establish a clean production branch

## Tasks

- [x] Ensure all experiment findings and JSON reports are committed on `experiment/render-cache-lab`.
- [x] Ensure `docs/render-cache-lab-results.md` records the final decisions.
- [x] Do **not** merge `experiment/render-cache-lab` wholesale.
- [x] Checkout current `main`.
- [x] Pull latest `main`.
- [x] Create a clean branch:

```powershell
git checkout main
git pull --ff-only origin main
git checkout -b refactor/lazy-sixel-cache
```

- [x] Record the starting SHA in the PR/refactor notes.
- [x] Run baseline:

```powershell
go test ./...
go vet ./...
```

- [x] Record failures before modifying code.

## Success criteria

- [x] New refactor branch is based directly on current `main`.
- [x] Experiment-only packages/commands are not present unless they already exist on `main`.
- [x] Baseline test state is documented.
- [x] No production code has been changed yet.

---

# 4. Phase 1 â€” Lock render configuration and sizing policy

Relevant existing areas likely include:

```text
internal/config/
internal/render/sprite/frame.go
internal/render/sprite/sixel/
internal/tui/
```

Current `frame.go` still expresses six stepped sizes via `MinFrameSize`, `MaxFrameSize`, and `FrameStep`; replace this with explicit supported buckets.

## 4.1 Replace six-frame stepping with explicit buckets

- [x] Introduce the canonical supported bucket set:

```go
var FrameBuckets = [...]int{24, 32, 40}
```

or an equivalent immutable internal representation.

- [x] Update `FrameSteps()` to return exactly `24,32,40`.
- [x] Update `FrameStepsMaxFirst()` accordingly.
- [x] Update `ClampFrameSide()` so below 24 clamps to 24 for APIs that require a bucket; 24â€“31 -> 24; 32â€“39 -> 32; 40+ -> 40.
- [x] Preserve the special behaviour of `LargestFrameFitting`: it must return `0` if even padded 24 cannot fit.
- [x] Update minimum terminal dimensions to use 24 as the true minimum.
- [x] Remove assumptions that `FrameStep == 4` generates the supported set.
- [x] Update all tests that expect 20/28/36.

## 4.2 Add sixel palette config

Add:

```go
type Render struct {
    ...
    SixelColors int `yaml:"sixel_colors"`
}
```

- [x] Default to 64.
- [x] Add `const DefaultSixelColors = 64`.
- [x] Add a helper equivalent to:

```go
func (c Config) EffectiveSixelColors() int {
    switch c.Render.SixelColors {
    case 64, 128, 256:
        return c.Render.SixelColors
    default:
        return DefaultSixelColors
    }
}
```

- [x] Do not make unsupported sixel colour counts a fatal validation error.
- [x] If warnings are emitted, ensure config load still succeeds.
- [x] Wire the effective value into sixel encoder creation.
- [x] Remove hardcoded `MaxColors = 256` behaviour as the effective production policy.
- [x] Ensure non-sixel renderers are unaffected.

## Tests

- [x] `FrameSteps()` == `[24,32,40]`.
- [x] `LargestFrameFitting()` returns 0 when 24 cannot fit.
- [x] 24â€“31 maps to 24.
- [x] 32â€“39 maps to 32.
- [x] 40+ maps to 40.
- [x] unset `sixel_colors` resolves to 64.
- [x] 0 resolves to 64.
- [x] 64 resolves to 64.
- [x] 128 resolves to 128.
- [x] 256 resolves to 256.
- [x] 254 resolves to 64.
- [x] negative values resolve to 64.
- [x] invalid palette does not prevent startup/config load.

## Success criteria

- [x] No production sixel code assumes six historical buckets.
- [x] Six- vs three-bucket behaviour is covered by unit tests.
- [x] 64 is the effective default everywhere.
- [x] 128/256 are supported opt-ins.
- [x] Unsupported values fall directly back to 64.

---

# 5. Phase 2 â€” Define the new native render key and destructive compatibility boundary

Current native keys include source hash, protocol, pixel dimensions, background and algorithm version. Extend them so palette is explicit.

## Tasks

- [x] Add effective palette to native sixel cache identity.
- [x] Ensure cache identity remains based on canonical PNG SHA256.
- [x] Ensure sixel palette is included only where relevant; do not break unrelated ANSI/chafa cache keys unnecessarily.
- [x] Decide whether protocol-specific render version should be independent from general `AlgorithmVersion`.
- [x] Bump native render/cache version because prior native frames are incompatible with the new identity/storage format.
- [x] On incompatible version/layout detection, invalidate/delete old **derived/native** artefacts.
- [x] Do not migrate historical native artefacts.
- [x] Remove native named-key fallback paths where still present.
- [x] Keep species invalidation hash-driven: resolve stored species asset hashes and invalidate native artefacts derived from those hashes.

## Recommended conceptual identity

```text
sha256(
  source_sha256
  + protocol
  + pixel_width
  + pixel_height
  + background
  + algorithm_version
  + sixel_colors
)
```

The filesystem naming format may remain human-readable or hash-based, but there must be one unambiguous identity source.

## Tests

- [x] Same source/settings -> same key.
- [x] 64 vs 128 -> different key.
- [x] 128 vs 256 -> different key.
- [x] 24 vs 32 -> different key.
- [x] 32 vs 40 -> different key.
- [x] background change -> different key.
- [x] source PNG SHA change -> different key.
- [x] old incompatible native cache is invalidated, not migrated.
- [x] native key parser accepts only the new canonical format expected by production.

## Success criteria

- [x] No 64-colour frame can be served for a 128/256 request.
- [x] No native cache compatibility/migration layer is introduced.
- [x] Source SHA256 is the canonical source identity.

---

# 6. Phase 3 â€” Replace native disk storage with gzip artefacts

Introduce a small, testable persistent native-frame store.

## 6.1 Storage API

Prefer an API around bytes rather than terminal strings where practical:

```go
type NativeFrameStore interface {
    Load(ctx context.Context, key string) ([]byte, bool, error)
    Store(ctx context.Context, key string, frame []byte) error
    Delete(ctx context.Context, key string) error
}
```

Adapt existing interfaces incrementally if a full interface replacement causes unnecessary churn.

## 6.2 File format

Recommended:

```text
<cache>/native/<render-key>.sixel.gz
```

Exact extension is optional, but file purpose should be obvious.

## 6.3 Atomic write

Write:

```text
same-directory temp file
-> gzip
-> flush/close
-> atomic rename
```

Windows behaviour must be tested explicitly. Do not expose partially written `.gz` files.

## 6.4 Read

```text
open
-> gzip.NewReader
-> read decompressed bytes
-> close
-> return decompressed frame bytes
```

Use straightforward gzip initially. Do not introduce `sync.Pool` or reusable gzip-reader machinery unless new profiling demonstrates a production benefit.

## 6.5 Corruption

If a gzip native artefact cannot be decoded:

```text
treat as cache miss
delete/quarantine corrupt derived artefact
regenerate when requested
```

Do not crash the TUI for corrupt derived cache data.

## Tests

- [x] gzip round-trip is byte-identical.
- [x] raw sixel bytes before compression equal decompressed bytes after read.
- [x] corrupted gzip becomes a cache miss.
- [x] corrupted entry does not crash browse.
- [x] atomic store never exposes partial final file.
- [x] concurrent lookup/store of different keys is race-safe.
- [x] same-key collision is handled deterministically.
- [x] Windows path/file semantics are covered where possible.

## Success criteria

- [x] Persistent sixel storage is gzip.
- [x] Compression is lossless.
- [x] Disk cache read remains off the source-PNG/decode/encode path.
- [x] Corrupt derived cache can self-heal through regeneration.

---

# 7. Phase 4 â€” Create a dedicated decompressed native-frame memory LRU

The hot path must not repeatedly gunzip a frame already seen in the process.

## Tasks

- [x] Separate/clarify native memory-frame caching from ANSI/chafa caching if the existing cache mixes semantics.
- [x] Store decompressed sixel frame bytes keyed by the full native render key.
- [x] Make reads concurrency-safe.
- [x] Keep memory bounded by bytes, not merely entry count.
- [x] Preserve or rationalise the existing memory budget rather than adding arbitrary large defaults.
- [x] Invalidate entries by canonical source hash/species asset hash when artwork changes.
- [x] Do not store gzip bytes in the hot memory cache.

## Tests

- [x] first disk read gunzips and populates LRU.
- [x] second read hits LRU without disk access.
- [x] eviction respects byte budget.
- [x] invalidated source hash cannot survive in LRU.
- [x] concurrent access is race-safe.

## Success criteria

Expected lookup hierarchy is structurally explicit:

```text
memory
-> gzip disk
-> cold render
```

and no normal warm lookup reads PNG source data.

---

# 8. Phase 5 â€” Extract a single foreground cold-render scheduler

Do this before changing the TUI extensively. The scheduler should be independently unit-testable.

## Required state

Conceptually:

```go
type ColdScheduler struct {
    active  *Request
    pending *Request
}
```

Exact implementation may use channels/goroutines, but behaviour is what matters.

## Rules

- [x] max one active expensive foreground render.
- [x] max one pending foreground request.
- [x] newest pending request replaces the previous pending request.
- [x] no unbounded channel/queue.
- [x] cache hits do not enter the cold scheduler.
- [x] stale completion is never displayed.
- [x] valid stale completion may be cached/persisted.
- [x] after active completes, latest pending begins immediately.
- [x] final currently selected PokÃ©mon eventually renders.

## Mandatory deterministic tests

- [x] `TestColdSchedulerAtMostOneActive`
- [x] `TestColdSchedulerAtMostOnePending`
- [x] `TestColdSchedulerLatestPendingWins`
- [x] `TestColdSchedulerDoesNotFIFOQueue`
- [x] `TestColdSchedulerStaleResultNotDisplayed`
- [x] `TestColdSchedulerFinalSelectionDisplayed`
- [x] `TestColdSchedulerCachedHitBypassesColdWorker`
- [x] `TestColdSchedulerCachedHitNotDebounced`
- [x] `TestColdSchedulerStaleValidResultCanPersist`
- [x] `TestColdSchedulerWheel20RemainsBounded`
- [x] `TestColdSchedulerBurst40RemainsBounded`

Use barriers/fake renders rather than timing-sensitive sleeps.

Canonical test:

```text
select 1
render 1 blocks

select 2
select 3
select 4
select 5

assert:
active = 1
pending = 5
2/3/4 are not queued

release 1

assert:
next expensive render = 5
```

## Success criteria

- [x] Scheduling correctness can be proven without Bubble Tea.
- [x] No code path can create a growing queue of sixel encodes.
- [x] Mouse wheel event rate does not change scheduler semantics.

---

# 9. Phase 6 â€” Rewire the TUI selection path

Relevant current areas may include:

```text
internal/tui/
internal/uiimage/
internal/render/sprite/
```

## Required selection pipeline

On every selection change:

1. update cursor/list immediately;
2. update text/dossier state immediately;
3. determine active frame bucket from current geometry;
4. derive source/native render key;
5. memory lookup;
6. disk gzip lookup;
7. hit -> display immediately;
8. miss -> request foreground scheduler;
9. while cold work is active, show existing loading/spinner state if needed;
10. stale cold frame must never overwrite current selection.

## Tasks

- [x] Remove any image debounce timer.
- [x] Remove any special keyboard-vs-mouse scheduling difference.
- [x] Mouse wheel and Up/Down must feed identical selection/render semantics.
- [x] Ensure a warm cache hit can update image immediately even if unrelated cold work was previously active.
- [x] Ensure stale completion cannot reintroduce an older PokÃ©mon image.
- [x] Ensure resize recomputes active bucket.
- [x] If resize stays within the same bucket, do not invalidate/re-render needlessly.
- [x] If resize crosses bucket boundary, current PokÃ©mon gets immediate lookup/cold request for new bucket.

## Tests

- [x] cursor changes are not blocked by image generation.
- [x] text selection remains responsive during blocked fake cold render.
- [x] wheel-like rapid event burst leaves only newest pending cold request.
- [x] warm hit renders immediately during fast selection.
- [x] stale frame cannot overwrite current image.
- [x] resize 32 -> 40 requests 40 key.
- [x] resize inside 32 bucket does not regenerate.

## Success criteria

TUI behaviour matches measured cold-scroll assumptions exactly.

---

# 10. Phase 7 â€” Make persistence asynchronous from frame display

After foreground encode succeeds:

```text
frame bytes ready
-> publish result
-> memory cache
-> enqueue small persistence operation
```

Persistence should not become a new unbounded queue.

## Tasks

- [x] Separate `frameReady` from `persistComplete`.
- [x] UI does not wait for gzip write.
- [x] Ensure at most a tightly bounded number of persistence writes exist.
- [x] If a stale render finishes validly, allow persistence.
- [x] On shutdown, best-effort finish/close persistence worker without hanging indefinitely.
- [x] Persistence failure logs/debugs but does not remove already displayed frame.

## Queue policy

Because renders themselves are single-lane and relatively slow, persistence traffic should be naturally bounded. Keep the implementation simple. If a channel is used, use a small explicit capacity and document why.

## Tests

- [x] intentionally slow disk persistence does not delay frame-ready event.
- [x] persistence error does not prevent display.
- [x] stale valid result can persist.
- [x] shutdown does not leak goroutines.
- [x] race detector passes.

## Success criteria

Gzip/write latency is outside the user-visible cold frame-ready critical path.

---

# 11. Phase 8 â€” Implement opportunistic active-bucket background warming

This phase deliberately retains background warming, but replaces the old â€œmake the entire native cache completeâ€ architecture.

## 11.1 Warmer scope

At any moment it has exactly one active target specification:

```text
protocol = sixel
bucket = current terminal bucket
palette = current effective palette
background/render identity = current render spec
```

It warms missing PokÃ©mon for this specification only. It does not warm all 3 buckets, all palettes, or alternative backgrounds.

## 11.2 Foreground priority

Required arbitration:

```text
foreground cold request exists?
YES -> foreground goes next
NO  -> background may encode one missing entry
```

- [x] No 4-worker pool.
- [x] No adaptive worker count.
- [x] No fast-scroll mode.
- [x] No foreground starvation.
- [x] No background starvation requirement; it is opportunistic by design.

## 11.3 Bucket switch

When terminal changes bucket, e.g. 32 -> 40:

- [x] stop scheduling new @32 background entries;
- [x] allow currently active @32 background encode to finish if cancellation is not trivial;
- [x] persist valid finished @32 result;
- [x] switch background target to @40;
- [x] foreground current @40 request has priority;
- [x] retain @32 disk artefacts.

## 11.4 Palette/background/spec switch

Use the same behaviour if the effective render specification changes. Do not rename old cache entries. Their identity remains valid for the prior spec.

## 11.5 Order

Implement a simple useful order.

Recommended from current `N`:

```text
N+1 .. max
N-1 .. 1
```

Skip entries whose exact native key already exists. Do not repeatedly scan the entire native directory for every item. Prefer direct exact-key existence checks and/or an efficient store index if one already exists and is safe.

## Tests

- [x] warmer only creates current bucket.
- [x] warmer does not generate other buckets.
- [x] foreground request pre-empts next background scheduling slot.
- [x] bucket switch changes future warm spec.
- [x] old bucket remains readable.
- [x] palette change changes warm identity.
- [x] cache hit is skipped without PNG decode/render.
- [x] no unbounded work queue exists.
- [x] shutdown cancels/stops cleanly.

## Success criteria

While user reads a dossier, cache coverage can grow. While user scrolls, foreground remains the priority and the warmer cannot create a backlog.

---

# 12. Phase 9 â€” Remove obsolete native-cache completion machinery

This phase is important: delete superseded architecture rather than leaving it disabled.

Identify code whose only purpose was whole-dex native completeness, all-size cache build, multi-worker native warming, native progress totals, per-size completion state, or bulk native worker tuning.

## Candidates to remove/simplify

- [x] all-size native generation loops.
- [x] native worker count settings/constants no longer used.
- [x] four-worker/interactive worker selection.
- [x] completion bookkeeping such as â€œall required native sizes readyâ€.
- [x] â€œBuilding Image Cache: N/1025â€ progress state if it refers specifically to derived sixel generation.
- [x] native warm sessions that require fixed totals.
- [x] per-frame/per-size full-dex completion manifests.
- [x] native background priority levels that are no longer needed.
- [x] directory scans performed merely to determine whole-cache completeness.
- [x] repeated manifest snapshots used only by the old warmer.
- [x] code that assumes a successful warm session eventually creates 24/32/40 for every PokÃ©mon.
- [x] benchmark-driven `InteractiveWorkers` production tuning if no longer used.

Be careful not to remove **source artwork acquisition** behaviour simply because native derived-cache warming changed. Source PNG acquisition and derived sixel generation are separate concerns.

## Success criteria

- [x] Production has one understandable native frame lookup/render path.
- [x] No hidden worker still tries to complete all native sizes.
- [x] No stale UI progress claims full native-cache construction.
- [x] Background warming is best-effort/opportunistic, not a completion transaction.

---

# 13. Phase 10 â€” Simplify native manifest/storage state

Previous architecture had manifest authority/race complexity. The new design should minimise central mutable metadata.

Goal:

> Exact content-addressed render key existence should be enough to answer whether a native frame is cached.

## Tasks

- [x] Audit all native manifest reads/writes.
- [x] Remove native manifest fields only required for full-dex completion accounting.
- [x] Avoid writing/fsyncing a global manifest after every generated frame.
- [x] Prefer immutable frame files + simple compatibility/version metadata.
- [x] If a manifest remains necessary for species/source invalidation, keep it narrowly scoped and single-authority.
- [x] Ensure no competing writers can resurrect stale state.
- [x] Ensure artwork/source hash change invalidates only frames derived from affected source hash/spec.
- [x] Avoid O(NÂ²) snapshots as cache grows.

## Desired model

Native frame files are derived, disposable artefacts. The authoritative inputs are source PNG SHA256 and render specification. The filesystem artefact is merely memoised output.

## Success criteria

- [x] Cache correctness does not depend on a giant mutable â€œcompletionâ€ manifest.
- [x] No per-frame global fsync bottleneck.
- [x] No known competing-manifest-writer race.
- [x] Invalid derived state can simply be deleted and regenerated.

---

# 14. Phase 11 â€” Keep source artwork and native cache responsibilities separate

The source PNG is not the native sixel frame.

## Rules

Source artwork:

```text
may be downloaded/installed according to existing consent/config policy
is canonical render input
has SHA256 identity
```

Native sixel:

```text
is derived local artefact
never needs network
is gzip persistent
is disposable/rebuildable
```

## Tasks

- [x] Verify ordinary native cache hit never performs HTTP.
- [x] Verify native cold render reads existing local source artwork.
- [x] If source artwork is unavailable, preserve existing fallback behaviour rather than embedding network into the native renderer.
- [x] Ensure background **native** warming does not trigger uncontrolled source downloads unless explicitly intended by existing artwork mode.
- [x] Keep copyright-sensitive derived frames local; do not add remote native-frame distribution.

## Success criteria

Normal TUI cache/render reads remain offline-capable when dataset/source assets are already installed.

---

# 15. Phase 12 â€” Revisit terminal pixel sizing without blocking this refactor

Current code uses a fixed sixel cell conversion near `8 x 16` pixels per terminal cell. That is an approximation, not universally correct.

Do not let this expand the current refactor uncontrollably.

## Required scope

- [x] Preserve bucket selection in terminal-cell units: 24/32/40.
- [x] Document that sixel pixel dimensions are derived from terminal cell geometry.
- [x] If terminal cell pixel dimensions are already available from capability probing, use them consistently.
- [x] If not, retain the existing fallback for this refactor.
- [x] Create a follow-up issue/TODO for CSI 16 t / terminal cell pixel querying if production support is incomplete.

Cache identity must use resulting **actual pixel dimensions**, so a future cell-size improvement naturally creates the correct distinct artefact.

## Success criteria

This refactor does not regress existing terminals and does not hardwire an assumption deeper than it already is.

---

# 16. Phase 13 â€” UI behaviour for background warming

The new warmer is an opportunistic optimisation. Do not present it as a blocking application phase.

## Recommendation

Do **not** show the old persistent:

```text
Building Image Cache: â–ˆâ–ˆâ–ˆâ–ˆâ–ˆ...
```

for opportunistic sixel warming.

Rationale: the cache does not have to become complete; only the active bucket is proactively warmed; resize changes the active target; warming may pause indefinitely while foreground work exists; and the application is fully usable without completion.

If product design strongly wants visibility, use a subtle non-blocking status that does not imply completion is required. This is optional and should not block the refactor.

## Tasks

- [x] Remove old sixel-native full-cache progress footer/state.
- [x] Do not show spinner for background-only work.
- [x] Spinner/loading belongs only to the currently selected true cold foreground frame.
- [x] Ensure cached foreground image immediately replaces spinner.
- [x] Background completion must not cause unsolicited image replacement.

## Success criteria

Users never feel they must wait for the background cache to finish before using the PokÃ©dex.

---

# 17. Phase 14 â€” Performance regression tests

Production need not retain the full experimental CLI, but add focused Go benchmarks/tests around critical primitives.

## Microbenchmarks

Add/retain useful benchmarks for:

```text
native memory hit
gzip disk hit 24
gzip disk hit 32
gzip disk hit 40
gzip decode 24
gzip decode 32
gzip decode 40
64-colour sixel cold render
```

Use `b.ReportAllocs()` and `b.SetBytes(...)` where relevant. Do not make microbenchmark timing a brittle CI pass/fail gate.

## Scheduler correctness is a CI gate

The bounded scheduling tests are correctness tests, not merely performance tests.

## Optional integration benchmark

Keep a small deterministic integration benchmark/test fixture that can simulate 20 selection changes/sec semantics without depending on real wall-clock terminal rendering.

## Success criteria

- [x] Hot path has allocation/performance observability.
- [x] Queue boundedness is permanently covered by tests.
- [x] No production benchmark requires live terminal sixel output.

---

# 18. Phase 15 â€” Measure opportunistic warming before final merge

Because the recommendation is to retain one opportunistic active-bucket warmer, perform one focused production-branch measurement.

This can be temporary instrumentation or a small benchmark, but do not retain unnecessary benchmark CLI surface after the decision.

## Scenario A â€” background only

Warm #001â€“150:

```text
bucket = 32
palette = 64
one background render lane
```

Measure wall time, average CPU, peak CPU, and peak working set.

## Scenario B â€” mixed foreground/background

Run continuous background warming, then inject foreground cold requests.

Compare:

```text
foreground p50/p95 with warmer OFF
foreground p50/p95 with warmer ON
```

## Acceptance

Required:

- [x] foreground scheduling invariants remain `active<=1`, `pending<=1`.
- [x] background cannot create a foreground backlog.
- [x] no sustained CPU behaviour resembling the old multi-worker ~90% case.
- [x] foreground p95 regression is negligible or clearly acceptable.

Preferred:

```text
foreground p95 regression <= ~5 ms
```

If separate foreground/background encoders materially contend, change to one shared expensive-encode lane where foreground always chooses the next job.

## Success criteria

The warmer is proven to be an opportunistic UX benefit rather than a source of foreground contention.

---

# 19. Phase 16 â€” Cache lifecycle and invalidation

## On terminal resize

Same bucket:

```text
no cache policy change
```

New bucket:

```text
lookup/render current at new bucket
switch background target
retain old bucket files
```

## On palette change

```text
new exact cache identity
old palette files remain valid but inactive
background target uses new palette
```

## On theme/background change

Same as palette: new spec identity.

## On source artwork change

```text
new source SHA
old-source frames are no longer selected
invalidate/delete old-source derived artefacts when species hash mapping permits
```

## On application upgrade with incompatible native format

```text
delete/invalidate incompatible native derived cache
no migration
```

## On app exit

```text
keep persistent cache
stop warmer
best-effort finish/stop persistence worker
do not delete valid native cache
```

## Success criteria

- [x] No resize unnecessarily destroys useful previous buckets.
- [x] No render-spec change returns wrong frame bytes.
- [x] No migration code is added.

---

# 20. Phase 17 â€” End-to-end acceptance tests

Run these manually on Windows Terminal/sixel plus automated equivalents where feasible.

## First launch / cold cache

- [x] Open browse on a species with no native frame.
- [x] Text/UI appears immediately.
- [x] Spinner/loading is shown only for selected cold image if required.
- [x] Selected image appears quickly.
- [x] Gzip cache file appears after frame-ready.
- [x] Background begins warming current bucket only.

## Reading dossier

- [x] Leave cursor stationary for several seconds.
- [x] Verify warmer gradually creates additional current-bucket artefacts.
- [x] CPU remains reasonable.
- [x] TUI remains responsive.

## Mouse wheel

- [x] Scroll approximately 10 PokÃ©mon/sec.
- [x] Scroll approximately 20 PokÃ©mon/sec.
- [x] Cursor/text tracks input immediately.
- [x] Cached frames appear immediately.
- [x] Cold frames do not create a FIFO backlog.
- [x] Stop on a PokÃ©mon; final image settles quickly.

## Keyboard

- [x] Single Up/Down behaves identically to wheel selection semantics.
- [x] Held key repeat remains bounded.

## Resize

- [x] Start in 32 bucket and allow some warming.
- [x] Resize into 40.
- [x] Current PokÃ©mon uses 40.
- [x] Future background frames are 40.
- [x] Previously generated 32 frames remain.
- [x] Resize back to 32 and confirm existing cache reuse.

## Palette

- [x] Default config renders at 64.
- [x] Set 128 and confirm distinct cache identity.
- [x] Set 256 and confirm distinct cache identity.
- [x] Set 254 and confirm effective 64 without startup failure.

## Restart

- [x] Quit.
- [x] Reopen.
- [x] Previously cached active-size frames load from persistent gzip cache.
- [x] No full native cache rebuild is required before use.

## Corruption

- [x] Corrupt one `.gz`.
- [x] Browse that PokÃ©mon.
- [x] Application treats it as cache miss and regenerates.
- [x] No fatal TUI error.

---

# 21. Phase 18 â€” Race, static analysis, and final quality gates

Run:

```powershell
$files = Get-ChildItem -Recurse -Filter *.go | ForEach-Object FullName
gofmt -w $files

go test ./...
go vet ./...
```

On race-capable environment/CI:

```text
go test -race ./...
```

## Required gates

- [x] all unit tests pass.
- [x] all scheduler invariant tests pass.
- [x] no race detector failures.
- [x] no stale-render visual replacement.
- [x] no manifest/store competing-writer race.
- [x] no goroutine leak in scheduler/warmer/persistence shutdown.
- [x] no network request on ordinary native cache lookup.
- [x] no fatal config error for unsupported sixel palette.
- [x] no background all-size cache population.
- [x] no multi-worker native bulk generation remains.

---

# 22. Phase 19 â€” Documentation cleanup

## Production docs

- [ ] Document `render.sixel_colors`.
- [ ] State supported values 64/128/256.
- [ ] State invalid values fall back to 64.
- [ ] Document active-bucket caching behaviour.
- [ ] Document persistent gzip native cache as disposable derived state.
- [ ] Document that resizing may begin warming a different supported bucket.
- [ ] Document that previous bucket artefacts remain reusable.

## Internal architecture docs

Add a short flow diagram:

```text
selection
   |
   v
memory LRU
   | miss
   v
gzip native disk cache
   | miss
   v
foreground latest-selection-wins render
   |
   +--> display + memory
   |
   +--> async persist

when no foreground cold work:
   |
   v
opportunistic active-bucket warmer
```

## Experiment docs

Do not delete the benchmark experiment evidence. Preserve it on the experiment branch and/or copy the final measured decision summary into normal engineering docs if desired.

---

# 23. Suggested commit sequence

Recommended commits:

```text
1. refactor(sprite): use 24 32 40 render buckets
2. feat(config): add sixel palette setting with 64 default
3. refactor(sprite): include palette in canonical native render identity
4. refactor(cache): store native sixel frames as gzip artefacts
5. refactor(sprite): add decompressed native frame LRU
6. refactor(sprite): add bounded latest-selection-wins cold scheduler
7. refactor(tui): route selection rendering through bounded scheduler
8. refactor(sprite): persist cold frames asynchronously
9. refactor(sprite): add foreground-priority active-bucket warmer
10. refactor(cache): remove native whole-cache completion machinery
11. refactor(tui): remove obsolete native cache progress flow
12. test(sprite): add resize palette corruption and scheduler coverage
13. docs(render): document lazy active-bucket native cache
```

Adjust names to match actual package ownership, but avoid one giant commit.

---

# 24. Explicit non-goals

Do not turn this refactor into unrelated backend redesign.

Out of scope unless required to make the above correct:

```text
remote sixel cache hosting
GitHub-hosted pre-rendered Pokemon frames
new artwork redistribution
renderer-ready RGBA master format
indexed/paletted persistent raster intermediate
zstd production dependency
terminal-specific fast-scroll heuristics
multi-core render scheduler
full data-layer schema migration
new dataset architecture
unrelated dossier/network refactors
```

The renderer-master experiments require a very large measured win to justify extra production complexity and are not part of this plan.

---

# 25. Final production success definition

This refactor is complete only when all of the following are true:

- [ ] Sixel supports exactly 24/32/40 cache buckets.
- [ ] 64 colours is the default.
- [ ] Config supports 64/128/256.
- [ ] Unsupported palette values fall directly back to 64.
- [ ] Palette is part of sixel cache identity.
- [ ] Native frames are gzip persisted.
- [ ] Memory LRU contains decompressed frames.
- [ ] Warm cache hit never reads/decodes source PNG.
- [ ] Foreground cold work is bounded to one active + one latest pending.
- [ ] No FIFO render backlog exists.
- [ ] No debounce exists for image selection.
- [ ] Mouse wheel uses the same selection contract as keyboard.
- [ ] Stale cold results never overwrite current selection.
- [ ] Cold frame may display before gzip persistence finishes.
- [ ] Opportunistic background warming remains.
- [ ] Background warming targets only the active terminal bucket.
- [ ] Background warming uses the current effective palette/render spec.
- [ ] Foreground always has priority over the next background encode.
- [ ] Resize switches background target instead of building every size.
- [ ] Previously cached buckets remain reusable.
- [ ] Old all-size/full-dex native completion machinery is removed.
- [ ] Old native cache schema is invalidated rather than migrated.
- [ ] No remote/native artwork cache distribution is introduced.
- [ ] Windows Terminal cold-scroll remains responsive at ~20 selections/sec.
- [ ] Mixed foreground/background benchmark shows negligible foreground regression.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] race tests pass in CI/supported environment.

---

# 26. Final design summary for reviewers

The resulting backend should be understandable in one paragraph:

> Pokedex renders sixel using three supported cell buckets (24/32/40) and defaults to a measured 64-colour palette. Native sixel frames are identified by canonical source SHA256 plus the complete render specification, cached as gzip files on disk, and retained decompressed in a bounded in-process LRU. Foreground rendering is latest-selection-wins with one active expensive render and one replaceable pending request, so rapid keyboard or 10â€“20 PokÃ©mon/sec mouse-wheel input cannot create a backlog. A single opportunistic background warmer uses spare foreground-free capacity to populate only the currently active terminal-size bucket; resizing simply switches the warm target while preserving previously generated buckets. Frame display never waits on gzip persistence, cache state is disposable derived data rather than a migration burden, and the old multi-worker/all-size native completion architecture is removed.


# Pokedex `lazy-sixel-cache` Fix Plan

> **Target branch:** `refactor/lazy-sixel-cache`  
> **Purpose:** Repair the remaining native sixel cache blockers found during code review and manual Windows Terminal testing.  
> **Merge status:** Do **not** merge to `main` until every blocker and final acceptance criterion below passes.

## Agent instructions

- [x] Work through this plan in order and change `[ ]` to `[x]` only after implementation **and verification** are complete.
- [x] Keep fixes on `refactor/lazy-sixel-cache`.
- [x] Preserve the good refactor work already present; do not restart from scratch.
- [x] Keep commits small and phase-oriented.
- [x] Do not add debounce, fast-scroll mode, adaptive render workers, or an unbounded render queue.
- [x] Proactively warm **only the currently active terminal-size bucket**.
- [x] Keep previously generated buckets on disk for reuse after resize.
- [x] Do not add cache/schema migration code. Incompatible derived native cache state should be invalidated and rebuilt.
- [x] Keep native visual artefacts local.
- [x] On PowerShell, format recursively with:

```powershell
$files = Get-ChildItem -Recurse -Filter *.go | ForEach-Object FullName
gofmt -w $files
```

Do not rely on `gofmt -w .`.

---

# 1. Required end-user behaviour

The corrected application must behave like this:

```text
open browse
-> determine current sixel bucket
-> load/render selected Pokemon
-> start background population of that same bucket
-> continue progressively through the Pokedex while foreground is free
```

If the user leaves the cursor stationary for several seconds, the number of files in the native cache directory must continue increasing.

While the user reads a dossier:

```text
foreground idle
-> warm next missing active-bucket Pokemon
-> persist gzip frame
-> update progress
-> repeat
```

While navigating:

```text
cache hit
-> display immediately
-> no spinner

true cold miss
-> spinner
-> one foreground cold render
-> one latest pending selection only
```

On terminal resize:

```text
32 -> 40

current Pokemon @40 gets foreground priority
background warm target switches to @40
existing @32 files remain reusable
```

Do not proactively build 24 + 32 + 40 together.

---

# 2. Blocker A — Background native renders are not persisted

## Problem

The current native warmer calls `NativeWarmRender`, but successful background results only become process-local memory-cache entries. They are not reliably persisted into `.sixel.gz` files.

That creates a false warm state:

```text
background encode
-> memory hit this process
-> no persistent file
-> benefit lost on restart
```

## Tasks

- [x] Extend the native-warmer renderer contract so a completed background frame can be persisted.
- [x] Ensure a valid background result exposes/uses `CacheKey` and frame bytes/string.
- [x] After a valid `NativeWarmRender`, enqueue the frame into the same gzip persistence path used for native foreground results.
- [x] Do not persist placeholders.
- [x] Do not persist empty-cache-key results.
- [x] Ensure background and foreground use the exact same canonical render key builder.
- [x] Verify background-persisted frames are readable through normal `LookupCached`.
- [x] Keep persistence outside the foreground display critical path.

## Tests

- [x] Fix fake native-renderer tests so rendering and persistence are separate observable operations.
- [x] Add `TestNativeWarmRenderPersistsFrame`.
- [x] Add `TestNativeWarmPlaceholderDoesNotPersist`.
- [x] Add `TestNativeWarmEmptyCacheKeyDoesNotPersist`.
- [x] Add `TestBackgroundPersistedFrameIsForegroundDiskHit`.
- [x] Add `TestBackgroundPersistedFrameSurvivesNewEngineSession`.

## Success criteria

- [x] With an empty `/native` directory, leaving the Pokedex open without input causes `.sixel.gz` files to appear.
- [x] Background-created files are usable after restart.
- [x] Background warming is not merely warming the in-process LRU.

---

# 3. Blocker B — Native warming is limited to a tiny neighbour window

## Problem

Native warming is currently coupled to the TUI prefetch radius. That means it warms only a small local window and then stops.

The intended design is a **bounded worker with a progressive whole-active-bucket traversal**, not a full-Dex FIFO queue.

## Required traversal

Recommended order around selected species `N`:

```text
N+1
N+2
...
max
N-1
N-2
...
1
```

Visible neighbours may be prioritised first, but traversal must continue beyond them.

## Tasks

- [x] Decouple native warm coverage from `spritePrefetchRadius`.
- [x] Remove `warmerNeighborRadius = spritePrefetchRadius` as the definition of total native warm scope.
- [x] Replace radius-only warming with progressive active-bucket traversal.
- [x] Keep traversal state compact and bounded.
- [x] Do not enqueue the whole Dex into a channel.
- [x] After each cache hit, successful render/persist, or recoverable skip, continue to the next candidate.
- [x] Make `Prioritize(speciesID)` an immediate-priority hint, not a reset of all remaining traversal work.
- [x] A selection change may reposition the traversal anchor but must not reduce warm scope back to `±1`.
- [x] Skip already-cached exact keys cheaply.
- [x] Do not repeatedly scan the native directory for every candidate.
- [x] Continue until the current active render specification is complete or the app exits/spec changes.

## Tests

- [x] Add `TestNativeWarmerContinuesBeyondImmediateNeighbours`.
- [x] Add `TestNativeWarmerEventuallyTraversesWholeActiveBucket`.
- [x] Add `TestNativeWarmerTraversalStateIsBounded`.
- [x] Add `TestSelectionChangeDoesNotDiscardWholeWarmCoverage`.
- [x] Add `TestPrioritizeDoesNotPreventContinuedTraversal`.
- [x] Add `TestAlreadyCachedCandidatesAreSkipped`.
- [x] Use a fake index of at least 100 species and prove stationary browse progresses well beyond the old queue/radius limit.
- [x] Rewrite any benchmark that waits for more renders than its production queue can actually contain and succeeds only by timing out.

## Success criteria

- [x] Leave the cursor stationary for 10–30 seconds on a cold cache.
- [x] Persisted native file count continues increasing beyond nearest neighbours.
- [x] The warmer can eventually cover all warmable species for the current bucket/specification.
- [x] Queue/state size does not scale linearly with Dex size.

---

# 4. Blocker C — Normal selection changes cancel active cold work

## Problem

The current foreground scheduler cancels the active cold render when a new selection arrives. That is not the measured latest-selection-wins model.

Required policy:

```text
one active
one latest pending

new selection while active:
-> replace pending with newest
-> let active finish
-> never display active result if stale
-> persist/cache valid stale result
-> run newest pending next
```

## Tasks

- [x] Remove active-render cancellation on ordinary selection changes.
- [x] Keep cancellation only for lifecycle/teardown or a genuinely forced invalid replacement.
- [x] Preserve one latest pending request.
- [x] Let active work finish.
- [x] Never display stale completion.
- [x] Persist valid stale completion.
- [x] Start newest pending request immediately after active completion.

## Tests

- [x] Add/update `TestColdSchedulerSelectionDoesNotCancelActive`.
- [x] Keep `TestColdSchedulerAtMostOneActive`.
- [x] Keep `TestColdSchedulerAtMostOnePending`.
- [x] Keep `TestColdSchedulerLatestPendingWins`.
- [x] Keep `TestColdSchedulerDoesNotFIFOQueue`.
- [x] Add `TestColdSchedulerStaleCompletedFramePersists`.
- [x] Add deterministic blocked-render test:

```text
select 1
block render 1
select 2
select 3
select 4
select 5

assert active=1
assert pending=5
assert render 1 was not cancelled

release 1
assert 1 never displays
assert 1 may persist
assert next expensive render=5
```

## Success criteria

- [x] Rapid navigation never creates more than one active + one pending cold foreground request.
- [x] Normal selection changes no longer throw away already-paid encode work.
- [x] Stale frames never overwrite current selection.

---

# 5. Blocker D — Native image-cache progress is missing

## Product requirement

Show a real native image-cache progress bar while the current active bucket is being persistently built.

This is intentional UX feedback so users can see useful work is still occurring and are less likely to close the app before cache construction finishes.

## Progress semantics

Progress must represent the exact current native render specification:

```text
protocol
bucket
palette
background/theme identity
render algorithm version
```

Track:

```text
Done  = species whose exact active-spec native frame is persisted/readable
Total = species targeted by the current active-spec warm pass
```

**Do not increment `Done` merely because a frame encoded into RAM.** The progress bar is meant to tell the user how much persistent work is safe if they exit.

Suggested display:

```text
Image Cache 137/1025 · 32
```

or an equivalent existing footer style.

## Tasks

- [x] Add real progress reporting to `nativewarm.Service`.
- [x] Emit `Stage: "native"`.
- [x] Include active bucket in `Cells`.
- [x] Make `Snapshot()` return meaningful values instead of `0,0`.
- [x] Connect native warmer progress into `progressCh` / `WarmEvents`.
- [x] Remove the footer suppression that currently hides native-stage progress.
- [x] Add a visible native cache progress bar using the existing footer style.
- [x] Show it while the active-spec target is incomplete.
- [x] Update it only as persisted/readable coverage grows.
- [x] Do not clear it merely because an immediate neighbour queue drained.
- [x] Clear it only when the active target truly reaches completion, native warming is disabled/unavailable, or the app exits.
- [x] On bucket change, recompute/reset progress for the new active bucket.
- [x] Keep prior-bucket files on disk.
- [x] Palette/background/version changes must create a new progress target because the render identity changed.

## Optional UX

- [x] If useful, show a brief `Checking Image Cache...` state before accurate `Done/Total` is known.

## Tests

- [x] Add `TestNativeProgressStartsWhenBucketIncomplete`.
- [x] Add `TestNativeProgressDoneIncrementsOnlyAfterPersistence`.
- [x] Add `TestNativeProgressDoesNotCompleteWhenImmediateQueueEmpties`.
- [x] Add `TestNativeProgressCompletesAtTotal`.
- [x] Add `TestNativeProgressBucketSwitchResetsTarget`.
- [x] Add `TestNativeProgressDoesNotDeletePriorBucket`.
- [x] Replace tests that expect native progress formatting to return an empty string.
- [x] Add TUI test proving native progress is visible.
- [x] Add TUI test proving native progress clears on true completion.

## Success criteria

- [x] User can visibly see active-bucket cache construction progressing.
- [x] `Done/Total` corresponds to persistent/readable frames.
- [x] Progress does not disappear when a tiny queue empties.
- [x] Completion means the current active render target is genuinely persisted.

---

# 6. Blocker E — Persistence queue must not discard valid cache-build work

## Problem

The persistence queue is bounded, which is correct, but its current “drop oldest, keep newest” behaviour applies latest-selection-wins semantics to completed cache artefacts.

That is incorrect for background cache construction.

A completed valid frame is useful work and should not be silently discarded.

## Required policy

Foreground display must remain independent from disk persistence.

Background warming may be backpressured by persistence capacity because there is no UX benefit in encoding faster than frames can be safely stored.

## Tasks

- [x] Remove silent dropping of accepted valid native persistence jobs.
- [x] Keep persistence memory bounded.
- [x] Add bounded backpressure for background warming.
- [x] Preserve non-blocking foreground display.
- [x] Ensure valid stale foreground frames can still persist.
- [x] Ensure clean shutdown drains accepted jobs.
- [x] If persistence fails, log it, do not count progress as done, and allow future retry/regeneration.

## Tests

- [x] Add `TestPersistQueueDoesNotDropAcceptedBackgroundFrames`.
- [x] Add `TestForegroundDisplayDoesNotWaitForSlowPersistence`.
- [x] Add `TestBackgroundWarmerBackpressuresOnPersistQueue`.
- [x] Add `TestPersistFailureDoesNotIncrementNativeProgress`.
- [x] Add `TestShutdownDrainsAcceptedPersistenceJobs`.

## Success criteria

- [x] A completed warm pass produces all expected persistent files.
- [x] Progress cannot outrun persistence.
- [x] Foreground frame-ready latency remains outside gzip/write latency.

---

# 7. Blocker F — `coldSchedBridge.lastRes` is unsafe shared state

## Problem

The bridge stores result metadata in one mutable `lastRes`. Interleaved lookup/render/completion activity can associate metadata from the wrong request with a frame.

## Tasks

- [x] Remove `coldSchedBridge.lastRes`.
- [x] Extend `coldsched.ColdResult` so result metadata is request-local.
- [x] Carry at least:
  - frame;
  - cache key;
  - cache state;
  - native flag;
  - asset kind;
  - placeholder state;
  - timings/error.
- [x] Make the completion/display callback receive the exact result belonging to the exact request.
- [x] Route cache hits and cold results through the same request-local metadata model.

## Tests

- [x] Add interleaved/concurrent result test proving request A cannot receive request B metadata.
- [x] Add native/artwork metadata preservation test for cached and cold results.
- [x] Run race detector across scheduler/bridge paths.

## Success criteria

- [x] No shared “last result” state exists.
- [x] Race detector is clean.
- [x] Native overlay metadata always belongs to the displayed request.

---

# 8. Blocker G — Duplicate foreground cache lookup and spinner flash

## Problem

The TUI performs a cache lookup and, after a miss, the scheduler immediately performs another lookup. This duplicates hot-path work and can produce avoidable spinner flashes if the cache becomes warm between the two probes.

## Required rule

One authoritative initial cache decision per selection:

```text
LookupCached
    hit -> display immediately
    miss -> set loading and submit genuine cold miss
```

A cache recheck is still useful **when a pending request is later promoted to active**, because background work may have filled it while it waited.

## Tasks

- [x] Remove the immediate duplicate cache lookup.
- [x] Keep a cache recheck when promoting a pending request to active.
- [x] Enter `ImageLoading` only after the authoritative current-selection lookup has genuinely missed.
- [x] Cache hit must never start the spinner.
- [x] Memory/disk hit must update image immediately.

## Tests

- [x] Add lookup call-count test for a fresh selection.
- [x] Add `TestCachedSelectionDoesNotStartSpinner`.
- [x] Add `TestPendingPromotionRechecksCache`.
- [x] Add scenario where a pending miss becomes a background cache hit before promotion and therefore avoids cold encode.

## Success criteria

- [x] No duplicate immediate cache probe.
- [x] No spinner for a frame already warm at selection time.
- [x] Pending requests benefit from cache work completed while they waited.

---

# 9. Blocker H — Persistence worker is not explicitly drained on browse shutdown

## Tasks

- [x] Ensure `openBrowse` owns and closes/drains the engine persistence worker.
- [x] Use `defer engine.ClosePersist()` or equivalent lifecycle ownership.
- [x] Stop warmers/schedulers in a safe order relative to persistence drain.
- [x] Avoid shutdown races.
- [x] Do not hang indefinitely on close.

## Tests

- [x] Add lifecycle test with queued writes followed immediately by shutdown.
- [x] Assert accepted frames exist after clean shutdown.
- [x] Run race detector.

## Success criteria

- [x] Clean exit does not abandon already-accepted gzip writes.
- [x] No persistence goroutine leaks.

---

# 10. Foreground/background arbitration

## Required priority

```text
foreground cold work waiting?
YES -> do not start next background encode
NO  -> background may process one next candidate
```

An already-running background encode may finish. Do not add complex cancellation.

After background completion:

```text
persist valid result
-> check foreground again
-> foreground goes next if waiting
```

## Tasks

- [x] Ensure `SetForegroundBusy` reflects the true foreground scheduler state.
- [x] Wake/resume native warmer when foreground becomes idle.
- [x] Ensure warmer does not remain permanently paused after a cold render.
- [x] A foreground cache hit must not unnecessarily suppress background work.
- [x] Do not add 400 ms/native idle debounce unless measurement proves it necessary.
- [x] Keep keyboard and mouse-wheel semantics identical.

## Tests

- [x] Add `TestNativeWarmerResumesAfterForegroundCompletes`.
- [x] Add `TestWarmCacheHitDoesNotPauseNativeWarmer`.
- [x] Add `TestForegroundColdGetsNextEncodeBeforeBackground`.
- [x] Add `TestBackgroundActiveMayFinishThenForegroundRuns`.
- [x] Keep `TestWheel20KeepsSchedulerBounded`.

## Success criteria

- [x] Background progresses while the user reads.
- [x] Foreground gets priority as soon as the user needs cold work.
- [x] No debounce and no fast-scroll mode are introduced.

---

# 11. Progress counting without resurrecting the old completion manifest

Do not rebuild the old giant full-Dex mutable completion-manifest architecture.

## Preferred model

The warmer owns rebuildable per-active-spec progress state, for example:

```go
type WarmProgressState struct {
    SpecKey string
    Done    int
    Total   int
}
```

A candidate counts as done when:

```text
exact persistent cache hit already exists
OR
this session successfully persisted that exact key
```

The actual `.sixel.gz` artefacts remain the cache truth.

## Tasks

- [x] Do not make progress metadata authoritative for cache correctness.
- [x] Do not update/fsync a large global manifest after every frame.
- [x] If initial coverage must be calculated, perform one deliberate active-spec audit rather than repeated full directory scans.
- [x] Alternatively let progressive traversal discover hits, provided `Done/Total` remains truthful.
- [x] Ensure deleting progress state cannot invalidate otherwise valid frame files.

## Success criteria

- [x] Exact native frame files remain the source of truth.
- [x] Progress is reconstructable.
- [x] No old manifest write bottleneck returns.

---

# 12. Spinner UX acceptance

The spinner is appropriate only for the **currently selected true cold miss**.

## Tasks

- [x] Spinner must not appear on native memory hit.
- [x] Spinner must not appear on gzip disk hit.
- [x] Spinner must not appear because background warming is active.
- [x] Spinner must stop immediately when current selected frame becomes available.
- [x] On selection change, spinner state must belong to the new current request.
- [x] Stale completion must not incorrectly clear a newer selection's spinner.

## Tests

- [x] Add `TestSpinnerOnlyOnTrueColdMiss`.
- [x] Add `TestSpinnerNotShownForNativeMemoryHit`.
- [x] Add `TestSpinnerNotShownForNativeDiskHit`.
- [x] Add `TestStaleCompletionDoesNotClearNewerSpinner`.
- [x] Add `TestCurrentCompletionClearsSpinner`.

## Success criteria

- [x] Browsing through background-warmed entries is effectively spinner-free.

---

# 13. Required real integration tests

Unit tests previously failed to expose missing persistence. Add tests using a real temporary cache store.

## Test A — Stationary warm persistence

```text
1. create temp cache
2. provide local source PNG fixtures
3. start native warmer at bucket 32
4. anchor/select species 1
5. generate no further selection events
6. wait for background progress
7. inspect temp /native
```

Assertions:

- [x] file count becomes greater than 1 without cursor movement.
- [x] file count continues increasing.
- [x] files are `.sixel.gz`.
- [x] gzip frames are readable.
- [x] progress tracks persisted count.

## Test B — Foreground reads background cache

```text
background warm species 2
wait for persistence
foreground LookupCached(species 2)
```

Assertions:

- [x] cache hit.
- [x] no PNG read.
- [x] no decode.
- [x] no resize.
- [x] no sixel encode.

## Test C — Restart

```text
session A:
background warm species 1..N
close cleanly

session B:
new Engine + same cache directory
lookup species 1..N
```

Assertions:

- [x] disk hits.
- [x] no cold encodes.

## Test D — Resize

```text
warm some @32
switch to @40
```

Assertions:

- [x] future background writes are @40.
- [x] @32 files remain.
- [x] returning to @32 reuses them.

## Test E — Progress truth

Artificially slow persistence:

- [x] encode completes.
- [x] progress `Done` does **not** increment yet.
- [x] persistence completes.
- [x] progress increments.

---

# 14. Manual Windows Terminal acceptance run

This manual run is mandatory before merge.

## Preparation

- [ ] Use Windows Terminal with sixel active.
- [ ] Use default 64 colours.
- [ ] Identify active bucket.
- [ ] Clear only derived native cache.
- [ ] Keep source artwork and dataset installed.
- [ ] Record native file count before launch.

## Test 1 — No-input background build

- [ ] Launch Pokedex.
- [ ] Do not touch keyboard or mouse for 15 seconds.
- [ ] Native progress bar remains visible while incomplete.
- [ ] Progress count increases.
- [ ] `/native/*.sixel.gz` file count increases continuously.
- [ ] Warming progresses beyond current nearest neighbours.

**Fail if:** cache files only appear after navigation.

## Test 2 — Slow keyboard navigation

- [ ] Press Down once every ~0.5–1 second.
- [ ] Previously warmed species display without spinner.
- [ ] New true cold species may briefly show spinner.
- [ ] Background cache resumes whenever foreground is free.

**Fail if:** only rapid wheel activity causes persistent cache creation.

## Test 3 — Wheel navigation

- [ ] Scroll around 10 species/sec.
- [ ] Scroll around 20 species/sec.
- [ ] Text/list tracks input.
- [ ] `max active <= 1`.
- [ ] `max pending <= 1`.
- [ ] no FIFO backlog.
- [ ] stop and verify final selected frame settles quickly.

## Test 4 — Resume after navigation

- [ ] Stop interaction.
- [ ] Background progress resumes automatically.
- [ ] Native file count continues increasing.

## Test 5 — Completion and restart

- [ ] Let current bucket finish.
- [ ] Progress reaches `Total/Total`.
- [ ] Progress bar clears or transitions to a brief completed state.
- [ ] Restart app.
- [ ] Browse warmed species without rebuild/spinner for that same render spec.

## Test 6 — Resize

- [ ] Resize into another bucket.
- [ ] Progress now represents the new bucket.
- [ ] Existing old-bucket files remain.
- [ ] Background fills the new bucket.
- [ ] Resize back and verify old bucket reuse.

---

# 15. Performance regression gates

Do not fix preloading by returning to the old high-CPU multi-worker architecture.

## Background-only measurement

Active bucket 32, palette 64:

- [ ] Record average CPU.
- [ ] Record peak CPU.
- [ ] Record working set.
- [ ] Record frames persisted/sec.

## Mixed foreground/background measurement

- [ ] Record foreground p50.
- [ ] Record foreground p95.
- [ ] Record final settle.
- [ ] Record max active.
- [ ] Record max pending.

## Gates

- [ ] foreground active maximum = 1.
- [ ] foreground pending maximum = 1.
- [ ] no old-style sustained ~90% CPU behaviour.
- [ ] foreground latency remains close to measured experiment behaviour.
- [ ] preferred foreground p95 regression with warmer enabled is <= ~5 ms.
- [ ] If foreground/background encoders materially contend, move to one shared expensive-encode lane with foreground-next priority.

---

# 16. Preserve the already-correct refactor decisions

- [ ] Native buckets remain exactly `24 / 32 / 40`.
- [ ] Default sixel palette remains `64`.
- [ ] Optional palettes remain `128 / 256`.
- [ ] Invalid palette values fall directly back to `64`.
- [ ] Palette remains part of sixel cache identity.
- [ ] Source PNG SHA256 remains canonical source identity.
- [ ] Native frames remain gzip-persisted.
- [ ] Runtime native LRU contains decompressed frames.
- [ ] Warm cache hits do not read/decode source PNG.
- [ ] Previously cached buckets survive resize.
- [ ] No schema migration is added.
- [ ] No remote sixel cache is introduced.

---

# 17. Suggested commit order

```text
1. fix(nativewarm): persist completed background sixel frames
2. refactor(nativewarm): traverse active bucket beyond neighbours
3. fix(coldsched): let active render finish and keep latest pending
4. fix(sprite): make native persistence reliable and bounded
5. refactor(coldsched): carry result metadata per request
6. fix(tui): remove duplicate cache lookup and spinner flash
7. feat(tui): show real native image-cache progress
8. fix(app): drain native persistence on browse shutdown
9. test(nativewarm): add real disk persistence and restart coverage
10. test(tui): add stationary warm and spinner acceptance coverage
11. docs(cache): update native cache behaviour and acceptance notes
```

---

# 18. Final merge success criteria

Do not merge until every item below is `[x]`.

## Persistent warming

- [ ] Opening the Pokedex starts active-bucket background native warming.
- [ ] Background-warmed frames are persisted as `.sixel.gz`.
- [ ] Native file count grows while the cursor is stationary.
- [ ] Warming progresses beyond nearest neighbours.
- [ ] Warming can eventually cover the whole current active bucket.
- [ ] Only the active bucket is proactively warmed.
- [ ] Old buckets remain reusable.

## Progress feedback

- [ ] Native image-cache progress bar is visible while incomplete.
- [ ] Progress shows meaningful `Done/Total`.
- [ ] `Done` represents persistent/readable cache entries.
- [ ] Progress does not disappear because an immediate queue empties.
- [ ] Progress completes only when the active render-spec target is complete.
- [ ] Resize switches progress to the new active bucket.

## Foreground UX

- [ ] Memory hit -> immediate display, no spinner.
- [ ] Gzip disk hit -> immediate display, no spinner.
- [ ] Spinner appears only on a true selected cold miss.
- [ ] Foreground scheduler remains one active + one latest pending.
- [ ] Ordinary selection changes do not cancel active encode.
- [ ] Stale result never displays.
- [ ] Valid stale result may persist.
- [ ] No debounce.
- [ ] No fast-scroll mode.

## Reliability

- [ ] Accepted valid persistence jobs are not silently dropped.
- [ ] Background persistence survives restart.
- [ ] Clean shutdown drains accepted persistence jobs.
- [ ] `coldSchedBridge.lastRes` or equivalent shared mutable result state is removed.
- [ ] Race detector passes.
- [ ] Corrupt native gzip is treated as cache miss and regenerates.

## Manual acceptance

- [ ] Stationary 15-second Windows Terminal test visibly grows `/native`.
- [ ] Slow Up/Down navigation uses warmed frames.
- [ ] 10–20 species/sec wheel test remains responsive and bounded.
- [ ] Stopping navigation automatically resumes background progress.
- [ ] Restart reuses background-built cache.
- [ ] Resize changes only the active warm target.

## Quality gates

- [ ] PowerShell `gofmt` completed.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `go test -race ./...` passes on supported CI/host.
- [ ] `docs/native-cache-acceptance.md` reflects the corrected behaviour.

---

# 19. Final architecture summary

```text
                          +------------------+
selection -------------->| native mem cache |
                          +--------+---------+
                                   | miss
                                   v
                          +------------------+
                          | gzip disk cache  |
                          +--------+---------+
                                   | true miss
                                   v
                    +---------------------------+
                    | foreground cold scheduler |
                    | 1 active + 1 latest       |
                    +-------------+-------------+
                                  | frame ready
                                  +------> display immediately
                                  +------> decompressed memory LRU
                                  +------> reliable async gzip persistence

when no foreground cold work is waiting:

                    +---------------------------+
                    | active-bucket native warm |
                    | progressive whole-Dex     |
                    | traversal, one encode     |
                    +-------------+-------------+
                                  |
                                  +------> exact cache hit -> progress
                                  |
                                  +------> encode
                                           -> memory
                                           -> persist
                                           -> persisted progress++
                                           -> next species
```

> **Product rule:** The application should become progressively more instant simply by being left open. The user should be able to see persistent native cache construction happening through a truthful image-cache progress bar.

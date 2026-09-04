# Refactor plan

Grounded in [CODEBASE.md](CODEBASE.md) and the current tree (including uncommitted TUI/sixel/test work). This is not a rewrite of `plans/pokedex-backend-refactor-plan.md` (that plan is a sixel/cache product spec). This plan is about ownership, typed state, layering, and leftover-surface consolidation.

**Constraint:** preserve public behaviour unless the user explicitly approves a change. Product-behaviour slices (footer for non-native stages, copy unification) stay documented-only. Deleting proved-dead internal symbols is not a product-behaviour change.

**Constraint:** no generic utility packages; no file split/merge for size alone; no speculative abstractions with one use. Three similar heap warmers are not a licence to extract a shared scheduler.

**Scope of this refactor:** slices A–C are done. Remaining work is consolidation around the target architecture below — not a further round of “typed strings in `cache`”.

---

## Target architecture

Stages belong to the subsystem that produces the event. `cache` must not become a global event-types package. Slice A’s `cache.Stage` was a useful intermediate migration, not the end state.

```
internal/cache
    dataset / cache storage
    DatasetProgress / DatasetStage
    no knowledge of native / artwork / dossier presentation

internal/render/sprite/nativewarm
    native active-bucket traversal
    nativewarm.Progress
    no cache.Progress dependency

internal/render/sprite/assetwarm
    artwork download scheduling
    assetwarm.Progress
    no cache.Progress dependency

internal/render/sprite/warm
    chafa artefact prewarm
    warm.Progress
    no cache.Progress dependency

internal/cache/dossierwarm
    dossier precomputation
    dossierwarm.Progress
    owns its own semantics

internal/app
    constructs services
    owns lifecycle
    adapts subsystem progress → TUI events
    maps CLI progress → CLI presentation

internal/tui
    WarmStage
    WarmProgress
    presentation / state only
    no metadata-cache dependency
```

`nativewarm` stays algorithmically separate (single lane + disk audit). Do not force it into a heap-warmer abstraction.

`ImageCacheCoordinator` and the image completion manifest are **not** the destination. Native-cache source of truth is the content-addressed `.sixel.gz` artefacts on disk; progress can be reconstructed from those files. Do not wire the coordinator into browse, and do not resurrect a second mutable completeness authority that can disagree with the files.

---

## Internal-dependency contract

These rules also live in [CODEBASE.md](CODEBASE.md). Simple architecture/import tests should lock the forbidden edges.

| Allowed | Forbidden |
| --- | --- |
| `app` constructs services, owns lifecycle, and **translates** subsystem progress into TUI/CLI presentation | TUI (browse, `tui/bootstrap`, `tui/setup`) importing `cache` |
| `cache` / `dataset` own storage, snapshots, dataset bootstrap mechanics | `cache` / `dataset` importing `app` or `tui` |
| Each warmer owns its progress type and emit path | Rendering warmers (`nativewarm`, `assetwarm`, `warm`) importing `cache` **just for event types** |
| `dossierwarm` owns dossier progress semantics | Renderer packages knowing presentation terminology (`WarmStage`, footer copy, bootstrap titles) |
| TUI owns `WarmStage` / `WarmProgress` and view policy | `cache.Stage` (or successor) as a global progress namespace for native/artwork/dossier/chafa |

`app` is the only layer that may assemble those packages and classify warmer/dataset events into `tui.WarmStage` (or CLI lines).

---

## Non-regression requirements

Structural cleanup must not undo the performance architecture. These hold for every remaining slice:

- No synchronous full-Dex native-cache audit on the Bubble Tea path.
- First UI render is not blocked by background warmers.
- Foreground cache hits remain immediate.
- One-active / one-latest foreground scheduling (`coldsched`) remains intact.
- Native warming remains **active-bucket-only** (24/32/40).
- Native progress continues to represent **persisted** frames (readable `.sixel.gz`), not in-memory encodes.

See [docs/native-cache-acceptance.md](docs/native-cache-acceptance.md). Existing startup / native-cache acceptance behaviour must remain unchanged.

---

## Findings

Post A–C: TUI browse stages and bootstrap/setup progress callbacks are typed and local. Remaining structural debt is `cache` still acting as a shared progress bus, warmer→`cache` DTO imports, leftover API surface, and unused machinery.

### Duplicated behaviour

| What | Sites | Notes |
| --- | --- | --- |
| Heap warmer (P0/P1/P2, `queued`/`done`, wake chan, `Prioritize`, `SetNeighborsFromList`, `Snapshot`) | `render/sprite/warm`, `render/sprite/assetwarm`, `cache/dossierwarm` | Real 3-site clone. `nativewarm` is a different algorithm (single lane + disk audit) — do **not** force it into the same abstraction. Similar shape ≠ shared framework. |
| Dataset stage → user copy | `app.formatBootstrapProgress`, `tui/bootstrap.stageLabel`, `Handler.ensureIndex` | Same stage keys; **wording is not identical** (CLI includes percents; TUI titles are short). Unifying copy would change public text. **Rejected** (Slice G). |
| Progress DTO | `cache.DatasetProgress`, warmer-owned `Progress` types, `tui.WarmProgress`, `tui/bootstrap.Progress`, `tui/setup.Progress` | Same-shaped DTOs have subsystem-specific meanings. **Done (D):** warmers own their progress types and `app` maps them into presentation types. |
| Warmer Mode string enums | `warm.Mode`, `assetwarm.Mode`, `dossierwarm.Mode` | Typed aliases already, but values are still strings and each package redefines `off`/`window`. Fine as package-local types. Do not merge into one Mode type (different value sets). |

### Unclear ownership

| Issue | Today (after A–C) | Should own |
| --- | --- | --- |
| Progress bus | **Fixed (D):** cache uses `DatasetProgress` / `DatasetStage`; each warmer owns `Progress`; app adapters classify warmer progress into `tui.WarmProgress`. | Mechanics emit typed events in owning packages; app maps; TUI renders. `cache` keeps **dataset** progress only (`DatasetStage` / `DatasetProgress`). |
| Native cache completeness | **Fixed (F):** `nativewarm` counts Done/Total from disk; the orphan `ImageCacheCoordinator` and aggregate image manifest are deleted. | On-disk `.sixel.gz` files are the source of truth. `nativewarm` owns opportunistic encode. The small `NativeManifest` version/protocol stamp remains separate from completeness. |
| Bootstrap presentation | App formats CLI lines; `tui/bootstrap` formats TUI; both switch on typed stages (Slice C) | TUI owns TUI labels; app owns CLI labels. Keep copy separate. |
| Artwork warm start | `app/setup` + `app/browse` + `assetwarm` | App orchestrates; `assetwarm` downloads; TUI does not construct warmers (already true for browse) |
| `app.Cache` interface | Defined in `contracts.go`; browse uses `*cache.Service` | Leave unless a second implementation appears. Collapse in API-surface cleanup if it is a one-implementation leftover with no mock justification. |

### Overly generic state

- **Done (A–D):** `tui.WarmProgress.Stage` and `Model.warmStage` are `tui.WarmStage`; `tui/bootstrap` and `tui/setup` own local progress types; cache exposes dataset-only `DatasetStage` / `DatasetProgress`; warmers own their `Progress` types; and `assetwarm.Config` no longer accepts a cache stage.
- `formatWarmFooterRight` still ignores everything except native (current product behaviour; changing that is Slice H, needs approval).

### Inappropriate dependencies

| Edge | Why it is wrong | Status |
| --- | --- | --- |
| `tui/bootstrap` and `tui/setup` import `cache.ProgressFunc` | Presentation knew cache’s progress type | **Fixed** (Slice C) |
| TUI browse importing `cache` | Presentation / metadata-cache coupling | Browse package itself does **not** import `cache`. Keep that. Contract: no TUI package imports `cache`. |
| Warmers import `cache` only to emit `cache.Progress` | Download/encode packages depended on the metadata cache package for a DTO | **Fixed** (Slice D) |
| `app.sendWarmProgress` copies subsystem progress → `tui.WarmProgress` | App is the classification boundary between mechanics and presentation | **Fixed** (Slice D): app maps `nativewarm.Progress` / `assetwarm.Progress` / `warm.Progress` / `dossierwarm.Progress` to explicit TUI stages |

### Bloated files

No split/merge recommended for size. `update.go`, `engine.go`, `service.go`, and `bootstrap.go` are large because they own real state machines. Splitting without a new owner would scatter the same concerns.

### Reusable components with 2+ real use sites

| Candidate | Uses | Verdict |
| --- | --- | --- |
| Typed stage constants | 6+ emitters + 4+ consumers | **Done** (A–C). Next is splitting the cache namespace, not more constants in `cache`. |
| Bootstrap/setup progress callback local to TUI | 2 TUI packages + app adapters | **Done** (C) |
| Shared heap-queue warmer | 3 packages | **No presumption it will be implemented.** `assetwarm` has ready/retry, sprite warmers have activity rules, dossier uses different work units. Extract only if after A–D remaining duplicate code has **truly identical invariants**. A little duplication is preferable to a generic scheduler with configuration switches. |
| `sendWarmProgress` non-blocking complete-must-land | 1 function, many callers | Already centralized in app; do not extract further |
| Footer bar renderer | `formatNativeImageCacheFooter` + unused-looking `formatWarmBar` | `formatWarmBar` has no production caller except possibly tests — prove unused, then delete (Slice I). Do not generalize. |

---

## Completed slices

Order below is historical Phase 4 sequence for A–D. They stay in the plan as completed work, not as pending tasks.

### Slice R — Regression locks (done)

Tests locking current labels, footer-only-native, and stage string **values** (not the type location). Do not weaken these while moving types.

### Slice A — Typed `cache.Stage` (done — intermediate)

**Benefit:** compile-time stage set; stopped new magic strings at cache emitters. No user-visible change; constant values stayed the historical strings.

**What it is not:** the destination architecture. `cache.Stage` still names dataset, artwork, native, chafa, dossier, and pixel-asset phases. Slice D must stop treating this as a global progress namespace (`DatasetStage` in cache; per-warmer `Progress` elsewhere).

**Affected tests (historical):** `nativewarm/warm_test.go`, `warm/warm_test.go`, `cache/bootstrap_test.go`, `app/dataset_bootstrap_test.go`, `app/browse_test.go`.

**Migration that landed:**

1. `type Stage string` + constants in `internal/cache` matching then-current literals.
2. `Progress.Stage` became `Stage`.
3. Emitters use constants.
4. `string(stage)` at presentation boundaries until B/C.

**Risk:** low; residual `Stage("foo")` still possible. Ban new raw conversions in review. After D, warmer stages must not live on this type at all.

### Slice B — Typed TUI warm stages + footer/reload branches (done)

**Benefit:** TUI no longer compares raw `"native"` / `"artwork"`. Presentation policy (footer only for native; artwork completion may refresh sprite) is explicit.

**Affected tests:** `warm_footer_test.go`, `warm_footer_internal_test.go`, `spinner_ux_test.go`, `native_overlay_test.go`, `export_test.go` helpers, `browse_test.go`.

**Migration that landed:**

1. `tui.WarmStage` constants with the same string values the channel already carried.
2. `WarmProgress.Stage` and `warmProgressMsg.Stage` are `WarmStage`.
3. `app.sendWarmProgress` maps `cache.Stage` → `tui.WarmStage` (string conversion is the adapter). After D this adapter classifies **subsystem** progress types, not `cache.Stage` for native/artwork/dossier.
4. Footer and `refreshSpriteAfterWarm` switch on typed constants.

**Risk:** low; values unchanged. Do not start showing artwork/dossier bars (that is Slice H, needs approval).

### Slice C — Decouple TUI bootstrap/setup from `cache.ProgressFunc` (done)

**Benefit:** presentation packages stopped importing `cache` for a DTO. App remains the adapter.

**Affected tests:** `tui/bootstrap/model_test.go`, `tui/setup/setup_test.go`, `app/dataset_bootstrap_test.go`, `app/bootstrap_test.go`.

**Migration that landed:**

1. `tui/bootstrap` owns `Progress` + `ProgressFunc` + `Stage` (dataset stages only).
2. `tui/setup` owns a local progress callback (two small types, not a shared generic progress package).
3. App wrappers convert `cache.Progress` → TUI progress.
4. `stageLabel` switches on typed bootstrap stages; labels **unchanged**.

**Risk:** accidental label drift — labels stay locked by Slice R tests. After D, app wrappers convert `DatasetProgress` (not a global `cache.Progress`) for dataset/CLI paths.

---

## Remaining slices

Recommended sequence after A–D. D was the consolidation gate; API cleanup and dead-code deletion follow.

### Slice D — Subsystem-owned progress types (done)

**Benefit:** `nativewarm` / `assetwarm` / `warm` / `dossierwarm` stop importing the metadata cache package merely to borrow `Progress`. `cache.Stage` ceases to be a global progress namespace. Encode/download stay in owning packages; cache keeps dataset/storage progress.

This is definition-of-done for the layering refactor, not a time-boxed optional follow-up.

**Affected tests:** all warmer unit tests that construct `cache.Progress` or `cache.ProgressFunc`; `app/browse_test.go` (`sendWarmProgress`); nativewarm integration tests; any cache bootstrap test that still assumes warmer stage constants live on `cache.Stage`.

**Migration:**

1. In `internal/cache`, keep dataset/enrich/bootstrap progress as `DatasetStage` / `DatasetProgress` (rename or split from today’s `Stage` / `Progress`). Constants: `index`, `enrich`, `records`, `evolution`, `moves`, `dataset` only. Drop `StageArtwork`, `StageNative`, `StageSpritesRender`, `StageDossier`, `StageSprites` from cache.
2. Each warmer defines its own `Progress` (and a small `ProgressFunc` if needed):
   - `nativewarm.Progress`
   - `assetwarm.Progress`
   - `warm.Progress` (chafa)
   - `dossierwarm.Progress`
   Duplicated 3-field structs are fine. Meanings already differ (cells vs species vs dossier units). **Do not** invent `internal/progress`.
3. `assetwarm.Config.Stage` must not take a cache stage string/constant. Artwork identity is owned by `assetwarm` (and classified by `app` into `tui.WarmStageArtwork`).
4. `app` adapters:
   - `forwardWarm` / `sendWarmProgress` classify subsystem progress → `tui.WarmProgress` / `tui.WarmStage`.
   - Dataset/CLI paths map `DatasetProgress` → `tui/bootstrap.Progress` or CLI copy.
5. Production code: no remaining warmer import of `cache` that exists solely for progress types. `dossierwarm` may still depend on cache **storage** APIs; it must not depend on cache **event** types.
6. No new package whose only job is “progress DTO”.

**Risk:** medium (many call sites). Missed `cache.StageNative` leftovers will fail compile if those constants are removed. Do not leave `cache.Stage` as a compatibility alias for warmer stages.

### Slice J — API-surface cleanup (**Done**)

Inspect every interface, adapter, and exported internal symbol **touched by the type migration**. Remove scaffolding that A–D leave beside the new types.

**In scope:**

- Interfaces with only one implementation where mocking does not justify them.
- Compatibility adapter methods no longer called.
- Duplicated conversion functions (`cache.Progress` ↔ TUI, leftover `string(stage)` helpers, `AsCacheProgress`, dual `ProgressFunc` wrappers).
- Old stage/string helpers and warmer-stage constants left on `cache`.
- `assetwarm.Config.Stage` (or equivalent) if it exists only to inject a global label.

**Out of scope:** inventing new abstractions; splitting large files; unifying CLI/TUI copy.

**Affected tests:** whatever still constructs removed helpers; `app/browse_test.go`; warmer tests after D.

**Risk:** medium if deletions are guessed. Prove each symbol unused (call-site search + compile) before deleting. Product behaviour must not change.

### Slice I — Prove dead, then delete (done)

Internal dead code must be **proven unused**, then deleted. User approval is required for **product behaviour** changes, not for deleting a private function with no production caller. Do not keep dead code because it might support a future multi-bucket warmer.

Completed deletion proof:

- `scheduleSpritePrefetchIfIdle` returned `nil`; its only callers were the orphan prefetch-result handler. The prefetch scheduler itself was reachable only through test exports. Deleted the no-op, prefetch commands/messages/state/handlers, their test-only exports and tests, and the now-unused neighbour-render metric.
- `Engine.NativeWarmBatch` and its result type had no production callers; only `engine_batch_test.go` called them. Deleted `engine_batch.go` and that symbol-only test file, including private helpers used only by the batch implementation.
- `formatWarmBar` had no callers in production or tests. Deleted it while retaining the production `formatNativeImageCacheFooter` path and its focused tests.

Call-site searches covered production and tests. After removal, `go test ./internal/tui ./internal/render/sprite ./internal/render/sprite/nativewarm ./internal/app` passes.

**Risk:** low if proof is real; high if “unused” is assumed. `NativeWarmBatch` deletion is API-surface cleanup of `engine_batch.go` and its tests — still not a user-visible browse change.

### Slice F — `ImageCacheCoordinator` / image manifest (done)

**Rejected as destination:** do **not** wire `ImageCacheCoordinator` into browse. That would introduce a second completeness authority next to the files.

**Native-cache design:** exact content-addressed `.sixel.gz` artefacts are the source of truth. Progress is reconstructed from those files (`nativewarm` disk audit / `HasNativeFile`). Do not resurrect a giant mutable completion manifest.

**Required investigation:** if `ImageCacheCoordinator`, `image_manifest.go`, `image_session.go`, `StartImageCacheSessionDeferred`, and related tests have **no production responsibility** after the refactor (browse does not use them; CLI does not use them; they only exist as unused machinery), delete them. If something still has a documented production job (e.g. a CLI audit path that actually runs), keep that job and document it in CODEBASE.md — without making the manifest an alternate Done/Total for the TUI.

**Behaviour change if deleted:** none for browse, if truly unwired today. Tests that only exist to exercise the orphan coordinator go with it.

**Do not implement** “join coordinator + nativewarm for one source of truth” as an improvement. The files already are that source of truth.

**Decision and proof:** production call-site searches found no caller of
`ImageCacheCoordinator`, `StartImageCacheSession`,
`StartImageCacheSessionDeferred`, `ReconcileImageCacheSession`,
`ImageCacheSpec`, or the aggregate `ImageCacheManifest` outside their own
implementation. Browse and CLI paths do not construct them. Deleted
`image_session.go`, `image_manifest.go`, `image_spec.go`, and the three test
files that existed only for that machinery. The artwork invalidation test now
asserts the real contract directly: changing an artwork hash deletes native
artefacts for the old content hash.

The separate `native/manifest.json` `NativeManifest` is retained. It is a
small version/protocol stamp used by compatibility and acceptance-audit
tooling, and `scripts/acceptance_audit.go` is a production caller of its read
and filesystem-count APIs. It contains no Pokémon completion map and is not
used for nativewarm Done/Total.

### Slice K — Architecture import tests (done)

Protect the forbidden edges in the [dependency contract](#internal-dependency-contract). Keep the tests small: `go/packages` or equivalent compile-time import graph checks for the highest-value edges, not a full dep-linter framework.

Minimum edges:

- `internal/tui` and `internal/tui/bootstrap`, `internal/tui/setup` must not import `internal/cache`.
- `internal/render/sprite/nativewarm`, `assetwarm`, `warm` must not import `internal/cache`.
- `internal/cache` and `internal/dataset` must not import `internal/app` or `internal/tui`.

`dossierwarm` importing parent `cache` for **storage** is allowed; importing cache **progress types** after D is not. Encode the latter as a type-existence / import assertion once D lands, or as a grep-in-test that `cache.Progress` / `DatasetProgress` is not referenced from warmer packages that should own their own types.

**Risk:** low. Do not overfit to package-internal test imports (`cache_test`) that are legitimate.

### Slice E — Shared heap warmer core (no presumption)

**Not scheduled.** Do not automatically implement. High risk; three similar implementations are not a shared framework.

`assetwarm` has ready/retry semantics; sprite warmers have activity-gate rules; dossier deals with different work units. Extract **only** if after A–D the remaining duplicate code has truly identical invariants. Prefer “a little duplication” to a generic scheduler with configuration switches.

If that identical-invariants bar is ever met: extract only the heap + wake + started/cancel/wg into a domain-named package (`internal/warmqueue`), and **add dossierwarm tests first**. `nativewarm` stays out.

**Affected tests (only if ever extracted):** `warm/warm_test.go`, `assetwarm/warm_test.go`, `dossierwarm` (no dedicated test file today — add one before extract).

**Do not implement in this pass** unless the identical-invariants review after D explicitly says so.

### Slice G — Unify CLI/TUI bootstrap wording (rejected)

Different interfaces; no universal copy source. CLI percents vs TUI short titles are intentional. **Do not implement.**

### Slice H — Show non-native warm stages in the footer (needs approval)

**Benefit:** users see artwork/dossier/chafa progress.

**Behaviour change:** footer currently **hides** those stages by design (sixel wipe + product decision). Tests lock that (`warm_footer_internal_test.go`).

**Do not implement** without explicit product approval.

### Formatting (touched files only)

Do **not** mix repo-wide `gofmt` into this refactor. Baseline format check already **FAIL**s on dozens of pre-existing unformatted files.

**End state for this branch:** every **touched** Go file is `gofmt`-clean. Leave untouched drift alone.

A separate mechanical formatting PR can normalise the rest of the repo later. Leaving the repo permanently unable to pass its documented formatting check is not the long-term goal — it is just out of scope for this consolidation.

---

## Definition of done (consolidation gate)

The refactor is **not** finished when A–C compile. It is finished when all of the following hold:

| Criterion | Status |
| --- | --- |
| No raw production comparisons against progress stage strings | **PASS** — only test assertions in `app/browse_test.go`; production uses typed `WarmStage` / `DatasetStage` switches |
| No warmer imports `cache` solely for progress types | **PASS** — `nativewarm`, `assetwarm`, `warm` have no production `cache` import; `dossierwarm` imports neither cache nor progress types |
| TUI presentation has no `cache` dependency (browse, bootstrap, setup) | **PASS** — enforced by `internal/app/architecture_test.go` |
| `cache` no longer exports a global stage enum covering native/artwork/dossier/chafa | **PASS** — `DatasetStage` / `DatasetProgress` only; warmer stages removed from cache |
| Proved-dead prefetch / batch / format helpers deleted | **PASS** (Slice I) |
| No unused native-completeness coordinator or manifest machinery | **PASS** (Slice F) |
| All **touched** Go files formatted | **PASS** — 48 touched `*.go` files, `gofmt -l` clean |
| `go vet ./...`, `go test ./...`, `go test -race ./...` (CGO on) | **PASS** — see [End-of-plan validation](#end-of-plan-validation) |
| Cross-build (`CGO_ENABLED=0`) | **PASS** — linux/amd64, darwin/amd64, darwin/arm64, windows/amd64 → `.cross-build/` |
| Existing startup / native-cache acceptance behaviour unchanged | **PASS** — non-regression locks retained; no browse-path coordinator resurrection |

**Consolidation gate: PASS.** Remaining slices E (not presumed), G (rejected), H (needs approval) are out of scope.

---

## Phase 4 status

| Slice | Status |
| --- | --- |
| R — regression tests locking current labels, footer-only-native, stage string values | **Done** |
| A — typed `cache.Stage` | **Done** (intermediate; not the final namespace) |
| B — typed `tui.WarmStage` | **Done** |
| C — bootstrap/setup drop `cache.ProgressFunc` | **Done** |
| D — subsystem-owned progress; warmers drop `cache` progress import; `cache.Stage` not global | **Done** |
| J — API-surface cleanup after D | **Done** |
| I — prove-dead-then-delete (prefetch, `NativeWarmBatch`, `formatWarmBar`, …) | **Done** |
| F — ImageCacheCoordinator / manifest | **Done** — orphan coordinator and aggregate completion manifest deleted; small native version/protocol stamp retained |
| K — architecture import tests | **Done** |
| E — shared heap queue | **No presumption.** Only if identical invariants remain after A–D |
| H — footer for artwork/dossier/chafa | **Needs approval** — product behaviour |
| G — unify CLI/TUI copy | **Rejected** |
| Repo-wide gofmt | **Out of scope** for this branch; touched files must be clean |

---

## Phase 3 baseline

Recorded on the working tree before production refactors. Commands are Windows PowerShell from the repo root.

See [Baseline results](#baseline-results) at the bottom — filled after the suite runs.

### Validation commands

```powershell
# Format: CI uses gofmt -l on all *.go; Makefile uses cmd/internal/pkg only.
# This refactor formats touched files only. Do not mass-gofmt the repo here.
Get-ChildItem -Recurse -Filter *.go | Where-Object { $_.FullName -notmatch '\\(vendor|bin)\\' } | ForEach-Object { $_.FullName } | ForEach-Object { gofmt -l $_ }

go vet ./...
go test ./...

# Race detector runs on Windows with CGO_ENABLED=1 (gcc required). Recorded PASS ~197s
# on this tree. Linux CI (`go test -race ./...` on ubuntu-latest) remains the merge gate.
$env:CGO_ENABLED = "1"; go test -race ./...

# Cross-build (Makefile cross-build)
$env:GOOS = "linux";   $env:GOARCH = "amd64"; go build -o .cross-build/poke-linux-amd64 ./cmd/poke
$env:GOOS = "darwin";  $env:GOARCH = "amd64"; go build -o .cross-build/poke-darwin-amd64 ./cmd/poke
$env:GOOS = "darwin";  $env:GOARCH = "arm64"; go build -o .cross-build/poke-darwin-arm64 ./cmd/poke
$env:GOOS = "windows"; $env:GOARCH = "amd64"; go build -o .cross-build/poke-windows-amd64.exe ./cmd/poke
```

Equivalent: `make fmt-check vet test race cross-build` when Make/bash/`gcc` are available.

---

## Phase 4 execution notes

- After every remaining slice: focused tests for touched packages; `gofmt` those files.
- After D+J+I+K (and coordinator investigation): full vet, unit, race (`CGO_ENABLED=1`), cross-build with `CGO_ENABLED=0`.
- Stop before any user-visible string or footer change (Slice H still needs approval; G is rejected).
- Do not commit; do not touch `bin/poke.exe` except as a local build output.
- Do not implement Slice E unless the post-D identical-invariants review is written down and affirmative.

---

## Baseline results

Recorded on the current working tree **before** production refactors (Windows, PowerShell, Go 1.26).

| Check | Result | Notes |
| --- | --- | --- |
| Format (`gofmt -l` all `*.go` except vendor/bin) | **FAIL (pre-existing)** | Dozens of files already unformatted, including `internal/app/bootstrap.go`, much of `internal/cache`, `internal/dataset`, `internal/cli/root.go`, several sprite/TUI files, and `scripts/`. **Not fixed** as part of this refactor. Touched files on this branch must still be gofmt-clean. |
| `go vet ./...` | **PASS** | |
| `go test ./...` | **PASS** | ~24s; packages with no tests listed as `?` |
| `go test -race ./...` (`CGO_ENABLED=1`) | **PASS** | ~197s; `internal/render/sprite/image` dominates (~167s). Windows **can** run the race detector with CGO/gcc; this is not a Windows limitation. Linux CI remains the authoritative merge gate. |
| Cross-build CI targets | **PASS** with `CGO_ENABLED=0` | `linux/amd64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64` → `.cross-build/`. Fail if CGO is left on (host gcc cannot cross-compile `runtime/cgo`). CI/Makefile implicitly use CGO off for `GOOS`/`GOARCH` cross builds. |
| golangci-lint | **N/A** | Not configured |

No pre-existing test/vet/race failures. Format drift is unrelated and left alone except for files this refactor touches.

**After slices R+A+B+C:** `go vet` (touched packages), `go test ./...`, `go test -race ./...`, and `CGO_ENABLED=0` cross-build all **PASS**. Format still fails on the same pre-existing unformatted files (not mass-fixed).

---

## End-of-plan validation

Recorded after slices D+J+I+F+K on branch `codebase-cleanup` (Windows, PowerShell, Go 1.26). Commands run from repo root; no commit.

| Check | Result | Notes |
| --- | --- | --- |
| `gofmt -l` (48 touched `*.go` files only) | **PASS** | No dirty files; repo-wide drift unchanged |
| `go vet ./...` | **PASS** | |
| `go test ./...` | **PASS** | ~22s |
| `go test -race ./...` (`CGO_ENABLED=1`) | **PASS** | ~184s; `internal/render/sprite/image` dominates (~139s) |
| Cross-build (`CGO_ENABLED=0`) | **PASS** | `.cross-build/poke-{linux,darwin-amd64,darwin-arm64,windows-amd64}` |
| DoD grep: production stage string comparisons | **PASS** | Raw `"native"` / `"artwork"` comparisons only in tests |
| DoD grep: warmer packages → `cache` import | **PASS** | No production warmer imports cache for progress; `nativewarm/integration_test.go` test import only |

### Skipped / deferred slices (not blocking)

| Slice | Status | Reason |
| --- | --- | --- |
| E — shared heap warmer core | **Not scheduled** | No presumption; identical-invariants bar not met |
| G — unify CLI/TUI bootstrap wording | **Rejected** | Intentional copy differences |
| H — footer for artwork/dossier/chafa | **Needs approval** | Product behaviour change |
| Repo-wide gofmt | **Out of scope** | Pre-existing drift; separate mechanical PR |

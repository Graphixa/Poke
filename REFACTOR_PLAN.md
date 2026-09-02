# Refactor plan

Grounded in [CODEBASE.md](CODEBASE.md) and the current tree (including uncommitted TUI/sixel/test work). This is not a rewrite of `plans/pokedex-backend-refactor-plan.md` (that plan is a sixel/cache product spec). This plan is about ownership, typed state, and layering.

**Constraint:** preserve public behaviour unless the user explicitly approves a change. Slices marked **needs approval** are documented only.

**Constraint:** no generic utility packages; no file split/merge for size alone; no speculative abstractions with one use.

---

## Findings

### Duplicated behaviour

| What | Sites | Notes |
| --- | --- | --- |
| Heap warmer (P0/P1/P2, `queued`/`done`, wake chan, `Prioritize`, `SetNeighborsFromList`, `Snapshot`) | `render/sprite/warm`, `render/sprite/assetwarm`, `cache/dossierwarm` | Real 3-site clone. `nativewarm` is a different algorithm (single lane + disk audit) — do not force it into the same abstraction. |
| Dataset stage → user copy | `app.formatBootstrapProgress`, `tui/bootstrap.stageLabel`, `Handler.ensureIndex` | Same stage keys; **wording is not identical** (CLI includes percents; TUI titles are short). Unifying copy would change public text. |
| Progress DTO | `cache.Progress`, `tui.WarmProgress`, `tui/bootstrap.progressMsg`, `cache.ImageCacheProgress` | Same bag of Done/Total/Message/Stage. |
| Warmer Mode string enums | `warm.Mode`, `assetwarm.Mode`, `dossierwarm.Mode` | Typed aliases already, but values are still strings and each package redefines `off`/`window`. Fine as package-local types. Do not merge into one Mode type (different value sets). |

### Unclear ownership

| Issue | Today | Should own |
| --- | --- | --- |
| Progress bus | `cache.Progress` used by cache, all warmers, app, TUI bootstrap/setup | Mechanics emit typed events in owning packages; app maps; TUI renders |
| Native cache completeness | `nativewarm` counts Done/Total; `ImageCacheCoordinator` + image manifest exist but are **not wired** to browse | Cache owns on-disk completeness; `nativewarm` owns opportunistic encode; app may later join them (approval) |
| Bootstrap presentation | App formats CLI lines; `tui/bootstrap` formats TUI; both switch on raw strings | TUI owns TUI labels; app owns CLI labels; both switch on typed stages |
| Artwork warm start | `app/setup` + `app/browse` + `assetwarm` | App orchestrates; `assetwarm` downloads; TUI does not construct warmers (already true for browse) |
| `app.Cache` interface | Defined in `contracts.go`; browse uses `*cache.Service` | Leave unless a second implementation appears |

### Overly generic state

- `cache.Progress.Stage string` — dataset (`index`, `enrich`, `records`, `evolution`, `moves`, `dataset`), artwork (`artwork`), native (`native`), chafa (`sprites_render`), dossier (`dossier`), pixel assets (`sprites`).
- `tui.WarmProgress.Stage string` and `Model.warmStage string`.
- `tui/update.go` branches on `msg.Stage == "native"` and `"artwork"`.
- `formatWarmFooterRight(stage string)` ignores everything except `"native"` (current product behaviour).
- `assetwarm.Config.Stage string` lets callers invent labels (`app/browse.go` sets `"artwork"`).

### Inappropriate dependencies

| Edge | Why it is wrong |
| --- | --- |
| `tui/bootstrap` and `tui/setup` import `cache.ProgressFunc` | Presentation knows cache’s progress type |
| Warmers import `cache` only to emit `cache.Progress` | Download/encode packages depend on the metadata cache package for a DTO |
| `app.sendWarmProgress` copies `cache.Progress` → `tui.WarmProgress` field-for-field | Acceptable adapter **if** types become distinct and mapped; today they are clones |

TUI browse package itself does **not** import `cache`. Keep that.

### Bloated files

No split/merge recommended for size. `update.go`, `engine.go`, `service.go`, and `bootstrap.go` are large because they own real state machines. Splitting without a new owner would scatter the same concerns.

### Reusable components with 2+ real use sites

| Candidate | Uses | Verdict |
| --- | --- | --- |
| Typed stage constants | 6+ emitters + 4+ consumers | **Do now** |
| Bootstrap/setup progress callback local to TUI | 2 TUI packages + app adapters | **Do now** |
| Shared heap-queue warmer | 3 packages | **Safe later**, high risk of silent priority/progress drift — extract only after typed stages exist and with golden warmer tests |
| `sendWarmProgress` non-blocking complete-must-land | 1 function, many callers | Already centralized in app; do not extract further |
| Footer bar renderer | `formatNativeImageCacheFooter` + unused-looking `formatWarmBar` | `formatWarmBar` has no production caller except tests possibly — do not generalize |

---

## Slices (independently testable)

Order is the recommended Phase 4 sequence. Each slice lists benefit, tests, migration, risk, and **safe now** vs **needs approval**.

### Slice A — Typed `cache.Stage` (safe now)

**Benefit:** compile-time stage set; stops new magic strings; callers cannot invent `"warming"` in cache emitters without a constant. No user-visible change if constant values stay the current strings.

**Affected tests:** any test comparing `Progress.Stage == "native"` etc. (`nativewarm/warm_test.go`, `warm/warm_test.go`, `cache/bootstrap_test.go`, `app/dataset_bootstrap_test.go`, `app/browse_test.go`). They keep compiling if `Stage` is a string-defined type.

**Migration:**

1. Add `type Stage string` + constants in `internal/cache` matching current literals.
2. Change `Progress.Stage` to `Stage`.
3. Replace every emitter literal with the constant.
4. Keep `string(stage)` at presentation boundaries until Slice B/C.

**Risk:** low. Missed literal still type-checks if assigned from a raw string (`Stage("foo")` possible). Ban new raw conversions in review.

### Slice B — Typed TUI warm stages + footer/reload branches (safe now)

**Benefit:** TUI no longer compares raw `"native"` / `"artwork"`. Presentation policy (footer only for native; artwork completion may refresh sprite) is explicit.

**Affected tests:** `warm_footer_test.go`, `warm_footer_internal_test.go`, `spinner_ux_test.go`, `native_overlay_test.go`, `export_test.go` helpers, `browse_test.go`.

**Migration:**

1. Add `tui.WarmStage` constants with the same string values the channel already carries.
2. Change `WarmProgress.Stage` and `warmProgressMsg.Stage` to `WarmStage`.
3. `app.sendWarmProgress` maps `cache.Stage` → `tui.WarmStage` (string conversion is the adapter).
4. Footer and `refreshSpriteAfterWarm` switch on typed constants.

**Risk:** low if values unchanged. Do not start showing artwork/dossier bars (that is Slice X).

### Slice C — Decouple TUI bootstrap/setup from `cache.ProgressFunc` (safe now)

**Benefit:** presentation packages stop importing `cache` for a DTO. App remains the adapter. Matches “UI in TUI, orchestration in app”.

**Affected tests:** `tui/bootstrap/model_test.go`, `tui/setup/setup_test.go`, `app/dataset_bootstrap_test.go`, `app/bootstrap_test.go`.

**Migration:**

1. Give `tui/bootstrap` its own `Progress` + `ProgressFunc` + `Stage` (dataset stages only).
2. Give `tui/setup` a similarly local progress callback (or reuse bootstrap’s if the fields match — only if both are dataset/artwork progress with the same shape **and** both already exist as use sites; prefer two small types over a shared “generic progress” package).
3. App wrappers convert `cache.Progress` → TUI progress.
4. `stageLabel` switches on typed bootstrap stages; labels **unchanged**.

**Risk:** low. Accidental label drift if someone “cleans up” copy — lock labels with tests first (Slice R).

### Slice D — Stop warmers depending on `cache` just for `Progress` (safe now, after A)

**Benefit:** `nativewarm` / `assetwarm` / `warm` no longer import the metadata cache package. Encode/download stay in owning packages.

**Affected tests:** all warmer unit tests that construct `cache.Progress` or `cache.ProgressFunc`.

**Migration:**

1. After Slice A, either:
   - define a tiny `Progress` + `Stage` in each warmer (duplicated 3-field struct), **or**
   - keep emitting `cache.Progress` until a dedicated typed event exists per warmer.
2. Preferred without a new package: each warmer gets `type Progress = struct{ Stage … }` **or** a function type `func(done, total int, id int)` plus a typed stage field on Config.
3. App `forwardWarm` / `sendWarmProgress` maps warmer progress → `tui.WarmProgress`.

**Risk:** medium (many call sites). Do **not** invent `internal/progress`. Prefer per-package progress structs — they already have different meanings.

**Phase 4 choice:** implement A+B+C now; leave D if time-boxed, documented as next.

### Slice E — Shared heap warmer core (safe later)

**Benefit:** one priority-queue implementation for the three identical warmers; fewer missed `Stop`/`Snapshot` bugs.

**Affected tests:** `warm/warm_test.go`, `assetwarm/warm_test.go`, `dossierwarm` (no dedicated test file today — **add one before extract**).

**Migration:** extract only the heap + wake + started/cancel/wg into an unexported helper **inside one of the three packages** and have the others… that would create an odd import (`assetwarm` importing `warm`). A new package is only justified with 3 use sites **and** a stable API. That new package would be a warmer queue, not a generic util — acceptable if named for the domain (`internal/warmqueue`) **after** dossierwarm tests exist.

**Risk:** high. Priority, retry, and progress semantics differ slightly (`assetwarm.ready`, `warm.ActivityGate`, dossier core vs full). **Do not implement in this pass.**

### Slice F — Wire `ImageCacheCoordinator` into browse (needs approval)

**Benefit:** one completeness source of truth; restart progress matches disk.

**Behaviour change:** Done/Total and footer timing may change (deferred audit vs nativewarm’s in-process count). User-visible.

**Do not implement.**

### Slice G — Unify CLI/TUI bootstrap wording (needs approval)

**Benefit:** one copy source.

**Behaviour change:** CLI percents vs TUI short titles differ today. Unifying changes one of them.

**Do not implement.**

### Slice H — Show non-native warm stages in the footer (needs approval)

**Benefit:** users see artwork/dossier/chafa progress.

**Behaviour change:** footer currently **hides** those stages by design (sixel wipe + product decision). Tests lock that (`warm_footer_internal_test.go`).

**Do not implement.**

### Slice I — Remove dead prefetch / `NativeWarmBatch` / unused `formatWarmBar` (needs approval or extra proof)

`scheduleSpritePrefetchIfIdle` is a no-op; prefetch cmds still exist; `NativeWarmBatch` is unused by `nativewarm`. Deleting is a behaviour/API surface change for tests and possible future multi-bucket warm.

**Do not implement** unless a follow-up explicitly wants dead-code removal.

---

## Phase 4 status

| Slice | Status |
| --- | --- |
| R — regression tests locking current labels, footer-only-native, stage string values | **Done** |
| A — typed `cache.Stage` | **Done** |
| B — typed `tui.WarmStage` | **Done** |
| C — bootstrap/setup drop `cache.ProgressFunc` | **Done** |
| D — warmers drop `cache` import | Remaining (next) |
| E — shared heap queue | Remaining (needs dossierwarm tests first) |
| F, G, H, I | **Needs approval** — not implemented |

---

## Phase 3 baseline

Recorded on the working tree before production refactors. Commands are Windows PowerShell from the repo root.

See [Baseline results](#baseline-results) at the bottom — filled after the suite runs.

### Validation commands

```powershell
# Format (CI uses gofmt -l on all *.go; Makefile uses cmd/internal/pkg only)
Get-ChildItem -Recurse -Filter *.go | Where-Object { $_.FullName -notmatch '\\(vendor|bin)\\' } | ForEach-Object { $_.FullName } | ForEach-Object { gofmt -l $_ }

go vet ./...
go test ./...
# Race needs CGO/gcc on Windows; CI runs this on ubuntu
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

- After every slice: focused tests for touched packages.
- After A+B+C: full format, vet, unit, race (if CGO), cross-build.
- Stop before any user-visible string or footer change.
- Do not commit; do not touch `bin/poke.exe` except as a local build output.

---

## Baseline results

Recorded on the current working tree **before** production refactors (Windows, PowerShell, Go 1.26).

| Check | Result | Notes |
| --- | --- | --- |
| Format (`gofmt -l` all `*.go` except vendor/bin) | **FAIL (pre-existing)** | Dozens of files already unformatted, including `internal/app/bootstrap.go`, much of `internal/cache`, `internal/dataset`, `internal/cli/root.go`, several sprite/TUI files, and `scripts/`. **Not fixed** as part of this refactor. |
| `go vet ./...` | **PASS** | |
| `go test ./...` | **PASS** | ~24s; packages with no tests listed as `?` |
| `go test -race ./...` (`CGO_ENABLED=1`) | **PASS** | ~197s; `internal/render/sprite/image` dominates (~167s) |
| Cross-build CI targets | **PASS** with `CGO_ENABLED=0` | `linux/amd64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64` → `.cross-build/`. Fail if CGO is left on (host gcc cannot cross-compile `runtime/cgo`). CI/Makefile implicitly use CGO off for `GOOS`/`GOARCH` cross builds. |
| golangci-lint | **N/A** | Not configured |

No pre-existing test/vet/race failures. Format drift is unrelated and left alone.

**After slices R+A+B+C:** `go vet` (touched packages), `go test ./...`, `go test -race ./...`, and `CGO_ENABLED=0` cross-build all **PASS**. Format still fails on the same pre-existing unformatted files (not mass-fixed).

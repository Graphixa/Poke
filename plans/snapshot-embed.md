# Embedded dataset snapshot

Speed up first launch and `poke refresh` by shipping the Pokédex JSON inside the binary. The PokeAPI crawl stays as the fallback when that file is missing or the unpack fails.

Branch: `optimizations`. Do not commit this work to `main`.

## Outcome

A fresh install opens the browse UI after one local unpack plus the existing artwork download. It does not issue thousands of PokeAPI requests unless the embedded snapshot cannot be used.

Measured on this machine (`%LocalAppData%\pokedex\datasets\2026-11-18T064106`, 1,025 species, manifest `complete: true`):

| File | Size |
| --- | --- |
| `learnsets.json` | 63.9 MiB |
| `encounters.json` | 22.0 MiB |
| `species.json` | 3.0 MiB |
| `pokemon.json` | 0.9 MiB |
| Other 9 JSON files | 1.5 MiB |
| **Raw total** | **91.3 MiB** |

`poke_1.0.0_windows_amd64.zip` is 15.9 MiB. Gzip of this JSON is expected around 15–25 MiB, so the release zip lands around 30–40 MiB. Winget and Chocolatey verify SHA256 of that zip. Size is not one of their checks. The nupkg stays a downloader script (`build/chocolatey/tools/chocolateyInstall.ps1`).

## Locked decisions

1. Keep the aggregate JSON files in `internal/dataset/layout.go`. No SQLite. No `001-bulbasaur` folder per Pokémon for this data. Those folders stay pictures only (`PokemonFolderKey` in `internal/cache/paths.go`).
2. `manifest.json` stays a short header. Add `source_sha` (the api-data commit, or a hash of the packed bytes when the pack was built from a local dataset).
3. Ship the gzip inside the exe with `//go:embed`, so install.ps1, Winget, Chocolatey, and Homebrew keep installing one binary.
4. The gzip is produced in the release workflow before Goreleaser. Do not commit the real ~20 MiB blob. A tiny committed fixture gzip exists only so `go test` compiles. Release builds overwrite `internal/bundled/dataset.tar.gz` before `go build`.
5. `poke refresh` and first launch try the embedded snapshot first. They call `BootstrapDataset` only when the unpack fails.
6. Artwork stays a CDN download into `sprites/` and `official-artwork/`. It is not in the gzip.
7. Include PokeAPI's BSD-3-Clause `LICENSE.txt` inside the gzip. Keep the README credit and the Nintendo trademark line. Do not bundle PNGs.

## What the user sees

First run (`runSetupIfNeeded`, `Ask: true`) keeps its questions.

1. Pick a theme.
2. Answer the update-check question.
3. Intro, replacing `setupIntro` in `internal/tui/setup/model.go`:

```
Poke will import the Pokémon data and download the artwork.
This only happens once, and it won't take long.
```

4. Bar stage `import`: "Importing Pokémon data". Seconds. One stage.
5. Bar stage `artwork`: "Downloading Pokémon artwork". This is still the long part. Use this label on both the embed path and the crawl path.
6. Browse opens.

`downloadReserve` in `internal/app/setup.go` keeps the large floors (`records` 1025, `evolution` 500, `moves` 900, `ability` 300, `form` 1600) for the crawl only. The embed path reports `import` then `artwork`. It must not pretend to walk those floors.

A cleared cache with setup already done skips the questions. Title stays "Refreshing Pokédex". Same order: unpack, then artwork.

Crawl fallback labels stay as they are: "Gathering Pokémon records", "Learning Pokémon movesets", and the rest of `stageLabels`.

## Runtime order

`prepareWork` in `internal/app/setup.go` is the only user-facing entry. `RefreshDataset` in `internal/cache/service.go` is the other. Both use one helper.

```
need a dataset?
  embedded sha == active manifest source_sha
    and the active snapshot is complete
      -> do nothing
  else unpack embed into datasets/<id>/ and ActivateDataset
  unpack failed (missing file, bad gzip, incomplete manifest, write error)
      -> BootstrapDataset (today's crawl)
then artwork, unchanged
```

Rules:

- Cancel (`context.Canceled`) does not fall through to the crawl.
- A failed unpack leaves any previous active snapshot in place. `ActivateDataset` already works this way. Do not delete the active id first.
- Unpack writes a new dataset id, then activates. Same staging rule as bootstrap.
- `source_sha` on the embedded manifest is compared to the active manifest. Equal and complete: skip the data stage.
- `poke refresh` on the same binary is a no-op for JSON. A newer binary carries a new sha and unpacks beside the old snapshot.

## Gzip layout

One tar.gz. Paths inside the archive match `internal/dataset/layout.go`, plus the license:

```
manifest.json          # includes source_sha, complete: true
browse.json
pokemon.json
species.json
learnsets.json
moves.json
evolutions.json
encounters.json
versions.json
version_groups.json
types.json
abilities.json
forms.json
LICENSE.txt            # BSD-3-Clause, PokéAPI contributors
```

`bootstrap.json` is a crawl report. Leave it out of the embed.

`DatasetManifest` gains:

```go
SourceSHA string `json:"source_sha,omitempty"`
```

Schema version stays 4. Old snapshots have an empty `source_sha`, so the first run of a new binary unpacks once.

## Code map

| Piece | Where | What |
| --- | --- | --- |
| Embed | `internal/bundled/dataset.go` | `//go:embed dataset.tar.gz`, `Bytes()`, `SHA()` of the embedded manifest |
| Unpack | `internal/cache/embed.go` | gunzip + tar into a new dataset dir, require `complete: true`, then `ActivateDataset` |
| Setup | `internal/app/setup.go` `prepareWork` | try unpack, else `BootstrapDataset` |
| Refresh | `internal/cache/service.go` `RefreshDataset` | same helper, not a second copy of the branch |
| Labels | `internal/tui/setup/model.go` | intro, `import`, artwork wording |
| Bar floors | `internal/app/setup.go` `downloadReserve` | floors only when the crawl actually runs |
| Pack script | `scripts/pack_dataset.go` | stdlib `archive/tar` + `compress/gzip`. Input: a dataset directory. Output: `internal/bundled/dataset.tar.gz` |
| Release | `.goreleaser.yaml` `before.hooks` or the release workflow | pack or download the built gzip, then Goreleaser. Tests use the fixture, not the release blob |

Reuse `writeAtomicRename`, `ActivateDataset`, and `ReadDatasetManifest`. Do not add a database driver. Do not add a second snapshot format.

## Fallback

`BootstrapDataset` stays. Tests that point the client at `httptest` keep using it. The embed path is an earlier return, not a deletion of the crawl.

Fall back when:

- the embedded gzip is the test fixture (manifest not `complete`, or `source_sha` empty and `pokemon_count` is 0)
- gzip or tar is corrupt
- a required JSON file is missing
- manifest `schema_version` is not `dataset.SchemaVersion`
- manifest `complete` is false
- the disk write fails

Do not fall back when the context is canceled.

An incomplete gzip (manifest not `complete`, or `pokemon_count` is 0) is rejected by unpack and by the release check.

## Release workflow

CI has no copy of the dataset, so `internal/bundled/dataset.tar.gz` in git is the snapshot the binary embeds. It is the pack of the local complete snapshot, plus `LICENSE.txt`.

Goreleaser `before.hooks` runs `go run ./scripts/pack_dataset.go -release` before `go build`:

1. If `POKE_DATASET_DIR` is set, repack that directory over `dataset.tar.gz`. `POKE_DATASET_SHA` or `-sha` sets `source_sha`. Otherwise the sha is the SHA-256 of the manifest.
2. Refuse to build when the archive is incomplete, missing a required JSON file, or missing `LICENSE.txt`.

Checksums, Winget, and Chocolatey are unchanged. They hash the zip Goreleaser uploads.

Building that directory from a pinned `PokeAPI/api-data` commit is still later. This branch does not download api-data.

## Legal

The packed `LICENSE.txt` is the BSD-3-Clause text from `https://github.com/PokeAPI/api-data/blob/master/LICENSE.txt` (copyright PokéAPI contributors). Redistribution is allowed with that notice kept. Do not name the file or the app "PokeAPI". README trademark line stays. No PNGs in the archive.

## Out of scope

- SQLite, per-Pokémon JSON folders, GraphQL, ETag walks, species-list sniffing.
- A `poke-data` repo.
- A weekly rebuild. A newer snapshot ships inside the next app release.
- Bundling artwork.
- Deleting `internal/pokeapi` or the crawl.

## Tests

One test file, `internal/cache/embed_test.go`:

1. Unpack a tiny complete gzip (one manifest with `complete: true`, `source_sha: "abc"`, plus empty `{}` objects for each required JSON file). Active id changes. Second unpack with the same sha does not write another id.
2. A gzip with `complete: false` returns an error and does not change the active id.
3. Canceled context returns `context.Canceled` and does not report a fallback-worthy success.

`internal/tui/setup/setup_test.go` already checks stage labels. Update the artwork expectation from "Saving Pokémon artwork" to "Downloading Pokémon artwork", and assert the import label.

Do not add a test that hits pokeapi.co.

## Work order

1. Manifest field `source_sha`.
2. `Unpack` plus `embed_test.go`.
3. Fixture gzip and `//go:embed`.
4. `prepareWork` and `RefreshDataset` call the helper. Crawl only after a non-cancel unpack error.
5. Intro and stage labels. Bar floors only on the crawl path.
6. `scripts/pack_dataset.go`. Pack `%LocalAppData%\pokedex\datasets\2026-11-18T064106` once locally to confirm the gzip is in the 15–25 MiB range. Do not commit that gzip.
7. `go test` for `internal/cache` and `internal/tui/setup`.

Stop after step 7. The api-data CI fetch is a later change.

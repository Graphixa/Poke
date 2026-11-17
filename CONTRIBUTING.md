# Contributing to poke

Thanks for poking around. Small, focused changes are welcome.

## Setup

Requires Go 1.26+.

```bash
git clone https://github.com/Graphixa/Poke.git
cd Poke
go build -o poke ./cmd/poke
```

## Checks

```bash
make check          # fmt-check, vet, staticcheck, gosec, race, cross-build
make release-check  # goreleaser check + snapshot + archive verify
```

On Windows without `bash` on `PATH`:

```powershell
pwsh -File scripts/verify-release-archives.ps1
```

## Architecture

See [docs/CODEBASE.md](docs/CODEBASE.md) for package ownership, flows, and import rules. Prefer fixing shared helpers once over patching every caller.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/) subjects (`feat`, `fix`, `docs`, …). Release notes are grouped from those prefixes by GoReleaser.


## Documentation

The [README](README.md) is the short introduction for end users. Keep command reference material, configuration details and troubleshooting in the linked [user documentation](docs/USER_GUIDE.md).

When changing a command, setting or keyboard shortcut, update the relevant guide. Check relative links and keep examples aligned with the implementation. Theme palette swatches in `assets/theme-palettes.svg` use the colours in `internal/theme/themes/*.yaml`; update the swatches if those palettes change. Use genuine app captures for screenshots.

## Releases

CI checks formatting, vet, staticcheck, gosec, race tests, cross-platform builds and snapshot archives. Pushing a `v*` tag runs the release workflow, which publishes GitHub Release archives and checksums, updates the Homebrew tap and submits a Winget pull request. Package-manager availability may depend on review after the release. Chocolatey packaging is handled by its separate workflow.

## Licence

By contributing, you agree that your contributions are licensed under the [GPL-3.0](LICENSE) licence that covers this project.


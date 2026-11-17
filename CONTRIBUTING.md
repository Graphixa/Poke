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

## License

By contributing, you agree that your contributions are licensed under the [GPL-3.0](LICENSE) license that covers this project.

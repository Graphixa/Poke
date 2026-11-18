.PHONY: fmt fmt-check vet staticcheck gosec govulncheck test race build cross-build check release-check verify-archives

GO ?= go
PKGS := ./...
CROSS_DIR := .cross-build
GORELEASER ?= goreleaser
STATICCHECK ?= staticcheck
GOSEC ?= gosec
GOVULNCHECK ?= govulncheck
STATICCHECK_VERSION := 2026.2.1
GOSEC_VERSION := v2.29.0
GOVULNCHECK_VERSION := v1.10.0
GO_SRC := $$(find cmd internal pkg scripts -name '*.go' -type f)

# Prefer Git Bash on Windows when `bash` is not on PATH.
BASH ?= $(shell command -v bash 2>/dev/null || command -v "C:/Program Files/Git/bin/bash.exe" 2>/dev/null || echo bash)

fmt:
	gofmt -w $(GO_SRC)

fmt-check:
	@files=$$(gofmt -l $(GO_SRC)); \
	if [ -n "$$files" ]; then echo "gofmt needed on:"; echo "$$files"; exit 1; fi

vet:
	$(GO) vet $(PKGS)

staticcheck:
	@$(GO) install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	$(STATICCHECK) $(PKGS)

# G104/G115/G301/G304/G306/G404: local CLI noise (Close-on-err, color casts, user-dir perms, path opens, math/rand pick).
gosec:
	@$(GO) install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	$(GOSEC) -exclude=G104,G115,G301,G304,G306,G404 $(PKGS)

govulncheck:
	@$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	$(GOVULNCHECK) $(PKGS)

test:
	$(GO) test $(PKGS)

race:
	@echo "Note: -race requires CGO (gcc). On Windows without a C toolchain, rely on CI."
	CGO_ENABLED=1 $(GO) test -race $(PKGS)

build:
	$(GO) build -o poke$(shell go env GOEXE) ./cmd/poke

cross-build:
	mkdir -p $(CROSS_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build -o $(CROSS_DIR)/poke-linux-amd64 ./cmd/poke
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(CROSS_DIR)/poke-darwin-amd64 ./cmd/poke
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(CROSS_DIR)/poke-darwin-arm64 ./cmd/poke
	GOOS=windows GOARCH=amd64 $(GO) build -o $(CROSS_DIR)/poke-windows-amd64.exe ./cmd/poke

check: fmt-check vet staticcheck gosec govulncheck race cross-build

# Validates Goreleaser config, builds snapshot archives (no publish), checks poke+pokedex names.
release-check:
	$(GORELEASER) check
	$(GORELEASER) release --snapshot --clean --skip=publish
	$(MAKE) verify-archives

verify-archives:
	@$(BASH) scripts/verify-release-archives.sh

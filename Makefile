.PHONY: fmt fmt-check vet test race build cross-build check release-check verify-archives

GO ?= go
PKGS := ./...
CROSS_DIR := .cross-build
GORELEASER ?= goreleaser

# Prefer Git Bash on Windows when `bash` is not on PATH.
BASH ?= $(shell command -v bash 2>/dev/null || command -v "C:/Program Files/Git/bin/bash.exe" 2>/dev/null || echo bash)

fmt:
	gofmt -w $$(find cmd internal pkg -name '*.go' -type f)

fmt-check:
	@test -z "$$(gofmt -l $$(find cmd internal pkg -name '*.go' -type f))"

vet:
	$(GO) vet $(PKGS)

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

check: fmt-check vet race cross-build

# Validates Goreleaser config, builds snapshot archives (no publish), checks poke+pokedex names.
release-check:
	$(GORELEASER) check
	$(GORELEASER) release --snapshot --clean --skip=publish
	$(MAKE) verify-archives

verify-archives:
	@$(BASH) scripts/verify-release-archives.sh

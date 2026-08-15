# atomicfile Makefile
#
# Lib profile (conform.json): the verb contract minus deploy —
#   check — fast gate: vet + lint + test + build + conform
#   audit — exhaustive: check + race + govulncheck
# Run `make help` for the full target list.

.DEFAULT_GOAL := check

SHELL := /bin/bash
.SHELLFLAGS := -euo pipefail -c

.PHONY: help check audit vet lint test build race vuln selfcheck

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z0-9_-]+:.*?## / { printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

check: vet lint test build selfcheck ## Fast gate: vet + lint + test + build + conform
	@echo "=== check pass ==="

audit: check race vuln ## Exhaustive: check + race + govulncheck
	@echo "=== audit pass ==="

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (version pinned in .sandbox/project.conf)
	golangci-lint run ./...

test: ## Run tests
	go test -count=1 ./...

build: ## Compile all packages
	go build ./...

race: ## Run tests under the race detector
	go test -race -count=1 ./...

vuln: ## Run govulncheck
	govulncheck ./...

# Fleet gate (sd-th5.18): conform is pinned as a go.mod tool dependency
# (go.sum-verified); bumping the pin is a deliberate PR.
selfcheck: ## Run conform (fleet SDLC checker) against this repo
	go tool conform

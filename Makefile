# atomicfile Makefile
#
# Lib profile (docs/repo-standard.json): the verb contract minus deploy —
#   check — fast gate: vet + lint + test + build + repo-standard
#   audit — exhaustive: check + race + fuzz + dupe + nilcheck + deadcode + omitempty + vuln
# Run `make help` for the full target list.

.DEFAULT_GOAL := check

SHELL := /bin/bash
.SHELLFLAGS := -euo pipefail -c

# Tool pins. The audit tools run at these pins through `go run` or npx, so no one
# installs them by hand. golangci-lint's pin lives in .sandbox/project.conf.
REPO_STANDARD_VER := v0.4.0
NILAWAY_VER := v0.0.0-20260918162853-acb8859b9031
GOVULNCHECK_VER := v1.9.0
DEADCODE_VER := v0.52.0
JSCPD_VER := 5.1.2
OMITEMPTYSCALAR_VER := v0.4.0

.PHONY: help check audit vet lint test build race fuzz dupe nilcheck deadcode omitempty vuln standard clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z0-9_-]+:.*?## / { printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

check: vet lint test build standard ## Fast gate: vet + lint + test + build + repo-standard
	@echo "=== check pass ==="

audit: check race fuzz dupe nilcheck deadcode omitempty vuln ## Exhaustive: check + race + fuzz + dupe + nilcheck + deadcode + omitempty + vuln
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

# Runs each Fuzz function it finds, FUZZTIME apiece.
FUZZTIME ?= 10s
fuzz: ## Fuzz every Fuzz test (FUZZTIME=10s each)
	@for pkg in $$(go list ./...); do \
		list=$$(go test -list '^Fuzz' $$pkg) || exit $$?; \
		for f in $$(printf '%s\n' "$$list" | sed -n '/^Fuzz/p'); do \
			echo "fuzz $$pkg $$f"; \
			go test -run '^$$' -fuzz "^$$f$$" -fuzztime=$(FUZZTIME) $$pkg || exit 1; \
		done; \
	done

# jscpd exits 0 at any duplication unless given a threshold; 3% is the fleet's value.
dupe: ## Fail above 3% code duplication (jscpd, pinned)
	npx --yes jscpd@$(JSCPD_VER) --threshold 3 --ignore '**/.git/**,**/.beads/**,**/.sandbox/**,**/.claude/**' .

nilcheck: ## Run nilaway on this module's packages (pinned)
	go run go.uber.org/nilaway/cmd/nilaway@$(NILAWAY_VER) -include-pkgs=$$(go list -m) ./...

# deadcode lists each function no path from main reaches, and exits 0 either way,
# so the target fails on any output. A library has no main, so -test makes the
# test binaries' mains the roots.
deadcode: ## Fail on any function unreachable from main or a test (deadcode, pinned)
	@out=$$(go run golang.org/x/tools/cmd/deadcode@$(DEADCODE_VER) -test ./...) || { echo "deadcode: could not run"; exit 1; }; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

# omitemptyscalar exits 3 on a ,omitempty tag on a bool (a false lost on write).
omitempty: ## Fail on a bool field tagged omitempty (omitemptyscalar, pinned)
	go run github.com/dkoosis/repo-standard/cmd/omitemptyscalar@$(OMITEMPTYSCALAR_VER) ./...

vuln: ## Scan for known vulnerabilities (govulncheck, pinned)
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VER) ./...

# Fleet gate: repo-standard grades the tree the rest of check passed.
standard: ## Run repo-standard (fleet SDLC checker) against this repo
	go run github.com/dkoosis/repo-standard/cmd/repo-standard@$(REPO_STANDARD_VER)

clean: ## Remove build outputs
	go clean ./...

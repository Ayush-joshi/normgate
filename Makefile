GO ?= go
GOFMT ?= gofmt
PYTHON ?= python3
NODE ?= node
NPM ?= npm
FUZZ_TIME ?= 30s
GOVULNCHECK ?= .tools/bin/govulncheck

.PHONY: generate check-generated fmt lint test-unit test-contract test-race test-fuzz-smoke coverage verify build setup license-check vuln test-policy test-mutation benchmark-policy

generate:
	GOFMT="$(GOFMT)" $(PYTHON) scripts/generate.py

check-generated:
	GOFMT="$(GOFMT)" $(PYTHON) scripts/generate.py --check

fmt:
	GOFMT="$(GOFMT)" $(PYTHON) scripts/format.py --write

lint:
	GOFMT="$(GOFMT)" $(PYTHON) scripts/format.py
	$(GO) vet ./...
	$(NPM) run typecheck
	$(PYTHON) -m compileall -q scripts sdk/python/src
	$(GO) test ./test/architecture

test-unit:
	$(GO) test $(if $(PKG),$(PKG),./internal/... ./cmd/...)

test-contract:
	$(GO) test ./test/contract
	$(PYTHON) scripts/check_vectors.py
	$(NODE) scripts/check_vectors.mjs

test-race:
	$(GO) test -race ./...

test-fuzz-smoke:
	$(GO) test ./internal/normalization -run='^$$' -fuzz=FuzzNGF004Canonicalize -fuzztime=$(FUZZ_TIME) -parallel=2
	$(GO) test ./internal/config -run='^$$' -fuzz=FuzzNGF005Load -fuzztime=$(FUZZ_TIME) -parallel=2
	$(GO) test ./internal/contracts -run='^$$' -fuzz=FuzzNGF002Validate -fuzztime=$(FUZZ_TIME) -parallel=2
	$(GO) test ./internal/policy -run='^$$' -fuzz=FuzzBundle -fuzztime=$(FUZZ_TIME) -parallel=2
	$(GO) test ./internal/policy -run='^$$' -fuzz=FuzzManifest -fuzztime=$(FUZZ_TIME) -parallel=2
	$(GO) test ./internal/policy -run='^$$' -fuzz=FuzzComposition -fuzztime=$(FUZZ_TIME) -parallel=2
	$(GO) test ./internal/policy -run='^$$' -fuzz=FuzzExplanation -fuzztime=$(FUZZ_TIME) -parallel=2

test-policy:
	$(GO) test ./internal/policy/...
	$(GO) run ./cmd/normgate policy test policies/baseline

test-mutation:
	GO="$(GO)" $(PYTHON) scripts/mutate_policy.py

benchmark-policy:
	$(GO) test -run='^$$' -bench=. -benchmem ./internal/policy/...

coverage:
	$(GO) test -coverpkg=normgate.dev/normgate/... -coverprofile=coverage.out ./...
	$(PYTHON) scripts/coverage.py coverage.out

license-check:
	GO="$(GO)" $(PYTHON) scripts/licenses.py

vuln:
	$(GOVULNCHECK) ./...
	$(NPM) audit --audit-level=low

build:
	$(GO) build -trimpath -o bin/normgate ./cmd/normgate

setup:
	$(GO) mod download all
	$(NPM) ci --ignore-scripts --no-audit --no-fund
	GOBIN="$(CURDIR)/.tools/bin" $(GO) install golang.org/x/vuln/cmd/govulncheck@v1.8.0

# Keep the gate sequential even when the caller invokes make -j.
verify:
	$(MAKE) check-generated
	$(MAKE) lint
	$(MAKE) test-unit
	$(MAKE) test-contract
	$(MAKE) test-policy
	$(MAKE) test-race
	$(MAKE) coverage
	$(MAKE) test-fuzz-smoke
	$(MAKE) test-mutation
	$(MAKE) license-check
	$(MAKE) vuln
	$(MAKE) build

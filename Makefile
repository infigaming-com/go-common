ALL_SRC_FILES := $(shell find . -type f -name '*.go' | sort)
OS ?= $(if $(filter Darwin,$(shell uname -s)),darwin,linux)
GO ?= go
GOFMT := gofmt
GOIMPORTS := goimports
GCI := gci

.PHONY: all-mod-download
all-mod-download:
	@$(GO) mod download

.PHONY: all-tidy
all-tidy:
	@$(GO) mod tidy

.PHONY: all-fmt
all-fmt:
	@$(GOFMT) -w -s ./
	@$(GOIMPORTS) -w $(ALL_SRC_FILES)
	@$(GCI) write -s standard -s default -s dot ./

.PHONY: all-upgrade
all-upgrade:
	@echo "Fetching latest Go 1.23.x version..."
	$(eval LATEST_GO_VERSION := $(shell curl -s https://go.dev/dl/?mode=json | grep -o '"version": "go1\.23\.[0-9]*"' | sort -V | tail -n1 | cut -d'"' -f4 | sed 's/go//'))
	@if [ -z "$(LATEST_GO_VERSION)" ]; then \
		echo "Error: No Go 1.23.x version found"; \
		exit 1; \
	fi
	@echo "Latest Go 1.23.x version: $(LATEST_GO_VERSION)"
	@if [ "$(OS)" = "darwin" ]; then \
		sed -i '' 's/^go .*/go $(LATEST_GO_VERSION)/' go.mod; \
	else \
		sed -i 's/^go .*/go $(LATEST_GO_VERSION)/' go.mod; \
	fi
	@echo "Updating dependencies..."
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@echo "Go version and dependencies updated successfully!"

.PHONY: build-all
build-all:
	@$(GO) build ./...
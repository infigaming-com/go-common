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
	@echo "Fetching latest Go version..."
	$(eval LATEST_GO_VERSION := $(shell curl -s https://go.dev/VERSION?m=text | head -n 1 | sed 's/go//'))
	@echo "Latest Go version: $(LATEST_GO_VERSION)"
	@if [ "$(OS)" = "darwin" ]; then \
		sed -i '' 's/^go .*/go $(LATEST_GO_VERSION)/' go.mod; \
	else \
		sed -i 's/^go .*/go $(LATEST_GO_VERSION)/' go.mod; \
	fi
	@echo "Updating dependencies..."
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@echo "Go version and dependencies updated successfully!"
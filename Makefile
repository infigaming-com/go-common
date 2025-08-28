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
	@$(GO) get -u ./...
	@$(GO) mod tidy

.PHONY: build-all
build-all:
	@$(GO) build ./...

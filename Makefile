BUILD_VERSION ?= dev
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo N/A)

SHORTENER_LDFLAGS := -X main.buildVersion=$(BUILD_VERSION) -X main.buildDate=$(BUILD_DATE) -X main.buildCommit=$(BUILD_COMMIT)

.PHONY: build-shortener
build-shortener:
	cd cmd/shortener && go build -buildvcs=false -ldflags "$(SHORTENER_LDFLAGS)" -o shortener

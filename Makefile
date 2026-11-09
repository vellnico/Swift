SHELL := /bin/sh
.DEFAULT_GOAL := build

GO        ?= go
NPM       ?= npm
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
EXE       := $(shell $(GO) env GOEXE)
# npm packages sometimes ship .go files; keep them out of go tooling.
GO_PKGS    = $(shell $(GO) list ./... | grep -v /node_modules/)
STATICCHECK_VERSION := 2026.2.2

export CGO_ENABLED := 0

.PHONY: dev build web-deps web go-build test lint xray golden clean

web/node_modules/.package-lock.json: web/package.json web/package-lock.json
	cd web && $(NPM) ci --no-fund --no-audit

web-deps: web/node_modules/.package-lock.json

web: web-deps
	cd web && $(NPM) run build
	@touch web/dist/.gitkeep

go-build:
	$(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/swift$(EXE) ./cmd/swift

build: web go-build

test: web-deps
	$(GO) test $(GO_PKGS)
	cd web && $(NPM) test

lint: web-deps
	$(GO) vet $(GO_PKGS)
	$(GO) run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) $(GO_PKGS)
	@out="$$(gofmt -l $$(git ls-files '*.go'; git ls-files --others --exclude-standard '*.go'))"; \
		if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	cd web && $(NPM) run check
	cd web && $(NPM) run lint

xray:
	$(GO) run ./tools/fetchxray -dir .cache/xray

golden:
	$(GO) test $(GO_PKGS) -update

# Go with live reload (air) on SWIFT_PANEL_PORT plus the Vite dev server.
dev: web-deps
	@mkdir -p .data
	SWIFT_DATA_DIR=.data SWIFT_PANEL_PORT=24680 SWIFT_PANEL_PATH=dev SWIFT_DEV=true \
		$(GO) run github.com/air-verse/air@v1.67.4 -c .air.toml & \
		cd web && SWIFT_PANEL_PORT=24680 SWIFT_PANEL_PATH=dev $(NPM) run dev; \
		kill %1 2>/dev/null || true

clean:
	rm -rf bin web/dist/assets web/dist/index.html

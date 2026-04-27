.PHONY: lint test test-integration test-all docker-build docker-run clean \
       build-installer build-nx build-nx-installer build-nx-all

# ============================================================================
# NX Brand Configuration
#
# All NX-specific naming is handled via ldflags — no source modifications.
# Upstream code (github.com/nikogura/dynamic-binary-toolkit) is used unmodified.
# ============================================================================

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

NX_PKG   = github.com/nikogura/dynamic-binary-toolkit/pkg/dbt
NX_INST  = github.com/nikogura/dynamic-binary-toolkit/cmd/dbt-installer/installer

NX_BRAND_LDFLAGS = \
  -X $(NX_PKG).BrandName=nx \
  -X $(NX_PKG).BrandDir=.nx \
  -X $(NX_PKG).BrandBinary=nx \
  -X $(NX_PKG).BrandConfigFile=nx.json \
  -X $(NX_PKG).BrandToolsPath=nx-tools \
  -X $(NX_PKG).BrandEnvPrefix=NX \
  -X $(NX_PKG).VERSION=$(VERSION)

NX_INSTALLER_BRAND_LDFLAGS = \
  -X $(NX_INST).BrandName=nx \
  -X $(NX_INST).BrandDir=.nx \
  -X $(NX_INST).BrandBinary=nx \
  -X $(NX_INST).BrandConfigFile=nx.json \
  -X $(NX_INST).BrandToolsPath=nx-tools \
  -X $(NX_INST).BrandOIDCClientID=nx \
  -X $(NX_INST).BrandOIDCSSHClientID=nx-ssh

PLATFORMS = darwin/amd64 darwin/arm64 linux/amd64

# ============================================================================
# NX Build Targets
# ============================================================================

build-nx:
	@echo "Building nx ($(VERSION))..."
	go build -ldflags "$(NX_BRAND_LDFLAGS)" -o bin/nx ./cmd/dbt

build-nx-installer:
	@if [ -z "$(SERVER_URL)" ]; then \
		echo "ERROR: SERVER_URL is required"; \
		echo "Example: SERVER_URL=https://nx-dbt.sre.nxvms.dev make build-nx-installer"; \
		exit 1; \
	fi
	@echo "Building nx-installer for all platforms..."
	@LDFLAGS="$(NX_INSTALLER_BRAND_LDFLAGS) -X main.serverURL=$(SERVER_URL)"; \
	[ -n "$(SERVER_NAME)" ] && LDFLAGS="$$LDFLAGS -X main.serverName=$(SERVER_NAME)"; \
	[ -n "$(TOOLS_URL)" ] && LDFLAGS="$$LDFLAGS -X main.toolsURL=$(TOOLS_URL)"; \
	[ -n "$(S3_REGION)" ] && LDFLAGS="$$LDFLAGS -X main.s3Region=$(S3_REGION)"; \
	[ -n "$(ISSUER_URL)" ] && LDFLAGS="$$LDFLAGS -X main.issuerURL=$(ISSUER_URL)"; \
	[ -n "$(OIDC_AUDIENCE)" ] && LDFLAGS="$$LDFLAGS -X main.oidcAudience=$(OIDC_AUDIENCE)"; \
	[ -n "$(OIDC_CLIENT_ID)" ] && LDFLAGS="$$LDFLAGS -X main.oidcClientID=$(OIDC_CLIENT_ID)"; \
	[ -n "$(OIDC_CLIENT_SECRET)" ] && LDFLAGS="$$LDFLAGS -X main.oidcClientSecret=$(OIDC_CLIENT_SECRET)"; \
	[ -n "$(CONNECTOR_ID)" ] && LDFLAGS="$$LDFLAGS -X main.connectorID=$(CONNECTOR_ID)"; \
	[ -n "$(INSTALLER_VERSION)" ] && LDFLAGS="$$LDFLAGS -X main.version=$(INSTALLER_VERSION)"; \
	for pair in $(PLATFORMS); do \
		OS=$$(echo $$pair | cut -d/ -f1); \
		ARCH=$$(echo $$pair | cut -d/ -f2); \
		echo "  $$OS/$$ARCH..."; \
		mkdir -p dist/nx-installer/$$OS/$$ARCH; \
		CGO_ENABLED=0 GOOS=$$OS GOARCH=$$ARCH \
			go build -ldflags "$$LDFLAGS" -o dist/nx-installer/$$OS/$$ARCH/nx-installer ./cmd/dbt-installer; \
	done
	@echo "Installers built in dist/nx-installer/"

build-nx-all:
	@echo "Building nx binaries for all platforms ($(VERSION))..."
	@for pair in $(PLATFORMS); do \
		OS=$$(echo $$pair | cut -d/ -f1); \
		ARCH=$$(echo $$pair | cut -d/ -f2); \
		echo "  $$OS/$$ARCH..."; \
		mkdir -p dist/nx/$$OS/$$ARCH dist/catalog/$$OS/$$ARCH; \
		CGO_ENABLED=0 GOOS=$$OS GOARCH=$$ARCH \
			go build -ldflags "$(NX_BRAND_LDFLAGS)" -o dist/nx/$$OS/$$ARCH/nx ./cmd/dbt; \
		CGO_ENABLED=0 GOOS=$$OS GOARCH=$$ARCH \
			go build -ldflags "$(NX_BRAND_LDFLAGS)" -o dist/catalog/$$OS/$$ARCH/catalog ./cmd/catalog; \
	done
	@echo "Binaries built in dist/"

# ============================================================================
# Upstream / Generic Targets
# ============================================================================

lint:
	@echo "Running namedreturns linter..."
	namedreturns ./...
	@echo "Running golangci-lint..."
	golangci-lint run

test:
	go test -v -race -cover -count=1 ./...

test-integration:
	@echo "Running integration tests..."
	go test -tags=integration -v -timeout=10m -count=1 ./test/integration/...

test-all: test lint test-integration

docker-build:
	@echo "Building reposerver Docker image..."
	docker build -f dockerfiles/reposerver/Dockerfile -t dbt-reposerver:latest .

docker-run:
	docker run -p 9999:9999 dbt-reposerver:latest

clean:
	rm -rf dist/ bin/
	docker rm -f dbt-integration-test 2>/dev/null || true

.PHONY: lint test test-integration docker-build docker-run clean build-installer

# Lint runs custom namedreturns linter followed by golangci-lint
lint:
	@echo "Running namedreturns linter..."
	namedreturns ./...
	@echo "Running golangci-lint..."
	golangci-lint run

# Test runs all unit tests
test:
	go test -v -race -cover -count=1 ./...

# Test-integration runs the integration tests (requires Docker)
test-integration:
	@echo "Running integration tests..."
	go test -tags=integration -v -timeout=10m -count=1 ./test/integration/...

# Test-all runs unit tests, lint, and integration tests
test-all: test lint test-integration

# Docker-build builds the reposerver Docker image
docker-build:
	@echo "Building reposerver Docker image..."
	docker build -f dockerfiles/reposerver/Dockerfile -t dbt-reposerver:latest .

# Docker-run runs the reposerver container (mount your repo at /var/dbt)
docker-run:
	@echo "Starting reposerver container..."
	@echo "Mount your repository: docker run -p 9999:9999 -v /path/to/repo:/var/dbt dbt-reposerver:latest"
	docker run -p 9999:9999 dbt-reposerver:latest

# Build-installer builds the dbt-installer binary for all platforms.
# This target requires configuration via environment variables or LDFLAGS override.
#
# Required variables (set via environment or INSTALLER_LDFLAGS):
#   SERVER_URL      - Base URL of dbt repository (e.g., https://dbt.example.com or s3://bucket)
#
# Optional variables:
#   SERVER_NAME     - Alias for server in config (defaults to derived from URL)
#   TOOLS_URL       - Tools repository URL (defaults to SERVER_URL/dbt-tools)
#   S3_REGION       - AWS region for S3 (auto-detected if not set)
#   ISSUER_URL      - OIDC issuer URL (required for OIDC auth)
#   OIDC_AUDIENCE   - OIDC audience (defaults to SERVER_URL)
#   OIDC_CLIENT_ID  - OAuth2 client ID (defaults to dbt-ssh or dbt)
#   OIDC_CLIENT_SECRET - OAuth2 client secret (optional)
#   CONNECTOR_ID    - OIDC connector ID (use "ssh" for SSH-OIDC)
#   INSTALLER_VERSION - Version string (defaults to "dev")
#
# Example:
#   SERVER_URL=https://dbt.example.com ISSUER_URL=https://dex.example.com \
#   CONNECTOR_ID=ssh make build-installer
#
build-installer:
	@if [ -z "$(SERVER_URL)" ]; then \
		echo "ERROR: SERVER_URL is required"; \
		echo "Example: SERVER_URL=https://dbt.example.com make build-installer"; \
		exit 1; \
	fi
	@echo "Building dbt-installer for all platforms..."
	@mkdir -p dist/installer/darwin/amd64 dist/installer/darwin/arm64 dist/installer/linux/amd64
	@LDFLAGS="-X main.serverURL=$(SERVER_URL)"; \
	[ -n "$(SERVER_NAME)" ] && LDFLAGS="$$LDFLAGS -X main.serverName=$(SERVER_NAME)"; \
	[ -n "$(TOOLS_URL)" ] && LDFLAGS="$$LDFLAGS -X main.toolsURL=$(TOOLS_URL)"; \
	[ -n "$(S3_REGION)" ] && LDFLAGS="$$LDFLAGS -X main.s3Region=$(S3_REGION)"; \
	[ -n "$(ISSUER_URL)" ] && LDFLAGS="$$LDFLAGS -X main.issuerURL=$(ISSUER_URL)"; \
	[ -n "$(OIDC_AUDIENCE)" ] && LDFLAGS="$$LDFLAGS -X main.oidcAudience=$(OIDC_AUDIENCE)"; \
	[ -n "$(OIDC_CLIENT_ID)" ] && LDFLAGS="$$LDFLAGS -X main.oidcClientID=$(OIDC_CLIENT_ID)"; \
	[ -n "$(OIDC_CLIENT_SECRET)" ] && LDFLAGS="$$LDFLAGS -X main.oidcClientSecret=$(OIDC_CLIENT_SECRET)"; \
	[ -n "$(CONNECTOR_ID)" ] && LDFLAGS="$$LDFLAGS -X main.connectorID=$(CONNECTOR_ID)"; \
	[ -n "$(INSTALLER_VERSION)" ] && LDFLAGS="$$LDFLAGS -X main.version=$(INSTALLER_VERSION)"; \
	echo "LDFLAGS: $$LDFLAGS"; \
	GOOS=darwin GOARCH=amd64 go build -ldflags "$$LDFLAGS" -o dist/installer/darwin/amd64/dbt-installer ./cmd/dbt-installer && \
	GOOS=darwin GOARCH=arm64 go build -ldflags "$$LDFLAGS" -o dist/installer/darwin/arm64/dbt-installer ./cmd/dbt-installer && \
	GOOS=linux GOARCH=amd64 go build -ldflags "$$LDFLAGS" -o dist/installer/linux/amd64/dbt-installer ./cmd/dbt-installer
	@echo "Installers built in dist/installer/"

# Clean removes build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf dist/
	docker rm -f dbt-integration-test 2>/dev/null || true

.PHONY: lint test test-integration docker-build docker-run clean bump-version

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

# Clean removes build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf dist/
	docker rm -f dbt-integration-test 2>/dev/null || true

# Bump-version bumps the version number and regenerates test fixtures
# Usage:
#   make bump-version              # Bump patch (3.7.0 -> 3.7.1)
#   make bump-version V=4.0.0      # Set specific version
#   make bump-version V=minor      # Bump minor (3.7.0 -> 3.8.0)
#   make bump-version V=major      # Bump major (3.7.0 -> 4.0.0)
bump-version:
	@./scripts/bump-version.sh $(V)

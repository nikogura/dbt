.PHONY: lint test test-integration docker-build docker-run clean

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

# Downstream DBT Publishing
#
# Publishes dbt artifacts from upstream GitHub releases to your infrastructure.
# Copy this file to your own repo and customize the variables below.

.PHONY: publish dry-run build-installer help

# Path to nikogura/dbt repo (for building installers)
# In CI, this is set to the checkout path of nikogura/dbt
DBT_REPO ?= ../../

# Reposerver configuration — customize these for your organization
SERVER_URL     := https://dbt.example.com
SERVER_NAME    := prod
ISSUER_URL     := https://dex.example.com
OIDC_AUDIENCE  := https://dbt.example.com
OIDC_CLIENT_ID := dbt-ssh
CONNECTOR_ID   := ssh

# Installer version (defaults to date-based)
INSTALLER_VERSION ?= $(shell date +%Y%m%d)

# Default: publish latest release from GitHub to reposerver
publish:
	./publish-downstream.sh -y

# Publish specific version
publish-%:
	./publish-downstream.sh -v $* -y

# Publish with dry-run (show what would be done)
dry-run:
	./publish-downstream.sh -d

help:
	@echo "Downstream DBT Publishing"
	@echo ""
	@echo "Targets:"
	@echo "  make publish              Publish latest release to reposerver"
	@echo "  make publish-3.7.5        Publish specific version to reposerver"
	@echo "  make dry-run              Show what would be published"
	@echo ""
	@echo "Installer targets:"
	@echo "  make build-installer      Build installer for your reposerver"
	@echo ""
	@echo "Environment variables:"
	@echo "  GPG_IDENTITY        GPG key for signing"
	@echo "  DBT_REPO            Path to nikogura/dbt repo (default: ../../)"

# Build installer for your reposerver
build-installer:
	@echo "Building installer..."
	cd $(DBT_REPO) && make build-installer \
		SERVER_URL=$(SERVER_URL) \
		SERVER_NAME=$(SERVER_NAME) \
		ISSUER_URL=$(ISSUER_URL) \
		OIDC_AUDIENCE=$(OIDC_AUDIENCE) \
		OIDC_CLIENT_ID=$(OIDC_CLIENT_ID) \
		CONNECTOR_ID=$(CONNECTOR_ID) \
		INSTALLER_VERSION=$(INSTALLER_VERSION)
	@mkdir -p dist
	cp $(DBT_REPO)/dist/installer/darwin/amd64/dbt-installer dist/dbt-installer-darwin-amd64
	cp $(DBT_REPO)/dist/installer/darwin/arm64/dbt-installer dist/dbt-installer-darwin-arm64
	cp $(DBT_REPO)/dist/installer/linux/amd64/dbt-installer dist/dbt-installer-linux-amd64
	@echo "Installers built in dist/"

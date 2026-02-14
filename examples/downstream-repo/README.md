# Downstream DBT Repository (Template)

This is a template for organizations that maintain their own [DBT](https://github.com/nikogura/dbt) deployment. Copy this directory into your own GitHub repository and customize it.

## What This Repo Does

1. **Downloads** release artifacts from upstream [nikogura/dbt](https://github.com/nikogura/dbt) GitHub releases
2. **Signs** artifacts with your organization's GPG key
3. **Uploads** signed artifacts to your reposerver via HTTP PUT with bearer token auth
4. **Builds** platform-specific installer binaries with your server configuration baked in
5. **Publishes** installer binaries as GitHub releases for your team to download

No local builds of dbt itself are required — binaries come directly from upstream releases.

## Setup

### 1. Copy This Directory

Copy the contents of this directory into a new GitHub repository in your organization:

```bash
mkdir my-org-dbt && cd my-org-dbt
cp -r /path/to/nikogura/dbt/examples/downstream-repo/* .
cp -r /path/to/nikogura/dbt/examples/downstream-repo/.* .  # .gitignore, .github/
git init && git add -A && git commit -m "Initial downstream dbt repo"
```

### 2. Copy the Publish Script

Copy the upstream publish script into your repo:

```bash
cp /path/to/nikogura/dbt/scripts/publish-downstream.sh ./publish-downstream.sh
chmod +x publish-downstream.sh
```

The script is generic — it supports S3, HTTP, and multiple auth methods out of the box.

### 3. Customize the Makefile

Edit `Makefile` and update these variables for your organization:

| Variable | Description | Example |
|----------|-------------|---------|
| `SERVER_URL` | Your reposerver URL | `https://dbt.example.com` |
| `SERVER_NAME` | Config alias for this server | `prod` |
| `ISSUER_URL` | OIDC issuer URL (if using OIDC auth) | `https://dex.example.com` |
| `OIDC_AUDIENCE` | OIDC token audience | `https://dbt.example.com` |
| `OIDC_CLIENT_ID` | OAuth2 client ID | `dbt-ssh` |
| `CONNECTOR_ID` | OIDC connector ID (for Dex) | `ssh` |

If you're not using OIDC, remove the OIDC-related variables and adjust `build-installer` accordingly.

### 4. Configure GitHub Secrets

The CI workflow requires these secrets:

| Secret | Description |
|--------|-------------|
| `GPG_SIGNING_KEY` | Base64-encoded GPG private key (`gpg --export-secret-keys <email> \| base64`) |
| `SIGNING_EMAIL` | Email address of the GPG key |
| `DBT_PUBLISH_TOKEN` | Static bearer token for reposerver PUT auth |

### 5. Push and Run

```bash
git remote add origin git@github.com:your-org/dbt.git
git push -u origin main
```

The CI workflow triggers on push to `main` and:
- Builds installer binaries for macOS (Intel + Apple Silicon) and Linux
- Creates a GitHub release with the installer binaries
- Publishes signed upstream dbt artifacts to your reposerver

## Quick Start (Local)

```bash
# Publish latest release to reposerver
make publish

# Publish specific version
make publish-3.7.5

# Dry run (see what would be published)
make dry-run

# Build installer binary
make build-installer
```

## Distributing Installers

After CI runs, your team can download installers from this repo's **GitHub Releases** page. Each release includes platform-specific binaries:

- `dbt-installer-darwin-amd64` — macOS Intel
- `dbt-installer-darwin-arm64` — macOS Apple Silicon
- `dbt-installer-linux-amd64` — Linux x86_64

Direct your team to the releases page:

```
https://github.com/your-org/dbt/releases/latest
```

## Requirements

- `curl` — HTTP requests
- `jq` — JSON parsing
- `gpg` — Artifact signing

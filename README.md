# Wiro CLI

Wiro CLI is a terminal client for discovering models, submitting runs, and watching task progress on Wiro AI.

## Features

- Interactive first-run setup (API key + API secret + project name)
- Model search and inspection
- Dynamic input prompts based on model schema
- Task execution with live progress events (WebSocket + polling fallback)
- Task detail, cancel, and kill commands
- Automatic output download with readable filenames

## Installation

### npm (recommended)

```bash
npm i -g @radioheavy/wiro-cli
```

### Script installer (macOS/Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/radioheavy/wiro_cl-/main/scripts/install.sh | sh
```

## Quick Start

Start the interactive flow:

```bash
wiro
```

On first run, the CLI asks for:
1. API key
2. API secret
3. Project name (optional)

Then it continues to model selection and input prompts.

## Common Commands

```bash
wiro
wiro run [owner/model] [--project <name|apikey>] [--set key=value] [--set-file key=/path] [--set-url key=https://...] [--advanced] [--watch=false] [--json]
wiro task detail <taskid|tasktoken>
wiro task cancel <taskid>
wiro task kill <taskid>
wiro model search [query]
wiro model inspect <owner/model>
wiro project ls
wiro project use <name|apikey>
wiro auth login
wiro auth verify <verifytoken> <code> [--authcode <2fa>]
wiro auth set --api-key <key> [--api-secret <secret>] [--name <project-name>]
wiro auth status
wiro auth logout
```

## Auth Modes

Wiro CLI supports three auth header modes, selected automatically:

- `signature`: `x-api-key`, `x-nonce`, `x-signature`
- `bearer`: `Authorization: Bearer <token>`
- `apikey-only`: `x-api-key`

If a project requires `signature` and its API secret is missing, the CLI will ask for it in interactive mode.

## Config, State, and Secrets

The base config directory is `<UserConfigDir>/wiro`, where `<UserConfigDir>` comes from the OS.

Typical paths:

- macOS: `~/Library/Application Support/wiro`
- Linux: `~/.config/wiro`

Files:

- config: `<base>/config.json`
- state: `<base>/state.json`
- fallback secrets store: `<base>/secrets.json` (mode `0600`)

Secret storage behavior:

- macOS: uses Keychain (`security` CLI) when available
- fallback: file-based `secrets.json`

## Outputs

- Default output root: `~/Downloads/wiro-outputs`
- Per-task folder: `~/Downloads/wiro-outputs/<taskid>`
- Filename format: `<prompt-first-two-words>-<index>.<ext>`

## npm Wrapper Behavior

The npm package downloads a platform-specific binary from GitHub Releases during `postinstall`.

Expected release assets:

- `wiro-darwin-x64`
- `wiro-darwin-arm64`
- `wiro-linux-x64`
- `wiro-linux-arm64`
- `wiro-win32-x64.exe`
- `wiro-win32-arm64.exe`

Default download URL format:

`https://github.com/radioheavy/wiro_cl-/releases/download/v<version>/<asset>`

Supported environment overrides:

- `WIRO_CLI_DOWNLOAD_BASE`
- `WIRO_CLI_SKIP_DOWNLOAD=1`
- `WIRO_CLI_BIN`
- `WIRO_CLI_REPO`

## Local Development

Build and run locally:

```bash
go build -o wiro ./cmd/wiro
./wiro
```

Run tests:

```bash
go test ./...
```

Build all release assets:

```bash
./scripts/build-release-assets.sh
```

## Release and Publish

### Standard CI flow

1. Push tag `vX.Y.Z`
2. `release.yml` builds and uploads binary assets to GitHub Release
3. `npm-publish.yml` publishes `@radioheavy/wiro-cli` from the release event

Required repository secret:

- `NPM_TOKEN`

### Manual fallback (without GitHub Actions)

If CI fails, run:

```bash
./scripts/prepare-manual-release.sh X.Y.Z
```

This script:

- runs `go test ./...`
- builds all release binaries into `dist/release-vX.Y.Z/`
- generates `SHA256SUMS`
- syncs `npm/package.json` version to `X.Y.Z`

Then follow the script output to:

- create/upload GitHub release assets with `gh release`
- publish manually with `cd npm && npm publish`

## Troubleshooting

### `npm install -g @radioheavy/wiro-cli@<version>` fails with 404

Cause: release assets for that version are missing in GitHub Release.

Fix:

1. Upload all required assets listed above.
2. Verify a direct asset URL returns `302` (GitHub redirect), not `404`.
3. Re-run npm install.

### `wiro` does not show first-time setup

Cause: existing local config and/or stored Keychain secrets.

Fix: remove local Wiro config/state files and related Keychain entries, then run `wiro` again.

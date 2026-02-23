# Wiro CLI

Wiro CLI brings the Wiro model workflow to terminal:
- project selection
- model search/inspect
- dynamic model input form
- run task
- live watch (WebSocket + polling fallback)
- task detail/cancel/kill

## Quick install (user flow)

```bash
npm i -g @toolbridge/wiro-cli
wiro
```

Alternative installer (macOS/Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/wiro-ai/wiro-cli/main/scripts/install.sh | sh
```

On first run, CLI asks:
1. API key
2. API secret
3. Project name (optional)

Then it continues with project/model/input selection.

## Common commands

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

## Local development

```bash
go build -o wiro ./cmd/wiro
./wiro
```

Build release assets locally:

```bash
./scripts/build-release-assets.sh
```

## Config paths

- Config: `~/.config/wiro/config.json`
- State: `~/.config/wiro/state.json`
- Default output directory: `~/Downloads/wiro-outputs/<taskid>`
- Output file naming: `<prompt-first-two-words>-<index>.<ext>`

## Secret storage

- macOS: Keychain (`security` CLI)
- fallback: `~/.config/wiro/secrets.json` (mode `0600`) if keychain is unavailable

## npm wrapper

Wrapper package is in `npm/` and expects release assets named:

- `wiro-darwin-x64`
- `wiro-darwin-arm64`
- `wiro-linux-x64`
- `wiro-linux-arm64`
- `wiro-win32-x64.exe`
- `wiro-win32-arm64.exe`

Postinstall default download source:

`https://github.com/wiro-ai/wiro-cli/releases/download/v<version>/<asset>`

Environment overrides:

- `WIRO_CLI_DOWNLOAD_BASE`
- `WIRO_CLI_SKIP_DOWNLOAD=1`
- `WIRO_CLI_BIN`
- `WIRO_CLI_REPO`

## Release and Publish

- Push tag `vX.Y.Z` to trigger GitHub Release workflow.
- Release workflow uploads these assets:
  - `wiro-darwin-x64`
  - `wiro-darwin-arm64`
  - `wiro-linux-x64`
  - `wiro-linux-arm64`
  - `wiro-win32-x64.exe`
  - `wiro-win32-arm64.exe`
- When release is published, npm publish workflow runs and publishes `@toolbridge/wiro-cli`.
- Required GitHub secret for npm workflow: `NPM_TOKEN`.

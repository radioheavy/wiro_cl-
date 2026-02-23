# Wiro CLI

Wiro CLI brings the Wiro model workflow to terminal:
- project selection
- model search/inspect
- dynamic model input form
- run task
- live watch (WebSocket + polling fallback)
- task detail/cancel/kill

## Build

```bash
go build -o wiro ./cmd/wiro
```

## Commands

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

## Config paths

- Config: `~/.config/wiro/config.json`
- State: `~/.config/wiro/state.json`

## Secret storage

- macOS: Keychain (`security` CLI)
- fallback: `~/.config/wiro/secrets.json` (mode `0600`) if keychain is unavailable

## npm wrapper

Wrapper package is located at `npm/` and expects release assets named:

- `wiro-darwin-x64`
- `wiro-darwin-arm64`
- `wiro-linux-x64`
- `wiro-linux-arm64`
- `wiro-win32-x64.exe`
- `wiro-win32-arm64.exe`

The postinstall downloader fetches from:

`https://github.com/wiro-ai/wiro-cli/releases/download/v<version>/<asset>`

You can override with:

- `WIRO_CLI_DOWNLOAD_BASE`
- `WIRO_CLI_SKIP_DOWNLOAD=1`
- `WIRO_CLI_BIN`

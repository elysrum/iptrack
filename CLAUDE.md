# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build ./...          # compile
go vet ./...            # static analysis
./iptrack --help        # usage
```

## Configuration

All flags accept an equivalent environment variable:

| Flag               | Env var              | Default                              |
|--------------------|----------------------|--------------------------------------|
| `--pushover-token` | `PUSHOVER_TOKEN`     | required                             |
| `--pushover-user`  | `PUSHOVER_USER`      | required                             |
| `--state-file`     | `IPTRACK_STATE_FILE` | `~/.local/share/iptrack/ip`          |
| `--title`          | `IPTRACK_TITLE`      | `"IP Address Changed"`               |
| `--daemon`         | `IPTRACK_DAEMON`     | `false`                              |
| `--interval`       | `IPTRACK_INTERVAL`   | `5m`                                 |

CLI flags take precedence over environment variables (cobra/viper precedence order). `--interval` accepts any Go duration string (`5m`, `1h`, `30s`) and is only used in daemon mode.

## Architecture

Single `main` package, four files:

- **`main.go`** — cobra/viper setup, `runOnce()` for one-shot mode, `runDaemon()` for continuous mode, shared `checkIP()` logic
- **`ip.go`** — `fetchIP()`: GET `https://ipconfig.io/ip`, returns trimmed IP string
- **`pushover.go`** — `notify(token, user, title, message)`: POST to Pushover API
- **`store.go`** — `readIP(path)` / `writeIP(path, ip)`: plain-text state file

## Docker

`Dockerfile` builds a minimal Alpine image with `ca-certificates` and `tzdata`.
`compose.yaml` runs in daemon mode with:
- `TZ: Europe/London` — logs in local time (tzdata must be present in the image)
- Log rotation: 3 × 10 MB JSON files (`max-size: 10m`, `max-file: 3`)

```bash
docker compose build    # rebuild after code or Dockerfile changes
docker compose up -d    # start
docker compose logs -f  # follow logs
```

## Behaviour

**One-shot mode** (default, no `--daemon`):
- First run (no state file): notifies with current IP, stores it, exits 0.
- IP unchanged: exits 0.
- IP changed: notifies, updates state file, exits 1.

**Daemon mode** (`--daemon`):
- Checks immediately on startup, then repeats on every `--interval` tick.
- IP changes are logged and notified; the loop always continues.
- Exits cleanly on `SIGTERM`/`SIGINT`.

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
| `--interval`       | `IPTRACK_INTERVAL`   | `5m`                                 |

CLI flags take precedence over environment variables (cobra/viper precedence order). `--interval` accepts any Go duration string (`5m`, `1h`, `30s`).

## Architecture

Single `main` package, four files:

- **`main.go`** — cobra command setup, viper flag/env binding, `run()` loop with ticker and signal handling, `checkIP()` orchestration
- **`ip.go`** — `fetchIP()`: GET `https://ipconfig.io/ip`, returns trimmed IP string
- **`pushover.go`** — `notify(token, user, title, message)`: POST to Pushover API
- **`store.go`** — `readIP(path)` / `writeIP(path, ip)`: plain-text state file

## Behaviour

Long-running process: checks IP immediately on startup, then on every `--interval` tick. Exits cleanly on `SIGTERM`/`SIGINT`.

- **First run** (no state file): sends Pushover alert `"IP address is: {ip}"`, stores IP.
- **IP unchanged**: logs current IP, no notification.
- **IP changed**: sends Pushover alert `"IP changed: {old} → {new}"`, updates state file.
- Errors (fetch failure, state read/write) are logged and skipped; the loop continues.

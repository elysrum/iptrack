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

CLI flags take precedence over environment variables (cobra/viper precedence order).

## Architecture

Single `main` package, four files:

- **`main.go`** — cobra command setup, viper flag/env binding, orchestration logic
- **`ip.go`** — `fetchIP()`: GET `https://ipconfig.io/ip`, returns trimmed IP string
- **`pushover.go`** — `notify(token, user, title, message)`: POST to Pushover API
- **`store.go`** — `readIP(path)` / `writeIP(path, ip)`: plain-text state file

## Behaviour

- **First run** (no state file): sends Pushover alert `"IP address is: {ip}"`, stores IP, exits 0.
- **IP unchanged**: exits 0 silently.
- **IP changed**: sends Pushover alert `"IP changed: {old} → {new}"`, updates state file, exits 1.
- Notification failures are logged to stderr as warnings; state is still updated and exit code reflects IP change status.

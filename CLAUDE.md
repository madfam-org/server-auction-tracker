# CLAUDE.md — server-auction-tracker

## Project Overview

Go CLI tool for Hetzner Server Auction intelligence. Binary name: `foundry-scout`.

## Build & Test

```bash
# Build (requires CGO for SQLite)
CGO_ENABLED=1 go build -o foundry-scout ./cmd/foundry-scout

# Test
go test -race ./...

# Lint
golangci-lint run

# Vet
go vet ./...
```

## Architecture

- `cmd/foundry-scout/` — Cobra CLI commands (main, scan, watch, history, simulate)
- `internal/config/` — Viper-based YAML config loading
- `internal/scanner/` — HTTP fetch + parse Hetzner auction JSON
- `internal/scorer/` — Cluster-aware scoring engine
- `internal/cpu/` — CPU model string parser
- `internal/store/` — SQLite repository (price history, scan results)
- `internal/notify/` — Notification interface (future milestone)
- `internal/order/` — Auto-order interface (future milestone)

## Conventions

- Use `logrus` for structured logging
- Use `testify` for test assertions
- Follow Go standard project layout (`cmd/`, `internal/`)
- Hard filters and scoring weights defined in config, not hardcoded
- Data source: `https://www.hetzner.com/_resources/app/jsondata/live_data_sb.json`

## Milestones

- M1 (implemented): Core Scanner — fetch, filter, score, store, print
- M3 (implemented): Price History — query by CPU model, show stats
- M2 (stub): Notifications
- M4 (stub): Cluster Simulation
- M5 (stub): Auto-Order

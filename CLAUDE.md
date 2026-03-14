# CLAUDE.md — server-auction-tracker

## Project Overview

Go CLI tool for Hetzner Server Auction intelligence. Binary name: `foundry-scout`.

## Build & Test

```bash
# Build (requires CGO for SQLite)
CGO_ENABLED=1 go build -o foundry-scout ./cmd/foundry-scout

# Test
CGO_ENABLED=1 go test -race ./...

# Lint
golangci-lint run

# Vet
go vet ./...
```

## Architecture

- `cmd/foundry-scout/` — Cobra CLI commands (main, scan, watch, history, simulate, order)
- `internal/config/` — Viper-based YAML config loading (filters, scoring, watch, notify, cluster, order)
- `internal/scanner/` — HTTP fetch + parse Hetzner auction JSON, retry client with backoff
- `internal/scorer/` — Cluster-aware scoring engine
- `internal/cpu/` — CPU model string parser (Ryzen, Intel, EPYC, Xeon)
- `internal/store/` — SQLite repository (price history, scan results, order audit)
- `internal/notify/` — Notification backends (enclii Switchyard, Slack, Discord) with dedup tracker
- `internal/order/` — Hetzner Robot API client with eligibility gates
- `internal/simulate/` — Cluster simulation engine (CPU/RAM/Disk utilization impact)
- `deploy/k8s/` — Kustomize manifests (CronJob, ConfigMap, PVC, NetworkPolicy)
- `deploy/argocd/` — ArgoCD Application

## Conventions

- Use `logrus` for structured logging
- Use `testify` for test assertions
- Follow Go standard project layout (`cmd/`, `internal/`)
- Hard filters and scoring weights defined in config, not hardcoded
- Data source: `https://www.hetzner.com/_resources/app/data/app/live_data_sb_EUR.json`
- Notifications route through enclii Switchyard API (default), with Slack/Discord fallback

## Milestones

- M1 (implemented): Core Scanner — fetch, filter, score, store, print
- M2 (implemented): Notifications — enclii/Slack/Discord backends, watch command with dedup
- M3 (implemented): Price History — query by CPU model, show stats, deal quality %
- M4 (implemented): Cluster Simulation — CPU/RAM/Disk impact analysis
- M5 (implemented): Auto-Order — Robot API with safety gates, audit logging

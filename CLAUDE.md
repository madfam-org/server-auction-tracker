# CLAUDE.md — server-auction-tracker

## Project Overview

Go monorepo for Hetzner Server Auction intelligence. Two binaries:
- `foundry-scout` — CLI tool (scan, watch, history, simulate, order)
- `deal-sniper` — Web dashboard served at `sniper.madfam.io` (port 4205)

## Build & Test

```bash
# Build (requires CGO for SQLite)
CGO_ENABLED=1 go build -o foundry-scout ./cmd/foundry-scout
CGO_ENABLED=1 go build -o deal-sniper ./cmd/deal-sniper

# Test
CGO_ENABLED=1 go test -race ./...

# Lint
golangci-lint run

# Vet
go vet ./...
```

## Architecture

- `cmd/foundry-scout/` — Cobra CLI commands (main, scan, watch, history, simulate, order)
- `cmd/deal-sniper/` — HTTP web server (stdlib net/http, embedded static files)
  - `cmd/deal-sniper/web/` — Frontend (vanilla HTML + Tailwind CDN + Chart.js + vanilla JS)
  - `cmd/deal-sniper/handlers.go` — API handler helpers
  - `cmd/deal-sniper/handlers_test.go` — Handler unit tests
- `internal/config/` — Viper-based YAML config loading (filters, scoring, watch, notify, cluster, order)
- `internal/scanner/` — HTTP fetch + parse Hetzner auction JSON, retry client with backoff
- `internal/scorer/` — Cluster-aware scoring engine
- `internal/cpu/` — CPU model string parser (Ryzen, Intel, EPYC, Xeon)
- `internal/store/` — SQLite repository (price history, scan results, order audit)
- `internal/notify/` — Notification backends (enclii Switchyard, Slack, Discord) with dedup tracker
- `internal/order/` — Hetzner Robot API client with eligibility gates
- `internal/simulate/` — Cluster simulation engine (CPU/RAM/Disk utilization impact)
- `deploy/k8s/` — Kustomize manifests (CronJob, Deployment, Service, ConfigMap, PVC, NetworkPolicy)
- `deploy/argocd/` — ArgoCD Application

## Web API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Liveness probe |
| GET | `/api/latest` | Latest 50 scored servers |
| GET | `/api/history?cpu=X&limit=N` | Price history for a CPU model |
| GET | `/api/stats` | Min/max/avg per CPU model |
| GET | `/api/stats/{cpu}` | Stats for specific CPU |
| GET | `/api/simulate/{server_id}` | Cluster impact analysis |
| GET | `/api/orders` | Order audit log |
| GET | `/api/config` | Current config (secrets redacted) |
| POST | `/api/order/check` | Eligibility pre-check (requires Bearer auth) |
| POST | `/api/order/confirm` | Place order via Robot API (requires Bearer auth) |
| GET | `/` | Dashboard (embedded static files) |

## Deployment

- **CronJob**: `foundry-scout watch --once` every 5 min → writes SQLite
- **Deployment**: `deal-sniper --config /config/scout.yaml` → reads SQLite (read-only mount)
- **PVC**: `foundry-scout-data` (1Gi Longhorn, ReadWriteOnce, shared via same node)
- **Service**: `deal-sniper-web` (ClusterIP, port 80 → 4205)
- **Domain**: `sniper.madfam.io` via Cloudflare Tunnel → `deal-sniper-web.foundry-scout.svc:80`
- **Port**: 4205 (Enclii 4200-4299 block)

## Conventions

- Use `logrus` for structured logging
- Use `testify` for test assertions
- Follow Go standard project layout (`cmd/`, `internal/`)
- Hard filters and scoring weights defined in config, not hardcoded
- Data source: `https://www.hetzner.com/_resources/app/data/app/live_data_sb_EUR.json`
- Notifications route through enclii Switchyard API (default), with Slack/Discord fallback
- Frontend: no build tools, no Node.js — vanilla HTML/CSS/JS embedded via Go `embed`

## Milestones

- M1 (implemented): Core Scanner — fetch, filter, score, store, print
- M2 (implemented): Notifications — enclii/Slack/Discord backends, watch command with dedup
- M3 (implemented): Price History — query by CPU model, show stats, deal quality %
- M4 (implemented): Cluster Simulation — CPU/RAM/Disk impact analysis
- M5 (implemented): Auto-Order — Robot API with safety gates, audit logging
- M6 (implemented): Web Dashboard — Deal Sniper UI at sniper.madfam.io
- M7 (implemented): Buy Now Flow — Two-step order (check + confirm) with Bearer auth, score breakdown display

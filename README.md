# Deal Sniper

Hetzner Server Auction intelligence — automated scoring, price history, notifications, cluster simulation, and a web dashboard for capacity expansion.

**CLI Binary**: `foundry-scout` | **Web Binary**: `deal-sniper` | **Dashboard**: [sniper.madfam.io](https://sniper.madfam.io)

## Features

- Fetch and parse live Hetzner Server Auction listings
- Filter servers by RAM, CPU cores, drives, price, datacenter, ECC, and NVMe
- Score servers using a cluster-aware weighted formula with PassMark CPU benchmarks
- **Deal quality badges** — per-row "18% below avg" / "Top deal" indicators with percentile ranking
- **Time-on-market tracking** — "Listed 2h ago" urgency signals per server
- Persist scan results to SQLite for price history and trend analysis (ECC, setup price, bandwidth, next price reduce)
- **Web dashboard** with live deals, price charts, cluster simulator, order audit log, market analytics, and Buy Now flow
- **Market analytics dashboard** — AMD vs Intel price trends, datacenter distribution, top value CPUs, price histogram
- **Shareable filter URLs** — bookmark and share filtered views via URL params
- Query historical pricing by CPU model with min/max/avg stats and deal quality
- Watch mode with dedup and notifications (enclii, Slack, Discord, **Webhook**, **Telegram**) — multi-channel dispatch
- **Curated digest notifications** — daily/weekly top-N deal summaries
- Simulate cluster impact of adding a server
- Auto-order via Hetzner Robot API with safety gates

## Web Dashboard

The Deal Sniper web UI is served at port 4205 and provides:

- **Live Deals** — Table of latest scored servers with auto-refresh (60s), deal quality badges, ECC indicators, time-on-market, and upcoming price reductions. Click any row to simulate cluster impact.
- **Price History** — Line charts showing price/score over time per CPU model.
- **Analytics** — AMD vs Intel price trends, datacenter distribution, top value CPUs, price histogram (Chart.js).
- **Order Log** — Audit trail of all order attempts.
- **Config** — Read-only view of current filters, scoring weights, and cluster profile.
- **Buy Now** — Two-step order flow: check eligibility, then confirm. Requires Bearer token auth (stored in session, cleared on tab close). Score breakdown visualization in the simulation modal.
- **Shareable URLs** — Filter and sort state encoded in URL params. Share button copies URL to clipboard.

```bash
# Run locally
./deal-sniper --config scout.yaml
# Open http://localhost:4205
```

## Install

```bash
# From source (requires Go 1.24+ and CGO)
CGO_ENABLED=1 go install github.com/madfam-org/server-auction-tracker/cmd/foundry-scout@latest

# Or clone and build both binaries
git clone https://github.com/madfam-org/server-auction-tracker.git
cd server-auction-tracker
CGO_ENABLED=1 go build -o foundry-scout ./cmd/foundry-scout
CGO_ENABLED=1 go build -o deal-sniper ./cmd/deal-sniper
```

## CLI Usage

```bash
# One-shot scan — fetch, filter, score, and display matching servers
foundry-scout scan --config scout.yaml

# View price history for a CPU model (includes deal quality %)
foundry-scout history --cpu "Ryzen 5 3600" --config scout.yaml

# Watch — poll every 5min with notifications
foundry-scout watch --config scout.yaml

# Watch — single iteration (for K8s CronJob)
foundry-scout watch --once --config scout.yaml

# Simulate cluster impact of adding a server
foundry-scout simulate --server-id 2873962 --config scout.yaml

# Order a server (requires Robot API credentials)
foundry-scout order --server-id 2873962 --config scout.yaml
```

## Configuration

Copy `scout.yaml.example` to `scout.yaml` and customize:

```yaml
filters:
  min_ram_gb: 128
  min_cpu_cores: 8
  min_drives: 2
  min_drive_size_gb: 512
  max_price_eur: 85
  datacenter_prefix: "HEL1"

scoring:
  cpu_weight: 0.25
  ram_weight: 0.20
  storage_weight: 0.15
  nvme_weight: 0.10
  cpu_gen_weight: 0.10
  locality_weight: 0.05
  benchmark_weight: 0.10   # PassMark CPU benchmark per dollar (0.0 to disable)
  ecc_weight: 0.05         # ECC memory bonus (0.0 to disable)

database:
  path: "foundry-scout.db"

watch:
  interval: "5m"
  dedup_window: "1h"

notify:
  type: "enclii"           # enclii | slack | discord | webhook | telegram
  min_score: 0             # only notify for servers scoring above this threshold
  # channels:              # multi-channel dispatch (overrides type when set)
  #   - type: "slack"
  #   - type: "telegram"
  enclii:
    api_url: "http://switchyard-api.enclii.svc.cluster.local"
    project_slug: "foundry-scout"
    webhook_secret: ""
  slack:
    webhook_url: ""
  discord:
    webhook_url: ""
  webhook:
    url: ""
    headers: {}
  telegram:
    bot_token: ""
    chat_id: ""

cluster:
  cpu_millicores: 12000
  cpu_requested: 10460
  ram_gb: 64
  ram_requested_gb: 25
  disk_gb: 98
  disk_used_gb: 80
  nodes: 2

order:
  enabled: false
  robot_url: "https://robot-ws.your-server.de"
  robot_user: ""
  robot_password: ""
  min_score: 90
  max_price_eur: 80
  require_approval: true

digest:
  enabled: false
  schedule: "daily"         # "daily" or "weekly"
  top_n: 5
  min_score: 70
```

## Scoring Algorithm

Each component is normalized to 0-1 relative to the best server in the current scan, then weighted:

```
raw_score = normalize(cpu_per_dollar)       * 0.25
          + normalize(ram_per_dollar)       * 0.20
          + normalize(storage_per_dollar)   * 0.15
          + nvme_ratio                      * 0.10
          + cpu_generation_score            * 0.10
          + datacenter_match                * 0.05
          + normalize(benchmark_per_dollar) * 0.10  (PassMark scores for ~90 CPU models)
          + ecc_bonus                       * 0.05  (1.0 if ECC memory)
```

Final score scaled to 0-100. Benchmark and ECC weights default to 0.0 for backward compatibility.

## Deployment

### Docker

```bash
docker build -t foundry-scout .
docker run --rm -v $(pwd)/scout.yaml:/config/scout.yaml -v scout-data:/data \
  foundry-scout scan --config /config/scout.yaml

# Run web dashboard
docker run --rm -p 4205:4205 -v $(pwd)/scout.yaml:/config/scout.yaml -v scout-data:/data \
  --entrypoint ./deal-sniper foundry-scout --config /config/scout.yaml
```

### Kubernetes

Manifests in `deploy/k8s/`:

```bash
# Validate manifests
kubectl apply --dry-run=client -f deploy/k8s/base/

# Deploy with Kustomize
kubectl apply -k deploy/k8s/production/
```

- CronJob runs `foundry-scout watch --once` every 5 minutes (writes to SQLite)
- Deployment runs `deal-sniper` web server (reads from same SQLite PVC)
- SQLite persisted via 1Gi PVC (Longhorn)
- Notifications route through enclii Switchyard API
- Web UI exposed at `sniper.madfam.io` via Cloudflare Tunnel
- ArgoCD Application in `deploy/argocd/application.yaml`

## Notifications

Notifications route through the **enclii Switchyard API** by default. For standalone use, direct Slack, Discord, Telegram, or generic Webhook are also supported.

Set `notify.type` in config to `enclii`, `slack`, `discord`, `webhook`, or `telegram`.

For multi-channel dispatch (e.g., Slack + Telegram simultaneously), use the `channels` array:

```yaml
notify:
  channels:
    - type: "slack"
    - type: "telegram"
  min_score: 75            # only notify for deals scoring 75+
  slack:
    webhook_url: "https://hooks.slack.com/..."
  telegram:
    bot_token: "123:ABC"
    chat_id: "-100123"
```

### Digest Notifications

Enable daily or weekly top-deal summaries:

```yaml
digest:
  enabled: true
  schedule: "daily"   # or "weekly"
  top_n: 5
  min_score: 70
```

## License

MIT

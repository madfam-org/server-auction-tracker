# server-auction-tracker

Hetzner Server Auction intelligence — automated scoring, price history, notifications, and cluster simulation for capacity expansion.

**Binary**: `foundry-scout`

## Features

- Fetch and parse live Hetzner Server Auction listings
- Filter servers by RAM, CPU cores, drives, price, and datacenter
- Score servers using a cluster-aware weighted formula
- Persist scan results to SQLite for price history and trend analysis
- Query historical pricing by CPU model with min/max/avg stats and deal quality
- Watch mode with dedup and notifications (enclii, Slack, Discord)
- Simulate cluster impact of adding a server
- Auto-order via Hetzner Robot API with safety gates

## Install

```bash
# From source (requires Go 1.24+ and CGO)
CGO_ENABLED=1 go install github.com/madfam-org/server-auction-tracker/cmd/foundry-scout@latest

# Or clone and build
git clone https://github.com/madfam-org/server-auction-tracker.git
cd server-auction-tracker
CGO_ENABLED=1 go build -o foundry-scout ./cmd/foundry-scout
```

## Usage

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
  min_ram_gb: 64
  min_cpu_cores: 8
  min_drives: 2
  min_drive_size_gb: 512
  max_price_eur: 90
  datacenter_prefix: "HEL1"

scoring:
  cpu_weight: 0.30
  ram_weight: 0.25
  storage_weight: 0.20
  nvme_weight: 0.10
  cpu_gen_weight: 0.10
  locality_weight: 0.05

database:
  path: "foundry-scout.db"

watch:
  interval: "5m"
  dedup_window: "1h"

notify:
  type: "enclii"           # enclii | slack | discord
  enclii:
    api_url: "http://switchyard-api.enclii.svc.cluster.local"
    project_slug: "foundry-scout"
    webhook_secret: ""
  slack:
    webhook_url: ""
  discord:
    webhook_url: ""

cluster:
  cpu_millicores: 12000
  cpu_requested: 10460
  ram_gb: 64
  ram_requested_gb: 25
  disk_gb: 98
  disk_used_gb: 77
  nodes: 2

order:
  enabled: false
  robot_url: "https://robot-ws.your-server.de"
  robot_user: ""
  robot_password: ""
  min_score: 90
  max_price_eur: 80
  require_approval: true
```

## Scoring Algorithm

Each component is normalized to 0-1 relative to the best server in the current scan, then weighted:

```
raw_score = normalize(cpu_per_dollar)     * 0.30
          + normalize(ram_per_dollar)     * 0.25
          + normalize(storage_per_dollar) * 0.20
          + nvme_ratio                    * 0.10
          + cpu_generation_score          * 0.10
          + datacenter_match              * 0.05
```

Final score scaled to 0-100.

## Deployment

### Docker

```bash
docker build -t foundry-scout .
docker run --rm -v $(pwd)/scout.yaml:/config/scout.yaml -v scout-data:/data \
  foundry-scout scan --config /config/scout.yaml
```

### Kubernetes (CronJob)

Manifests in `deploy/k8s/`:

```bash
# Validate manifests
kubectl apply --dry-run=client -f deploy/k8s/base/

# Deploy with Kustomize
kubectl apply -k deploy/k8s/production/
```

- CronJob runs `watch --once` every 5 minutes
- SQLite persisted via 1Gi PVC (Longhorn)
- Notifications route through enclii Switchyard API
- ArgoCD Application in `deploy/argocd/application.yaml`

## Notifications

Notifications route through the **enclii Switchyard API** by default, which fans out to Slack/Discord/Telegram. For standalone CLI use, direct Slack or Discord webhooks are also supported.

Set `notify.type` in config to `enclii`, `slack`, or `discord`.

## License

MIT

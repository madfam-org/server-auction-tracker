# server-auction-tracker

Hetzner Server Auction intelligence — automated scoring, price history, and notifications for capacity expansion.

**Binary**: `foundry-scout`

## Features

- Fetch and parse live Hetzner Server Auction listings
- Filter servers by RAM, CPU cores, drives, price, and datacenter
- Score servers using a cluster-aware weighted formula
- Persist scan results to SQLite for price history and trend analysis
- Query historical pricing by CPU model with min/max/avg stats

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

# View price history for a CPU model
foundry-scout history --cpu "Ryzen 5 3600" --config scout.yaml

# Planned for future milestones:
foundry-scout watch      # Poll every N minutes with notifications
foundry-scout simulate   # Simulate cluster impact of adding a server
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

## Docker

```bash
docker build -t foundry-scout .
docker run --rm -v $(pwd)/scout.yaml:/app/scout.yaml foundry-scout scan --config /app/scout.yaml
```

## License

MIT

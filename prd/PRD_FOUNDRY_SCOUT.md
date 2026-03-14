# PRD: Foundry Scout — Hetzner Auction Server Intelligence

> **Status**: Implemented | **Author**: Aldo Ruiz Luna | **Date**: 2026-03-14
> **Repo**: `madfam-org/server-auction-tracker`
> **Binary**: `foundry-scout`

## Problem Statement

MADFAM operates a 2-node k3s production cluster on Hetzner dedicated servers. Capacity expansion requires finding the right server at the right price from Hetzner's Server Auction — a real-time marketplace where servers appear and disappear within hours.

**Current process**: Manual browsing, error-prone, slow, unscored, untracked.

**Desired outcome**: Automated notifications when matching servers appear, with objective value scores and price history.

## Architecture

- **Language**: Go (single binary, K8s CronJob friendly)
- **Storage**: SQLite (price history, order audit)
- **Notifications**: Enclii Switchyard API (primary), Slack/Discord webhooks (fallback)
- **Order API**: Hetzner Robot API (optional, gated)
- **Deployment**: K8s CronJob with ArgoCD, Kustomize manifests

## Core Features

### F1: Cluster-Aware Scoring Engine

Score auction listings against cluster profile (CPU headroom, storage pressure, memory utilization). Pressure-aware boost for servers that solve bottlenecks.

### F2: Smart CPU Identification

Parse model strings to extract generation, cores, threads, clock speed. Supports Ryzen, Intel Core, EPYC, and Xeon families.

### F3: Price History & Trend Detection

SQLite-backed price stats. Deal quality = % below/above average for similar configs.

### F4: Notifications

Enclii Switchyard API (primary) with HMAC-signed lifecycle events. Direct Slack/Discord webhooks for standalone CLI use. Dedup tracker prevents duplicate notifications within configurable window.

### F5: Auto-Order (Optional, Gated)

Hetzner Robot API with confirmation flow. Safety gates: order.enabled, min score 90, max price cap, re-fetch confirmation, require approval, audit logging.

### F6: Cluster Simulation

Calculate CPU/RAM/Disk utilization impact of adding a server. Bottleneck relief analysis with health labels.

## CLI Interface

```bash
foundry-scout scan --config scout.yaml           # One-shot scan
foundry-scout watch --config scout.yaml          # Poll every 5min
foundry-scout watch --once --config scout.yaml   # Single iteration (CronJob)
foundry-scout history --cpu "Ryzen 9 3900"       # Price history + deal %
foundry-scout simulate --server-id 12345         # Cluster impact
foundry-scout order --server-id 12345            # Place order (gated)
```

## Deployment Options

- Local CLI
- K8s CronJob (`*/5 * * * *`) with PVC-backed SQLite
- GitHub Actions
- Docker container

## Milestones

- **M1: Core Scanner (MVP) — implemented** (scan, filter, score, store, print)
- **M2: Notifications — implemented** (enclii/Slack/Discord, watch command, dedup)
- **M3: Price History — implemented** (SQLite queries, stats by CPU model, deal quality %)
- **M4: Cluster Simulation — implemented** (CPU/RAM/Disk utilization impact)
- **M5: Auto-Order — implemented** (Robot API, eligibility gates, audit logging)

## Data Source

`https://www.hetzner.com/_resources/app/data/app/live_data_sb_EUR.json` (undocumented, polled every 5min by community tools)

## Non-Goals (v1)

- Web UI
- Multi-provider support
- Price prediction / ML
- Mobile app

# PRD: Foundry Scout — Hetzner Auction Server Intelligence

> **Status**: Draft — Scaffold In Progress | **Author**: Aldo Ruiz Luna | **Date**: 2026-03-13
> **Repo**: `madfam-org/server-auction-tracker`
> **Binary**: `foundry-scout`

## Problem Statement

MADFAM operates a 2-node k3s production cluster on Hetzner dedicated servers. Capacity expansion requires finding the right server at the right price from Hetzner's Server Auction — a real-time marketplace where servers appear and disappear within hours.

**Current process**: Manual browsing, error-prone, slow, unscored, untracked.

**Desired outcome**: Automated notifications when matching servers appear, with objective value scores and price history.

## Architecture

- **Language**: Go (single binary, K8s CronJob friendly)
- **Storage**: SQLite (price history)
- **Notifications**: Webhook (Slack, Discord, HTTP)
- **Order API**: Hetzner Robot API (optional, gated)

## Core Features

### F1: Cluster-Aware Scoring Engine

Score auction listings against cluster profile (CPU headroom, storage pressure, memory utilization). Pressure-aware boost for servers that solve bottlenecks.

### F2: Smart CPU Identification

Parse model strings to extract generation, cores, threads, clock speed.

### F3: Price History & Trend Detection

SQLite-backed price stats. Deal quality = % below/above average for similar configs.

### F4: Notifications

Slack/Discord webhooks with score breakdown and cluster impact analysis.

### F5: Auto-Order (Optional, Gated)

Hetzner Robot API with confirmation flow. Safety: min score 90, max price cap, require approval.

## CLI Interface

```bash
foundry-scout scan --config scout.yaml      # One-shot scan
foundry-scout watch --config scout.yaml     # Poll every 5min
foundry-scout history --cpu "Ryzen 9 3900"  # Price history
foundry-scout simulate --server-id 12345    # Cluster impact
```

## Deployment Options

- Local CLI
- K8s CronJob (`*/5 * * * *`)
- GitHub Actions

## Milestones

- **M1: Core Scanner (MVP) — implemented** (scan, filter, score, store, print)
- M2: Notifications — planned
- **M3: Price History — implemented** (SQLite queries, stats by CPU model)
- M4: Cluster Simulation — planned
- M5: Auto-Order — planned

## Data Source

`https://www.hetzner.com/_resources/app/data/app/live_data_sb_EUR.json` (undocumented, polled every 5min by community tools)

## Non-Goals (v1)

- Web UI
- Multi-provider support
- Price prediction / ML
- Mobile app

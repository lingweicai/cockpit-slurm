# Introduction (2026)

A good direction for your [cockpit-slurm project](https://github.com/lingweicai/cockpit-slurm) is to evolve from a simple Cockpit frontend plugin into a layered HPC management platform with:

* PatternFly React frontend
* Cockpit transport/API layer
* Go backend bridge daemon
* Unix socket IPC
* Slurm CLI abstraction + cache/event system
* Optional future REST/gRPC API
* HA-ready architecture

This matches Cockpit’s philosophy of “using existing Linux APIs and tools instead of reinventing subsystems.” ([Reddit][1])

The recommended starting point remains the official [cockpit-project/starter-kit](https://github.com/cockpit-project/starter-kit) template, which already includes:

* React + PatternFly setup
* Cockpit integration
* Build system
* RPM packaging
* CI/testing scaffolding
* Development watch mode ([GitHub][2])

# Updated Development Plan for cockpit-slurm (2026)

# 0. Project Vision

## Goal

Build a modern web-based HPC cluster management platform for Slurm using:

* Cockpit
* PatternFly React
* Go backend services
* Slurm CLI + SlurmDB
* Event-driven updates
* Streaming APIs
* HA-ready architecture

# 1. Recommended High-Level Architecture

## System Architecture Diagram

```text
┌─────────────────────────────────────────────────────────────┐
│                     Browser / Web UI                       │
│                                                             │
│          PatternFly ReactJS + TypeScript                    │
│                                                             │
│  Dashboards / Tables / Forms / Charts / Job Views          │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ HTTPS/WSS
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Cockpit Web Service                   │
│                                                             │
│                     cockpit-ws / cockpit-bridge             │
└─────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┴───────────────────┐
          │                                       │
          ▼                                       ▼

┌──────────────────────┐              ┌────────────────────────┐
│ Cockpit React Plugin │              │ cockpit.channel() API  │
│  (Frontend Package)  │              │ Stream/Event Transport │
└──────────────────────┘              └────────────────────────┘
                                                  │
                                                  │ JSON Messages
                                                  ▼

┌─────────────────────────────────────────────────────────────┐
│               cockpit-slurm-channel helper                 │
│                                                             │
│ Small proxy executable launched by Cockpit                 │
│                                                             │
│ Responsibilities:                                          │
│ - authenticate user                                        │
│ - open unix socket                                         │
│ - proxy websocket/channel traffic                          │
│ - enforce permissions                                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Unix Socket
                              ▼

┌─────────────────────────────────────────────────────────────┐
│               cockpit-slurm-bridge daemon                  │
│                                                             │
│ Persistent Go daemon                                       │
│                                                             │
│ Responsibilities:                                          │
│ - cache Slurm state                                        │
│ - poll/watch Slurm                                         │
│ - aggregate entities                                       │
│ - publish events                                           │
│ - RBAC                                                     │
│ - job lifecycle logic                                      │
│ - future REST/gRPC APIs                                    │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼

┌───────────────┐   ┌────────────────┐   ┌──────────────────┐
│ Slurm CLI     │   │ Slurm REST API │   │ slurmdbd/MySQL   │
│ sinfo         │   │ (future)       │   │ accounting DB    │
│ squeue        │   │                │   │                  │
│ sacct         │   │                │   │                  │
│ sacctmgr      │   │                │   │                  │
│ scontrol      │   │                │   │                  │
└───────────────┘   └────────────────┘   └──────────────────┘
```

# 2. Repository Structure

Start from:

[cockpit-project/starter-kit](https://github.com/cockpit-project/starter-kit)

Then evolve into:

```text
cockpit-slurm/
├── src/                         # React frontend
│   ├── app/
│   ├── components/
│   ├── pages/
│   ├── hooks/
│   ├── context/
│   ├── models/
│   ├── services/
│   ├── i18n/
│   └── types/
│
├── bridge/                      # Go daemon
│   ├── cmd/
│   │   └── cockpit-slurm-bridge
│   ├── internal/
│   │   ├── cache/
│   │   ├── slurm/
│   │   ├── events/
│   │   ├── models/
│   │   ├── socket/
│   │   ├── auth/
│   │   └── scheduler/
│   └── pkg/
│
├── channel/                     # Cockpit channel helper
│   ├── cmd/
│   │   └── cockpit-slurm-channel
│   └── internal/
│
├── packaging/
│
├── test/
│
├── docs/
│   ├── architecture/
│   ├── api/
│   └── models/
│
└── scripts/
```

# 3. Development Phases

---

# Phase 0 — Bootstrap Foundation

## Goals

* Fork starter-kit
* Rename package
* Setup CI/CD
* Setup coding standards
* Setup TypeScript models
* Setup Go workspace

## Tasks

### GitHub Repository

```bash
git clone https://github.com/cockpit-project/starter-kit.git
mv starter-kit cockpit-slurm
```

### Rename identifiers

Use:

```bash
find -iname '*starter*'
git grep -i starter
```

(as recommended by Cockpit docs) ([GitHub][2])

# Phase 1 — Frontend MVP

## Goals

Build initial operational dashboard.

## Features

* Cluster overview
* Nodes table
* Partitions table
* Jobs table
* User/account pages
* PatternFly charts
* i18n
* dark mode support

## Recommended Frontend Structure

```text
src/
 ├── pages/
 ├── cards/
 ├── modals/
 ├── tables/
 ├── charts/
 ├── hooks/
 ├── providers/
 └── models/
```

This aligns with the refactoring direction you already started using:

* providers/context
* shared types
* reusable cards
* modal-driven UX

# Phase 2 — Data Model Standardization

This is extremely important.

You already identified the correct direction:

* define models from Slurm JSON outputs
* not directly from DB schema

That is the right choice.

## Recommended Sources

Priority:

1. `--json` outputs
2. Slurm CLI semantics/manual
3. Slurm REST/OpenAPI
4. DB schema only for accounting internals

# Core Entities

## Cluster

## Node

## Partition

## Job

## Reservation

## User

## Account

## QOS

## Association

## Scheduler

## License

## Federation

# Phase 3 — Go Bridge Daemon

## Most important architectural transition

Move away from:

```text
React -> cockpit.spawn()
```

toward:

```text
React
  -> cockpit.channel()
      -> channel helper
          -> persistent daemon
```

This is the correct scalable architecture.

# Why Persistent Daemon?

## Benefits

### 1. Centralized cache

Avoid repeated:

```bash
sinfo
squeue
sacctmgr
```

### 2. Event streaming

Push updates instead of polling.

### 3. Better performance

Large clusters become manageable.

### 4. Shared state

Multiple browser sessions reuse same backend state.

### 5. Future HA support

Bridge can later become:

* clustered
* distributed
* API server

# Phase 4 — Event System

## Event Types

```text
NODE_UPDATED
JOB_UPDATED
PARTITION_UPDATED
RESERVATION_UPDATED
USER_UPDATED
```

# Recommended Event Flow

```text
slurm polling/watch
        ↓
state diff engine
        ↓
event bus
        ↓
websocket/channel stream
        ↓
React context updates
```

# Phase 5 — RBAC and Security

## Layers

### Cockpit authentication

### Slurm account permissions

### Unix socket permissions

### sudo policy

### SELinux policy

### audit logging

# Recommended Socket

```text
/run/cockpit-slurm/bridge.sock
```

# Recommended systemd Units

## Daemon

```text
cockpit-slurm-bridge.service
```

## Optional socket activation

```text
cockpit-slurm-bridge.socket
```

You previously asked about `.service` vs `.socket`; this is exactly where socket activation becomes valuable.

# Phase 6 — Advanced Features

## Suggested roadmap

### Live job updates

### GPU monitoring

### Power management

### Reservations UI

### Federations

### Multi-cluster support

### Topology visualization

### Node health analytics

### Accounting dashboards

### Job efficiency reports

### Notification system

# Phase 7 — HA / Enterprise Architecture

Future evolution:

```text
cockpit-slurm-agent
cockpit-slurm-controller
cockpit-slurm-api
cockpit-slurm-web
```

Potential future:

* gRPC
* REST API
* Prometheus metrics
* OpenTelemetry
* Kubernetes deployment


# Recommended Technology Stack

| Layer           | Technology              |
| --------------- | ----------------------- |
| Frontend        | React + TypeScript      |
| UI              | PatternFly v6           |
| Transport       | cockpit.channel()       |
| IPC             | Unix socket             |
| Backend         | Go                      |
| Service Manager | systemd                 |
| State Cache     | in-memory Go cache      |
| Packaging       | RPM                     |
| Testing         | Cockpit test framework  |
| CI              | GitHub Actions + Packit |
| Metrics         | Prometheus (future)     |

---

# Recommended Frontend Communication Pattern

## Avoid

```typescript
cockpit.spawn(["sinfo"])
```

everywhere in components.

## Prefer

```typescript
useSinfo()
useJobs()
usePartitions()
```

backed by:

```text
React Hook
    ↓
Context Provider
    ↓
cockpit.channel()
    ↓
bridge daemon
```

You already started moving toward this architecture with:

* `SinfoProvider`
* refresh propagation
* shared hooks
* centralized models

That is the correct direction.

# Recommended Go Backend Modules

```text
internal/
 ├── slurm/
 ├── cache/
 ├── events/
 ├── pubsub/
 ├── auth/
 ├── models/
 ├── scheduler/
 ├── websocket/
 ├── channels/
 └── metrics/
```

# Recommended Initial Milestones

## Milestone 1

Starter-kit fork + working dashboard

## Milestone 2

Unified TypeScript models

## Milestone 3

Go bridge daemon

## Milestone 4

Unix socket + channel helper

## Milestone 5

Live event streaming

## Milestone 6

Multi-cluster support

## Milestone 7

Enterprise packaging + HA

# Recommended First 90 Days

## Month 1

* fork starter-kit
* repo structure
* dashboard MVP
* models
* CI/CD

## Month 2

* Go daemon
* socket IPC
* channel proxy
* cache layer

## Month 3

* event streaming
* React hooks/providers
* live updates
* RBAC/security

# Strategic Recommendation

Your current architectural direction is already stronger than many Cockpit plugins because you are moving toward:

```text
stateful backend service
+
event-driven frontend
+
typed models
+
streaming APIs
```

instead of:

```text
many cockpit.spawn() calls
```

That is the correct long-term design for large HPC environments.

Also, your idea of:

* defining canonical Slurm entities
* using Go daemon cache
* streaming via cockpit.channel()
* separating frontend/backend responsibilities

is the right foundation for eventually turning cockpit-slurm into:

* an enterprise HPC management platform
* or even a commercial product around OpenHPC + Slurm.

[1]: https://www.reddit.com/r/linux/comments/p2v0te "Cockpit 250 — Cockpit Project"
[2]: https://github.com/cockpit-project/starter-kit "GitHub - cockpit-project/starter-kit: Everything you need to develop, test and deploy your own cockpit plugin"

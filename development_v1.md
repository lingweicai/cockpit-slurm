# Define JSON data model for the project

The JSON data model should become the canonical contract layer shared by both:

* frontend (React + TypeScript)
* backend (Go daemon/bridge)

That is the most scalable architecture for your cockpit-slurm project.

The important design principle is:

```text
Slurm CLI / config / accounting
            ↓
     Canonical JSON Models
            ↓
  Go backend + React frontend
```

instead of:

```text
React directly parsing CLI text output
```

or:

```text
Frontend tightly coupled to database schema
```

Your idea of using:

* Slurm official manuals
* `--json` outputs
* configuration semantics

as the foundation of the data model is the correct long-term strategy.

The official Slurm manuals are effectively the authoritative API specification for:

* entity semantics
* field meaning
* lifecycle
* relationships
* constraints
* operational behavior

For example:

* `squeue` defines runtime job state
* `sacct` defines historical/accounting job state
* `sacctmgr` defines persistent administrative entities
* `scontrol` defines cluster operational state
* `slurm.conf` defines scheduler configuration semantics

Those are different domains and should become different model layers. 

Three important software tools for the development: 
* PatternFly: https://www.patternfly.org/ 
* cockpit-project: https://cockpit-project.org/guide/latest/development.html
* Slurm: https://slurm.schedmd.com/documentation.html 

# Updated Architecture Principle

## Canonical Model Layer

You should introduce a dedicated phase very early:

```text
Slurm Sources
    ↓
Canonical JSON Models
    ↓
Go Internal Models
    ↓
Cockpit Channel Messages
    ↓
TypeScript Frontend Models
```

The canonical JSON models become:

* transport schema
* cache schema
* event schema
* API schema
* persistence schema
* frontend contract

This is extremely important.

# Updated Recommended Architecture

```text id="4x0d4v"
┌───────────────────────────────────────────────────────┐
│                 PatternFly React UI                  │
│                                                       │
│  TypeScript interfaces generated from canonical JSON │
└───────────────────────────────────────────────────────┘
                           ▲
                           │
                    cockpit.channel()
                           │
                           ▼

┌───────────────────────────────────────────────────────┐
│              cockpit-slurm-bridge daemon             │
│                                                       │
│  Canonical Go structs                                │
│  Cache/Event engine                                  │
│  Slurm adapters                                       │
└───────────────────────────────────────────────────────┘
                           ▲
                           │
                   Slurm Adapters
                           │
       ┌───────────────────┼───────────────────┐
       │                   │                   │
       ▼                   ▼                   ▼

   CLI JSON          Config Parsers        slurmdbd
   Adapters           slurm.conf           Accounting
                                            Sources
```

# UPDATED DEVELOPMENT PLAN

# Phase 0 — Repository Bootstrap

## Goals

Initialize repository and engineering standards.

## Tasks

### Fork starter-kit

Use:
[cockpit-project/starter-kit](https://github.com/cockpit-project/starter-kit)

### Initial Repository Structure

```text
cockpit-slurm/
├── src/
├── bridge/
├── channel/
├── models/
├── schemas/
├── docs/
└── packaging/
```

# NEW Phase 1 — Canonical Slurm Data Modeling

This becomes one of the most important phases.

Do this BEFORE large-scale frontend/backend coding.

# Goals

Define:

* canonical entities
* canonical JSON schemas
* field naming standards
* relationships
* lifecycle semantics
* event semantics

# Design Principle

## Sources of Truth Priority

### 1. Slurm official manuals

Use manuals as semantic authority:

* [sinfo manual](https://slurm.schedmd.com/sinfo.html)
* [squeue manual](https://slurm.schedmd.com/squeue.html)
* [scontrol manual](https://slurm.schedmd.com/scontrol.html)
* [sacctmgr manual](https://slurm.schedmd.com/sacctmgr.html)
* [sacct manual](https://slurm.schedmd.com/sacct.html)
* [sreport manual](https://slurm.schedmd.com/sreport.html)
* [sbatch manual](https://slurm.schedmd.com/sbatch.html)
* [salloc manual](https://slurm.schedmd.com/salloc.html)
* [srun manual](https://slurm.schedmd.com/srun.html)
* [slurm.conf manual](https://slurm.schedmd.com/slurm.conf.html)
* [slurmdbd.conf manual](https://slurm.schedmd.com/slurmdbd.conf.html)

### 2. Slurm `--json` outputs

Use as transport structure reference.

Example:

```bash
sinfo --json
squeue --json
scontrol show nodes --json
sacct --json
```
### 3. Slurm REST/OpenAPI

Use for future compatibility.

### 4. slurmdbd schema

Use only when needed internally.

Avoid exposing DB schema directly to frontend.


# Phase 1A — Entity Taxonomy

## Define entity categories


# Operational Entities

Generated mainly from:

* `sinfo`
* `scontrol`
* `squeue`

## Examples

```text
Cluster
Node
Partition
Job
Step
Reservation
Topology
Federation
License
BurstBuffer
GRES
SchedulerState
```

# Administrative Entities

Generated mainly from:

* `sacctmgr`

## Examples

```text
User
Account
Association
QOS
WCKey
Coordinator
ClusterConfig
Tres
Bank
Fairshare
```

# Accounting Entities

Generated mainly from:

* `sacct`
* `sreport`

## Examples

```text
JobHistory
UsageReport
BillingRecord
EfficiencyReport
UtilizationReport
ChargebackRecord
```

# Configuration Entities

Generated from:

* `slurm.conf`
* `slurmdbd.conf`

## Examples

```text
SlurmConfig
PartitionConfig
NodeConfig
SchedulerConfig
PriorityConfig
AccountingConfig
DatabaseConfig
AuthConfig
TopologyConfig
```

# Submission Entities

Generated from:

* `sbatch`
* `salloc`
* `srun`

## Examples

```text
JobSubmitRequest
InteractiveSession
BatchScript
ResourceRequest
JobConstraints
EnvironmentVariables
```

# Phase 1B — Canonical JSON Schema Design

Create canonical schemas.

# Recommended Directory

```text
schemas/
├── cluster/
├── node/
├── partition/
├── job/
├── accounting/
├── config/
├── events/
└── requests/
```

# Example Schema Strategy

## Canonical Node Schema

```json
{
  "node_name": "c001",
  "state": "IDLE",
  "cpus": 128,
  "real_memory": 512000,
  "alloc_memory": 0,
  "features": [],
  "gres": [],
  "partitions": [],
  "boot_time": "",
  "slurmd_version": ""
}
```

# Naming Standard Recommendation

Use:

```text
snake_case in JSON
camelCase in TypeScript
PascalCase in Go structs
```

Example:

```json
{
  "job_id": 123,
  "job_state": "RUNNING"
}
```

↓

Go:

```go
type Job struct {
    JobID int    `json:"job_id"`
    JobState string `json:"job_state"`
}
```

↓

TypeScript:

```ts
interface Job {
  jobId: number;
  jobState: string;
}
```

---

# Phase 1C — Model Documentation

Create documentation for every entity.

---

# Recommended Documentation Structure

```text
docs/models/
├── node.md
├── job.md
├── partition.md
├── qos.md
├── account.md
└── config.md
```

Each document should include:

## Source Commands

Example:

```text
Sources:
- scontrol show node --json
- sinfo --json
```

## Field Definitions

| Field     | Type   | Source   | Description        |
| --------- | ------ | -------- | ------------------ |
| node_name | string | scontrol | Node hostname      |
| state     | string | sinfo    | Current node state |


## Relationships

```text
Partition -> Nodes
Job -> User
Job -> Account
Job -> QOS
```

# Phase 1D — Event Modeling

VERY IMPORTANT.

Define canonical event schemas early.

---

# Example

```json
{
  "event_type": "JOB_UPDATED",
  "timestamp": "2026-05-23T12:00:00Z",
  "cluster": "clusterA",
  "entity_id": "12345",
  "payload": {}
}
```

# Phase 1E — Versioning Strategy

Critical for long-term maintenance.

---

# Recommended

```text
schema_version
slurm_version
compatibility_version
```

Example:

```json
{
  "schema_version": "1.0",
  "slurm_version": "25.11"
}
```

---

# Phase 2 — Frontend MVP

NOW frontend development becomes easier.

# Frontend uses generated/shared models

Recommended:

```text
schemas/json
        ↓
typescript generation
        ↓
React interfaces
```



# Recommended Frontend Data Layer

```text
hooks/
providers/
services/
models/
```


# Phase 3 — Go Backend Bridge

Backend now uses canonical schemas too.


# Backend Layering

```text
Slurm CLI JSON
       ↓
adapter/parser
       ↓
canonical models
       ↓
cache
       ↓
events
       ↓
cockpit.channel()
```

# Recommended Backend Modules

```text
internal/
├── adapters/
│   ├── sinfo/
│   ├── squeue/
│   ├── scontrol/
│   ├── sacct/
│   ├── sacctmgr/
│   └── config/
│
├── models/
├── cache/
├── events/
├── pubsub/
└── socket/
```

---

# Phase 4 — Configuration Modeling

Important and often overlooked.

# slurm.conf Modeling

Treat configuration as first-class entities.

Example:

```text
PartitionConfig
NodeConfig
SchedulerConfig
PriorityConfig
SelectTypeConfig
```

# Recommended Feature

Eventually support:

```text
Configuration diff
Configuration validation
Configuration visualization
Configuration editor
```

# Phase 5 — Submission Workflow Modeling

Model:

* `sbatch`
* `salloc`
* `srun`

as structured request entities.

# Example

```json
{
  "job_name": "gpu-test",
  "partition": "gpu",
  "nodes": 2,
  "gpus": 4,
  "time_limit": "01:00:00"
}
```

This becomes:

* UI form schema
* backend validation schema
* audit schema

# Phase 6 — Reporting & Analytics

Based on:

* `sacct`
* `sreport`

# Recommended Models

```text
UtilizationSeries
ClusterEfficiency
UserConsumption
AccountBilling
GPUUtilization
```

# Phase 7 — Event Streaming Architecture

Now event models are mature.

```text
Slurm polling
      ↓
Diff engine
      ↓
Canonical event schema
      ↓
cockpit.channel()
      ↓
React providers/hooks
```

# Phase 8 — Multi-Cluster & Federation

Models should already support:

```json
{
  "cluster": "clusterA"
}
```

everywhere.

This becomes very important later.

Branch Strategy
main
develop
feature/*
release/*
hotfix/*

# Strategic Recommendation

## Branch Strategy
* main
* develop
* feature/*
* release/*
* hotfix/*

Your strongest architectural decision is likely this one:

```text
Canonical Slurm JSON Model Layer
```

because it enables:

* shared frontend/backend contracts
* future REST APIs
* event streaming
* cache normalization
* testing
* versioning
* HA architecture
* federation support
* plugin ecosystem
* documentation generation

This is the same architectural pattern used by:

* Kubernetes
* OpenStack
* Nomad
* cloud control planes
* enterprise schedulers

and it is the correct direction for cockpit-slurm.

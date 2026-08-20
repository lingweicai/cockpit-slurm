# Canonical Resource Model

**Status:** Proposed
**Version:** 1.0
**Project:** cockpit-slurm
**Related documents:**

* `architecture.md`
* `ipc-protocol.md`
* `event-model.md`
* `command-model.md`
* `user-context.md`

---

## 1. Purpose

This document defines the **Canonical Resource Model (CRM)** used internally by `cockpit-slurm-bridge` and exposed through the `cockpit-slurm` IPC protocol.

The purpose is to provide a stable resource representation between:

```text
Slurm
  │
  │ native commands / REST API / events
  ▼
Slurm Adapter
  │
  ▼
Canonical Resource Model
  │
  ├── Cache
  ├── Query Service
  ├── Command Service
  ├── Event Manager
  └── Subscription Manager
  │
  ▼
IPC Protocol
  │
  ▼
cockpit-slurm-channel
  │
  ▼
React UI
```

The Canonical Resource Model deliberately separates the cockpit-slurm application model from:

* Slurm CLI output
* Slurm REST/OpenAPI generated structures
* `sinfo --json`
* `squeue --json`
* `sacctmgr` output
* database schemas
* React component models

The Slurm adapters translate native Slurm representations into canonical resources.

---

# 2. Design Goals

The resource model must provide:

1. Stable resource identity.
2. Consistent metadata.
3. Generation tracking.
4. Resource versioning.
5. Clear separation of desired/configured state and observed state.
6. Resource-independent IPC messages.
7. Efficient cache updates.
8. Incremental event delivery.
9. Support for Slurm 25.05 and future versions.
10. Independence from any particular Slurm adapter.
11. Efficient consumption by React.
12. Support for authorization and audit.
13. Support for CRUD resources and runtime resources.
14. Support for relationships between resources.

---

# 3. Core Principle

The Canonical Resource Model is **not a copy of the Slurm OpenAPI model**.

The relationship is:

```text
Slurm OpenAPI / CLI / database
              │
              ▼
        Adapter Layer
              │
              ▼
     Canonical Resource
              │
              ▼
          Cache/API
```

Therefore:

```text
Slurm model
    ≠
Canonical resource
```

A Slurm adapter may use one or more native Slurm structures to construct a single canonical resource.

Conversely, one Slurm resource may be represented by multiple canonical resources when this makes the application architecture clearer.

---

# 4. Resource Envelope

Every canonical resource has a common envelope.

Example:

```json
{
  "kind": "Node",
  "apiVersion": "slurm.cockpit/v1",
  "metadata": {
    "name": "node001",
    "generation": 42,
    "resourceVersion": "42-000001"
  },
  "spec": {},
  "status": {}
}
```

The common structure is:

```text
Resource
├── apiVersion
├── kind
├── metadata
├── spec
└── status
```

---

# 5. `apiVersion`

`apiVersion` identifies the canonical resource schema.

Initial value:

```text
slurm.cockpit/v1
```

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1"
}
```

The value is controlled by cockpit-slurm rather than by Slurm.

It should not contain:

```text
25.05
25.05.7
v0.0.43
```

because those identify Slurm or its REST API rather than the cockpit-slurm resource contract.

---

# 6. `kind`

`kind` identifies the resource type.

Examples:

```text
Cluster
Node
Partition
Job
Account
User
Association
QOS
Reservation
TRES
Wckey
Federation
Event
```

Example:

```json
{
  "kind": "Node"
}
```

The value is singular and uses PascalCase.

Therefore:

```text
Node
Job
Account
User
```

rather than:

```text
nodes
jobs
accounts
users
```

The plural form is used by the IPC query API:

```json
{
  "resource": "nodes"
}
```

Thus:

```text
IPC resource name      Canonical kind

nodes                  Node
jobs                   Job
accounts               Account
users                  User
```

---

# 7. Metadata

All resources have common metadata.

Initial structure:

```json
{
  "metadata": {
    "name": "node001",
    "uid": "node-001",
    "generation": 42,
    "resourceVersion": "42-000001",
    "createdAt": "2026-08-15T03:00:00Z",
    "updatedAt": "2026-08-15T03:10:00Z"
  }
}
```

The metadata fields are:

| Field             |    Required | Description                     |
| ----------------- | ----------: | ------------------------------- |
| `name`            |         yes | Resource's canonical name       |
| `uid`             | recommended | Stable resource identifier      |
| `generation`      |         yes | Cache collection generation     |
| `resourceVersion` | recommended | Version of this resource        |
| `createdAt`       |    optional | Resource creation time if known |
| `updatedAt`       |    optional | Last observed change            |
| `labels`          |    optional | Non-semantic classification     |
| `annotations`     |    optional | Additional metadata             |

---

# 8. Resource Identity

A resource must have a stable identity.

The minimum identity is:

```text
apiVersion
kind
name
```

For resources whose names are not globally unique, a scope is required.

Example:

```text
Cluster/tux
```

or:

```text
Account/science
```

For an association:

```text
Association/
    cluster=tux
    account=science
    user=alice
    partition=compute
```

The canonical identity should therefore be representable as:

```text
<ResourceKind>/<scope>/<name>
```

or, for composite resources:

```text
<ResourceKind>/<canonical-key>
```

---

# 9. UID

`uid` is an internal canonical-resource identifier.

It should remain stable during the lifetime of the resource.

Example:

```json
{
  "metadata": {
    "name": "node001",
    "uid": "node-01HXYZ..."
  }
}
```

The UID does not have to equal a Slurm database ID.

For example:

```text
Slurm job ID = 12345
```

does not imply:

```text
canonical UID = "12345"
```

The adapter may use Slurm's identifier as part of the canonical UID when appropriate.

---

# 10. Generation

Generation identifies the state of a resource collection in the Bridge cache.

Example:

```text
Node collection
generation = 100

Job collection
generation = 351

Account collection
generation = 25
```

A change to a Node collection may cause:

```text
generation 100
      ↓
node001 changed
      ↓
generation 101
```

The generation is used by:

* query responses
* snapshots
* events
* subscriptions
* resynchronization

---

# 11. Resource Version

`resourceVersion` identifies a particular version of one resource.

Example:

```json
{
  "metadata": {
    "name": "node001",
    "generation": 101,
    "resourceVersion": "101-000034"
  }
}
```

The distinction is:

```text
generation
    = collection/cache state

resourceVersion
    = individual resource revision
```

This allows the client to determine whether it has received a stale resource.

---

# 12. `spec` and `status`

Resources use two logical sections:

```text
spec
status
```

`spec` represents configuration or desired properties.

`status` represents observed Slurm state.

Example:

```json
{
  "kind": "Node",
  "spec": {
    "cpus": 64
  },
  "status": {
    "state": "idle"
  }
}
```

However, not every resource requires both sections.

For a primarily observed resource such as Job:

```json
{
  "kind": "Job",
  "spec": {},
  "status": {
    "state": "RUNNING"
  }
}
```

For an Account:

```json
{
  "kind": "Account",
  "spec": {
    "description": "Research account",
    "parent": "science"
  },
  "status": {
    "associations": 12
  }
}
```

---

# 13. Do Not Optimistically Modify Resources

The Bridge must not treat a command as a resource update.

For example:

```text
React
  │
  │ cancel job
  ▼
Command Service
  │
  ▼
Slurm
```

does not immediately imply:

```text
Job.status.state = CANCELLED
```

Instead:

```text
Slurm
  │
  ▼
Adapter
  │
  ▼
Canonical Job
  │
  ▼
Cache
  │
  ▼
Event
```

Only the observed Slurm state updates the canonical resource.

This is a core consistency rule.

---

# 14. Resource Lifecycle

A resource follows:

```text
discovered
    │
    ▼
created
    │
    ▼
active
    │
    ├───────────────┐
    ▼               │
modified            │
    │               │
    └───────────────┘
    │
    ▼
deleted
```

The Bridge should distinguish:

```text
resource does not exist
```

from:

```text
resource exists but is temporarily unavailable from an adapter
```

---

# 15. Resource Categories

The initial resources can be divided into categories.

## 15.1 Infrastructure

```text
Cluster
Node
Partition
TRES
Federation
```

## 15.2 Workload

```text
Job
Reservation
Event
```

## 15.3 Accounting

```text
Account
User
Association
QOS
Wckey
```

This categorization is for architecture/UI organization only.

It does not change the canonical resource envelope.

---

# 16. Cluster

A Cluster represents a Slurm cluster known to the accounting system.

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "Cluster",
  "metadata": {
    "name": "cluster01"
  },
  "spec": {
    "clusterName": "cluster01"
  },
  "status": {
    "state": "up",
    "nodeCount": 128
  }
}
```

Slurm's accounting model uses Cluster as a key component of associations.

---

# 17. Node

A Node represents a compute node managed by Slurm.

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "Node",
  "metadata": {
    "name": "node001",
    "generation": 42,
    "resourceVersion": "42-001"
  },
  "spec": {
    "cpus": 64,
    "realMemory": 256000,
    "boards": 1,
    "socketsPerBoard": 2,
    "coresPerSocket": 16,
    "threadsPerCore": 2,
    "gres": [
      "gpu:a100:4"
    ]
  },
  "status": {
    "state": "idle",
    "reason": null,
    "allocCPUs": 0,
    "allocMemory": 0,
    "freeCPUs": 64,
    "freeMemory": 256000
  }
}
```

The exact field mapping should be implemented by the Node adapter.

The canonical model should avoid copying every field from `sinfo` or the Slurm OpenAPI schema unless the field is useful to the application.

---

# 18. Partition

A Partition represents a Slurm scheduling partition.

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "Partition",
  "metadata": {
    "name": "compute"
  },
  "spec": {
    "nodes": [
      "node001",
      "node002"
    ],
    "default": true,
    "maxTime": "7-00:00:00",
    "priority": 10
  },
  "status": {
    "state": "up",
    "nodeCount": 128,
    "runningJobs": 20,
    "pendingJobs": 5
  }
}
```

Node membership should preferably be represented as references rather than duplicating complete Node objects.

---

# 19. Job

A Job represents a Slurm workload.

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "Job",
  "metadata": {
    "name": "12345",
    "uid": "job-12345"
  },
  "spec": {
    "user": "alice",
    "account": "research",
    "partition": "compute",
    "name": "simulation",
    "command": "run.sh",
    "requested": {
      "cpus": 16,
      "memory": 64000,
      "nodes": 2
    }
  },
  "status": {
    "state": "RUNNING",
    "reason": null,
    "startTime": "2026-08-15T03:00:00Z",
    "elapsed": 3600,
    "nodes": [
      "node001",
      "node002"
    ]
  }
}
```

A Job is primarily an observed/runtime resource.

---

# 20. Job Identity

The canonical Job identity should preserve Slurm's job identity.

For normal jobs:

```text
Job/<jobId>
```

For jobs where Slurm provides additional identity components, the canonical identity must prevent collisions.

The Bridge must not assume that:

```text
job name
```

is unique.

The job name is metadata, not identity.

---

# 21. Account

An Account represents a Slurm accounting account.

Slurm accounts may form a hierarchy, for example:

```text
science
├── chemistry
└── physics
```

and account names are unique within the accounting database hierarchy.

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "Account",
  "metadata": {
    "name": "chemistry"
  },
  "spec": {
    "description": "Chemistry research",
    "organization": "Research",
    "parent": "science"
  },
  "status": {
    "userCount": 25,
    "associationCount": 30
  }
}
```

---

# 22. Account Hierarchy

The hierarchy should be represented using references.

Example:

```json
{
  "kind": "Account",
  "metadata": {
    "name": "chemistry"
  },
  "spec": {
    "parentRef": {
      "kind": "Account",
      "name": "science"
    }
  }
}
```

Do not embed the entire parent Account.

This prevents recursive resource structures.

---

# 23. User

A User represents a unique Slurm accounting user.

This distinction is important:

```text
User
```

is not the same thing as:

```text
Association
```

Slurm explicitly states that there is one User entity per unique username, while a user can have multiple associations.

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "User",
  "metadata": {
    "name": "alice"
  },
  "spec": {
    "defaultAccount": "research",
    "defaultWCKey": null
  },
  "status": {
    "adminLevel": "none",
    "coordinatorAccounts": [
      "research"
    ]
  }
}
```

---

# 24. User AdminLevel

Slurm's `AdminLevel` values include:

```text
None
Operator
Admin
```

and these are stored as user accounting properties.

The canonical representation should normalize these to:

```text
none
operator
admin
```

Example:

```json
{
  "status": {
    "adminLevel": "operator"
  }
}
```

The Bridge must not confuse:

```text
AdminLevel
```

with:

```text
Coordinator
```

They are separate concepts.

---

# 25. Coordinator

Coordinator is a relationship between:

```text
User
Account
```

A coordinator has privileges over specific accounts rather than being globally equivalent to Admin or Operator. Slurm documents Coordinator as a special privilege associated with particular accounts.

Therefore, Coordinator should **not** be represented simply as:

```json
{
  "adminLevel": "coordinator"
}
```

Instead:

```json
{
  "kind": "User",
  "metadata": {
    "name": "alice"
  },
  "status": {
    "adminLevel": "none",
    "coordinatorAccounts": [
      "research",
      "chemistry"
    ]
  }
}
```

This is important for your role-based home-page architecture.

---

# 26. Association

Association is a first-class canonical resource.

Slurm defines an association using:

```text
cluster
account
user
partition (optional)
```

and multiple associations can exist for the same User.

Therefore:

```text
User
    │
    ├── Association
    ├── Association
    └── Association
```

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "Association",
  "metadata": {
    "name": "tux:research:alice:compute"
  },
  "spec": {
    "cluster": "tux",
    "account": "research",
    "user": "alice",
    "partition": "compute"
  },
  "status": {
    "fairShare": 1,
    "maxJobs": 20,
    "maxNodesPerJob": 4
  }
}
```

---

# 27. Association Identity

The canonical association identity should be based on:

```text
cluster
account
user
partition
```

where partition may be empty.

Conceptually:

```text
Association/
    <cluster>/
    <account>/
    <user>/
    <partition>
```

Example:

```text
Association/tux/research/alice/compute
```

and:

```text
Association/tux/research/alice/
```

represent different associations.

---

# 28. Why Association Must Not Be Embedded in User

The User resource may contain references:

```json
{
  "status": {
    "associationRefs": [
      "Association/tux/research/alice/compute",
      "Association/tux/project/alice/"
    ]
  }
}
```

but should not embed complete associations.

This avoids:

```text
User
 └── Association
      └── User
           └── Association
```

and makes incremental cache updates much easier.

---

# 29. QOS

A QOS represents Slurm Quality of Service configuration.

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "QOS",
  "metadata": {
    "name": "normal"
  },
  "spec": {
    "priority": 0,
    "maxJobs": 100,
    "maxWallDuration": 86400
  },
  "status": {
    "usage": {
      "cpu": 12000
    }
  }
}
```

The canonical resource should expose application-relevant QOS attributes rather than reproducing every `sacctmgr` output field.

---

# 30. Reservation

A Reservation represents a Slurm resource reservation.

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "Reservation",
  "metadata": {
    "name": "maintenance"
  },
  "spec": {
    "startTime": "2026-08-16T00:00:00Z",
    "endTime": "2026-08-16T04:00:00Z",
    "nodes": [
      "node001",
      "node002"
    ]
  },
  "status": {
    "state": "ACTIVE"
  }
}
```

Reservations are particularly relevant to Operator/Admin authorization.

Slurm documents Operator privileges including creation, alteration, and deletion of reservations on a served `slurmctld`.

---

# 31. TRES

TRES represents a Trackable RESource.

Examples include:

```text
CPU
Memory
Node
Energy
GRES
License
```

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "TRES",
  "metadata": {
    "name": "cpu"
  },
  "spec": {
    "type": "cpu",
    "id": 1
  }
}
```

TRES should normally be referenced by other resources rather than embedded repeatedly.

---

# 32. Wckey

A Wckey represents a Workload Characterization Key.

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "Wckey",
  "metadata": {
    "name": "research-key"
  },
  "spec": {
    "cluster": "tux",
    "user": "alice"
  }
}
```

---

# 33. Federation

A Federation represents a group of cooperating Slurm clusters.

Example:

```json
{
  "apiVersion": "slurm.cockpit/v1",
  "kind": "Federation",
  "metadata": {
    "name": "research-federation"
  },
  "spec": {
    "clusters": [
      "cluster01",
      "cluster02"
    ]
  }
}
```

---

# 34. Event as a Resource

Events require special consideration.

There are two concepts:

```text
Resource Event
```

and:

```text
IPC Resource-Change Event
```

They must not be confused.

A Slurm accounting Event may represent a persistent Slurm event, such as a node down event.

An IPC event represents:

```text
"Node resource changed"
```

The IPC event is defined by `ipc-protocol.md`.

A persistent Slurm Event resource is an independent canonical resource.

---

# 35. Resource References

Relationships should use references.

Example:

```json
{
  "kind": "Job",
  "spec": {
    "userRef": {
      "kind": "User",
      "name": "alice"
    },
    "accountRef": {
      "kind": "Account",
      "name": "research"
    },
    "partitionRef": {
      "kind": "Partition",
      "name": "compute"
    }
  }
}
```

A reference has:

```json
{
  "kind": "User",
  "name": "alice"
}
```

and may optionally contain:

```json
{
  "kind": "User",
  "name": "alice",
  "uid": "..."
}
```

---

# 36. Reference Rule

References should identify resources.

They should not contain complete resource objects.

Good:

```json
{
  "accountRef": {
    "kind": "Account",
    "name": "research"
  }
}
```

Avoid:

```json
{
  "account": {
    "kind": "Account",
    "metadata": {},
    "spec": {},
    "status": {}
  }
}
```

This reduces duplication and prevents stale embedded objects.

---

# 37. Resource Collections

The IPC protocol exposes collections using plural resource names.

Example:

```json
{
  "resource": "nodes"
}
```

The Bridge maps this to:

```text
nodes → Node
```

A collection response is:

```json
{
  "resource": "nodes",
  "generation": 42,
  "items": []
}
```

---

# 38. Collection Metadata

A collection may contain:

```json
{
  "resource": "nodes",
  "generation": 42,
  "items": [],
  "metadata": {
    "total": 128,
    "returned": 50
  }
}
```

Pagination should be introduced only when required.

For initial implementation, the Bridge should preferably support:

```text
filter
limit
offset/cursor
```

only after the basic collection API is stable.

---

# 39. Filtering

Queries may specify filters.

Example:

```json
{
  "resource": "nodes",
  "filter": {
    "state": [
      "idle",
      "allocated"
    ]
  }
}
```

For Jobs:

```json
{
  "resource": "jobs",
  "filter": {
    "user": "alice",
    "state": [
      "RUNNING",
      "PENDING"
    ]
  }
}
```

Filters are canonical application-level filters.

They must not expose raw Slurm CLI arguments.

---

# 40. Sorting

A query may eventually support:

```json
{
  "sort": [
    {
      "field": "name",
      "direction": "asc"
    }
  ]
}
```

The initial implementation should keep sorting simple.

The UI should not need to know whether the Bridge implements sorting using:

```text
Go
cache index
Slurm API
```

---

# 41. Resource Projection

The Bridge may support projections for large resources.

Example:

```json
{
  "resource": "jobs",
  "fields": [
    "metadata.name",
    "spec.user",
    "spec.account",
    "status.state"
  ]
}
```

This should be considered an optimization rather than a requirement for v1.

---

# 42. Cache Model

The Bridge cache stores canonical resources.

Conceptually:

```text
ResourceCache
│
├── Node
│    ├── node001
│    ├── node002
│    └── ...
│
├── Job
│    ├── 12345
│    ├── 12346
│    └── ...
│
├── Account
│    ├── science
│    ├── chemistry
│    └── ...
│
└── User
     ├── alice
     ├── bob
     └── ...
```

Each resource collection maintains its own generation.

---

# 43. Cache Update

The only authoritative cache update path should be:

```text
Slurm
  ↓
Adapter
  ↓
Canonical Resource
  ↓
Cache
```

A Command Service must not directly modify cache state.

---

# 44. Cache Generations

Example:

```text
Node generation      = 100
Job generation       = 300
Account generation   = 25
User generation      = 17
```

A Node modification:

```text
Node generation 100
       ↓
node001 modified
       ↓
Node generation 101
```

does not change:

```text
Account generation
User generation
```

unless those resources actually changed.

---

# 45. Event Generation

A cache update produces an event.

Example:

```text
Cache:
node001
state = idle
generation = 100

        ↓

Slurm reports:
node001
state = allocated

        ↓

Cache:
node001
state = allocated
generation = 101

        ↓

Event:
MODIFIED Node/node001
generation = 101
```

This provides a direct relationship:

```text
Cache generation
       ↓
Event generation
```

---

# 46. Resource Event Payload

The event should contain enough information for the frontend to update its local state.

Example:

```json
{
  "event": "modified",
  "resource": "nodes",
  "generation": 101,
  "item": {
    "kind": "Node",
    "metadata": {
      "name": "node001",
      "resourceVersion": "101-000003"
    },
    "status": {
      "state": "allocated"
    }
  }
}
```

For `deleted`, the complete resource is not required.

---

# 47. Resource Deletion

A deletion event may contain:

```json
{
  "event": "deleted",
  "resource": "nodes",
  "generation": 102,
  "item": {
    "kind": "Node",
    "metadata": {
      "name": "node001",
      "uid": "node-01HXYZ"
    }
  }
}
```

The frontend removes the resource identified by:

```text
kind + uid
```

or:

```text
kind + name
```

when UID is unavailable.

---

# 48. Resource State vs Connection State

Do not mix these concepts.

Resource state:

```text
Node.status.state
Job.status.state
Reservation.status.state
```

Connection state:

```text
connected
disconnected
reconnecting
```

Bridge health:

```text
healthy
degraded
initializing
```

These belong to different models.

---

# 49. Adapter Boundary

Every Slurm adapter produces canonical resources.

Example:

```text
sinfo
  ↓
NodeAdapter
  ↓
Node

squeue
  ↓
JobAdapter
  ↓
Job

sacctmgr
  ↓
AccountAdapter
  ↓
Account

sacctmgr
  ↓
UserAdapter
  ↓
User

sacctmgr association output
  ↓
AssociationAdapter
  ↓
Association
```

The React UI never needs to know which adapter produced the resource.

---

# 50. Multiple Adapter Sources

A resource may be assembled from multiple sources.

For example:

```text
User
 ├── sacctmgr user
 ├── sacctmgr coordinator
 └── sacctmgr association
        │
        ▼
   User Adapter
        │
        ▼
 Canonical User
```

This is preferable to exposing the raw `sacctmgr` structures to React.

---

# 51. Slurm REST/OpenAPI

The Slurm REST/OpenAPI model is an important **source model**, not the canonical application model.

Therefore:

```text
slurm-openapi.gen.go
        ↓
REST Adapter
        ↓
Canonical Resource
```

This allows the project to continue working if:

```text
Slurm REST API changes
```

or if:

```text
REST API unavailable
```

and the Bridge falls back to:

```text
sinfo
squeue
sacctmgr
scontrol
```

without changing the React resource contract.

---

# 52. CLI Adapter Fallback

The architecture should support:

```text
Primary:
slurmrestd / native API

Fallback:
Slurm CLI
```

Example:

```text
NodeService
    │
    ├── REST Node Adapter
    │
    └── CLI Node Adapter
```

Both must produce:

```text
Canonical Node
```

---

# 53. Resource Ownership

The Bridge owns the canonical resource representation.

The frontend owns only a **client-side view/cache**.

Therefore:

```text
Bridge Cache
    = authoritative application state

React Provider
    = client-side projection
```

React must be able to discard its state and rebuild it from:

```text
snapshot + events
```

---

# 54. React Resource Model

A React provider should consume:

```typescript
interface ResourceCollection<T> {
    generation: number;
    items: T[];
    loading: boolean;
    error?: Error;
}
```

For example:

```typescript
interface NodeResource {
    apiVersion: "slurm.cockpit/v1";
    kind: "Node";

    metadata: {
        name: string;
        uid?: string;
        generation: number;
        resourceVersion?: string;
    };

    spec: NodeSpec;
    status: NodeStatus;
}
```

The TypeScript model should correspond to the canonical resource model.

---

# 55. Resource Type Registry

The Bridge should maintain a registry.

Conceptually:

```go
type ResourceDefinition struct {
    Kind          string
    Collection    string
    Namespaced    bool
    SupportsQuery bool
    SupportsWatch bool
    SupportsCreate bool
    SupportsUpdate bool
    SupportsDelete bool
}
```

Example:

```go
var resources = []ResourceDefinition{
    {
        Kind:       "Node",
        Collection: "nodes",
        SupportsQuery: true,
        SupportsWatch: true,
    },
    {
        Kind:       "Job",
        Collection: "jobs",
        SupportsQuery: true,
        SupportsWatch: true,
    },
}
```

This allows the Bridge to discover supported resources without hard-coding them throughout the codebase.

---

# 56. Resource Capabilities

A resource can advertise capabilities.

Example:

```json
{
  "kind": "Node",
  "capabilities": {
    "query": true,
    "watch": true,
    "create": false,
    "update": true,
    "delete": false
  }
}
```

This is useful because Slurm resources have different lifecycle semantics.

For example:

```text
Node
    query ✓
    watch ✓
    create ✗
    delete ✗
```

while:

```text
Account
    query ✓
    watch ✓
    create ✓
    update ✓
    delete ✓
```

---

# 57. CRUD Semantics

The canonical resource model supports:

```text
Create
Read
Update
Delete
```

only where the underlying Slurm resource supports the operation.

CRUD should not be assumed merely because the resource exists.

For example:

```text
Node
    Read
    Watch
```

whereas:

```text
Account
    Create
    Read
    Update
    Delete
    Watch
```

---

# 58. Command/Resource Separation

The resource model describes:

```text
state
```

The command model describes:

```text
operation
```

For example:

```text
Resource:
Job/12345
status.state = RUNNING
```

Command:

```text
cancel Job/12345
```

After Slurm processes the command:

```text
Resource:
Job/12345
status.state = CANCELLED
```

The two models should remain separate.

---

# 59. Authorization and Resources

The resource model does not itself determine authorization.

For example:

```json
{
  "kind": "Job",
  "metadata": {
    "name": "12345"
  }
}
```

does not contain:

```text
canCancel
canModify
canDelete
```

Authorization is determined by:

```text
UserContext
       +
Resource
       +
Operation
       +
Slurm policy
```

The UI may receive derived capabilities separately.

---

# 60. Sensitive Data

Canonical resources should expose only data required by the caller.

For example, a normal user should not automatically receive administrative information merely because it exists in the Bridge cache.

The flow is:

```text
Cache
  ↓
Authorization / Projection
  ↓
IPC response
```

not:

```text
Cache
  ↓
IPC
```

for all callers.

This is particularly important for Slurm's `PrivateData` and administrative information.

---

# 61. Root and SlurmUser

The canonical resource model must not treat:

```text
root
```

or:

```text
SlurmUser
```

as separate resource kinds.

They are operating-system/accounting identities.

They are represented through:

```text
User
UserContext
AdminLevel
capabilities
```

The Bridge may additionally determine whether the authenticated Linux user is:

```text
root
```

or the configured:

```text
SlurmUser
```

when calculating authorization context.

---

# 62. Role Model

The canonical User resource may contain:

```text
AdminLevel:
    none
    operator
    admin

Coordinator:
    account-scoped relationship
```

The application role model can then derive:

```text
Visitor / Not Set
User
Coordinator
Operator
Admin
```

Important:

```text
Coordinator
```

is not a replacement for:

```text
AdminLevel
```

A user may be:

```text
AdminLevel = none
Coordinator = research
```

and therefore have Coordinator privileges only within the appropriate account scope.

Slurm documents this distinction explicitly.

---

# 63. Canonical Role Calculation

The effective UI role may be calculated as:

```text
Linux identity
       +
Slurm User
       +
AdminLevel
       +
Coordinator relationships
       +
capabilities
       ↓
Effective UserContext
```

For example:

```json
{
  "username": "alice",
  "adminLevel": "none",
  "coordinatorAccounts": [
    "research"
  ],
  "capabilities": [
    "jobs.view",
    "jobs.cancel",
    "accounts.view",
    "accounts.modify"
  ]
}
```

The React UI uses capabilities rather than duplicating Slurm authorization logic.

---

# 64. Resource-to-UI Mapping

The canonical model should support the UI architecture:

```text
Dashboard
    ↓
Node
Job
Account
User
Partition
Reservation
```

For example:

```text
NodeProvider
    ↓
Node resources

JobProvider
    ↓
Job resources

AccountProvider
    ↓
Account + Association resources

UserProvider
    ↓
User + Association resources
```

---

# 65. First Implementation Scope

The first implementation should NOT implement every resource.

Implement:

```text
1. Node
2. Job
3. Account
4. User
5. Association
6. Partition
```

in that order.

This corresponds to the vertical-slice strategy:

```text
Node
 ↓
Job
 ↓
Account
 ↓
User
 ↓
Association
 ↓
Partition
```

---

# 66. Node as First Vertical Slice

The first complete resource should be Node.

Implementation:

```text
Slurm
  ↓
NodeAdapter
  ↓
Canonical Node
  ↓
NodeCache
  ↓
QueryService
  ↓
IPC
  ↓
Channel
  ↓
NodeProvider
  ↓
NodeTable
```

Then add:

```text
Node change
  ↓
Cache generation++
  ↓
Event
  ↓
Subscription
  ↓
NodeProvider
  ↓
React update
```

---

# 67. Job as Second Vertical Slice

Job introduces:

```text
high update frequency
user ownership
account relationship
partition relationship
commands
```

Therefore it is an important test of the canonical model.

Example relationships:

```text
Job
 ├── UserRef
 ├── AccountRef
 ├── PartitionRef
 └── NodeRefs
```

---

# 68. Account/User/Association Model

The accounting model should be represented as:

```text
                    ┌─────────────┐
                    │    User     │
                    │   alice     │
                    └──────┬──────┘
                           │
                           │
                    ┌──────▼──────┐
                    │ Association  │
                    │              │
                    │ cluster=tux  │
                    │ account=     │
                    │ research     │
                    │ partition=   │
                    │ compute      │
                    └──────┬──────┘
                           │
                           │
                    ┌──────▼──────┐
                    │   Account    │
                    │   research   │
                    └─────────────┘
```

This closely reflects Slurm's accounting model.

---

# 69. Why This Model Is Important

Without this separation, it is tempting to model:

```text
User
 ├── account
 ├── partition
 ├── limits
 └── fairShare
```

as if each user had only one account.

That is incorrect for Slurm.

A user can have:

```text
alice
 ├── research / compute
 ├── research / gpu
 └── teaching / cpu
```

These are separate associations.

Therefore:

```text
User
```

and:

```text
Association
```

must be separate canonical resources.

---

# 70. Example Complete Resource Set

A realistic example:

```json
{
  "resources": [
    {
      "apiVersion": "slurm.cockpit/v1",
      "kind": "User",
      "metadata": {
        "name": "alice"
      },
      "spec": {
        "defaultAccount": "research"
      },
      "status": {
        "adminLevel": "none",
        "coordinatorAccounts": [
          "research"
        ]
      }
    },
    {
      "apiVersion": "slurm.cockpit/v1",
      "kind": "Account",
      "metadata": {
        "name": "research"
      },
      "spec": {
        "parent": "root"
      }
    },
    {
      "apiVersion": "slurm.cockpit/v1",
      "kind": "Association",
      "metadata": {
        "name": "cluster01:research:alice:compute"
      },
      "spec": {
        "cluster": "cluster01",
        "account": "research",
        "user": "alice",
        "partition": "compute"
      }
    }
  ]
}
```

---

# 71. Serialization

The canonical resource is serialized as JSON for IPC.

Go:

```go
type Resource struct {
    APIVersion string          `json:"apiVersion"`
    Kind       string          `json:"kind"`
    Metadata   Metadata        `json:"metadata"`
    Spec       json.RawMessage `json:"spec,omitempty"`
    Status     json.RawMessage `json:"status,omitempty"`
}
```

Typed resources can then be defined:

```go
type Node struct {
    APIVersion string       `json:"apiVersion"`
    Kind       string       `json:"kind"`
    Metadata   ObjectMeta   `json:"metadata"`
    Spec       NodeSpec     `json:"spec"`
    Status     NodeStatus   `json:"status"`
}
```

---

# 72. TypeScript Serialization

React should use equivalent types.

Example:

```typescript
export interface ObjectMeta {
    name: string;
    uid?: string;
    generation: number;
    resourceVersion?: string;
    createdAt?: string;
    updatedAt?: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
}

export interface Resource<TSpec, TStatus> {
    apiVersion: string;
    kind: string;
    metadata: ObjectMeta;
    spec: TSpec;
    status: TStatus;
}
```

Then:

```typescript
export type NodeResource =
    Resource<NodeSpec, NodeStatus>;
```

---

# 73. Canonical Model vs Generated Models

Generated Slurm OpenAPI structures should remain in the adapter layer.

For example:

```text
internal/
├── adapter/
│   └── slurmrest/
│       └── generated/
│
└── resource/
    ├── node.go
    ├── job.go
    ├── account.go
    ├── user.go
    └── association.go
```

Do not expose:

```text
slurm-openapi.gen.go
```

directly to:

```text
React
IPC
Cache
```

The adapter converts:

```text
Slurm API model
        ↓
Canonical model
```

---

# 74. Compatibility

The canonical model provides a compatibility boundary.

If Slurm changes:

```text
Slurm 25.05
    ↓
Slurm 25.11
    ↓
Slurm 26.x
```

the adapter may change while:

```text
Node
Job
Account
User
Association
```

remain stable.

Similarly, if the project changes from:

```text
slurmrestd
```

to:

```text
CLI
```

the canonical resource contract does not change.

---

# 75. Versioning Rule

A breaking canonical model change requires:

```text
slurm.cockpit/v2
```

For example:

```text
slurm.cockpit/v1
```

may become:

```text
slurm.cockpit/v2
```

when a field's semantics change incompatibly.

Adding an optional field should normally remain:

```text
slurm.cockpit/v1
```

---

# 76. Resource Model and IPC Protocol

The relationship is:

```text
IPC Protocol
    │
    ├── resource = "nodes"
    │
    └── payload.items[]
              │
              ▼
       Canonical Node
```

The IPC protocol defines:

```text
how resources are transported
```

while this document defines:

```text
what those resources mean
```

---

# 77. Resource Model and Event Model

The event model defines how changes to canonical resources are communicated.

Example:

```text
Canonical Node
      │
      │ modified
      ▼
Event Model
      │
      ▼
IPC event
```

Therefore:

```text
resource-model.md
        ↓
event-model.md
        ↓
ipc-protocol.md
```

rather than putting all three responsibilities into one document.

---

# 78. Resource Model and Command Model

Commands operate on resources.

Example:

```text
Command:
    operation = cancel
    resource = Job
    target = Job/12345
```

The command changes Slurm state.

The resulting canonical resource change is then emitted as an event.

```text
Command
   ↓
Slurm
   ↓
Canonical Job
   ↓
Cache
   ↓
Event
```

---

# 79. Resource Model Invariants

The following invariants must be maintained.

### Invariant 1

Every resource has:

```text
apiVersion
kind
metadata.name
metadata.generation
```

### Invariant 2

A resource's identity must be stable.

### Invariant 3

A cache generation must never decrease.

### Invariant 4

A resource version must change when the resource changes.

### Invariant 5

Commands do not directly mutate the cache.

### Invariant 6

Events represent observed cache changes.

### Invariant 7

References do not contain complete embedded resources.

### Invariant 8

Slurm adapter models never become the public canonical model.

### Invariant 9

Authorization is not encoded into resource state.

### Invariant 10

AdminLevel and Coordinator remain separate concepts.

---

# 80. Initial Resource Registry

The initial registry should be:

| Collection     | Kind          | Query | Watch |          Create          |          Update          |          Delete          |
| -------------- | ------------- | :---: | :---: | :----------------------: | :----------------------: | :----------------------: |
| `clusters`     | `Cluster`     |   ✓   |   ✓   |            ✓*            |            ✓*            |            ✓*            |
| `nodes`        | `Node`        |   ✓   |   ✓   |             —            |          limited         |             —            |
| `partitions`   | `Partition`   |   ✓   |   ✓   |             —            |          limited         |             —            |
| `jobs`         | `Job`         |   ✓   |   ✓   |             —            |          limited         |             —            |
| `accounts`     | `Account`     |   ✓   |   ✓   |             ✓            |             ✓            |             ✓            |
| `users`        | `User`        |   ✓   |   ✓   |             ✓            |             ✓            |             ✓            |
| `associations` | `Association` |   ✓   |   ✓   |             ✓            |             ✓            |             ✓            |
| `qos`          | `QOS`         |   ✓   |   ✓   |             ✓            |             ✓            |             ✓            |
| `reservations` | `Reservation` |   ✓   |   ✓   |             ✓            |             ✓            |             ✓            |
| `tres`         | `TRES`        |   ✓   |   ✓   | implementation-dependent | implementation-dependent | implementation-dependent |
| `wckeys`       | `Wckey`       |   ✓   |   ✓   |             ✓            |             ✓            |             ✓            |

`*` depends on whether the particular operation is appropriate for the deployment.

The registry should be treated as an implementation capability matrix, not as a promise that every Slurm version exposes every operation in exactly the same way.

---

# 81. Recommended First Implementation

Implement only:

```text
Node
Job
Account
User
Association
Partition
```

and establish:

```text
Canonical Resource
        ↓
Cache
        ↓
Query
        ↓
Subscription
        ↓
Event
```

before expanding the registry.

---

# 82. Recommended Directory Structure

```text
internal/
├── resource/
│   ├── resource.go
│   ├── metadata.go
│   ├── reference.go
│   ├── collection.go
│   │
│   ├── node.go
│   ├── job.go
│   ├── partition.go
│   ├── account.go
│   ├── user.go
│   ├── association.go
│   ├── qos.go
│   ├── reservation.go
│   ├── tres.go
│   ├── wckey.go
│   └── cluster.go
│
├── cache/
│   ├── cache.go
│   ├── collection.go
│   └── generation.go
│
└── adapter/
    ├── slurmrest/
    ├── sacctmgr/
    ├── scontrol/
    ├── sinfo/
    └── squeue/
```

---

# 83. Final Architecture

The complete resource flow is:

```text
                 Slurm
                   │
       ┌───────────┼────────────┐
       │           │            │
   slurmrestd    sacctmgr     CLI
       │           │            │
       └───────────┼────────────┘
                   ▼
             Adapter Layer
                   │
                   ▼
        Canonical Resource Model
                   │
          ┌────────┼─────────┐
          │        │         │
        Cache    Query     Command
          │        │         │
          │        │         ▼
          │        │       Slurm
          │        │
          ▼        ▼
        Events   IPC responses
          │
          ▼
    Subscription Manager
          │
          ▼
         IPC
          │
          ▼
cockpit-slurm-channel
          │
          ▼
        Cockpit
          │
          ▼
         React
```

---

# 84. Summary

The most important architectural decision is:

```text
              Slurm-native models
                     │
                     ▼
                ADAPTER
                     │
                     ▼
        ┌────────────────────────┐
        │ Canonical Resources    │
        │                        │
        │ Node                   │
        │ Job                    │
        │ Account                │
        │ User                   │
        │ Association            │
        │ Partition              │
        │ ...                    │
        └───────────┬────────────┘
                    │
             authoritative
               Bridge Cache
                    │
        ┌───────────┴────────────┐
        │                        │
      Query                    Event
        │                        │
        └───────────┬────────────┘
                    │
                   IPC
                    │
                   UI
```

The Canonical Resource Model is therefore the **central contract of cockpit-slurm**, sitting between the Slurm-specific world and the Cockpit/React-specific world.

In particular, the `User → Association → Account` separation should be treated as a foundational design decision. Slurm's accounting model explicitly makes the association a distinct concept, with `cluster + account + user + optional partition` defining an association.

That will pay off later when you implement your **Coordinator**, **AdminLevel**, account hierarchy, account-scoped permissions, and role-based UI. Slurm's own permission model distinguishes global Admin/Operator privileges from account-scoped Coordinator privileges.

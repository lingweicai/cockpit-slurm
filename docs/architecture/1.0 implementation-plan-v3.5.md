# Implementation Plan for cockpit-slurm project (v3.5) 

The three v3.5 development plans together rather than treating them as three independent projects:

* **UI v2** — React/PatternFly presentation, role-based navigation/home pages, entity pages, subscriptions, and job submission. ([GitHub][1])
* **Bridge v3.5** — the actual application/service layer: canonical resources, adapters, cache, CQRS-style query/command paths, events, subscriptions, authorization, and audit. ([GitHub][2])
* **Channel v3.5** — deliberately thin transport between Cockpit's `cockpit.channel()` and the bridge Unix socket. It should not contain Slurm logic, cache, authorization, or resource models. ([GitHub][3])

The most important recommendation is:

> **Do not implement the three plans sequentially. Implement them as one vertical-slice project, with the Bridge as the architectural center, Channel as the transport, and UI as the consumer.**

This is particularly important because Cockpit's package bridge mechanism maps a `cockpit.channel()` open request to a spawned bridge process, while your persistent `cockpit-slurm-bridge` owns the actual cluster state. ([Cockpit Project][4])

# 1. Recommended overall implementation sequence

I would organize the work into **8 phases**:

| Phase | Main component         | Result                                               |
| ----- | ---------------------- | ---------------------------------------------------- |
| 0     | Architecture contracts | Protocol, resource, identity, error contracts frozen |
| 1     | Bridge foundation      | Go project skeleton + IPC server                     |
| 2     | Channel                | `cockpit.channel()` ↔ Unix socket works              |
| 3     | First vertical slice   | Account/Node/Job query through complete stack        |
| 4     | Bridge resource system | Canonical resources + cache + synchronization        |
| 5     | Commands/events        | Add/modify/delete + events + command tracking        |
| 6     | UI architecture        | Providers + navigation + role-based pages            |
| 7     | Production features    | authorization, audit, error handling, resilience     |
| 8     | Expansion              | remaining Slurm resources and enterprise UI          |

The key is that **Phase 3 should be your first real milestone**.

You should be able to demonstrate:

```text
React
  │
  │ cockpit.channel()
  ▼
cockpit-slurm-channel
  │
  │ Unix socket
  ▼
cockpit-slurm-bridge
  │
  ▼
Cache
  │
  ▼
Slurm
```

and then:

```text
Slurm → Bridge Cache → Event → Channel → React
```

before implementing dozens of resources.

---

# 2. Phase 0 — Freeze the contracts first

I strongly recommend doing this before substantial coding.

Create a small `docs/architecture/` area containing:

```text
docs/
└── architecture/
    ├── architecture.md
    ├── ipc-protocol.md
    ├── resource-model.md
    ├── user-context.md
    ├── event-model.md
    ├── command-model.md
    └── error-model.md
```

### Freeze these six contracts

### A. IPC envelope

For example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01...",
  "type": "query",
  "resource": "nodes",
  "payload": {}
}
```

and:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01...",
  "type": "snapshot",
  "resource": "nodes",
  "generation": 123,
  "payload": {}
}
```

Your channel plan already has essentially this direction. ([GitHub][3])

### B. Resource identity

For example:

```text
kind
name
namespace/cluster
generation
resourceVersion
```

### C. Event envelope

```text
event
 ├── eventId
 ├── generation
 ├── resource
 ├── operation
 └── resource data
```

### D. UserContext

This is particularly important for your Admin/Operator/Coordinator/User design:

```go
type UserContext struct {
    Username      string
    UID           uint32
    GID           uint32
    AdminLevel    AdminLevel
    Coordinator   []string
    Accounts      []string
    Capabilities  CapabilitySet
}
```

Your v3.5 channel plan already places UserContext establishment in the bridge rather than the React application. ([GitHub][3])

### E. Command model

```text
commandId
user
operation
resource
parameters
status
createdAt
completedAt
error
```

### F. Error model

Don't let every layer invent its own errors.

Define:

```text
authentication-error
authorization-error
validation-error
not-found
conflict
slurm-error
timeout
internal-error
transport-error
```

This will save considerable refactoring later.

---

# 3. Phase 1 — Build the Bridge foundation first

The Bridge should be your **primary development project**.

The v3.5 architecture correctly puts the canonical resource model, cache, adapters, services, events and subscriptions inside the bridge. ([GitHub][2])

I would implement it in this order:

```text
cmd/
└── cockpit-slurm-bridge/

internal/
├── ipc/
├── protocol/
├── identity/
├── service/
├── resource/
├── cache/
├── event/
├── subscription/
├── command/
└── adapter/
```

Then:

### Step 1. IPC server

First make:

```text
/run/cockpit-slurm/bridge.sock
```

work.

Don't implement Slurm yet.

Test:

```text
client → socket → bridge → response
```

### Step 2. Protocol dispatcher

Implement:

```go
switch message.Type {
case "query":
case "subscribe":
case "command":
case "unsubscribe":
case "ping":
}
```

### Step 3. UserContext

Establish the authenticated Cockpit user at the beginning of a connection.

### Step 4. Service interfaces

Define interfaces before implementations:

```go
type QueryService interface {}
type CommandService interface {}
type ResourceService interface {}
type SubscriptionService interface {}
```

This is consistent with the v3.5 plan's Application Service Layer. ([GitHub][2])

---

# 4. Phase 2 — Implement Channel very early

Although the Bridge is architecturally the center, the **Channel should be implemented immediately after the IPC skeleton**.

Why?

Because you need to validate that your Cockpit integration assumption is correct before building a large backend.

Cockpit's documented model is:

```text
React
  ↓
cockpit.channel()
  ↓
Cockpit bridge matching
  ↓
cockpit-slurm-channel
```

and package `manifest.json` can register a bridge using `"match"` and `"spawn"`. ([Cockpit Project][4])

Your channel should remain extremely small:

```text
cockpit-slurm-channel
├── Cockpit protocol
├── stdin/stdout
├── Unix socket client
├── forwarding
└── lifecycle
```

Exactly as your v3.5 plan specifies. ([GitHub][3])

### Do NOT put these in Channel

```text
❌ Slurm CLI
❌ slurm-openapi models
❌ cache
❌ event bus
❌ authorization
❌ command processing
❌ resource adapters
```

This separation is one of the strongest aspects of your current architecture.

---

# 5. Phase 3 — Build ONE complete vertical slice

This is the most important change I recommend to your development methodology.

Don't implement:

```text
all Nodes
all Jobs
all Accounts
all Users
all UI
...
```

Instead implement:

```text
Node
```

completely.

For example:

```text
Slurm
  ↓
Node Resource Adapter
  ↓
Canonical Node
  ↓
Node Cache
  ↓
Query Service
  ↓
IPC
  ↓
Channel
  ↓
cockpit.channel()
  ↓
NodeProvider
  ↓
NodeTable
```

Then implement:

```text
NodeChanged
  ↓
Event Bus
  ↓
Subscription Manager
  ↓
IPC
  ↓
Channel
  ↓
NodeProvider
  ↓
React table update
```

This proves your entire architecture.

---

# 6. Which resource should be first?

For your project I would use:

### First: `node`

Because it gives you:

* Slurm state
* relatively understandable data
* large-table behavior
* generation updates
* filtering
* expandable rows
* event updates

Then:

### Second: `job`

This tests:

* high update frequency
* user ownership
* command operations
* cancel/suspend/resume
* subscriptions

Then:

### Third: `account`

This tests:

* sacctmgr
* persistent configuration resources
* Coordinator authorization
* CRUD

Then:

### Fourth: `user`

This tests:

* identity
* AdminLevel
* Coordinator
* account associations

So:

```text
Node
 ↓
Job
 ↓
Account
 ↓
User
```

is a very good architectural progression.

---

# 7. Phase 4 — Build the Canonical Resource + Cache system

Once Node works end-to-end, generalize it.

The Bridge plan correctly says:

> Cache stores authoritative state.

and:

> Command Adapter never modifies Cache.

The write path instead goes:

```text
Command
 ↓
Slurm
 ↓
Resource Adapter
 ↓
Canonical Resource
 ↓
Cache
 ↓
Event
```

This is an important architectural rule. ([GitHub][2])

I would make this a hard invariant.

### Don't do this

```text
React
 ↓
Command
 ↓
Cache.Update()
 ↓
Slurm
```

because the cache could temporarily claim something happened when Slurm rejected the operation.

### Do this

```text
React
 ↓
Command
 ↓
Slurm
 ↓
Refresh
 ↓
Canonical Resource
 ↓
Cache
 ↓
Event
 ↓
React
```

This makes the cache a representation of **Slurm state**, not a prediction of desired state.

---

# 8. Phase 5 — Events and subscriptions

After query/snapshot works, implement:

```text
subscribe
 ↓
snapshot
 ↓
events
 ↓
unsubscribe
```

Your Bridge plan already describes this lifecycle. ([GitHub][2])

I recommend that subscriptions be **resource-oriented**, not UI-oriented.

Good:

```json
{
  "type": "subscribe",
  "resource": "node"
}
```

Possibly:

```json
{
  "type": "subscribe",
  "resource": "job",
  "filter": {
    "user": "alice"
  }
}
```

Avoid:

```json
{
  "type": "subscribe",
  "component": "RunningJobsTable"
}
```

The backend should know about resources, not React components.

---

# 9. Phase 6 — React architecture

Only after the first backend vertical slice works should you generalize the React architecture.

Your UI plan's idea of:

```text
NodeProvider
JobProvider
AccountProvider
```

is good. ([GitHub][2])

I would make the hierarchy:

```text
CockpitSlurmApp
│
├── UserContextProvider
│
├── SlurmConnectionProvider
│
├── NodeProvider
├── JobProvider
├── AccountProvider
└── UserProvider
```

Each provider owns:

```text
channel
subscription
snapshot
events
loading
error
```

But I would **not create one `cockpit.channel()` per table component**.

For example:

```text
NodeProvider
 ├── NodeTable
 ├── NodeDetail
 ├── NodeCharts
 └── NodeSummary
```

all consume the same Node state.

This will prevent excessive channels/subscriptions.

---

# 10. Phase 7 — Role-based UI

Your UI plan has five conceptual states:

```text
Visitor / Not Set
User
Coordinator
Operator
Admin
```

and different landing pages/navigation. ([GitHub][1])

I recommend implementing this **after UserContext works end-to-end**.

The flow should be:

```text
Cockpit authenticated identity
        ↓
cockpit-slurm-channel
        ↓
Bridge
        ↓
UserContext
        ↓
React
        ↓
NavigationPolicy
        ↓
HomePagePolicy
```

Do not make the frontend independently determine:

```typescript
if (username === "root")
```

or:

```typescript
if (isInWheel)
```

Instead let the backend establish the authoritative Slurm application context.

React should receive:

```typescript
interface UserContext {
    username: string;
    adminLevel: AdminLevel;
    coordinatorAccounts: string[];
    capabilities: Capability[];
}
```

Then UI decisions become:

```typescript
can("nodes.modify")
can("jobs.cancel")
can("accounts.modify")
```

rather than:

```typescript
adminLevel === "admin"
```

This gives you much more flexibility later.

---

# 11. Phase 8 — Implement navigation after authorization

Your proposed Cockpit navigation is good:

```text
SLURM
├── Dashboard
├── Compute
├── Workloads
├── Users & Accounts
├── Storage
├── Monitoring
├── Administration
└── Settings
```

with PatternFly tabs for the second level. ([GitHub][1])

But I would implement it using a **single navigation definition**:

```typescript
const navigation = [
    {
        id: "compute",
        path: "/compute",
        required: ["nodes.view"],
    },
    {
        id: "users",
        path: "/users",
        required: ["users.view"],
    },
];
```

Then:

```text
UserContext
      ↓
CapabilityResolver
      ↓
NavigationFilter
      ↓
Sidebar
```

This prevents the very common problem where:

> a button is hidden in React, but the backend still accepts the operation.

The backend must always enforce authorization; frontend hiding is only UX.

---

# 12. Recommended milestones

I would turn the three documents into these concrete milestones.

### M0 — Architecture frozen

Deliver:

```text
IPC protocol
Resource model
Event model
Command model
UserContext
Error model
```

### M1 — Bridge IPC

Deliver:

```text
cockpit-slurm-bridge
        ↑
Unix socket
```

with:

```text
ping
query
subscribe
```

but fake data initially.

### M2 — Channel integration

Deliver:

```text
React
 ↓
cockpit.channel()
 ↓
cockpit-slurm-channel
 ↓
Unix socket
 ↓
bridge
```

Cockpit package manifest is working.

### M3 — Node vertical slice

Deliver:

```text
Slurm → Node Adapter → Cache
                    ↓
                  Query
                    ↓
                 React
```

### M4 — Node real-time events

Deliver:

```text
Slurm
 ↓
Resource refresh
 ↓
Cache
 ↓
Event
 ↓
Subscription
 ↓
React
```

### M5 — Job resource

Add:

```text
JobProvider
JobTable
JobDetail
Job events
```

### M6 — Commands

Add:

```text
cancel
resume
suspend
```

with:

```text
Command ID
Command status
Authorization
Audit
```

### M7 — Account/User

Add:

```text
Account
Association
User
Coordinator
AdminLevel
```

### M8 — Role-based UI

Add:

```text
Visitor
User
Coordinator
Operator
Admin
```

### M9 — Production hardening

Add:

```text
reconnect
timeouts
backpressure
event recovery
stale generation detection
audit
metrics
logging
security
```

---

# 13. How I would divide your three development documents

I would **not maintain three independent development schedules**.

Instead:

### Bridge document

Becomes the **master implementation plan**.

It owns:

```text
architecture
resource model
cache
adapters
services
commands
events
subscriptions
authorization
audit
IPC server
```

### Channel document

Becomes a **small implementation plan**.

It owns only:

```text
Cockpit integration
stdin/stdout
channel framing
Unix socket client
forwarding
connection lifecycle
```

Cockpit itself already defines `cockpit.channel()` as a raw communication mechanism and advises that it is normally used through higher-level APIs, so keeping your channel thin is consistent with Cockpit's architecture. ([Cockpit Project][5])

### UI document

Becomes the **consumer/application plan**.

It owns:

```text
UserContextProvider
Resource Providers
Navigation
Role-based Home
Tables
Detail pages
Commands UI
Charts
Job submission
```

This produces:

```text
                 ┌──────────────────────┐
                 │      React UI        │
                 │                      │
                 │ Providers            │
                 │ Navigation           │
                 │ Pages                │
                 │ Tables               │
                 └──────────┬───────────┘
                            │
                    cockpit.channel()
                            │
                 ┌──────────▼───────────┐
                 │ cockpit-slurm-channel│
                 │                      │
                 │ Transport only       │
                 └──────────┬───────────┘
                            │
                      Unix socket
                            │
                 ┌──────────▼───────────┐
                 │ cockpit-slurm-bridge │
                 │                      │
                 │ IPC                  │
                 │ UserContext          │
                 │ Services             │
                 │ Commands             │
                 │ Queries              │
                 │ Subscriptions        │
                 │ Events               │
                 │ Cache                │
                 │ Canonical Resources  │
                 │ Slurm Adapters       │
                 └──────────┬───────────┘
                            │
                 ┌──────────▼───────────┐
                 │        Slurm         │
                 └──────────────────────┘
```

That is, in my view, the correct **v3.5 implementation architecture**.

---

# 14. One important change: don't implement the entire UI plan yet

Your UI document contains some excellent long-term features—global search, entity pages, job submission wizard, monitoring, storage, topology, etc. ([GitHub][1])

But I would classify them:

### Tier 1 — implement now

```text
Dashboard
Nodes
Jobs
Accounts
Users
Partitions
UserContext
Role-based navigation
```

### Tier 2 — implement after architecture stabilizes

```text
Node detail
Job detail
Account detail
User detail
Global search
Charts
Reservations
```

### Tier 3 — later

```text
Storage
Topology
GPU metrics
Energy
Alerts
Job templates
Job submission wizard
Advanced monitoring
```

Otherwise the UI will drive backend design prematurely.

---

# 15. Suggested Git development strategy

Given your existing `development-bridge-v3.5` branch, I would avoid three completely independent branches.

Instead use feature branches such as:

```text
development-bridge-v3.5
│
├── feature/ipc-protocol
├── feature/bridge-foundation
├── feature/channel-transport
├── feature/node-resource
├── feature/node-subscription
├── feature/job-resource
├── feature/command-service
├── feature/user-context
├── feature/account-resource
├── feature/react-providers
├── feature/role-navigation
└── feature/role-home-pages
```

Each feature should ideally be independently testable.

For example:

```text
feature/node-resource
```

should include:

```text
Go Node model
Node adapter
Node cache
Node query
Node subscription
Node IPC messages
NodeProvider
NodeTable
```

rather than putting all backend work in one branch and all frontend work in another for months.

---

# 16. My recommended first 10 implementation tasks

If you are ready to start coding now, I would do exactly these:

1. **Freeze v1.0 IPC protocol.**
2. **Create the Go Bridge package structure.**
3. **Implement `/run/cockpit-slurm/bridge.sock`.**
4. **Implement Bridge protocol dispatcher.**
5. **Implement `ping` and `query` with mock resources.**
6. **Implement `cockpit-slurm-channel` Unix-socket forwarding.**
7. **Register Channel through Cockpit `manifest.json`.**
8. **Create a minimal React `SlurmChannel` wrapper around `cockpit.channel()`.**
9. **Implement the complete Node vertical slice.**
10. **Replace mock Node data with the real Slurm adapter.**

At task 10, you should have a very valuable milestone:

> **A real Slurm node appears in Cockpit through the new v3.5 Bridge → Channel → `cockpit.channel()` architecture.**

Then add events.

That is much safer than trying to implement all three 500–900-line development plans at once.

[Cockpit package/bridge documentation](https://cockpit-project.org/guide/latest/packages.html?utm_source=chatgpt.com) [Cockpit channel documentation](https://cockpit-project.org/guide/latest/cockpit-channels.html?utm_source=chatgpt.com) [Your Bridge v3.5 development plan](https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/development-cockpit-slurm-bridge_v3.5.md?utm_source=chatgpt.com) [Your Channel v3.5 development plan](https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/development-cockpit-slurm-channel-v3.5.md?utm_source=chatgpt.com) [Your UI v2 development plan](https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/development-cockpit-slurm-UI-v2.md?utm_source=chatgpt.com)

[1]: https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/development-cockpit-slurm-UI-v2.md "cockpit-slurm/development-cockpit-slurm-UI-v2.md at development-bridge-v3.5 · lingweicai/cockpit-slurm · GitHub"
[2]: https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/development-cockpit-slurm-bridge_v3.5.md "cockpit-slurm/development-cockpit-slurm-bridge_v3.5.md at development-bridge-v3.5 · lingweicai/cockpit-slurm · GitHub"
[3]: https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/development-cockpit-slurm-channel-v3.5.md "cockpit-slurm/development-cockpit-slurm-channel-v3.5.md at development-bridge-v3.5 · lingweicai/cockpit-slurm · GitHub"
[4]: https://cockpit-project.org/guide/latest/packages.html?utm_source=chatgpt.com "Cockpit Packages"
[5]: https://cockpit-project.org/guide/latest/development?utm_source=chatgpt.com "Developer Guide"

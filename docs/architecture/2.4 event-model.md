# cockpit-slurm Event Model
**location:** `docs/architecture/event-model.md`

**Status:** Proposed  
**Version:** 1.0  
**Project:** cockpit-slurm

Related documents:

- `implementation-plan-v3.5.md`
- `ipc-protocol.md`
- `resource-model.md`
- `user-context.md`
- `development-cockpit-slurm-bridge_v3.5.md`
- `development-cockpit-slurm-channel-v3.5.md`
- `development-cockpit-slurm-UI-v2.md`


# 0. Background

Based on the current v3.5 architecture, I recommend making `event-model.md` the **contract between the Resource Cache, Event Bus, Subscription Manager, IPC protocol, and React providers**.

I reviewed the current architecture directory and, in particular, your IPC protocol and Bridge v3.5 documents. The repository currently defines the Event Bus as generating `Created`, `Updated`, and `Deleted` events **after Cache updates**, with the Subscription Manager delivering a snapshot followed by events. It also explicitly says that the Cache is the authoritative state and that React consumes snapshots plus events. ([GitHub][1])

One important refinement I would make in this new document is to distinguish **resource events** from **command-status events**. A command such as `CancelJob` has a lifecycle, while `JobUpdated` represents a change in canonical resource state. They should not be conflated.

Below is the proposed complete content.

# 1. Purpose

This document defines the event model used by `cockpit-slurm`.

The Event Model describes how changes in the authoritative Resource Cache are transformed into events and delivered to subscribed clients.

The main responsibilities are:

- define the event envelope
- define resource lifecycle events
- define event ordering
- define resource generations
- define snapshots
- define subscription delivery
- define filtering
- define reconnect behavior
- define event loss handling
- define command-related events
- define event authorization
- define the relationship between Cache, Event Bus and Subscription Manager
- define how React consumes events

The central architecture is:

```text
    Slurm
      |
      v
    Resource Adapter
      |
      v
    Canonical Resource
      |
      v
    Resource Cache
      |
      v
    Event Bus
      |
      v
    Subscription Manager
      |
      v
    cockpit-slurm-channel
      |
      v
    React
```
The most important rule is:

> Events describe changes to authoritative canonical resources. The Resource Cache remains the source of truth.

---

# 2. Design Principles

The event architecture follows these principles.

1. The Resource Cache is the authoritative state inside the Bridge.
2. Events are generated only after a successful Cache update.
3. Events do not replace the Cache.
4. Events describe state transitions or resource changes.
5. Every resource event identifies the resource kind and resource ID.
6. Resource generations provide ordering and change detection.
7. Subscriptions begin with a snapshot.
8. Events received after the snapshot update the local client state.
9. Events are asynchronous.
10. Events do not require a request/response pair.
11. Events are scoped to subscriptions.
12. Event delivery is filtered according to the subscription.
13. Event authorization is performed by the Bridge.
14. The Channel transports events but does not interpret them.
15. React should treat events as state updates, not as the authoritative state itself.
16. If an event gap is detected, the client must resynchronize from a snapshot.
17. Command lifecycle events and resource events are different concepts.
18. Event IDs are distinct from message IDs, command IDs and resource generations.
19. Event ordering is guaranteed within an appropriate resource/subscription stream, not necessarily globally across the entire cluster.
20. The design must support multiple concurrent Cockpit sessions.

---

# 3. Event Architecture

The complete event flow is:
```text

    ┌─────────────────────┐
    │    Slurm Cluster    │
    └──────────┬──────────┘
               │
               │ state
               ▼
    ┌─────────────────────┐
    │  Resource Adapter    │
    └──────────┬──────────┘
               │
               │ Canonical Resource
               ▼
    ┌─────────────────────┐
    │   Resource Cache     │
    │                      │
    │  authoritative state │
    └──────────┬──────────┘
               │
               │ successful mutation
               ▼
    ┌─────────────────────┐
    │      Event Bus       │
    └──────────┬──────────┘
               │
               │ Resource Event
               ▼
    ┌─────────────────────┐
    │ Subscription Manager │
    └──────────┬──────────┘
               │
               │ filtered event
               ▼
    ┌─────────────────────┐
    │ cockpit-slurm-       │
    │ channel              │
    └──────────┬──────────┘
               │
               │ IPC / Cockpit channel
               ▼
    ┌─────────────────────┐
    │      React UI        │
    └─────────────────────┘
```

The Event Bus does not directly communicate with React.

The Subscription Manager is responsible for deciding which connected clients receive each event.

---

# 4. Cache-First Rule

The most important event rule is:

    Resource Adapter
          |
          v
    Canonical Resource
          |
          v
    Cache Update
          |
          v
    Event Generation

Never:

    Resource Adapter
          |
          +----> Event
          |
          +----> Cache

because this can cause clients to observe an event before the Cache contains the corresponding state.

The correct ordering is:

    Cache successfully updated
            |
            v
        Event emitted

This ensures that an event always refers to state that can subsequently be retrieved from the Cache.

---

# 5. Single Source of Truth

The Resource Cache is the authoritative state.

Events are notifications that the state has changed.

Therefore:

    Cache = state

    Event = notification

This distinction is fundamental.

A client must never assume that an event stream alone is sufficient to reconstruct cluster state indefinitely.

For example:

    JobUpdated
        |
        v
    client updates Job/12345

is valid during normal operation.

But if the client misses:

    JobUpdated generation 105

it cannot safely continue assuming its local state is correct.

It must resynchronize.

---

# 6. Event Types

The initial resource event types are:

    Created
    Updated
    Deleted

These correspond to the lifecycle of canonical resources.

Examples:

    NodeCreated
    NodeUpdated
    NodeDeleted

    JobCreated
    JobUpdated
    JobDeleted

    AccountCreated
    AccountUpdated
    AccountDeleted

    UserCreated
    UserUpdated
    UserDeleted

    PartitionCreated
    PartitionUpdated
    PartitionDeleted

The protocol may represent these using:

    event.type = "resource"

and:

    event.action = "created"
    event.action = "updated"
    event.action = "deleted"

rather than creating a separate protocol message type for every resource kind.

This keeps the transport protocol extensible.

---

# 7. Resource Event vs Event Name

Avoid defining dozens of protocol message types such as:

    JobUpdated
    NodeUpdated
    AccountUpdated
    UserUpdated
    ...

Instead use:

    type = "event"

with:

    resource.kind
    resource.id
    action

Example:

    {
      "type": "event",
      "payload": {
        "action": "updated",
        "resource": {
          "kind": "Job",
          "id": "12345"
        }
      }
    }

This allows new resources to be introduced without changing the IPC transport protocol.

---

# 8. Resource Event Envelope

Recommended event structure:

    {
      "protocol": "cockpit-slurm",
      "version": "1.0",
      "messageId": "01MSG...",
      "type": "event",
      "timestamp": "2026-08-17T14:00:00Z",
      "payload": {
        "eventId": "01EVENT...",
        "subscriptionId": "01SUB...",
        "action": "updated",
        "resource": {
          "kind": "Job",
          "id": "12345"
        },
        "generation": 105,
        "previousGeneration": 104,
        "eventSequence": 7821
      }
    }

The outer `messageId` belongs to the IPC protocol.

The `eventId` identifies the application event.

The `subscriptionId` identifies the subscription receiving the event.

The resource generation identifies the version of the resource.

The event sequence identifies ordering within an event stream.

---

# 9. Event Fields

| Field | Type | Required | Description |
|---|---|---:|---|
| `protocol` | string | yes | IPC protocol identifier |
| `version` | string | yes | IPC protocol version |
| `messageId` | string | yes | IPC message identifier |
| `type` | string | yes | Must be `event` |
| `timestamp` | string | yes | Event delivery timestamp |
| `payload.eventId` | string | yes | Unique event identifier |
| `payload.subscriptionId` | string | yes | Target subscription |
| `payload.action` | string | yes | `created`, `updated`, or `deleted` |
| `payload.resource.kind` | string | yes | Canonical resource kind |
| `payload.resource.id` | string | yes | Canonical resource ID |
| `payload.generation` | integer | yes | Current resource generation |
| `payload.previousGeneration` | integer | optional | Previous resource generation |
| `payload.eventSequence` | integer | recommended | Ordered sequence within stream |

---

# 10. Event ID

Every event receives a unique `eventId`.

Example:

    eventId = "01K2EVENT123..."

The event ID is not the same as:

    messageId
    commandId
    subscriptionId
    resource generation

Recommended Go type:

    type EventID string

The Event Bus generates the event ID.

---

# 11. Message ID vs Event ID

The IPC protocol already defines `messageId`.

For example:

    messageId = 01MESSAGE

The Event Bus additionally defines:

    eventId = 01EVENT

The distinction is:

    messageId
        identifies an IPC message

    eventId
        identifies a logical event

An event may be delivered to multiple subscriptions.

Therefore:

    one logical event
        |
        +----> subscription A
        |
        +----> subscription B
        |
        +----> subscription C

The same `eventId` may therefore be associated with multiple deliveries.

---

# 12. Subscription ID

Every event delivered to a client belongs to a subscription.

Example:

    subscriptionId = "01SUB123"

The relationship is:

    connectionId
        |
        +-- subscriptionId
                |
                +-- event
                +-- event
                +-- event

A subscription defines:

- resource kind
- resource filter
- authorization scope
- starting generation
- delivery state

---

# 13. Resource Generation

Every canonical resource has a generation.

Example:

    Job/12345
        generation = 105

After an update:

    Job/12345
        generation = 106

The generation is associated with the canonical resource in the Resource Model.

It is not the same as the global event sequence.

---

# 14. Generation Semantics

A resource generation must monotonically increase for a particular resource.

Example:

    generation 100
        |
        v
    generation 101
        |
        v
    generation 102

The Bridge must not normally produce:

    100 -> 103 -> 102

for the same resource.

Generation comparison is useful for detecting stale updates and missed changes.

---

# 15. Resource Generation vs Event Sequence

These values have different meanings.

Example:

    Job/12345
        generation = 105

    Event stream
        eventSequence = 7821

Another resource may be:

    Node/node001
        generation = 54

    Event stream
        eventSequence = 7822

Therefore:

    resource generation
        = version of one resource

    event sequence
        = ordering of events in a stream

Do not use one in place of the other.

---

# 16. Event Sequence

The Event Bus should maintain an event sequence for each ordered event stream.

A simple initial implementation may use:

    global bridge event sequence

Example:

    eventSequence = 7821
    eventSequence = 7822
    eventSequence = 7823

However, the implementation must explicitly define its scope.

Recommended initial scope:

    Bridge instance

or:

    subscription/event stream

The client must not assume that two independent subscriptions have identical sequence numbers unless the protocol explicitly guarantees this.

---

# 17. Recommended Initial Ordering Guarantee

For version 1.0, guarantee:

> Events for the same subscription are delivered in event-sequence order.

For a single resource:

> Resource generations are strictly increasing.

Do not promise:

> A single globally ordered total event stream for every resource and every client.

That would unnecessarily constrain the implementation.

---

# 18. Event Ordering

For example:

    Job/12345 generation 100
          |
          v
    JobUpdated
          |
          v
    Job/12345 generation 101
          |
          v
    JobUpdated
          |
          v
    Job/12345 generation 102

The client should process these in order.

If it receives:

    generation 102

after:

    generation 100

without seeing:

    generation 101

it should consider whether the event stream has a gap.

---

# 19. Event Payload: Reference vs Full Resource

There are two possible designs.

### Design A

Event contains only a resource reference:

    {
      "kind": "Job",
      "id": "12345",
      "generation": 105
    }

### Design B

Event contains the entire canonical resource.

    {
      "kind": "Job",
      "id": "12345",
      "generation": 105,
      "resource": {...}
    }

For cockpit-slurm, the recommended design is:

> Use resource references by default, with optional resource payloads where useful.

The authoritative state remains in the Cache.

---

# 20. Recommended Updated Event

Example:

    {
      "type": "event",
      "payload": {
        "eventId": "01EVENT001",
        "subscriptionId": "01SUB001",
        "action": "updated",
        "resource": {
          "kind": "Job",
          "id": "12345"
        },
        "generation": 105,
        "previousGeneration": 104
      }
    }

React can then:

    event
       |
       v
    determine resource
       |
       v
    update local state

If full resource data is included, React may update immediately.

---

# 21. Why Not Always Include the Full Resource?

Including the complete resource in every event has several disadvantages:

- larger messages
- more IPC traffic
- duplicated state
- potentially expensive large Job or Node objects
- more complicated event generation
- greater sensitivity to stale event payloads

The Cache already contains the authoritative object.

Therefore references are the safer default.

---

# 22. Optional Resource Payload

An event may optionally contain:

    resourceData

Example:

    {
      "action": "updated",
      "resource": {
        "kind": "Node",
        "id": "node001"
      },
      "generation": 42,
      "resourceData": {
        ...
      }
    }

The payload is an optimization.

It must not change the semantics of the event.

---

# 23. Created Event

Example:

    {
      "type": "event",
      "payload": {
        "eventId": "01EVENT001",
        "subscriptionId": "01SUB001",
        "action": "created",
        "resource": {
          "kind": "Account",
          "id": "research"
        },
        "generation": 1,
        "previousGeneration": null
      }
    }

Meaning:

> A new canonical Account resource now exists in the Cache.

---

# 24. Updated Event

Example:

    {
      "type": "event",
      "payload": {
        "eventId": "01EVENT002",
        "subscriptionId": "01SUB001",
        "action": "updated",
        "resource": {
          "kind": "Account",
          "id": "research"
        },
        "generation": 2,
        "previousGeneration": 1
      }
    }

Meaning:

> Account/research changed from generation 1 to generation 2.

---

# 25. Deleted Event

Example:

    {
      "type": "event",
      "payload": {
        "eventId": "01EVENT003",
        "subscriptionId": "01SUB001",
        "action": "deleted",
        "resource": {
          "kind": "Account",
          "id": "old-account"
        },
        "generation": 7,
        "previousGeneration": 6
      }
    }

For a deleted resource, the final generation identifies the last known version.

The resource itself may no longer be present in the Cache.

---

# 26. Delete Semantics

After:

    ResourceDeleted

the Cache should normally no longer return the resource.

Therefore:

    GET Account/old-account

may return:

    NOT_FOUND

The event informs the client that it should remove the resource from its local state.

---

# 27. No-Op Updates

The Resource Adapter may periodically discover the same resource state.

For example:

    Adapter result
        generation 10

    Cache
        generation 10

If the semantic state has not changed:

> Do not emit an Updated event.

This prevents unnecessary event traffic.

The Cache should compare the canonical representation or an appropriate resource hash/version before generating an event.

---

# 28. Semantic Change

Events should represent meaningful resource changes.

For example:

    Node state:
        IDLE -> ALLOCATED

should produce:

    NodeUpdated

But repeatedly reading:

    state = ALLOCATED

should not produce:

    NodeUpdated

every polling cycle.

This is particularly important for fallback polling adapters.

---

# 29. Event Sources

Events may originate from several mechanisms.

Possible sources include:

    1. Command-triggered refresh
    2. Slurm event/trigger mechanisms
    3. slurmrestd changes
    4. periodic synchronization
    5. polling fallback
    6. future event adapters

Regardless of the source, the event generation path should remain:

    Source
      |
      v
    Resource Adapter
      |
      v
    Canonical Resource
      |
      v
    Cache
      |
      v
    Event Bus

This keeps event semantics independent of the source.

---

# 30. Command-Triggered Events

Suppose:

    React
      |
      v
    CancelJob
      |
      v
    Slurm

After successful execution:

    Slurm
      |
      v
    Resource Adapter
      |
      v
    Cache
      |
      v
    JobUpdated / JobDeleted
      |
      v
    Event Bus

The Command Adapter itself must NOT directly generate:

    JobUpdated

because it does not own canonical resource state.

---

# 31. Command Status vs Resource Event

These are different.

### Command status

Answers:

> What happened to my requested operation?

Example:

    command-status
        Pending
        Running
        Succeeded
        Failed

### Resource event

Answers:

> What changed in cluster state?

Example:

    JobUpdated

These must not be conflated.

---

# 32. Example: Cancel Job

The complete flow is:

    Command
       |
       v
    Command ID = cmd-001
       |
       v
    Slurm scancel
       |
       v
    Command status = Succeeded
       |
       v
    Resource refresh
       |
       v
    Job deleted/updated
       |
       v
    Resource event

The UI may receive:

    command-status:
        succeeded

and independently:

    event:
        JobUpdated

or:

    event:
        JobDeleted

depending on the resulting canonical state.

---

# 33. Command Event Relationship

A resource event may optionally reference the command that caused it.

Example:

    {
      "eventId": "01EVENT001",
      "action": "updated",
      "resource": {
        "kind": "Node",
        "id": "node001"
      },
      "generation": 42,
      "cause": {
        "type": "command",
        "commandId": "01CMD001"
      }
    }

This is useful for audit and UI correlation.

However:

> `commandId` is optional metadata, not the identity of the resource event.

---

# 34. External Changes

Not every resource event is caused by cockpit-slurm.

For example:

    Administrator uses Slurm CLI
          |
          v
    Slurm changes
          |
          v
    Resource Adapter detects change
          |
          v
    Cache update
          |
          v
    JobUpdated

The event may therefore have:

    cause.type = "external"

or simply omit the cause.

This is an important feature of the event-driven architecture.

---

# 35. Event Cause

Recommended optional field:

    cause

with:

    type

Possible values:

    command
    synchronization
    external
    startup
    recovery

Example:

    {
      "cause": {
        "type": "external"
      }
    }

For a command:

    {
      "cause": {
        "type": "command",
        "commandId": "01CMD001"
      }
    }

---

# 36. Event Source

The event may additionally identify the synchronization source.

Example:

    source = "slurmrestd"

or:

    source = "squeue"

or:

    source = "scontrol"

or:

    source = "sacctmgr"

This should normally be internal metadata rather than something the UI needs.

The canonical resource itself already has a `source` field in the v3.5 resource architecture.

---

# 37. Event Metadata

Recommended internal representation:

    type ResourceEvent struct {
        EventID             EventID
        SubscriptionID      SubscriptionID
        Action              EventAction
        Resource            ResourceRef
        Generation          uint64
        PreviousGeneration  *uint64
        EventSequence       uint64
        Timestamp           time.Time
        Cause               *EventCause
    }

---

# 38. Event Action

Recommended Go type:

    type EventAction string

    const (
        EventCreated EventAction = "created"
        EventUpdated EventAction = "updated"
        EventDeleted EventAction = "deleted"
    )

Unknown event actions must be rejected by the protocol decoder.

---

# 39. Event Resource Reference

Recommended:

    type ResourceRef struct {
        Kind string
        ID   string
    }

Examples:

    ResourceRef{
        Kind: "Node",
        ID:   "node001",
    }

    ResourceRef{
        Kind: "Job",
        ID:   "12345",
    }

    ResourceRef{
        Kind: "Account",
        ID:   "research",
    }

---

# 40. Event Bus Responsibility

The Event Bus is responsible for:

- accepting cache-change notifications
- creating logical resource events
- assigning event IDs
- assigning event sequence numbers
- preserving event ordering
- publishing events
- handling event fan-out
- providing events to Subscription Manager

The Event Bus does NOT:

- execute Slurm commands
- query Slurm
- authorize users
- maintain UI state
- own resource state

---

# 41. Event Bus Input

Recommended internal API:

    type ResourceChange struct {
        Action             EventAction
        Resource           ResourceRef
        Generation         uint64
        PreviousGeneration *uint64
        Cause              *EventCause
    }

Then:

    EventBus.Publish(change)

The Event Bus converts this into:

    ResourceEvent

---

# 42. Cache-to-Event Boundary

Recommended:

    Cache.Update(resource)
          |
          v
    Change detected?
       /       \
     no         yes
     |           |
   return        v
             ResourceChange
                   |
                   v
               Event Bus

The Cache should not know about:

    WebSocket
    Cockpit
    React
    Channel

It only exposes a change notification interface.

---

# 43. Event Bus-to-Subscription Boundary

The Event Bus should not need to know the details of each client.

Instead:

    Event Bus
        |
        v
    Subscription Manager
        |
        +-- subscription 1
        +-- subscription 2
        +-- subscription 3

The Subscription Manager applies:

- resource-kind filtering
- resource-ID filtering
- account filtering
- authorization filtering
- connection lifecycle

---

# 44. Subscription Model

A subscription contains:

    subscriptionId
    connectionId
    userContext
    resourceKind
    filter
    generation
    state

Example:

    {
      "subscriptionId": "sub-001",
      "connectionId": "conn-001",
      "resourceKind": "Job",
      "filter": {
        "account": "research"
      },
      "generation": 105
    }

---

# 45. Subscription Lifecycle

The lifecycle is:

    Connect
       |
       v
    UserContext established
       |
       v
    Subscribe
       |
       v
    Authorization
       |
       v
    Snapshot
       |
       v
    Event stream
       |
       v
    Unsubscribe / Disconnect

This is consistent with the v3.5 Bridge architecture.

---

# 46. Snapshot-Then-Events

A subscription should never simply start receiving events without an initial state.

Recommended:

    Subscribe(Job)
        |
        v
    Authorization
        |
        v
    Capture synchronization point
        |
        v
    Snapshot
        |
        v
    Events after synchronization point

The client therefore receives:

    Snapshot
      +
    subsequent events

This provides deterministic initial state.

---

# 47. Snapshot Consistency

The Bridge must ensure that the snapshot and event stream have a known relationship.

Conceptually:

    Snapshot
       |
       +-- snapshotGeneration = 105
       |
       v
    Events
       |
       +-- generation 106
       +-- generation 107
       +-- generation 108

The client can therefore establish:

    local state = snapshot at 105

and then apply:

    106
    107
    108

---

# 48. Subscription Start Generation

A subscription may specify a starting generation.

Example:

    subscribe:
      resource = Job
      generation = 105

Meaning:

> Deliver the state starting from generation 105 and subsequent changes.

The exact semantics should distinguish:

    snapshot generation

from:

    event sequence

because they represent different concepts.

---

# 49. Event Replay

Version 1.0 should NOT require a durable event log.

The Bridge may retain only a short in-memory event history.

For example:

    last 1,000 events

or:

    last 30 seconds / 5 minutes

This can support simple reconnect scenarios.

However:

> The Cache snapshot remains the authoritative recovery mechanism.

---

# 50. Why Not Depend on Event Replay?

A long-lived durable event log adds:

- storage
- retention policy
- replay semantics
- ordering guarantees
- persistence/recovery complexity

The project does not need that for the first implementation.

For a missed event:

    detect gap
       |
       v
    request snapshot
       |
       v
    rebuild local state

This is simpler and safer.

---

# 51. Event Gap Detection

Suppose the client receives:

    eventSequence = 100
    eventSequence = 101
    eventSequence = 103

The missing:

    102

indicates a possible event gap.

The client should transition to:

    RESYNC_REQUIRED

rather than blindly continuing.

---

# 52. Resynchronization

Recommended flow:

    Event gap detected
          |
          v
    Stop applying incremental events
          |
          v
    Request snapshot
          |
          v
    Replace local state
          |
          v
    Resume events

The local React provider should not attempt to guess the missing resource state.

---

# 53. Resource Generation Gap

For one resource:

    generation 20
    generation 22

without generation 21

may also indicate a missing update.

Depending on the event stream semantics, the client may:

    accept the newest state

or:

    request a snapshot

The recommended behavior for the initial implementation is conservative:

> If the client cannot establish that its local state is current, resynchronize.

---

# 54. Deleted Resource and Generation

For deletion:

    generation = final generation

Example:

    Job/12345
        generation 18

deleted:

    JobDeleted
        generation 19

The client removes:

    Job/12345

from local state.

If the resource later reappears:

    JobCreated
        generation 1

it is a new resource lifecycle.

---

# 55. Resource Recreation

A resource may disappear and later reappear.

For example:

    Partition/test
        deleted

then later:

    Partition/test
        created

The new resource may restart its generation sequence.

Therefore:

    generation

is not necessarily globally unique.

The unique identity of a lifecycle event is:

    eventId

The identity of the resource is:

    resource.kind + resource.id

---

# 56. Event Filtering

Subscriptions may filter events.

Examples:

    all Jobs

    Jobs for account "research"

    Node "node001"

    Nodes in partition "gpu"

    Users in account "research"

Filtering should be performed in the Bridge.

The client should not subscribe to all events and then perform security filtering locally.

---

# 57. Filter Example

Subscription:

    {
      "resource": "Job",
      "filter": {
        "account": "research"
      }
    }

Then:

    Job/account=research
        -> deliver

    Job/account=teaching
        -> do not deliver

The Bridge remains responsible for enforcing the filter.

---

# 58. Filter and Authorization

Filtering and authorization are different.

Filter answers:

> What does the client want to receive?

Authorization answers:

> What is the client allowed to receive?

Therefore:

    requested filter
          +
    UserContext
          |
          v
    effective subscription

Example:

    Coordinator:
        research

Requested:

    all jobs

Effective:

    jobs where account = research

---

# 59. Subscription Authorization

Authorization must happen before the subscription is established.

Flow:

    Subscribe
       |
       v
    UserContext
       |
       v
    Capability Check
       |
       v
    Scope Check
       |
       +---- denied
       |
       +---- allowed
                |
                v
             Snapshot
                |
                v
             Events

---

# 60. Event Authorization

Authorization is not a one-time event.

If UserContext changes during a long-lived connection, the effective event scope may change.

For example:

    Coordinator(research)

then:

    Coordinator relationship removed

The Bridge must prevent future unauthorized events from being delivered.

Possible strategies:

1. refresh UserContext periodically
2. invalidate the subscription
3. recalculate authorization when relevant identity changes
4. reconnect the subscription

The initial implementation should support subscription invalidation.

---

# 61. Subscription Invalidation

If authorization is revoked:

    UserContext changed
          |
          v
    Subscription no longer authorized
          |
          v
    Subscription invalidated
          |
          v
    client receives error/event
          |
          v
    client stops receiving resource events

Example:

    {
      "type": "error",
      "payload": {
        "code": "SUBSCRIPTION_REVOKED",
        "subscriptionId": "sub-001"
      }
    }

---

# 62. Event Delivery Failure

If the Channel connection disappears:

    Event Bus
       |
       v
    Subscription Manager
       |
       v
    connection closed
       |
       v
    remove subscriptions

The Event Bus should not retain the subscription indefinitely.

---

# 63. Slow Consumer

A React client may stop reading messages temporarily.

The Bridge must avoid unbounded memory growth.

Recommended:

- bounded per-connection queue
- bounded per-subscription queue
- backpressure
- disconnect slow clients when necessary
- force snapshot resynchronization after overflow

Do not allow one browser tab to consume unlimited Bridge memory.

---

# 64. Event Queue Overflow

If:

    subscription queue full

the Bridge may:

    mark subscription = RESYNC_REQUIRED

and then:

    discard queued incremental events
    |
    v
    send resync notification
    |
    v
    client requests snapshot

This is safer than trying to preserve an unbounded event backlog.

---

# 65. Event Delivery State

Recommended subscription states:

    Active
    ResyncRequired
    Closing
    Closed

Example:

    Active
      |
      | event queue overflow
      v
    ResyncRequired
      |
      | snapshot
      v
    Active

---

# 66. Event Coalescing

The Bridge may optionally coalesce events.

Example:

    JobUpdated generation 101
    JobUpdated generation 102
    JobUpdated generation 103

could potentially become:

    JobUpdated generation 103

if the client has not yet received any of them.

However, coalescing must not violate:

- resource generation ordering
- delete semantics
- subscription semantics

For version 1.0:

> Do not implement aggressive event coalescing initially.

Correctness is more important than optimization.

---

# 67. Event Batching

The Bridge may later support:

    {
      "type": "event-batch",
      "events": [...]
    }

This is useful when many resources change simultaneously.

For example:

    1,000 nodes update

A batch reduces IPC overhead.

However, the initial protocol should support individual events first.

Batching can be introduced as a backward-compatible protocol extension.

---

# 68. Burst Handling

HPC clusters may produce large bursts of events.

Examples:

    job start storm
    job completion storm
    node state change
    partition update
    accounting update

The Event Bus should therefore be designed to handle:

    high event rate

without blocking:

    Command Service
    Query Service
    IPC connections

The Event Bus should be asynchronous.

---

# 69. Event Bus Isolation

Recommended architecture:

    Cache
       |
       v
    Event Bus
       |
       +----> Subscription Manager
       |
       +----> Audit / Metrics
       |
       +----> future consumers

The Event Bus should not call:

    Slurm CLI
    slurmrestd
    Command Adapter

This maintains one-way data flow.

---

# 70. Event Processing Model

Recommended:

    Cache mutation
          |
          v
    publish ResourceChange
          |
          v
    Event Bus
          |
          v
    Subscription Manager
          |
          v
    connection queues
          |
          v
    IPC writer

Each connection should have an independent output path.

One slow client must not block every other client.

---

# 71. Connection Event Queue

Recommended internal structure:

    type ConnectionState struct {
        ID            ConnectionID
        UserContext   UserContext
        Subscriptions map[SubscriptionID]*Subscription
        OutboundQueue chan IPCMessage
    }

The Event Bus should enqueue delivery messages rather than writing directly to the socket.

The IPC layer owns socket I/O.

---

# 72. Separation of Event Generation and Delivery

This distinction is important.

### Event generation

Owned by:

    Event Bus

### Event selection

Owned by:

    Subscription Manager

### Event transport

Owned by:

    IPC Server / Channel

### Event consumption

Owned by:

    React

Therefore:

    Cache
      |
      v
    Event Bus
      |
      v
    Subscription Manager
      |
      v
    IPC Server
      |
      v
    Channel
      |
      v
    React

---

# 73. Event Bus Interface

Recommended Go interface:

    type EventBus interface {
        Publish(ctx context.Context, change ResourceChange) error
        Subscribe(handler EventHandler) SubscriptionHandle
        Close() error
    }

The exact API may be refined during implementation.

---

# 74. Event Handler

Conceptually:

    type EventHandler func(
        ctx context.Context,
        event ResourceEvent,
    )

The Subscription Manager can register as an Event Bus consumer.

---

# 75. Resource Change

Recommended internal model:

    type ResourceChange struct {
        Action             EventAction
        Resource           ResourceRef
        Generation         uint64
        PreviousGeneration *uint64
        Cause              *EventCause
        Timestamp          time.Time
    }

This is an internal domain event.

It does not need to be identical to the IPC message.

---

# 76. Domain Event vs IPC Event

This distinction should be explicit.

### Domain event

Used inside Bridge:

    ResourceChange
    ResourceEvent

### IPC event

Used across Channel ↔ Bridge:

    IPC envelope
        type = event
        payload = serialized ResourceEvent

This prevents the transport layer from leaking into the domain layer.

---

# 77. React Event Model

Each React Context Provider should normally own one subscription.

Example:

    JobProvider
        |
        +-- Subscribe(Job)
        |
        +-- Snapshot
        +-- Event stream

    NodeProvider
        |
        +-- Subscribe(Node)
        |
        +-- Snapshot
        +-- Event stream

    AccountProvider
        |
        +-- Subscribe(Account)
        |
        +-- Snapshot
        +-- Event stream

This matches the existing Bridge/UI architecture.

---

# 78. React Reducer

A provider can process events using a reducer.

Conceptually:

    Snapshot
       |
       v
    initial state
       |
       +-- Created
       +-- Updated
       +-- Deleted
       |
       v
    current state

Example:

    function resourceReducer(
        state,
        event,
    ) {
        switch (event.action) {
            case "created":
                return addResource(state, event);

            case "updated":
                return updateResource(state, event);

            case "deleted":
                return deleteResource(state, event);

            default:
                return state;
        }
    }

---

# 79. Snapshot Handling

When a subscription starts:

    Snapshot
       |
       v
    replace local provider state

Do not append snapshot resources to an existing state unless the protocol explicitly defines incremental snapshots.

A normal snapshot is authoritative for its scope.

---

# 80. Event Handling

After snapshot:

    Event
       |
       v
    validate subscription
       |
       v
    validate sequence/generation
       |
       v
    apply reducer

If validation fails:

    RESYNC_REQUIRED

---

# 81. React Resynchronization

React should have a recovery path:

    Event gap
       |
       v
    mark provider stale
       |
       v
    request snapshot
       |
       v
    replace state
       |
       v
    resume event processing

The user should ideally see:

    "Refreshing cluster state..."

rather than stale or inconsistent data.

---

# 82. Initial Subscription Protocol

Recommended message sequence:

    Client                         Bridge
      |                              |
      |---- subscribe -------------->|
      |                              |
      |<--- subscribe-response ------|
      |                              |
      |<--- snapshot ----------------|
      |                              |
      |<--- event ------------------|
      |<--- event ------------------|
      |<--- event ------------------|

The `subscribe-response` confirms creation of the subscription.

The snapshot establishes initial state.

The event stream maintains state.

---

# 83. Subscribe Request

Example:

    {
      "protocol": "cockpit-slurm",
      "version": "1.0",
      "messageId": "01SUBMSG001",
      "type": "subscribe",
      "payload": {
        "resource": {
          "kind": "Job"
        },
        "filter": {
          "account": "research"
        }
      }
    }

---

# 84. Subscribe Response

Example:

    {
      "protocol": "cockpit-slurm",
      "version": "1.0",
      "messageId": "01SUBRESP001",
      "type": "subscribe-response",
      "payload": {
        "subscriptionId": "01SUB001",
        "resource": {
          "kind": "Job"
        },
        "snapshotGeneration": 105
      }
    }

---

# 85. Snapshot Message

Recommended:

    {
      "protocol": "cockpit-slurm",
      "version": "1.0",
      "messageId": "01SNAP001",
      "type": "snapshot",
      "payload": {
        "subscriptionId": "01SUB001",
        "resource": {
          "kind": "Job"
        },
        "generation": 105,
        "items": [
          {
            ...
          }
        ]
      }
    }

The snapshot is logically associated with the subscription.

---

# 86. Snapshot Generation

The snapshot should contain the generation or synchronization marker from which subsequent events are interpreted.

For example:

    snapshotGeneration = 105

then:

    JobUpdated generation 106
    JobUpdated generation 107

The client knows that it has moved from:

    105 -> 106 -> 107

---

# 87. Snapshot and Concurrent Changes

A race condition must be avoided.

Bad implementation:

    1. read cache
    2. cache changes
    3. register subscription
    4. send snapshot

This may lose the event from step 2.

Recommended conceptual operation:

    1. establish subscription/synchronization point
    2. capture snapshot
    3. register event delivery from synchronization point
    4. send snapshot
    5. send subsequent events

The exact implementation may use a cache generation/sequence barrier.

---

# 88. Snapshot Barrier

A useful implementation concept is:

    subscription barrier

Example:

    current event sequence = 7821

Subscribe establishes:

    barrier = 7821

Snapshot represents:

    state at barrier

Events delivered afterward begin with:

    7822

This provides a deterministic boundary.

---

# 89. Event Stream Semantics

For a subscription:

    snapshot @ sequence 7821

the client expects:

    7822
    7823
    7824
    ...

If it receives:

    7822
    7824

it knows:

    7823

may have been lost.

The client should resynchronize.

---

# 90. Event Replay on Reconnect

A reconnecting client may optionally provide:

    lastEventSequence

Example:

    {
      "lastEventSequence": 7824
    }

The Bridge may:

    replay from 7825

if the event history is still available.

Otherwise:

    return snapshot

The client must support both.

---

# 91. Recommended Reconnect Strategy

Version 1.0:

    reconnect
       |
       v
    subscribe
       |
       v
    Bridge checks replay availability
       |
       +---- available --> replay
       |
       +---- unavailable -> snapshot

This provides optimization without making replay mandatory.

---

# 92. Event Retention

Initial implementation may use an in-memory bounded ring buffer.

Example:

    max events = 10,000

or:

    retention = 5 minutes

The exact value should be configurable.

The event history is not authoritative storage.

It is only an optimization for reconnect.

---

# 93. Event Persistence

Do not persist all resource events in the first implementation.

Persistent audit information belongs to:

    Audit Log

not:

    Event Bus

If future requirements require durable event history, it can be added independently.

---

# 94. Audit vs Event History

Audit log:

    who
    did what
    when
    result

Event history:

    resource changed
    generation
    event sequence

They serve different purposes.

A resource event should not be considered a complete audit record.

---

# 95. Event Security

Events may contain sensitive resource information.

Therefore:

    Event Bus
        |
        v
    Subscription Manager
        |
        v
    UserContext authorization
        |
        v
    filtered event

Never:

    Event Bus
        |
        v
    broadcast all events
        |
        v
    frontend filters

---

# 96. UserContext in Event Delivery

The Subscription Manager should retain:

    UserContext

with every subscription.

Example:

    Subscription
       |
       +-- connectionId
       +-- userContext
       +-- resourceKind
       +-- filter

This allows event delivery to respect:

    account scope
    capabilities
    resource visibility

---

# 97. Event Visibility Changes

Suppose:

    Coordinator(research)

and:

    Job/12345
        account = research

The client receives:

    JobUpdated

If the job changes to:

    account = teaching

the job may become invisible to the subscriber.

The resulting event behavior must account for visibility changes.

---

# 98. Filter Transition

For a filtered subscription, an update can cause a resource to:

    enter the filter
    remain in the filter
    leave the filter

Therefore an event may need to be interpreted as:

    Created from the subscription's perspective

even if the canonical resource itself was merely Updated.

Example:

    Job/12345
        account = teaching

changes to:

    account = research

For a subscription:

    account = research

the client should receive:

    Created

because the resource has entered the visible set.

---

# 99. Leaving a Filter

Conversely:

    Job/12345
        account = research

changes to:

    account = teaching

For:

    account = research

the client should receive:

    Deleted

from the subscription's perspective.

This is an important distinction:

    canonical lifecycle event

versus:

    subscription-visible lifecycle event

---

# 100. Effective Event Action

The Event Bus may generate:

    Updated

while the Subscription Manager transforms it into:

    Created
    Updated
    Deleted

depending on whether the resource enters or leaves the subscriber's effective view.

Example:

    Canonical:
        JobUpdated

    Subscriber A:
        Updated

    Subscriber B:
        Created

    Subscriber C:
        Deleted

This is valid when filters/scopes differ.

---

# 101. Recommended Event Layers

This suggests three concepts:

    1. Resource Change
    2. Domain Event
    3. Delivery Event

### Resource Change

Produced by Cache:

    "Job/12345 changed"

### Domain Event

Produced by Event Bus:

    JobUpdated generation 105

### Delivery Event

Produced for a subscription:

    JobCreated / JobUpdated / JobDeleted
    according to effective visibility

This separation provides a clean architecture.

---

# 102. Event Model

Recommended conceptual model:

    ResourceChange
        |
        v
    ResourceEvent
        |
        v
    SubscriptionEvent
        |
        v
    IPC Event

This should not necessarily mean four separate Go structs in the first implementation, but the semantic distinction should remain clear.

---

# 103. Subscription Event

Recommended:

    type SubscriptionEvent struct {
        EventID          EventID
        SubscriptionID   SubscriptionID
        Action           EventAction
        Resource         ResourceRef
        Generation       uint64
        EventSequence    uint64
        Timestamp        time.Time
        ResourceData     any
    }

The `Action` here is the action visible to the subscriber.

---

# 104. Event Delivery Semantics

For every subscription:

    exactly ordered delivery

should be attempted.

However:

    exactly-once delivery

should NOT be promised.

IPC disconnects, reconnects and process failures can cause duplicate delivery after replay/recovery.

Therefore clients should be designed to tolerate duplicates.

---

# 105. Idempotent Event Processing

React reducers should be safe against duplicate events.

For example:

    JobUpdated generation 105

received twice should not corrupt state.

The client can compare:

    incoming generation
    current generation

and ignore stale duplicates.

---

# 106. Stale Event

If:

    current generation = 105

and incoming:

    generation = 104

the client should normally ignore the event.

If:

    generation = 106

apply it.

If:

    generation = 108

and 107 is unexpectedly missing:

    resync

This gives a robust client state machine.

---

# 107. Recommended Client Event Algorithm

Conceptually:

    receive event
       |
       v
    validate subscription
       |
       v
    validate event sequence
       |
       +---- duplicate/stale --> ignore
       |
       +---- gap -------------> resync
       |
       v
    validate resource generation
       |
       +---- stale -----------> ignore
       |
       +---- gap -------------> resync
       |
       v
    apply event
       |
       v
    update local state

---

# 108. Event Sequence Scope

The protocol should explicitly document the scope of `eventSequence`.

Recommended version 1.0:

    eventSequence is monotonically increasing within
    the Bridge event stream.

If implementation complexity makes a global sequence undesirable, use:

    subscription-local sequence

but document this clearly.

The client must never compare sequence values from unrelated streams unless the protocol guarantees that they share the same sequence domain.

---

# 109. Bridge Restart

If the persistent Bridge restarts:

    old event sequence
        |
        X

new Bridge instance:

    event sequence starts again

Clients must not assume event sequences survive a Bridge restart.

The Bridge should therefore expose a:

    bridgeInstanceId

or:

    eventStreamId

in the handshake.

---

# 110. Bridge Instance ID

Recommended:

    bridgeInstanceId

Example:

    {
      "server": "cockpit-slurm-bridge",
      "bridgeInstanceId": "01BRIDGE001"
    }

The relationship becomes:

    bridgeInstanceId
        +
    eventSequence

This prevents clients from treating:

    sequence 100

from an old Bridge instance as equivalent to:

    sequence 100

from a new instance.

---

# 111. Resynchronization After Bridge Restart

If:

    bridgeInstanceId changes

the client should assume:

    event history unavailable

and perform:

    snapshot

rather than attempting replay.

---

# 112. Event Bus Startup

At Bridge startup:

    initialize Cache
        |
        v
    synchronize resources
        |
        v
    populate Cache
        |
        v
    establish Event Bus
        |
        v
    accept subscriptions

The Bridge should avoid exposing an apparently valid empty cache as if it represented the actual cluster.

---

# 113. Cache Warm-Up

Recommended state:

    CacheState:
        Initializing
        Ready
        Degraded

Subscriptions should normally wait until:

    Ready

or explicitly receive:

    cacheReady = false

depending on the implementation.

---

# 114. Event During Cache Warm-Up

The initial synchronization may produce many changes.

These should generally not be exposed as:

    NodeCreated
    JobCreated
    ...

to clients that have not yet subscribed.

Instead:

    Cache initialization
        |
        v
    snapshot

becomes the initial state.

Events are for changes after the synchronization point.

---

# 115. Periodic Synchronization

Suppose the Resource Adapter performs:

    refresh every 10 seconds

If it detects:

    NodeUpdated

the flow is:

    refresh
      |
      v
    compare
      |
      v
    cache mutation
      |
      v
    event

If nothing changed:

    no event

---

# 116. Event Storm Prevention

The Resource Adapter should avoid generating unnecessary changes.

The Event Bus should also avoid:

    duplicate events

for the same unchanged resource.

The Cache is the first line of defense.

Optional future optimization:

    debounce/coalesce

at the Event Bus or Subscription Manager.

---

# 117. Event Metrics

The Bridge should expose internal metrics such as:

    events_generated_total
    events_delivered_total
    events_dropped_total
    event_queue_size
    subscription_count
    resync_count
    event_gap_count
    event_replay_count

These are useful for production diagnostics.

---

# 118. Logging

Event logging should be structured.

Example:

    event_id=01EVENT001
    resource_kind=Job
    resource_id=12345
    action=updated
    generation=105
    sequence=7821

Do not log the entire resource by default.

Large Job or Node objects can create excessive logs.

---

# 119. Error Events

Protocol errors are not resource events.

For example:

    FORBIDDEN
    INVALID_REQUEST
    SUBSCRIPTION_NOT_FOUND

are protocol/application errors.

They use:

    type = "error"

rather than:

    type = "event"

---

# 120. Subscription Error

Example:

    {
      "type": "error",
      "payload": {
        "code": "SUBSCRIPTION_REVOKED",
        "subscriptionId": "01SUB001",
        "message": "Subscription is no longer authorized."
      }
    }

This is not a ResourceEvent.

---

# 121. Event Versioning

The Event Model should evolve independently of the transport protocol where possible.

For example:

    IPC protocol = 1.0

may support:

    event payload v1

A future compatible event extension could add:

    cause
    resourceData
    event metadata

without requiring:

    IPC protocol 2.0

provided existing clients can safely ignore unknown fields.

---

# 122. Unknown Fields

Clients should ignore unknown optional fields.

For example:

    {
      "eventId": "...",
      "action": "updated",
      "resource": {...},
      "generation": 105,
      "cause": {...}
    }

A v1.0 client that does not understand `cause` should still process the event.

---

# 123. Unknown Event Action

Unknown required event actions should not be silently interpreted.

For example:

    action = "reordered"

if not supported should result in:

    protocol error

or:

    resync

rather than treating it as:

    updated

---

# 124. Event Schema Example

Complete example:

    {
      "protocol": "cockpit-slurm",
      "version": "1.0",
      "messageId": "01MSG001",
      "type": "event",
      "timestamp": "2026-08-17T14:00:00Z",
      "payload": {
        "eventId": "01EVENT001",
        "subscriptionId": "01SUB001",
        "action": "updated",
        "resource": {
          "kind": "Job",
          "id": "12345"
        },
        "generation": 105,
        "previousGeneration": 104,
        "eventSequence": 7821,
        "cause": {
          "type": "command",
          "commandId": "01CMD001"
        }
      }
    }

---

# 125. Example: Node State Change

Initial:

    Node/node001
        state = IDLE
        generation = 10

Slurm changes:

    state = ALLOCATED

Resource Adapter produces:

    Node/node001
        generation = 11

Cache updates.

Event:

    NodeUpdated
        generation = 11

React:

    NodeProvider
        |
        v
    update node001
        |
        v
    PatternFly table/chart updates

---

# 126. Example: Job Completion

Initial:

    Job/12345
        state = RUNNING
        generation = 20

Slurm:

    job completed

Resource Adapter:

    Job/12345
        state = COMPLETED
        generation = 21

Event:

    JobUpdated
        generation = 21

If the Job is then removed from the active-job cache:

    JobDeleted
        generation = 22

The exact lifecycle depends on the resource semantics.

---

# 127. Active vs Historical Resources

The event model must respect resource semantics.

For example:

    Job

may represent:

    active job state

while:

    JobHistory

may represent:

    completed accounting records

A completed job disappearing from an active-job collection does not necessarily mean the underlying historical job has ceased to exist in Slurm accounting.

The Resource Model must define which collection a subscription represents.

---

# 128. Event Semantics Are Resource-Specific

The Event Model defines generic mechanics.

The Resource Model defines resource-specific semantics.

For example:

    Job
    Node
    Account
    User
    Partition

may have different lifecycle rules.

Therefore:

    event-model.md
        = generic event mechanics

    resource-model.md
        = resource identity/state semantics

---

# 129. Event and Canonical Resource Relationship

Every resource event must reference a canonical resource.

Example:

    Resource:
        kind = Job
        id = 12345

    Event:
        kind = Job
        id = 12345

The event must not reference raw Slurm CLI objects such as:

    squeue output row
    sacctmgr output line

The Resource Adapter converts those into canonical resources before events are generated.

---

# 130. Event and Generation Relationship

Recommended invariant:

    Event.generation
        == Cache resource generation
        at the time the event was generated

This makes it possible to compare:

    local generation
    event generation
    cache generation

---

# 131. Event Invariants

The implementation must maintain:

### Invariant 1

No event is generated for an unsuccessful Cache update.

### Invariant 2

Every resource event references a canonical resource.

### Invariant 3

Event generation corresponds to the resource generation.

### Invariant 4

Resource generations are monotonic.

### Invariant 5

Events are ordered within their defined stream.

### Invariant 6

Subscription filters are enforced by the Bridge.

### Invariant 7

UserContext authorization is enforced before event delivery.

### Invariant 8

Events do not become the source of truth.

### Invariant 9

A client can recover from a missed event through a snapshot.

### Invariant 10

Duplicate events do not corrupt client state.

---

# 132. Go Package Structure

Recommended Bridge package structure:

    internal/
    ├── auth/
    ├── resource/
    ├── cache/
    ├── event/
    │   ├── event.go
    │   ├── action.go
    │   ├── bus.go
    │   ├── sequence.go
    │   └── history.go
    │
    ├── subscription/
    │   ├── subscription.go
    │   ├── manager.go
    │   ├── filter.go
    │   └── delivery.go
    │
    ├── query/
    ├── command/
    ├── ipc/
    └── adapter/

---

# 133. Event Package

Recommended types:

    type EventID string
    type EventSequence uint64

    type EventAction string

    const (
        Created EventAction = "created"
        Updated EventAction = "updated"
        Deleted EventAction = "deleted"
    )

    type ResourceChange struct {
        Action             EventAction
        Resource           ResourceRef
        Generation         uint64
        PreviousGeneration *uint64
        Cause              *EventCause
    }

    type ResourceEvent struct {
        EventID            EventID
        Resource           ResourceRef
        Action             EventAction
        Generation         uint64
        PreviousGeneration *uint64
        Sequence           EventSequence
        Timestamp          time.Time
        Cause              *EventCause
    }

---

# 134. Event Cause

Recommended:

    type EventCause struct {
        Type      string
        CommandID *CommandID
    }

Possible values:

    command
    synchronization
    external
    startup
    recovery

---

# 135. Event Bus Interface

Initial implementation:

    type Bus interface {
        Publish(ctx context.Context, change ResourceChange) ResourceEvent
        Subscribe(handler Handler) Subscription
        Close() error
    }

The exact interface may change during implementation.

The important architectural requirement is that Event Bus does not depend on IPC.

---

# 136. Subscription Manager Interface

Conceptually:

    type Manager interface {
        Subscribe(
            ctx context.Context,
            user UserContext,
            request SubscribeRequest,
        ) (Subscription, Snapshot, error)

        Unsubscribe(
            ctx context.Context,
            subscriptionID SubscriptionID,
        ) error

        Publish(
            event ResourceEvent,
        )
    }

The manager performs:

    authorization
    filtering
    delivery

---

# 137. Cache Interface

The Cache should expose a change-aware update:

    type Cache interface {
        Get(ref ResourceRef) (Resource, bool)

        List(
            kind string,
            filter ResourceFilter,
        ) ([]Resource, SnapshotInfo, error)

        Upsert(
            resource Resource,
        ) (ChangeResult, error)

        Delete(
            ref ResourceRef,
        ) (ChangeResult, error)
    }

The Event Bus should receive the resulting change.

---

# 138. Recommended Cache Update Flow

For Upsert:

    resource
       |
       v
    Cache.Upsert()
       |
       +---- no semantic change
       |        |
       |        v
       |       no event
       |
       +---- created
       |        |
       |        v
       |      Created
       |
       +---- updated
                |
                v
              Updated

For Delete:

    Cache.Delete()
       |
       +---- not found
       |        |
       |        v
       |       no event
       |
       +---- deleted
                |
                v
              Deleted

---

# 139. Atomicity

The Cache update and event generation must be designed carefully.

The desired semantic guarantee is:

    resource state becomes authoritative
        before
    corresponding event becomes observable

A practical implementation may:

    lock cache
       |
       v
    mutate resource
       |
       v
    assign generation
       |
       v
    create ResourceChange
       |
       v
    unlock
       |
       v
    publish event

The exact locking strategy may differ.

---

# 140. Concurrency

Multiple Resource Adapters may update different resource kinds concurrently.

Example:

    Node Adapter
         |
         +---- Cache

    Job Adapter
         |
         +---- Cache

    Account Adapter
         |
         +---- Cache

The Cache must serialize generation changes per resource.

The Event Bus must preserve the documented event ordering.

Global locking should not unnecessarily serialize unrelated resources.

---

# 141. Per-Resource Ordering

For the same resource:

    Job/12345

events must be ordered.

For unrelated resources:

    Job/12345
    Node/node001

the implementation may process events concurrently.

This gives a good balance between correctness and scalability.

---

# 142. Multi-Session Example

Suppose:

    Browser A
        alice
        subscription A

    Browser B
        bob
        subscription B

Slurm changes:

    Job/12345

Event Bus produces one logical event:

    JobUpdated

Subscription Manager evaluates:

    A:
        authorized?
        filter?
        deliver?

    B:
        authorized?
        filter?
        deliver?

The Event Bus does not need to create two different domain events.

---

# 143. Event Delivery Example

    Event Bus
       |
       | JobUpdated
       v
    Subscription Manager
       |
       +---- Alice
       |       |
       |       v
       |    deliver
       |
       +---- Bob
               |
               v
            filtered

This is why subscriptions must be evaluated independently.

---

# 144. Event Model and IPC Protocol

`ipc-protocol.md` defines:

    framing
    message envelope
    message types
    message IDs
    connection IDs
    subscribe/unsubscribe
    event transport

This document defines:

    what an event means
    resource lifecycle
    generation
    sequence
    snapshot relationship
    filtering
    replay
    recovery

Therefore:

    ipc-protocol.md
        = transport contract

    event-model.md
        = event semantics

---

# 145. Event Model and UserContext

`user-context.md` defines:

    who is connected
    AdminLevel
    Coordinator
    capabilities
    resource scope

`event-model.md` defines:

    which events that user may receive

Relationship:

    UserContext
        +
    Subscription
        +
    Resource Event
        |
        v
    Delivery Event

---

# 146. Event Model and Resource Model

`resource-model.md` defines:

    Resource
    Metadata
    Spec
    Status
    Generation
    ResourceRef

`event-model.md` defines:

    what happens when those resources change

Relationship:

    Resource
       |
       | change
       v
    Generation++
       |
       v
    Event

---

# 147. Event Model and Command Model

Commands cause operations.

Events represent resulting state changes.

Therefore:

    Command
       |
       v
    Slurm operation
       |
       v
    Resource refresh
       |
       v
    Cache update
       |
       v
    Event

This prevents the Command Service from becoming a second source of resource state.

---

# 148. Event Model and React

React should implement:

    Snapshot
       +
    Event reducer

not:

    direct Slurm polling

The v3.5 UI architecture already recommends Context Providers that own subscriptions and consume snapshots followed by events.

---

# 149. First Implementation Scope

Version 1.0 should implement only:

    Created
    Updated
    Deleted

plus:

    eventId
    subscriptionId
    resource kind
    resource ID
    generation
    event sequence
    timestamp

Also implement:

    snapshot
    subscription
    event gap detection
    resync

Do not initially implement:

    durable event log
    complex replay
    event batching
    aggressive coalescing
    distributed event bus

---

# 150. Development Sequence

Recommended implementation order:

## Phase 1 — Event Domain Model

Implement:

    EventID
    EventAction
    ResourceChange
    ResourceEvent
    EventCause

---

## Phase 2 — Cache Integration

Implement:

    Cache mutation
        |
        v
    Change detection
        |
        v
    generation assignment

---

## Phase 3 — Event Bus

Implement:

    Publish
    sequence generation
    event fan-out

---

## Phase 4 — Subscription Manager

Implement:

    Subscribe
    Unsubscribe
    filtering
    authorization
    connection ownership

---

## Phase 5 — Snapshot

Implement:

    subscription barrier
    snapshot generation
    snapshot delivery

---

## Phase 6 — IPC

Implement:

    event serialization
    event message
    snapshot message
    subscription response

---

## Phase 7 — React

Implement:

    UserProvider
    NodeProvider
    JobProvider
    AccountProvider

Each provider should use:

    subscribe
       |
       v
    snapshot
       |
       v
    events

---

## Phase 8 — Recovery

Implement:

    event sequence validation
    duplicate handling
    gap detection
    snapshot resync

---

## Phase 9 — Performance

Implement:

    bounded queues
    metrics
    optional event coalescing
    optional replay

---

# 151. First Vertical Slice

The first event-driven vertical slice should be:

    Node

Flow:

    Slurm
      |
      v
    Node Resource Adapter
      |
      v
    Canonical Node
      |
      v
    Cache
      |
      v
    NodeUpdated
      |
      v
    Event Bus
      |
      v
    Subscription Manager
      |
      v
    IPC
      |
      v
    NodeProvider
      |
      v
    SinfoCard / Node UI

This should be completed before generalizing the implementation.

---

# 152. Second Vertical Slice

The second slice should be:

    Job

because it exercises:

    frequent updates
    filters
    resource generation
    command-triggered changes
    command status + resource event

Example:

    CancelJob
       |
       v
    Command Service
       |
       v
    Slurm
       |
       v
    Job refresh
       |
       v
    Cache
       |
       v
    JobUpdated / JobDeleted
       |
       v
    JobProvider

---

# 153. Third Vertical Slice

The third slice should be:

    Account

because it introduces:

    account-scoped Coordinator
    authorization filtering
    lower-frequency resource changes

Example:

    AccountUpdated
       |
       v
    Subscription Manager
       |
       v
    Coordinator
       |
       v
    account-scoped event

---

# 154. Fourth Vertical Slice

The fourth slice:

    User

should exercise:

    UserContext
    AdminLevel
    Coordinator
    account associations
    capability changes

This also provides the foundation for the role-based administration UI.

---

# 155. Testing Strategy

The Event Model requires tests at several levels.

## Unit Tests

Test:

    event ID generation
    sequence generation
    generation validation
    Created/Updated/Deleted
    duplicate detection
    stale events
    filters

## Cache Tests

Test:

    no-op update
    create
    update
    delete
    generation increments

## Subscription Tests

Test:

    subscribe
    snapshot
    event delivery
    filter
    authorization
    unsubscribe
    disconnect

## Integration Tests

Test:

    Resource Adapter
        ->
    Cache
        ->
    Event Bus
        ->
    Subscription Manager
        ->
    IPC

## React Tests

Test:

    snapshot
    event reducer
    duplicate event
    event gap
    resync

---

# 156. Event Test Example

Given:

    Job/12345
        generation = 10

Update:

    state = RUNNING
        generation = 11

Expect:

    action = updated
    resource.kind = Job
    resource.id = 12345
    generation = 11
    previousGeneration = 10

---

# 157. No-Op Test

Given:

    Node/node001
        generation = 20
        state = IDLE

Adapter returns:

    state = IDLE

Expect:

    generation remains 20
    no event

---

# 158. Filter Test

Subscription:

    account = research

Resource:

    Job/12345
        account = teaching

Expect:

    no event

Change:

    account = research

Expect subscriber-visible:

    created

---

# 159. Authorization Test

User:

    AdminLevel = none
    Coordinator = research

Subscription:

    all accounts

Expected effective scope:

    research only

The user must not receive:

    Account/teaching
    Account/finance

events unless separately authorized.

---

# 160. Event Gap Test

Client receives:

    sequence 100
    sequence 101
    sequence 103

Expected:

    RESYNC_REQUIRED

Then:

    snapshot

Then:

    resume events

---

# 161. Duplicate Test

Client receives:

    JobUpdated generation 105

twice.

Expected:

    first event applied
    second event ignored

No duplicate row should appear in the UI.

---

# 162. Stale Event Test

Current:

    generation = 105

Incoming:

    generation = 104

Expected:

    ignore

---

# 163. Future Event Test

Current:

    generation = 105

Incoming:

    generation = 107

Expected:

    resync

unless the subscription semantics explicitly allow skipped intermediate generations.

---

# 164. Operational Observability

Recommended metrics:

    cockpit_slurm_event_generated_total
    cockpit_slurm_event_delivered_total
    cockpit_slurm_event_dropped_total
    cockpit_slurm_event_gap_total
    cockpit_slurm_event_resync_total
    cockpit_slurm_subscription_active
    cockpit_slurm_subscription_revoked_total
    cockpit_slurm_event_queue_size

These metrics will become particularly useful for large HPC clusters.

---

# 165. Performance Goals

The event system should:

- avoid polling from React
- avoid unnecessary resource events
- avoid global locks where possible
- avoid unbounded queues
- avoid copying large resources unnecessarily
- avoid blocking Cache updates on slow clients
- allow multiple clients to receive the same event independently

---

# 166. Security Requirements

The event system must enforce:

    authenticated connection
        |
        v
    UserContext
        |
        v
    authorized subscription
        |
        v
    filtered event delivery

Never expose:

    all cluster events

to:

    all authenticated Cockpit users.

---

# 167. Failure Recovery

The event architecture must tolerate:

    Channel crash
    Browser refresh
    Browser disconnect
    Bridge restart
    event queue overflow
    temporary Slurm failure
    Resource Adapter failure

The common recovery mechanism is:

    establish subscription
        |
        v
    obtain authoritative snapshot
        |
        v
    continue event stream

---

# 168. Bridge Restart Recovery

After Bridge restart:

    Cache
       |
       v
    reinitialize
       |
       v
    synchronize Slurm
       |
       v
    Ready
       |
       v
    accept subscriptions

Existing clients must reconnect.

They should not attempt to continue using old event sequences.

---

# 169. Slurm Failure

If Slurm temporarily becomes unavailable:

    Resource Adapter
        |
        X
      Slurm

The Bridge should not generate false Deleted events simply because a query failed.

This is critical.

For example:

    squeue failed

does NOT mean:

    all Jobs deleted

The adapter should distinguish:

    empty result

from:

    query failure

---

# 170. Event Safety During Adapter Failure

If a Resource Adapter cannot establish authoritative state:

    do not mutate Cache

therefore:

    do not generate events

The Cache retains its last known authoritative state and may be marked:

    stale/degraded

---

# 171. Degraded State

The Bridge may expose:

    CacheStatus = degraded

during synchronization failures.

The UI can display:

    "Cluster state may be stale."

This is preferable to generating incorrect resource events.

---

# 172. Event Source Reliability

Different adapters may have different reliability.

For example:

    slurmrestd
    squeue
    scontrol
    sacctmgr

The Event Model remains independent of the source.

The Resource Adapter is responsible for deciding whether the retrieved information is authoritative enough to update the Cache.

---

# 173. Architectural Summary

The final architecture is:

                    Slurm
                      |
                      v
              Resource Adapter
                      |
                      v
              Canonical Resource
                      |
                      v
                Resource Cache
                      |
                successful change
                      |
                      v
                  Event Bus
                      |
                      v
             Subscription Manager
                      |
             UserContext + Filter
                      |
                      v
               Subscription Event
                      |
                      v
                 IPC Server
                      |
                      v
          cockpit-slurm-channel
                      |
                      v
                  React
                      |
             Snapshot + Events
                      |
                      v
              Local UI State

The authoritative relationship is:
```text
    Cache
      |
      +---- state
      |
      +---- generation
      |
      v
    Event
      |
      +---- notification
      |
      v
    Subscription
      |
      +---- delivery
      |
      v
    React
      |
      +---- presentation
```

# 174. Final Design Decisions

The following decisions are normative for Event Model v1.0.

1. **The Resource Cache is the source of truth.**
2. **Events are generated only after successful Cache changes.**
3. **The Event Bus is independent of IPC.**
4. **The Subscription Manager owns client-specific event delivery.**
5. **Events use `created`, `updated`, and `deleted`.**
6. **Events identify canonical resources using `kind` and `id`.**
7. **Resource generation and event sequence are separate concepts.**
8. **`eventId`, `messageId`, `commandId`, `subscriptionId`, and `connectionId` are separate identifiers.**
9. **Subscriptions begin with a snapshot.**
10. **Snapshots establish a synchronization point.**
11. **Events maintain state after the snapshot.**
12. **Missed events result in snapshot resynchronization.**
13. **Duplicate events must be safely ignored.**
14. **Stale events must not overwrite newer state.**
15. **The Bridge performs event authorization.**
16. **Coordinator scope is enforced during event delivery.**
17. **Command status is separate from resource events.**
18. **Command adapters do not directly generate resource events.**
19. **External Slurm changes also generate resource events after Cache synchronization.**
20. **Slurm query failures must never be interpreted as resource deletion.**
21. **The first implementation should use in-memory bounded event history only.**
22. **Durable event storage is not required for v1.0.**
23. **React consumes snapshots and events through Context Providers.**
24. **Event batching/coalescing are future optimizations, not v1.0 requirements.**
25. **The event model is transport-independent.**

---

# 175. Relationship to the v3.5 Architecture

The Event Model completes an important missing piece in the v3.5 architecture.

The architecture can now be expressed as:

```
┌────────────────────────────────────────────────┐
│              cockpit-slurm-bridge              │
│                                                │
│  UserContext                                   │ 
│       │                                        │
│       v                                        │
│  Authorization                                 │
│       │                                        │
│       ├──────── Query ──────────┐              │
│       │                         │              │
│       └──────── Command ──────┐ │              │
│                               │ │              │
│                               v v              │
│                         Resource Service       │
│                                │               │
│                         Resource Adapter       │
│                                │               │
│                                v               │
│                       Canonical Resource       │
│                                │               │
│                                v               │
│                         Resource Cache         │
│                                │               │
│                                v               │
│                           Event Bus            │
│                                │               │
│                                v               │
│                      Subscription Manager      │
│                                │               │
└────────────────────────────────┼───────────────┘
                                 │
                                 v
                       cockpit-slurm-channel
                                 │
                                 v
                              React
```

This fits the existing v3.5 principle that **Queries read from Cache, Commands modify Slurm, Resource Adapters synchronize state into canonical resources, Cache stores authoritative state, Events notify subscribers, and React consumes Snapshots + Events**. :contentReference[oaicite:1]{index=1}

## One architectural refinement I strongly recommend

There is one particularly important point that I would add to your v3.5 documents now:

### Do not make the Event Bus the source of truth or the Subscription Manager the event generator.

Keep these responsibilities strictly separated:

```text
| Component | Responsibility |
|---|---|
| **Resource Adapter** | Discover Slurm state |
| **Canonical Resource** | Represent normalized state |
| **Resource Cache** | Own authoritative state |
| **Event Bus** | Convert Cache changes into domain events |
| **Subscription Manager** | Decide which users receive which events |
| **IPC Server** | Serialize/transmit events |
| **Channel** | Forward events |
| **React Provider** | Maintain local UI state |
```

That separation is consistent with your existing v3.5 architecture, which explicitly places the Event Bus after the Cache and the Subscription Manager after the Event Bus. :contentReference[oaicite:2]{index=2}

### I would also change one subtle point from the existing v3.5 plan

Your current Bridge document says the Event Bus has the basic events:

```text
Created
Updated
Deleted
````

and the Subscription Manager contains:

```text
Connection
Resource Kind
Filter
Generation
```

([GitHub][2])

I recommend extending the subscription model to:

```text
Subscription
├── subscriptionId
├── connectionId
├── UserContext
├── resourceKind
├── filter
├── snapshotGeneration
├── eventSequence
└── state
```

The addition of **UserContext**, **snapshotGeneration**, and **eventSequence** is what makes the event architecture consistent with the `user-context.md` and `ipc-protocol.md` documents we have been developing.

In other words, the three documents now fit together as:

```text
                 resource-model.md
                        │
                        │ defines
                        ▼
               Canonical Resources
                        │
                        │ stored by
                        ▼
                 Resource Cache
                        │
                        │ produces
                        ▼
                  event-model.md
                        │
             ┌──────────┴──────────┐
             │                     │
             ▼                     ▼
         Event Bus          Subscription Manager
                                   │
                                   │ authorized by
                                   ▼
                            user-context.md
                                   │
                                   ▼
                              UserContext
                                   │
                                   ▼
                            IPC Event Message
                                   │
                                   │ defined by
                                   ▼
                           ipc-protocol.md
                                   │
                                   ▼
                         cockpit-slurm-channel
                                   │
                                   ▼
                                React
```

That gives your `docs/architecture` a very clean set of four foundational specifications:

1. **`resource-model.md`** — *What is the state?*
2. **`user-context.md`** — *Who is asking and what may they access?*
3. **`event-model.md`** — *How does state change propagate?*
4. **`ipc-protocol.md`** — *How do messages cross Channel ↔ Bridge?*

The existing repository currently contains the architecture plans plus `ipc-protocol.md` and `resource-model.md`; adding `event-model.md` and `user-context.md` makes the architecture specification much more complete. ([GitHub][1])

[cockpit-slurm architecture directory](https://github.com/lingweicai/cockpit-slurm/tree/development-bridge-v3.5/docs/architecture?utm_source=chatgpt.com)

[1]: https://github.com/lingweicai/cockpit-slurm/tree/development-bridge-v3.5/docs/architecture "cockpit-slurm/docs/architecture at development-bridge-v3.5 · lingweicai/cockpit-slurm · GitHub"
[2]: https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/docs/architecture/development-cockpit-slurm-bridge_v3.5.md "cockpit-slurm/docs/architecture/development-cockpit-slurm-bridge_v3.5.md at development-bridge-v3.5 · lingweicai/cockpit-slurm · GitHub"

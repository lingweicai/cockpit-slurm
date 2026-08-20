# cockpit-slurm IPC Protocol

**Status:** Proposed
**Version:** 1.0
**Project:** cockpit-slurm
**Components:** `cockpit-slurm-channel` ↔ `cockpit-slurm-bridge`

---

## Background 

Based on our previous architecture discussions, I recommend that ipc-protocol.md explicitly distinguish Cockpit's own channel protocol from your application IPC protocol.

That distinction is important because cockpit-slurm-channel is not directly speaking your JSON protocol to React. The path is:

```text
React
  │
  │ cockpit.channel()
  │  payload = "cockpit-slurm"
  ▼
Cockpit transport / cockpit-bridge
  │
  │ Cockpit channel protocol
  ▼
cockpit-slurm-channel
  │
  │ cockpit-slurm IPC protocol
  │ Unix socket
  ▼
cockpit-slurm-bridge
  │
  ▼
Slurm
```

Cockpit documents cockpit.channel() as a raw channel API, with messages delivered to channel.onmessage and sent with channel.send(). Package bridges are selected from manifest.json based on channel-open options such as payload.

So I would make docs/architecture/ipc-protocol.md specifically define the protocol between cockpit-slurm-channel and the persistent cockpit-slurm-bridge, while documenting the Cockpit side only as an integration boundary.

## 1. Purpose

This document defines the inter-process communication (IPC) protocol between:

* `cockpit-slurm-channel` — a lightweight, short-lived Cockpit package bridge process
* `cockpit-slurm-bridge` — the persistent Slurm state, cache, command, event, and subscription service

The protocol is intentionally independent of the Cockpit raw channel protocol.

The architecture is:

```text
┌───────────────────────┐
│       React UI        │
│                       │
│  cockpit.channel()    │
└───────────┬───────────┘
            │
            │ Cockpit channel
            ▼
┌───────────────────────┐
│    cockpit-bridge     │
│                       │
│  Cockpit transport    │
└───────────┬───────────┘
            │
            │ stdin/stdout
            ▼
┌───────────────────────┐
│ cockpit-slurm-channel │
│                       │
│  Cockpit adapter      │
│  IPC client           │
└───────────┬───────────┘
            │
            │ Unix socket
            │ cockpit-slurm IPC
            ▼
┌───────────────────────┐
│ cockpit-slurm-bridge  │
│                       │
│ IPC server            │
│ UserContext           │
│ Query Service         │
│ Command Service       │
│ Subscription Manager  │
│ Event Manager         │
│ Cache                 │
│ Resource Adapters     │
└───────────┬───────────┘
            │
            ▼
         Slurm
```

---

# 2. Design Principles

The IPC protocol follows these principles:

1. The Channel is a transport adapter, not an application server.
2. The Bridge owns Slurm state.
3. The Bridge owns authorization.
4. The Bridge owns canonical resource models.
5. The Bridge owns cache and generation numbers.
6. The Bridge owns commands and command status.
7. The Bridge owns subscriptions.
8. The Channel forwards messages without interpreting Slurm resources.
9. Requests and responses contain correlation IDs.
10. Events are asynchronous and are not required to have a request.
11. Protocol versioning is explicit.
12. Errors have a stable machine-readable structure.
13. Messages are designed so that future resources can be added without changing the transport protocol.

---

# 3. Two Different Protocol Boundaries

There are two communication protocols in the complete architecture.

## 3.1 Cockpit channel protocol

The browser uses:

```typescript
const channel = cockpit.channel({
    payload: "cockpit-slurm",
});
```

`cockpit.channel()` creates a raw Cockpit channel. Messages sent through `channel.send()` and received through `channel.onmessage` depend on the selected payload.

The Cockpit package manifest associates the payload with:

```json
{
    "bridges": [
        {
            "match": {
                "payload": "cockpit-slurm"
            },
            "spawn": [
                "/usr/libexec/cockpit-slurm-channel"
            ]
        }
    ]
}
```

Cockpit supports matching channel-open options in the package manifest and spawning the corresponding bridge process.

The Cockpit transport itself is **not defined by this document**.

---

## 3.2 cockpit-slurm IPC protocol

The second protocol is:

```text
cockpit-slurm-channel
          │
          │ Unix domain socket
          ▼
cockpit-slurm-bridge
```

This document defines this protocol.

The socket is expected to be:

```text
/run/cockpit-slurm/bridge.sock
```

for the system deployment.

A development/user-session socket may alternatively be:

```text
/run/user/<uid>/cockpit-slurm/bridge.sock
```

The actual path should be configurable.

---

# 4. Transport

## 4.1 Unix Domain Socket

The Channel connects to the Bridge using a Unix domain stream socket.

Example:

```text
unix:///run/cockpit-slurm/bridge.sock
```

Advantages:

* local-only communication
* no TCP port
* filesystem permissions can restrict access
* low latency
* suitable for persistent daemon ↔ short-lived process communication

---

# 5. Message Framing

The IPC protocol uses **length-prefixed JSON messages**.

Each message is encoded as:

```text
[length][JSON payload]
```

where:

```text
length = unsigned 32-bit integer
encoding = network byte order / big endian
payload = UTF-8 JSON
```

For example:

```text
00 00 00 7A
{
  "protocol": "cockpit-slurm",
  ...
}
```

The JSON object itself must not contain arbitrary framing delimiters.

This avoids ambiguity when payloads contain:

* strings
* error messages
* command scripts
* JSON objects
* multiline text

The protocol therefore does **not** depend on newline characters to identify message boundaries.

---

# 6. Common Message Envelope

Every IPC message uses a common envelope.

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01JXYZ...",
  "type": "query",
  "timestamp": "2026-08-14T06:00:00Z",
  "payload": {}
}
```

## 6.1 Envelope fields

| Field       | Type   |    Required | Description               |
| ----------- | ------ | ----------: | ------------------------- |
| `protocol`  | string |         yes | Protocol identifier       |
| `version`   | string |         yes | Protocol version          |
| `messageId` | string |         yes | Unique message identifier |
| `type`      | string |         yes | Message type              |
| `timestamp` | string | recommended | RFC 3339 UTC timestamp    |
| `payload`   | object |         yes | Message-specific data     |

---

# 7. Protocol Identifier

The protocol identifier is:

```text
cockpit-slurm
```

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0"
}
```

---

# 8. Protocol Version

The initial protocol version is:

```text
1.0
```

Versioning follows:

```text
MAJOR.MINOR
```

A major version change indicates an incompatible protocol change.

A minor version change indicates backward-compatible additions.

Examples:

```text
1.0
1.1
1.2
2.0
```

A Channel supporting protocol `1.x` should reject an incompatible `2.x` connection.

---

# 9. Message ID

Every request must contain a unique `messageId`.

Example:

```json
{
  "messageId": "01K2ABC123XYZ"
}
```

The identifier should preferably be generated as a UUID or ULID.

Example Go representation:

```go
type MessageID string
```

The Bridge must use the ID to correlate responses with requests.

---

# 10. Connection ID

A Channel connection receives a separate `connectionId`.

Example:

```json
{
  "connectionId": "01K2CONNECTION..."
}
```

The connection ID identifies the IPC connection rather than an individual request.

Example:

```text
connectionId = channel process / IPC connection
messageId    = individual message
commandId    = individual command
subscriptionId = individual subscription
```

These identifiers have different purposes.

---

# 11. Identifier Model

The protocol uses four important identifiers.

| Identifier       | Scope          | Purpose                     |
| ---------------- | -------------- | --------------------------- |
| `connectionId`   | IPC connection | Identify Channel connection |
| `messageId`      | Message        | Correlate request/response  |
| `commandId`      | Command        | Track asynchronous command  |
| `subscriptionId` | Subscription   | Track event subscription    |

Example:

```text
connectionId
    │
    ├── messageId: query-001
    ├── messageId: subscribe-002
    │       └── subscriptionId: sub-001
    │
    └── messageId: command-003
            └── commandId: cmd-001
```

---

# 12. Message Types

The initial protocol defines:

```text
hello
hello-response

query
query-response

subscribe
subscribe-response

unsubscribe
unsubscribe-response

command
command-response

command-status

event

error

ping
pong

close
```

---

# 13. Connection Handshake

The Channel should establish the IPC connection using a handshake.

```text
Channel                         Bridge
   │                              │
   │──── hello ─────────────────>│
   │                              │
   │<─── hello-response ─────────│
   │                              │
   │──── query/subscribe/... ───>│
```

---

# 14. Hello Request

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01HELLO001",
  "type": "hello",
  "timestamp": "2026-08-14T06:00:00Z",
  "payload": {
    "client": "cockpit-slurm-channel",
    "clientVersion": "0.1.0",
    "supportedVersions": [
      "1.0"
    ]
  }
}
```

---

# 15. Hello Response

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01HELLO002",
  "type": "hello-response",
  "timestamp": "2026-08-14T06:00:00Z",
  "payload": {
    "connectionId": "01CONNECTION001",
    "server": "cockpit-slurm-bridge",
    "serverVersion": "0.1.0",
    "protocolVersion": "1.0"
  }
}
```

The Bridge should also establish the effective `UserContext` for this connection.

---

# 16. UserContext

The Bridge is responsible for establishing the user context.

Example:

```json
{
  "username": "alice",
  "uid": 1001,
  "gid": 1001,
  "adminLevel": "operator",
  "coordinatorAccounts": [
    "research"
  ],
  "capabilities": [
    "nodes.view",
    "jobs.view",
    "jobs.cancel",
    "accounts.view"
  ]
}
```

The frontend must not be responsible for determining authoritative permissions.

---

# 17. Query

A query retrieves the current state from the Bridge cache.

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01QUERY001",
  "type": "query",
  "payload": {
    "resource": "nodes"
  }
}
```

---

# 18. Query With Filter

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01QUERY002",
  "type": "query",
  "payload": {
    "resource": "nodes",
    "filter": {
      "state": [
        "idle",
        "allocated"
      ]
    }
  }
}
```

---

# 19. Query Response

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01QUERY001",
  "type": "query-response",
  "payload": {
    "resource": "nodes",
    "generation": 42,
    "items": [
      {
        "kind": "Node",
        "name": "node001",
        "generation": 42,
        "state": "idle",
        "cpus": 64
      },
      {
        "kind": "Node",
        "name": "node002",
        "generation": 42,
        "state": "allocated",
        "cpus": 64
      }
    ]
  }
}
```

The response uses the same `messageId` as the request.

---

# 20. Generation

Every cached resource collection should have a generation number.

Example:

```text
nodes generation = 42
jobs generation  = 187
accounts generation = 23
users generation = 17
```

Generation is monotonically increasing for a resource collection.

Example:

```text
generation 41
      ↓
node001 changed
      ↓
generation 42
```

The generation allows the client to determine whether its state is current.

---

# 21. Resource Version

Individual resources may also contain a resource version.

Example:

```json
{
  "kind": "Node",
  "name": "node001",
  "generation": 42,
  "resourceVersion": "42-00017"
}
```

Generation represents the collection state.

Resource version identifies a particular resource revision.

---

# 22. Subscribe

A client subscribes to resource events.

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01SUB001",
  "type": "subscribe",
  "payload": {
    "resource": "nodes"
  }
}
```

---

# 23. Subscribe Response

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01SUB001",
  "type": "subscribe-response",
  "payload": {
    "subscriptionId": "01SUBSCRIPTION001",
    "resource": "nodes",
    "generation": 42
  }
}
```

---

# 24. Subscription Snapshot

A subscription should normally receive an initial snapshot.

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01SNAPSHOT001",
  "type": "event",
  "payload": {
    "subscriptionId": "01SUBSCRIPTION001",
    "event": "snapshot",
    "resource": "nodes",
    "generation": 42,
    "items": [
      {
        "kind": "Node",
        "name": "node001",
        "state": "idle"
      }
    ]
  }
}
```

This means a React provider does not need to perform:

```text
query()
subscribe()
```

unless explicitly desired.

A subscription can establish its initial state and then receive changes.

---

# 25. Resource Events

The basic event operations are:

```text
ADDED
MODIFIED
DELETED
```

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01EVENT001",
  "type": "event",
  "payload": {
    "subscriptionId": "01SUBSCRIPTION001",
    "event": "modified",
    "resource": "nodes",
    "generation": 43,
    "item": {
      "kind": "Node",
      "name": "node001",
      "state": "allocated"
    }
  }
}
```

---

# 26. Added Event

```json
{
  "type": "event",
  "payload": {
    "subscriptionId": "01SUB001",
    "event": "added",
    "resource": "nodes",
    "generation": 44,
    "item": {
      "kind": "Node",
      "name": "node003",
      "state": "idle"
    }
  }
}
```

---

# 27. Deleted Event

```json
{
  "type": "event",
  "payload": {
    "subscriptionId": "01SUB001",
    "event": "deleted",
    "resource": "nodes",
    "generation": 45,
    "item": {
      "kind": "Node",
      "name": "node003"
    }
  }
}
```

---

# 28. Unsubscribe

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01UNSUB001",
  "type": "unsubscribe",
  "payload": {
    "subscriptionId": "01SUB001"
  }
}
```

Response:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01UNSUB001",
  "type": "unsubscribe-response",
  "payload": {
    "subscriptionId": "01SUB001"
  }
}
```

---

# 29. Commands

Commands are different from queries.

A query asks:

```text
"What is the current state?"
```

A command asks:

```text
"Change the Slurm state."
```

Examples:

```text
cancel job
resume job
suspend job
create account
modify account
delete account
create user
modify user
delete user
```

---

# 30. Command Request

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01COMMAND001",
  "type": "command",
  "payload": {
    "commandId": "01CMD001",
    "operation": "cancel",
    "resource": "jobs",
    "target": {
      "jobId": 12345
    },
    "parameters": {}
  }
}
```

---

# 31. Command Response

The immediate response acknowledges that the command was accepted for processing.

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01COMMAND001",
  "type": "command-response",
  "payload": {
    "commandId": "01CMD001",
    "status": "accepted"
  }
}
```

The command is not necessarily complete at this point.

---

# 32. Command Status

The Bridge can subsequently send:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01STATUS001",
  "type": "command-status",
  "payload": {
    "commandId": "01CMD001",
    "status": "completed"
  }
}
```

Possible states:

```text
accepted
running
completed
failed
cancelled
```

---

# 33. Command Failure

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01STATUS002",
  "type": "command-status",
  "payload": {
    "commandId": "01CMD001",
    "status": "failed",
    "error": {
      "code": "authorization-error",
      "message": "User is not permitted to cancel this job"
    }
  }
}
```

---

# 34. Important Command/Data Rule

The Command Service must not optimistically modify the cache.

Incorrect:

```text
React
  ↓
Command
  ↓
Cache modified
  ↓
Slurm command
```

Correct:

```text
React
  ↓
Command Service
  ↓
Slurm
  ↓
Slurm state changes
  ↓
Resource Adapter
  ↓
Canonical Resource
  ↓
Cache
  ↓
Event
  ↓
React
```

The cache represents observed/confirmed Slurm state.

---

# 35. Error Message

All protocol errors use a common structure.

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01QUERY003",
  "type": "error",
  "payload": {
    "code": "not-found",
    "message": "Resource 'node999' was not found",
    "details": {
      "resource": "nodes",
      "name": "node999"
    }
  }
}
```

---

# 36. Error Codes

Initial error codes:

```text
invalid-request
invalid-message
unsupported-version
unsupported-message
authentication-error
authorization-error
not-found
already-exists
conflict
validation-error
slurm-error
slurm-unavailable
cache-unavailable
command-failed
timeout
internal-error
transport-error
```

The `code` field is intended for programmatic handling.

The `message` field is intended for logging/debugging and may be displayed to users after suitable sanitization.

---

# 37. Generation Conflict

If the client sends an outdated generation, the Bridge may return:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01QUERY004",
  "type": "error",
  "payload": {
    "code": "conflict",
    "message": "Client generation is stale",
    "details": {
      "clientGeneration": 41,
      "currentGeneration": 45,
      "resource": "nodes"
    }
  }
}
```

The client should recover by requesting a new snapshot.

---

# 38. Ping/Pong

The Channel can test the Bridge connection.

Request:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01PING001",
  "type": "ping",
  "payload": {}
}
```

Response:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01PONG001",
  "type": "pong",
  "payload": {}
}
```

---

# 39. Close

The Channel can gracefully close its IPC connection.

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "01CLOSE001",
  "type": "close",
  "payload": {
    "reason": "client-closed"
  }
}
```

The Bridge should automatically remove:

```text
connection
subscriptions
pending requests
pending event deliveries
```

associated with the connection.

---

# 40. Connection Lifecycle

The complete lifecycle is:

```text
                 Channel                         Bridge
                    │                               │
                    │──── Unix connect ────────────>│
                    │                               │
                    │──── hello ──────────────────>│
                    │<─── hello-response ──────────│
                    │                               │
                    │──── subscribe(nodes) ───────>│
                    │<─── subscribe-response ──────│
                    │<─── snapshot ────────────────│
                    │                               │
                    │<─── event: modified ─────────│
                    │<─── event: modified ─────────│
                    │                               │
                    │──── command ─────────────────>│
                    │<─── command-response ─────────│
                    │<─── command-status ───────────│
                    │<─── event: modified ─────────│
                    │                               │
                    │──── unsubscribe ─────────────>│
                    │                               │
                    │──── close ───────────────────>│
                    │                               │
                    X                               X
```

---

# 41. Request/Response Correlation

Requests and responses are correlated using `messageId`.

Example:

```text
Request:
messageId = A

Response:
messageId = A
```

A command additionally has:

```text
messageId = A
commandId = B
```

This allows multiple commands to execute concurrently.

Example:

```text
message A → command B → cancel job 123
message C → command D → cancel job 456
```

---

# 42. Asynchronous Events

Events do not require a corresponding request.

For example:

```text
React
  │
  │ subscribe
  ▼
Bridge
  │
  │ subscription established
  ▼
React

       later...

Slurm
  │
  ▼
Bridge
  │
  ▼
event
  │
  ▼
Channel
  │
  ▼
React
```

Therefore the React client must not assume that every incoming message is a response.

---

# 43. Message Dispatch

The Channel should treat messages according to their type:

```text
query-response
subscribe-response
unsubscribe-response
command-response
command-status
event
error
pong
```

The Channel should not interpret:

```text
Node
Job
Account
User
Partition
```

as application objects.

It should primarily forward the JSON payload.

---

# 44. Suggested Go Message Types

The common envelope can be represented as:

```go
type Message struct {
    Protocol  string          `json:"protocol"`
    Version   string          `json:"version"`
    MessageID string          `json:"messageId"`
    Type      string          `json:"type"`
    Timestamp time.Time       `json:"timestamp,omitempty"`
    Payload   json.RawMessage `json:"payload"`
}
```

This is intentionally generic.

The Bridge can then decode the payload according to `Type`.

---

# 45. Go Query Request

```go
type QueryRequest struct {
    Resource string                 `json:"resource"`
    Filter   map[string]interface{} `json:"filter,omitempty"`
}
```

Example dispatch:

```go
func (s *Server) handleMessage(
    ctx context.Context,
    msg Message,
) {
    switch msg.Type {
    case "hello":
        s.handleHello(ctx, msg)

    case "query":
        s.handleQuery(ctx, msg)

    case "subscribe":
        s.handleSubscribe(ctx, msg)

    case "unsubscribe":
        s.handleUnsubscribe(ctx, msg)

    case "command":
        s.handleCommand(ctx, msg)

    case "ping":
        s.handlePing(ctx, msg)

    default:
        s.sendError(
            msg.MessageID,
            "unsupported-message",
            "Unsupported message type",
        )
    }
}
```

---

# 46. Go Length-Prefixed Writer

Example implementation:

```go
func writeMessage(w io.Writer, data []byte) error {
    var header [4]byte

    binary.BigEndian.PutUint32(header[:], uint32(len(data)))

    if _, err := w.Write(header[:]); err != nil {
        return err
    }

    _, err := w.Write(data)
    return err
}
```

---

# 47. Go Length-Prefixed Reader

```go
func readMessage(r io.Reader) ([]byte, error) {
    var header [4]byte

    if _, err := io.ReadFull(r, header[:]); err != nil {
        return nil, err
    }

    length := binary.BigEndian.Uint32(header[:])

    if length > MaxMessageSize {
        return nil, fmt.Errorf(
            "message too large: %d bytes",
            length,
        )
    }

    data := make([]byte, length)

    if _, err := io.ReadFull(r, data); err != nil {
        return nil, err
    }

    return data, nil
}
```

Recommended initial limit:

```go
const MaxMessageSize = 16 * 1024 * 1024
```

The limit should be configurable.

---

# 48. Channel Architecture

The Channel should have three logical responsibilities:

```text
┌───────────────────────────────┐
│ cockpit-slurm-channel         │
│                               │
│ Cockpit Protocol Adapter      │
│             │                 │
│             ▼                 │
│ IPC Client                    │
│             │                 │
│             ▼                 │
│ Forwarder                     │
└───────────────────────────────┘
```

It should not contain:

```text
❌ Resource cache
❌ Slurm commands
❌ Slurm CLI parsing
❌ User authorization
❌ AdminLevel calculation
❌ Event generation
❌ Resource synchronization
```

---

# 49. Bridge Architecture

The Bridge should contain:

```text
cockpit-slurm-bridge
│
├── IPC Server
│
├── Connection Manager
│
├── UserContext
│
├── Protocol Dispatcher
│
├── Query Service
│
├── Command Service
│
├── Subscription Manager
│
├── Event Manager
│
├── Resource Cache
│
├── Canonical Resources
│
└── Slurm Adapters
```

---

# 50. Resource Names

The initial resource names are:

```text
clusters
nodes
partitions
jobs
accounts
users
associations
qos
reservations
tres
wckeys
```

The resource registry should allow additional resources without changing the IPC envelope.

Example:

```json
{
  "type": "query",
  "payload": {
    "resource": "accounts"
  }
}
```

and:

```json
{
  "type": "query",
  "payload": {
    "resource": "reservations"
  }
}
```

use the same protocol.

---

# 51. Canonical Resource Representation

A resource should contain common metadata.

Example:

```json
{
  "kind": "Node",
  "name": "node001",
  "generation": 42,
  "resourceVersion": "42-001",
  "metadata": {},
  "spec": {},
  "status": {}
}
```

For example:

```json
{
  "kind": "Node",
  "name": "node001",
  "generation": 42,
  "metadata": {
    "hostname": "node001"
  },
  "spec": {
    "cpus": 64
  },
  "status": {
    "state": "idle"
  }
}
```

The exact canonical resource schema is defined separately in `resource-model.md`.

---

# 52. Query/Cache Rule

Queries should normally read from the Bridge cache:

```text
Query
  ↓
Query Service
  ↓
Cache
  ↓
Response
```

They should not execute:

```text
React → Channel → Bridge → sinfo
```

for every UI request.

The Bridge's background synchronization mechanism keeps the cache updated.

---

# 53. State Synchronization

The Bridge is responsible for synchronization:

```text
Slurm
  ↓
Resource Adapter
  ↓
Canonical Resource
  ↓
Cache
  ↓
Generation++
  ↓
Event
```

Possible sources include:

```text
slurmrestd
Slurm CLI
Slurm events/triggers
Slurm logs
polling fallback
```

The IPC protocol does not depend on which adapter produced the resource.

---

# 54. Event Ordering

For a single resource collection, events should be delivered in increasing generation order.

Example:

```text
generation 40
generation 41
generation 42
generation 43
```

If a client detects:

```text
received 42
received 44
```

it should assume that generation 43 may have been missed and request a fresh snapshot.

---

# 55. Event Recovery

Example:

```text
Client generation = 42
Bridge generation = 45
```

If events 43 and 44 were missed, the Bridge may send:

```json
{
  "type": "event",
  "payload": {
    "event": "resync-required",
    "resource": "nodes",
    "generation": 45
  }
}
```

The client then requests:

```json
{
  "type": "query",
  "payload": {
    "resource": "nodes"
  }
}
```

Alternatively, the subscription manager may automatically send a new snapshot.

---

# 56. Connection Failure

If the Channel loses the Unix socket:

```text
Channel
   X
Bridge
```

the Channel should close the corresponding Cockpit channel.

The React application should then:

```text
detect close
    ↓
show connection state
    ↓
retry according to policy
```

The Bridge should clean up all connection-scoped subscriptions.

---

# 57. Bridge Restart

The Bridge is persistent but may restart.

After restart:

```text
old generation
      X

new Bridge instance
      ↓
cache initialization
      ↓
new generation state
```

Clients should not assume that generation numbers survive a Bridge restart unless a persistent generation mechanism is explicitly introduced.

For the initial implementation, generation numbers may be considered valid only within a Bridge instance.

---

# 58. Idempotency

Commands must contain a `commandId`.

Example:

```json
{
  "type": "command",
  "payload": {
    "commandId": "01CMD001",
    "operation": "cancel",
    "resource": "jobs",
    "target": {
      "jobId": 12345
    }
  }
}
```

The Bridge should detect duplicate command IDs within its retention period.

This prevents accidental duplicate execution when a Channel reconnects or retries a request.

---

# 59. Command Idempotency Example

If:

```text
commandId = CMD001
```

has already completed:

```json
{
  "commandId": "CMD001",
  "status": "completed"
}
```

and the same command is received again, the Bridge should return the existing command status rather than executing the Slurm operation again.

---

# 60. Authorization

Authorization is performed by the Bridge.

Example:

```text
React
 ↓
command: cancel job
 ↓
Bridge
 ↓
UserContext
 ↓
Capability check
 ↓
Command Service
```

Possible result:

```text
allowed → execute
```

or:

```text
denied → authorization-error
```

The frontend may hide unavailable operations, but hiding a button is not an authorization mechanism.

---

# 61. Example: Node Query

Complete flow:

```text
React
 │
 │ cockpit.channel()
 │
 │ {"type":"query","resource":"nodes"}
 ▼
cockpit-slurm-channel
 │
 │ IPC
 ▼
cockpit-slurm-bridge
 │
 │ QueryService
 ▼
Node Cache
 │
 │ generation=42
 ▼
cockpit-slurm-bridge
 │
 ▼
cockpit-slurm-channel
 │
 ▼
React
```

Response:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "Q001",
  "type": "query-response",
  "payload": {
    "resource": "nodes",
    "generation": 42,
    "items": [
      {
        "kind": "Node",
        "name": "node001",
        "status": {
          "state": "idle"
        }
      }
    ]
  }
}
```

---

# 62. Example: Node Event

Later:

```text
Slurm
 │
 │ node001 becomes allocated
 ▼
Bridge Adapter
 │
 ▼
Node Cache
 │
 │ generation 43
 ▼
Event Manager
 │
 ▼
Subscription Manager
 │
 ▼
Channel
 │
 ▼
React
```

Event:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "E001",
  "type": "event",
  "payload": {
    "subscriptionId": "SUB001",
    "event": "modified",
    "resource": "nodes",
    "generation": 43,
    "item": {
      "kind": "Node",
      "name": "node001",
      "status": {
        "state": "allocated"
      }
    }
  }
}
```

---

# 63. Example: Job Cancellation

The complete command flow is:

```text
React
 │
 │ command
 ▼
Channel
 │
 ▼
Bridge
 │
 ├── UserContext
 │
 ├── Authorization
 │
 ├── Command Service
 │
 └── Slurm Adapter
       │
       ▼
      Slurm
       │
       ▼
   Job state changed
       │
       ▼
      Cache
       │
       ▼
      Event
       │
       ▼
      React
```

The important point is that the command response and resource event are different messages.

```text
command-response
        ≠
resource event
```

The first means:

> The Bridge accepted/processed the command.

The second means:

> The Slurm resource state changed.

---

# 64. Security Requirements

The Unix socket must not be world-writable.

Recommended system deployment:

```text
/run/cockpit-slurm/bridge.sock
```

with ownership and permissions appropriate for the Cockpit service/session model.

The Bridge must not trust arbitrary values supplied by the Channel for:

```text
username
uid
adminLevel
capabilities
```

The authoritative user identity should originate from the authenticated Cockpit session / execution context.

The client may provide metadata for diagnostics, but the Bridge must establish its own `UserContext`.

---

# 65. Logging

Every important operation should be traceable using:

```text
connectionId
messageId
commandId
subscriptionId
username
resource
operation
```

Example:

```text
connection=CON001
message=MSG001
user=alice
type=command
command=CMD001
resource=jobs
operation=cancel
job=12345
```

Sensitive command parameters must not be logged indiscriminately.

---

# 66. Recommended Go Package Structure

The IPC implementation should be separated from the application services.

```text
internal/
├── ipc/
│   ├── protocol.go
│   ├── message.go
│   ├── codec.go
│   ├── reader.go
│   ├── writer.go
│   ├── server.go
│   ├── connection.go
│   └── errors.go
│
├── protocol/
│   ├── version.go
│   ├── types.go
│   ├── query.go
│   ├── command.go
│   ├── subscription.go
│   └── event.go
│
├── identity/
├── resource/
├── cache/
├── service/
├── command/
├── event/
├── subscription/
└── adapter/
```

The distinction is:

```text
ipc/
    transport mechanics

protocol/
    application message definitions

service/
    business/application logic
```

---

# 67. Protocol Testing

The protocol must be testable without Slurm.

Unit tests should cover:

```text
✓ encode/decode
✓ length framing
✓ invalid length
✓ oversized message
✓ invalid JSON
✓ unsupported version
✓ unsupported message
✓ request/response correlation
✓ command ID handling
✓ subscription lifecycle
✓ event ordering
✓ generation mismatch
✓ connection cleanup
```

---

# 68. Integration Test

A minimal integration test should run:

```text
Test Client
     │
     │ Unix socket
     ▼
cockpit-slurm-bridge
```

without Cockpit.

The test should:

```text
1. connect
2. hello
3. query nodes
4. subscribe nodes
5. receive snapshot
6. inject/change a resource
7. receive event
8. unsubscribe
9. close
```

This gives the Bridge protocol a stable test harness before React is involved.

---

# 69. Cockpit Integration Test

A second test should validate:

```text
React
 ↓
cockpit.channel()
 ↓
Cockpit
 ↓
cockpit-slurm-channel
 ↓
Unix socket
 ↓
cockpit-slurm-bridge
```

The first target should be:

```text
ping
```

followed by:

```text
query nodes
```

before implementing commands.

---

# 70. Protocol v1.0 Initial Scope

Version 1.0 should implement only:

```text
hello
ping/pong

query
query-response

subscribe
subscribe-response
event
unsubscribe
unsubscribe-response

command
command-response
command-status

error

close
```

The first resource should be:

```text
nodes
```

Then:

```text
jobs
accounts
users
partitions
```

can be added without changing the fundamental protocol.

---

# 71. What Should NOT Be Part of IPC Protocol v1.0

Do not put the following into the IPC protocol initially:

```text
❌ Slurm CLI command syntax
❌ sinfo arguments
❌ squeue arguments
❌ sacctmgr syntax
❌ slurmrestd URLs
❌ OpenAPI-specific transport details
❌ SQL
❌ React component names
❌ PatternFly concepts
❌ UI page names
❌ chart configuration
```

The IPC protocol describes **application intent and state**, not implementation details.

For example:

```text
GOOD:
operation = "cancel"
resource = "jobs"
jobId = 12345
```

rather than:

```text
BAD:
command = "scancel 12345"
```

This allows the Bridge to choose:

```text
slurmrestd
```

or:

```text
scancel
```

or another adapter without changing the frontend or Channel protocol.

---

# 72. Architectural Rule

The most important rule of this protocol is:

```text
Cockpit Channel Protocol
        │
        │ adapter
        ▼
cockpit-slurm IPC Protocol
        │
        │ application layer
        ▼
Canonical Resource / Command / Event Model
        │
        │ adapter
        ▼
Slurm
```

Each layer has a separate responsibility.

---

# 73. Final Reference Architecture

```text
                         Browser
                            │
                            │ cockpit.channel()
                            ▼
                 ┌─────────────────────┐
                 │   cockpit-bridge    │
                 └──────────┬──────────┘
                            │
                    Cockpit channel
                            │
                            ▼
                 ┌─────────────────────┐
                 │ cockpit-slurm-      │
                 │ channel             │
                 │                     │
                 │ Transport Adapter   │
                 └──────────┬──────────┘
                            │
                    Unix socket IPC
                            │
                            ▼
        ┌─────────────────────────────────────┐
        │       cockpit-slurm-bridge          │
        │                                     │
        │  IPC Server                         │
        │       │                             │
        │  UserContext                        │
        │       │                             │
        │  Query / Command Services           │
        │       │                             │
        │  Subscription Manager               │
        │       │                             │
        │  Event Manager                      │
        │       │                             │
        │  Canonical Resource Cache           │
        │       │                             │
        │  Resource Adapters                  │
        └────────────────┬────────────────────┘
                         │
                         ▼
                       Slurm
```

This protocol is therefore deliberately **resource-oriented, asynchronous, versioned, and transport-independent**.

---

## 74. Implementation Priority

The implementation order should be:

```text
1. Message envelope
2. Length-prefixed codec
3. Unix socket server
4. Unix socket client
5. hello
6. ping/pong
7. query
8. query-response
9. subscribe
10. snapshot
11. event
12. unsubscribe
13. command
14. command-status
15. error handling
16. generation/recovery
17. idempotency
18. authorization integration
```

The first end-to-end milestone should therefore be:

```text
React
  ↓
cockpit.channel()
  ↓
cockpit-slurm-channel
  ↓
Unix socket
  ↓
cockpit-slurm-bridge
  ↓
mock Node cache
  ↓
query-response
  ↓
React
```

Only after this works should the mock cache be replaced by the real Slurm Node adapter.

---

## 75. Relationship to Other Architecture Documents

This document should be referenced by:

```text
architecture.md
resource-model.md
event-model.md
command-model.md
user-context.md
```

The responsibilities should be:

```text
ipc-protocol.md
    How processes communicate

resource-model.md
    What a canonical Slurm resource looks like

event-model.md
    How resource changes are represented

command-model.md
    How state-changing operations work

user-context.md
    How identity, AdminLevel and capabilities work

architecture.md
    How all components fit together
```

This separation prevents `ipc-protocol.md` from becoming a description of the entire Bridge architecture.


# One important refinement to our earlier discussion

I would specifically adopt length-prefixed JSON over the Channel ↔ Bridge Unix socket, rather than newline-delimited JSON. It makes the IPC layer unambiguous and gives you a clean place to enforce MaxMessageSize.

Also, I would not try to make cockpit-slurm-channel implement or redefine Cockpit's underlying protocol. Its job is to adapt the Cockpit channel stream to your application IPC. Cockpit itself owns the raw channel transport; its documentation explicitly describes raw channels as the low-level communication mechanism and package bridges as processes selected from the package manifest.

This also gives you a very clean boundary for the Go implementation:

```text
cockpit-slurm-channel
    ├── Cockpit protocol adapter
    └── IPC client
             │
             │ length-prefixed JSON
             ▼
cockpit-slurm-bridge
    ├── IPC server
    └── application services
```

I think this is a better foundation for the Node → Job → Account → User vertical slices we discussed, because none of those resources need to change the transport protocol; they only add new resource, operation, and payload values.


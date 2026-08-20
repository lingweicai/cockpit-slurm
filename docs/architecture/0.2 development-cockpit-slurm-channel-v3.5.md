# Development Plan for cockpit-slurm-channel

The overall concept:

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
  ├── IPC Server
  ├── Subscription
  ├── Event
  ├── Command Service
  ├── Query Service
  ├── Cache
  ├── Canonical Resources
  └── Slurm Adapters
       │
       ▼
     Slurm
```

But **`cockpit-slurm-channel` should not itself be thought of as a second Slurm bridge**. It should be a very thin **Cockpit package bridge / transport proxy**. Cockpit's package `bridges` mechanism matches channel-open options and spawns the configured executable; Cockpit can also reuse a spawned bridge process for matching opens. ([Cockpit Project][1])

## 1. The important distinction

I would define the three components this way:

### `cockpit-slurm-bridge`

**Long-lived Slurm state/application service**

```text
cockpit-slurm-bridge
│
├── IPC Server
├── Protocol
├── UserContext / Authorization
├── Query Service
├── Command Service
├── Resource Service
├── Canonical Resources
├── Cache
├── Event Bus
├── Subscription Manager
└── Slurm Adapters
```

This is where your Version 3.5 architecture lives.

### `cockpit-slurm-channel`

**Short-lived or Cockpit-managed channel endpoint**

```text
cockpit-slurm-channel
│
├── Cockpit channel protocol
├── Connect to bridge.sock
├── Forward messages
└── Forward disconnect/close
```

It should **not** contain:

* Slurm models
* Slurm CLI execution
* cache
* event bus
* authorization rules
* resource adapters
* command adapters

That separation is particularly important because Cockpit itself already provides the channel mechanism and starts package bridges based on the package manifest. ([Cockpit Project][2])

---

# 2. One correction about "called by cockpit.channel()"

Technically, React does not directly execute:

```text
cockpit-slurm-channel
```

Instead:

```text
React
   │
   │ cockpit.channel({
   │   payload: "slurm"
   │ })
   ▼
Cockpit transport / cockpit-bridge
   │
   │ manifest.json matching
   ▼
cockpit-slurm-channel
```

Cockpit's `manifest.json` `bridges` section specifies which channel-open requests are matched and what executable gets spawned. The `spawn` command can receive values from matching channel options through `${...}` substitution. ([Cockpit Project][1])

So I would document the relationship as:

> **React opens a Cockpit channel; Cockpit routes the channel to `cockpit-slurm-channel` according to the package bridge configuration.**

That is more accurate than saying React directly calls the executable.

---

# 3. I would use a dedicated payload

For example, React could open:

```typescript
const channel = cockpit.channel({
    payload: "cockpit-slurm",
});
```

Then your manifest could contain something conceptually like:

```json
{
    "bridges": [
        {
            "match": {
                "payload": "cockpit-slurm"
            },
            "spawn": [
                "/usr/libexec/cockpit-slurm-channel"
            ],
            "problem": "internal-error"
        }
    ]
}
```

The exact executable installation path should of course be configurable by your packaging/build system.

The important part is:

```text
payload = cockpit-slurm
```

This gives you a clean namespace for your Cockpit channel.

---

# 4. Socket path configuration

Your proposed socket paths are reasonable:

### System installation

```text
/run/cockpit-slurm/bridge.sock
```

### User installation

```text
/run/user/<uid>/cockpit-slurm/bridge.sock
```

---

# 5. System vs user installation

I would actually make this a deployment decision rather than a runtime decision made by React.

### System installation

```text
Cockpit
   │
   ▼
cockpit-slurm-channel
   │
   ▼
/run/cockpit-slurm/bridge.sock
   │
   ▼
system cockpit-slurm-bridge
```

### User installation

```text
Cockpit user session
   │
   ▼
cockpit-slurm-channel
   │
   ▼
/run/user/<uid>/cockpit-slurm/bridge.sock
   │
   ▼
user cockpit-slurm-bridge
```

The channel executable can determine the appropriate configuration based on its execution environment, or receive a trusted configuration value through its server-side invocation.

---

# 6. Very important: don't confuse the two IPC protocols

This is something I would explicitly add to Version 3.5.

There are actually **two communication protocols** involved:

```text
┌──────────────────────────────┐
│ Cockpit Channel Protocol     │
│                              │
│ React ↔ Cockpit ↔ channel    │
└──────────────┬───────────────┘
               │
               │
       cockpit-slurm-channel
               │
               │
┌──────────────▼───────────────┐
│ cockpit-slurm IPC Protocol   │
│                              │
│ channel ↔ bridge             │
└──────────────┬───────────────┘
               │
               ▼
      cockpit-slurm-bridge
```

This is why `cockpit-slurm-channel` should be a **protocol adapter**.

It translates:

```text
Cockpit channel messages
```

into:

```text
cockpit-slurm IPC messages
```

and vice versa.

That is its primary job.

---

# 7. Example end-to-end flow

Suppose React wants the initial list of Jobs.

### Step 1 — React

```typescript
const channel = cockpit.channel({
    payload: "cockpit-slurm",
});
```

### Step 2 — Cockpit

Cockpit sees:

```text
payload = cockpit-slurm
```

and finds your matching package bridge.

### Step 3 — Cockpit starts/reuses

```text
cockpit-slurm-channel
```

Cockpit's bridge mechanism supports process reuse for matching bridge invocations, which is useful to keep in mind when deciding whether you want one channel process per connection or a reused process for matching opens. ([Cockpit Project][1])

### Step 4 — channel connects

```text
cockpit-slurm-channel
        │
        ▼
Unix socket
        │
        ▼
/run/cockpit-slurm/bridge.sock
```

### Step 5 — channel sends IPC request

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "123",
  "type": "subscribe",
  "resource": "job"
}
```

### Step 6 — bridge

```text
IPC Server
    ↓
Protocol Dispatcher
    ↓
Subscription Service
    ↓
Cache
```

### Step 7 — snapshot

```text
Cache
  ↓
Subscription
  ↓
IPC Server
  ↓
Unix socket
  ↓
cockpit-slurm-channel
  ↓
Cockpit channel
  ↓
React
```

---

# 8. Events work particularly well with this design

Suppose another administrator creates an account.

```text
Admin A
   │
   ▼
Command
   │
   ▼
Command Service
   │
   ▼
sacctmgr
   │
   ▼
Resource Adapter
   │
   ▼
Canonical Account
   │
   ▼
Cache
   │
   ▼
AccountCreated
   │
   ▼
Event Bus
```

Then:

```text
                    Event Bus
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
       Client A      Client B     Client C
          │            │            │
       channel       channel      channel
          │            │            │
        React        React        React
```

No React polling is necessary.

This is exactly the architecture I think you should aim for.

---

# 9. UserContext should come through the same channel

This is also important given our Version 3.5 discussion.

When the channel connects:

```text
Cockpit authenticated user
        │
        ▼
cockpit-slurm-channel
        │
        ▼
cockpit-slurm-bridge
        │
        ▼
UserContext
```

The bridge should establish:

```go
type UserContext struct {
    Username    string
    UID         uint32
    GID         uint32
    AdminLevel  AdminLevel
    Coordinator []Account
    Capabilities CapabilitySet
    Accounts    []string
}
```
https://slurm.schedmd.com/sacctmgr.html#OPT_coordinator, if the value to Coordinator is nil, the user is not a coordinator.

Then React can receive something like:

```json
{
  "type": "user-context",
  "payload": {
    "username": "alice",
    "uid": 1001,
    "adminLevel": "operator",
    "capabilities": [
      "jobs.view",
      "jobs.cancel",
      "nodes.view",
      "nodes.modify"
    ]
  }
}
```

This can drive your four UI experiences:

```text
Not Set
    ↓
Visitor Home

None
    ↓
User Home

Coordinator
    ↓
Coordinator Home

Operator
    ↓
Operator Home

Admin
    ↓
Administrator Home

```

But the **bridge remains the authorization authority**. React should use the capabilities to construct the UI, while the bridge independently checks authorization before executing every command.

---

# 10. Suggested Go project structure

For your current repository, I would aim toward something like:

```text
cockpit-slurm/
│
├── cmd/
│   │
│   ├── cockpit-slurm-bridge/
│   │   └── main.go
│   │
│   └── cockpit-slurm-channel/
│       └── main.go
│
├── internal/
│   │
│   ├── ipc/
│   │   ├── protocol/
│   │   ├── server/
│   │   └── client/
│   │
│   ├── identity/
│   │   ├── user.go
│   │   ├── context.go
│   │   └── authorization.go
│   │
│   ├── service/
│   │   ├── command/
│   │   ├── query/
│   │   ├── resource/
│   │   └── subscription/
│   │
│   ├── resource/
│   │   ├── account/
│   │   ├── user/
│   │   ├── job/
│   │   ├── node/
│   │   └── partition/
│   │
│   ├── cache/
│   │
│   ├── event/
│   │
│   ├── adapter/
│   │   ├── command/
│   │   └── resource/
│   │
│   └── models/
│
├── pkg/
├── src
├── manifest.json
├── package.json
└── ...
```

I would **not** put cache/event/resource logic into `cockpit-slurm-channel`.

---

# 11. One issue we should investigate before implementing the channel

There is one architectural question I would resolve first:

> **Does `cockpit-slurm-channel` need to be one process per Cockpit channel, or should it be reused for multiple matching channels?**

Cockpit's package bridge mechanism explicitly says matching bridge processes can be reused. ([Cockpit Project][1])

For your architecture, I currently lean toward:

```text
Cockpit channel
       │
       ▼
cockpit-slurm-channel
       │
       ▼
one bridge.sock connection
```

with **one channel process/connection corresponding to one Cockpit channel**, because it makes:

* subscription ownership
* disconnect handling
* UserContext
* message correlation
* cleanup

very straightforward.

The long-lived shared state remains exclusively in:

```text
cockpit-slurm-bridge
```

So you get:

```text
                  cockpit-slurm-bridge
                 /        |        \
                /         |         \
         Client A     Client B    Client C
            │             │           │
         channel       channel     channel
            │             │           │
         React         React       React
```

while all three clients share the **same authoritative cache and Event Bus** in the bridge.

---

## Recommended next step

I suggest we now design `cockpit-slurm-channel` **from the outside inward**, rather than starting with Go code:

1. **Cockpit `manifest.json` bridge configuration**
2. Exact `cockpit.channel()` options
3. Cockpit channel ↔ channel executable protocol
4. `cockpit-slurm-channel` process lifecycle
5. Configuration file and socket-path resolution
6. Unix-socket client implementation in Go
7. IPC framing and message forwarding
8. Connection/disconnection handling
9. UserContext propagation
10. Error/close propagation
11. Tests using a fake `cockpit-slurm-bridge`
12. Integration with the real bridge

That will let us keep `cockpit-slurm-channel` deliberately small—ideally a **thin Go transport/protocol adapter**, while almost all of the intelligence remains in `cockpit-slurm-bridge`. This is consistent with Cockpit's documented model where package bridges are additional server-side processes selected from `manifest.json` for matching channel opens. ([Cockpit Project][1])

[1]: https://cockpit-project.org/guide/latest/packages.html?utm_source=chatgpt.com "Cockpit Packages"
[2]: https://cockpit-project.org/guide/latest/cockpit-channels.html?utm_source=chatgpt.com "cockpit.js: Raw Channels"

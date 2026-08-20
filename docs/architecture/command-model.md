# cockpit-slurm Command Model

**location:** docs/architecture/command-model.md
**Version:** 1.0  
**Architecture:** cockpit-slurm v3.5  
**Status:** Architecture Contract  
**Scope:** Bridge Command Service, Command Adapter, IPC Command Messages, Authorization, Audit, and Command Lifecycle

---

The important architectural principle is that **Command Model should be the semantic contract for operations**, while `ipc-protocol.md` defines how a command is transported, `resource-model.md` defines what the target resource means, `event-model.md` defines the resulting state change, and `user-context.md` defines who is allowed to execute it. This is also consistent with the implementation plan's distinction between command processing and resource-state events. 


# 1. Purpose

This document defines the canonical command model for the `cockpit-slurm` architecture.

The Command Model describes how an authenticated user requests a state-changing operation against a Slurm resource.

It defines:

- command identity
- command lifecycle
- command structure
- command targets
- operations
- parameters
- authorization
- validation
- idempotency
- command execution
- command results
- error handling
- audit information
- relationship between commands and resource events

The Command Model is independent of the transport mechanism.

The IPC protocol defines how a command is transported between:

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
```

This document defines what the command means once it reaches the Bridge.

---

# 2. Architectural Position

The command system belongs to the Bridge.

```text
┌──────────────────────────────┐
│          React UI            │
│                              │
│  Button / Form / Action      │
└──────────────┬───────────────┘
               │
               │ command
               ▼
┌──────────────────────────────┐
│ cockpit-slurm-channel        │
│                              │
│ transport only               │
└──────────────┬───────────────┘
               │
               │ IPC
               ▼
┌──────────────────────────────┐
│ cockpit-slurm-bridge         │
│                              │
│  UserContext                 │
│       ↓                      │
│  Authorization               │
│       ↓                      │
│  Command Service             │
│       ↓                      │
│  Command Adapter             │
│       ↓                      │
│  Slurm                       │
│       ↓                      │
│  Resource Adapter             │
│       ↓                      │
│  Canonical Resource          │
│       ↓                      │
│  Cache                       │
│       ↓                      │
│  Event                       │
└──────────────┬───────────────┘
               │
               ▼
             Slurm
```

The Channel must not implement command semantics.

The Channel must not:

* execute Slurm commands
* perform authorization
* modify resources
* maintain command state
* access the cache
* implement command retries
* generate resource events

The Bridge owns these responsibilities.

---

# 3. Command Principle

A command represents:

> A request by an authenticated user to cause an operation against a Slurm resource.

For example:

```text
cancel Job/12345
```

or:

```text
suspend Job/12345
```

or:

```text
create Account/research
```

A command is an **intent**.

It is not itself a resource state change.

Therefore:

```text
Command
   ≠
Resource Event
```

A successful command may cause a resource state change.

For example:

```text
Command
  cancel Job/12345
       ↓
Slurm operation
       ↓
Job becomes CANCELLED
       ↓
Resource refresh
       ↓
Cache update
       ↓
Job Modified Event
```

The command says:

```text
"Please perform this operation."
```

The event says:

```text
"The resource state has changed."
```

This distinction must be preserved.

---

# 4. Command and Resource Model

Commands operate on canonical resources.

The relationship is:

```text
Command
   │
   ├── operation
   │
   ├── resource kind
   │
   ├── resource target
   │
   └── parameters
            │
            ▼
      Canonical Resource
```

For example:

```text
operation = cancel
resource  = Job
target    = Job/12345
```

The canonical resource model defines resource identity and representation.

A command must not use Slurm-specific API models as its public contract.

For example, the command should not expose:

```text
slurm_openapi_job_info
```

as its target model.

Instead:

```text
resource = Job
target   = Job/12345
```

Slurm-specific models belong inside the Adapter layer.

---

# 5. Command Structure

The canonical command contains:

```text
commandId
user
operation
resource
target
parameters
expectedResourceVersion
metadata
createdAt
```

A conceptual Go representation is:

```go
type Command struct {
    CommandID              string
    Operation              CommandOperation
    Resource              ResourceKind
    Target                ResourceReference
    Parameters             map[string]any
    ExpectedResourceVersion string
    Metadata               CommandMetadata
    CreatedAt              time.Time
}
```

The actual implementation may use strongly typed command parameters instead of:

```go
map[string]any
```

for operations that require strict validation.

---

# 6. Command Identity

Every command MUST contain a globally unique:

```text
commandId
```

Example:

```text
01JABC123COMMAND456
```

A UUID or ULID may be used.

ULID is recommended because it provides:

* uniqueness
* sortable timestamps
* compact representation
* useful logging characteristics

Example:

```json
{
  "commandId": "01K2ABCD123456789XYZ"
}
```

The `commandId` identifies the logical operation.

It is different from:

```text
messageId
connectionId
subscriptionId
```

---

# 7. Identifier Relationships

The architecture uses several identifiers.

| Identifier       | Purpose                              |
| ---------------- | ------------------------------------ |
| `connectionId`   | identifies an IPC connection         |
| `messageId`      | identifies one IPC message           |
| `commandId`      | identifies one logical command       |
| `subscriptionId` | identifies one resource subscription |
| `eventId`        | identifies one resource event        |

Example:

```text
connectionId
    CON001
       │
       ├── messageId MSG001
       │       └── commandId CMD001
       │
       └── messageId MSG002
               └── commandId CMD002
```

A command may be retried over another connection while retaining the same:

```text
commandId
```

This is important for idempotency.

---

# 8. Command Operations

Operations are resource-specific.

Common operations include:

```text
create
update
delete
cancel
suspend
resume
hold
release
allocate
deallocate
```

The implementation should not assume that every operation applies to every resource.

For example:

| Resource    | Operation      |
| ----------- | -------------- |
| Job         | cancel         |
| Job         | suspend        |
| Job         | resume         |
| Account     | create         |
| Account     | update         |
| Account     | delete         |
| User        | create         |
| User        | update         |
| User        | delete         |
| Association | create         |
| Association | update         |
| Association | delete         |
| QOS         | create         |
| QOS         | update         |
| QOS         | delete         |
| Node        | update/limited |
| Partition   | update/limited |

The actual capability matrix is defined by the resource registry.

---

# 9. Operation Semantics

## 9.1 Create

Creates a new Slurm resource.

Example:

```text
create Account/research
```

The command must contain the required specification.

Example:

```json
{
  "operation": "create",
  "resource": "accounts",
  "parameters": {
    "name": "research",
    "description": "Research account"
  }
}
```

The command does not directly insert the resource into the cache.

Instead:

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
Created Event
```

---

# 10. Update

Updates an existing resource.

Example:

```text
update Account/research
```

Example:

```json
{
  "operation": "update",
  "resource": "accounts",
  "target": {
    "name": "research"
  },
  "parameters": {
    "description": "Updated research account"
  }
}
```

The Bridge must validate:

* resource exists
* requested fields are mutable
* user is authorized
* parameters are valid
* optional optimistic-concurrency version matches

---

# 11. Delete

Deletes a resource.

Example:

```text
delete Account/research
```

Example:

```json
{
  "operation": "delete",
  "resource": "accounts",
  "target": {
    "name": "research"
  }
}
```

Deletion is considered successful only after Slurm accepts the operation.

The cache must not be deleted optimistically.

Correct:

```text
Command
   ↓
Authorization
   ↓
Validation
   ↓
Slurm
   ↓
Refresh
   ↓
Cache.Delete()
   ↓
Deleted Event
```

Incorrect:

```text
Command
   ↓
Cache.Delete()
   ↓
Slurm
```

The cache represents observed Slurm state.

---

# 12. Job Commands

Jobs are special because their commands are usually operational rather than CRUD operations.

Initial Job command operations should include:

```text
cancel
suspend
resume
```

Additional operations can be added later.

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "MSG001",
  "type": "command",
  "payload": {
    "commandId": "CMD001",
    "operation": "cancel",
    "resource": "jobs",
    "target": {
      "name": "12345"
    }
  }
}
```

The Bridge then performs:

```text
UserContext
     ↓
Authorization
     ↓
Command Validation
     ↓
Command Service
     ↓
Job Command Adapter
     ↓
Slurm
```

The resulting Job state change is subsequently detected by the resource synchronization mechanism.

---

# 13. Command Target

A command normally operates on a target resource.

A target should use the canonical resource identity.

Example:

```json
{
  "target": {
    "kind": "Job",
    "name": "12345"
  }
}
```

For resources requiring a namespace or cluster:

```json
{
  "target": {
    "kind": "Account",
    "name": "research",
    "namespace": "cluster-a"
  }
}
```

A Go representation:

```go
type ResourceReference struct {
    APIVersion string
    Kind       string
    Namespace  string
    Name       string
}
```

The exact fields must remain consistent with `resource-model.md`.

---

# 14. Command Parameters

Parameters contain operation-specific input.

Example:

```json
{
  "operation": "create",
  "resource": "accounts",
  "parameters": {
    "name": "research",
    "description": "Research account"
  }
}
```

Parameters must be validated by the Command Service before reaching Slurm.

Validation occurs at two levels.

## 14.1 Structural validation

Examples:

```text
required parameter exists
parameter type is correct
parameter value is within allowed range
```

## 14.2 Semantic validation

Examples:

```text
account does not already exist
target job exists
partition exists
user has access to requested account
```

Slurm remains the final authority for Slurm-specific validation.

---

# 15. Command Lifecycle

A command has a lifecycle.

Recommended states:

```text
accepted
   ↓
authorized
   ↓
validated
   ↓
running
   ↓
completed
```

Failure paths include:

```text
accepted
   ↓
rejected
```

or:

```text
accepted
   ↓
authorized
   ↓
validation-failed
```

or:

```text
accepted
   ↓
authorized
   ↓
running
   ↓
failed
```

Recommended status set:

```text
accepted
running
completed
failed
rejected
cancelled
timeout
```

The Bridge may internally use additional states, but the IPC contract should expose a small stable set.

---

# 16. Command State Machine

```text
                    ┌───────────────┐
                    │    received   │
                    └───────┬───────┘
                            │
                            ▼
                    ┌───────────────┐
                    │   accepted    │
                    └───────┬───────┘
                            │
                     authorization
                       /          \
                    deny          allow
                     │              │
                     ▼              ▼
                rejected       validated
                                    │
                                    ▼
                                 running
                                  /    \
                                 /      \
                                ▼        ▼
                           completed    failed
```

A command should not transition backward.

---

# 17. Accepted vs Completed

An important distinction is:

```text
accepted
```

does not necessarily mean:

```text
completed
```

For example:

```text
command received
      ↓
authorization successful
      ↓
command accepted
      ↓
Slurm operation executing
      ↓
completed
```

For asynchronous commands the initial response may therefore be:

```json
{
  "commandId": "CMD001",
  "status": "accepted"
}
```

The UI can subsequently query command status if required.

---

# 18. Command Response

A command response is an IPC message.

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "MSG002",
  "type": "command-response",
  "payload": {
    "commandId": "CMD001",
    "status": "completed"
  }
}
```

A successful command response may additionally contain:

```text
resource reference
result
Slurm job ID
command output
```

Example:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "MSG002",
  "type": "command-response",
  "payload": {
    "commandId": "CMD001",
    "status": "completed",
    "resource": {
      "kind": "Job",
      "name": "12345"
    }
  }
}
```

---

# 19. Command Response Is Not a Resource Event

The command response and resource event have different meanings.

```text
command-response
        │
        └── tells the client about command processing
```

while:

```text
event
   │
   └── tells subscribers about observed resource state changes
```

Example:

```text
cancel Job/12345
       │
       ▼
command-response
       │
       │
       ▼
Job state changes
       │
       ▼
resource event
```

Therefore:

```text
command-response ≠ resource event
```

This is a fundamental architectural rule.

---

# 20. Command and Event Relationship

The normal write path is:

```text
React
  ↓
Command
  ↓
Command Service
  ↓
Authorization
  ↓
Command Adapter
  ↓
Slurm
  ↓
Resource synchronization
  ↓
Canonical Resource
  ↓
Cache
  ↓
Event
  ↓
Subscription Manager
  ↓
Channel
  ↓
React
```

The Command Service must not manufacture the final resource state.

---

# 21. Cache Invariant

Commands must never directly modify the authoritative resource cache.

Do not implement:

```text
Command
   ↓
Cache.Update()
   ↓
Slurm
```

Implement:

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

This ensures that the cache represents actual Slurm state rather than the desired state.

This is consistent with the v3.5 architecture, where the cache is the authoritative representation of observed Slurm state and command processing does not directly mutate it. 

---

# 22. Authorization

Authorization is performed by the Bridge.

The frontend may determine whether to display an operation, but the Bridge must always perform the authoritative authorization check.

The flow is:

```text
Command
   ↓
UserContext
   ↓
Capability Resolver
   ↓
Authorization
```

For example:

```text
jobs.cancel
```

or:

```text
accounts.create
accounts.modify
accounts.delete
```

The Bridge must not trust capabilities supplied by React.

The authoritative UserContext is established by the Bridge.

---

# 23. Capability-Based Authorization

Commands should be authorized using capabilities rather than hard-coded role checks.

Prefer:

```text
can("jobs.cancel")
```

over:

```text
adminLevel == "admin"
```

The same principle applies to:

```text
nodes.modify
jobs.cancel
jobs.suspend
jobs.resume
accounts.create
accounts.modify
accounts.delete
users.create
users.modify
users.delete
```

This allows the implementation to evolve independently of the UI's role presentation.

---

# 24. AdminLevel and Coordinator

`AdminLevel` and Coordinator status are separate concepts.

For example:

```text
AdminLevel:
    none
    operator
    admin
```

while:

```text
Coordinator:
    account-A
    account-B
```

A user can therefore be:

```text
AdminLevel = none
Coordinator = [research]
```

and may have:

```text
accounts.modify
```

within the permitted Coordinator scope without becoming an Operator or Administrator.

The Command Service must evaluate:

```text
UserContext
+
Capability
+
Resource scope
```

---

# 25. Scope Authorization

Some commands require resource scope evaluation.

Example:

```text
Alice
AdminLevel = none
Coordinator = [research]
```

Command:

```text
modify Account/research
```

may be allowed.

But:

```text
modify Account/finance
```

may be denied.

Therefore authorization is not simply:

```text
operation allowed?
```

It is:

```text
user
+
operation
+
resource
+
resource scope
```

---

# 26. Command Validation Pipeline

The Bridge should process commands in the following order:

```text
Receive
   ↓
Decode
   ↓
Validate command envelope
   ↓
Check commandId
   ↓
Check idempotency
   ↓
Resolve UserContext
   ↓
Authorize
   ↓
Validate target
   ↓
Validate parameters
   ↓
Check resource version
   ↓
Execute
   ↓
Return command result
```

A command must not reach Slurm before authorization succeeds.

---

# 27. Idempotency

Every command must contain a `commandId`.

The Bridge must retain command identity for a configurable retention period.

If the same command is received again:

```text
same commandId
+
same logical command
```

the Bridge must not execute the Slurm operation again.

Instead it should return the existing command state.

Example:

```text
CMD001
   ↓
cancel Job/12345
   ↓
completed
```

Later:

```text
CMD001
   ↓
received again
```

The Bridge returns:

```text
completed
```

rather than executing another cancellation.

This is particularly important when the Channel reconnects or retries a request. The existing IPC architecture already establishes `commandId` as the idempotency key. 

---

# 28. Idempotency Record

The Bridge may maintain:

```go
type CommandRecord struct {
    CommandID   string
    User        string
    Operation   CommandOperation
    Resource    ResourceReference
    Status      CommandStatus
    Result      any
    Error       *CommandError
    CreatedAt   time.Time
    StartedAt   *time.Time
    CompletedAt *time.Time
}
```

The record may be stored:

```text
in memory
```

initially.

A persistent command journal can be introduced later if required.

---

# 29. Command Retention

The initial implementation should use bounded retention.

For example:

```text
command result retention = configurable
```

After expiration:

```text
commandId no longer known
```

A repeated command after expiration may be treated as a new command.

However, the implementation must document this behavior clearly.

For dangerous operations such as:

```text
delete
cancel
```

the client should not blindly retry after the retention period.

---

# 30. Optimistic Concurrency

Commands that modify persistent resources may optionally include:

```text
expectedResourceVersion
```

Example:

```json
{
  "operation": "update",
  "resource": "accounts",
  "target": {
    "name": "research"
  },
  "expectedResourceVersion": "42",
  "parameters": {
    "description": "New description"
  }
}
```

The Bridge compares:

```text
expectedResourceVersion
```

with the current cache resource version.

If they differ:

```text
conflict
```

is returned.

This prevents a stale UI from unintentionally overwriting a newer resource state.

---

# 31. Resource Version vs Generation

The Command Model must distinguish:

```text
generation
```

from:

```text
resourceVersion
```

Generally:

```text
generation
    = logical resource state generation

resourceVersion
    = version identifying the particular cached representation
```

The exact semantics are defined by `resource-model.md`.

A command may use `resourceVersion` for optimistic concurrency.

---

# 32. Command Errors

Command errors should use the common error model rather than inventing resource-specific error formats.

Typical categories include:

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

Examples:

```json
{
  "commandId": "CMD001",
  "status": "rejected",
  "error": {
    "code": "authorization-error",
    "message": "User is not authorized to cancel this job"
  }
}
```

Or:

```json
{
  "commandId": "CMD002",
  "status": "failed",
  "error": {
    "code": "slurm-error",
    "message": "Unable to cancel job"
  }
}
```

---

# 33. Authorization Error vs Slurm Error

These must remain distinguishable.

Example:

```text
User does not have permission
```

is:

```text
authorization-error
```

while:

```text
Slurm rejected the operation
```

is:

```text
slurm-error
```

This allows the UI to provide appropriate behavior.

For example:

```text
authorization-error
    → permission message

slurm-error
    → Slurm failure message

timeout
    → retry/recovery message
```

---

# 34. Timeout

Commands must have a bounded execution timeout.

For example:

```text
command
   ↓
running
   ↓
timeout
```

A timeout does not necessarily mean that Slurm did not execute the operation.

Therefore the Bridge should distinguish:

```text
command execution timeout
```

from:

```text
confirmed Slurm failure
```

After a timeout, the Bridge should normally refresh the affected resource before reporting final state to the UI.

---

# 35. Asynchronous Commands

Some commands may take longer to complete.

The Bridge may support asynchronous execution:

```text
command
   ↓
accepted
```

followed by:

```text
running
```

and later:

```text
completed
```

The client may obtain status using a command-status query.

Example:

```json
{
  "type": "query-command",
  "payload": {
    "commandId": "CMD001"
  }
}
```

This mechanism is optional for the initial implementation.

---

# 36. Command Status Query

A future command-status API may return:

```json
{
  "commandId": "CMD001",
  "status": "completed",
  "createdAt": "2026-08-20T10:00:00Z",
  "completedAt": "2026-08-20T10:00:01Z"
}
```

The command status is independent of resource events.

---

# 37. Audit Information

Every state-changing command should produce an audit record.

At minimum:

```text
timestamp
username
uid
commandId
connectionId
messageId
operation
resource
target
status
```

Example:

```text
2026-08-20T10:00:01Z
user=alice
command=CMD001
operation=cancel
resource=Job
target=12345
status=completed
```

Sensitive parameters must not be logged indiscriminately.

---

# 38. Audit vs Event

Audit records and resource events have different purposes.

Audit:

```text
Who requested what?
```

Event:

```text
What resource state changed?
```

For example:

```text
Audit:
Alice requested cancel Job/12345

Event:
Job/12345 changed from RUNNING to CANCELLED
```

One command can therefore produce:

```text
one audit record
+
zero or more resource events
```

---

# 39. Command Correlation

The command should be traceable through the entire system.

Recommended correlation:

```text
connectionId
     │
     ▼
messageId
     │
     ▼
commandId
     │
     ▼
Slurm operation
     │
     ▼
resource generation
     │
     ▼
eventId
```

This is especially useful for debugging:

```text
Why did Job/12345 change?
```

The system should be able to correlate:

```text
event
 → resource change
 → command
 → user
```

when the change originated from a command.

---

# 40. Command Cause

Resource events may optionally contain a cause:

```text
user-command
scheduler
external-change
periodic-refresh
reconciliation
```

For a command-driven change:

```text
Event
    cause.type = user-command
    cause.commandId = CMD001
```

This creates a useful relationship:

```text
Command CMD001
       ↓
Slurm operation
       ↓
Job event EVT001
```

The Event Model remains responsible for the semantics of the event.

---

# 41. Command Result

A command result should contain only information necessary to report command execution.

Example:

```json
{
  "commandId": "CMD001",
  "status": "completed",
  "result": {
    "accepted": true
  }
}
```

If the operation creates a resource with a known identity:

```json
{
  "commandId": "CMD002",
  "status": "completed",
  "result": {
    "resource": {
      "kind": "Account",
      "name": "research"
    }
  }
}
```

The complete resource representation should normally be obtained through the resource query/event mechanism rather than embedded unnecessarily in the command response.

---

# 42. Create Command Example

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "MSG001",
  "type": "command",
  "payload": {
    "commandId": "CMD001",
    "operation": "create",
    "resource": "accounts",
    "parameters": {
      "name": "research",
      "description": "Research account"
    }
  }
}
```

Processing:

```text
React
  ↓
Channel
  ↓
Bridge
  ↓
UserContext
  ↓
Authorization
  ↓
Validation
  ↓
Account Command Adapter
  ↓
sacctmgr / Slurm API
  ↓
Refresh
  ↓
Account Adapter
  ↓
Canonical Account
  ↓
Cache
  ↓
Created Event
```

---

# 43. Update Command Example

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "MSG010",
  "type": "command",
  "payload": {
    "commandId": "CMD010",
    "operation": "update",
    "resource": "accounts",
    "target": {
      "kind": "Account",
      "name": "research"
    },
    "parameters": {
      "description": "Updated research account"
    }
  }
}
```

---

# 44. Delete Command Example

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "MSG020",
  "type": "command",
  "payload": {
    "commandId": "CMD020",
    "operation": "delete",
    "resource": "accounts",
    "target": {
      "kind": "Account",
      "name": "research"
    }
  }
}
```

---

# 45. Job Cancel Example

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "MSG030",
  "type": "command",
  "payload": {
    "commandId": "CMD030",
    "operation": "cancel",
    "resource": "jobs",
    "target": {
      "kind": "Job",
      "name": "12345"
    }
  }
}
```

Flow:

```text
React
  ↓
command CMD030
  ↓
Bridge authorization
  ↓
Job Command Adapter
  ↓
Slurm
  ↓
Job state changes
  ↓
Cache
  ↓
Job Modified Event
```

The command response may be:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "MSG031",
  "type": "command-response",
  "payload": {
    "commandId": "CMD030",
    "status": "completed"
  }
}
```

The resource event is separate:

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "EVT100",
  "type": "event",
  "payload": {
    "subscriptionId": "SUB001",
    "event": "modified",
    "resource": "jobs",
    "generation": 43,
    "item": {
      "kind": "Job",
      "name": "12345",
      "status": {
        "state": "CANCELLED"
      }
    }
  }
}
```

---

# 46. Command Service

The Bridge should provide a Command Service.

Conceptual interface:

```go
type CommandService interface {
    Execute(
        ctx context.Context,
        command Command,
    ) CommandResult
}
```

The Command Service is responsible for:

```text
command validation
authorization
idempotency
dispatch
execution
result handling
audit
```

It should not directly implement Slurm-specific operations.

---

# 47. Command Handler

A resource-specific command handler may be introduced.

For example:

```go
type CommandHandler interface {
    Execute(
        ctx context.Context,
        command Command,
    ) CommandResult
}
```

Handlers may be registered by operation/resource:

```text
Job.cancel
Job.suspend
Job.resume

Account.create
Account.update
Account.delete

User.create
User.update
User.delete
```

Example registry:

```go
type CommandRegistry interface {
    Register(
        resource ResourceKind,
        operation CommandOperation,
        handler CommandHandler,
    )

    Resolve(
        resource ResourceKind,
        operation CommandOperation,
    ) (CommandHandler, error)
}
```

---

# 48. Command Adapter

The Command Adapter translates the canonical command into a Slurm-specific operation.

```text
Canonical Command
       ↓
Command Adapter
       ↓
Slurm CLI / slurmrestd
```

For example:

```text
Command:
    resource = Job
    operation = cancel
    target = 12345
```

may become:

```text
scancel 12345
```

or an equivalent Slurm API request.

The Slurm command/API implementation must remain inside the adapter.

---

# 49. Command Adapter Must Not Update Cache

The Command Adapter must return execution results.

It must not call:

```go
cache.Update(...)
```

The correct architecture is:

```text
Command Adapter
       ↓
Slurm
       ↓
result
```

then:

```text
Resource Adapter
       ↓
canonical resource
       ↓
cache
```

This maintains a single authoritative resource-state path.

---

# 50. Command Dispatcher

The dispatcher maps:

```text
resource + operation
```

to the appropriate handler.

Example:

```go
handler, err := registry.Resolve(
    command.Resource,
    command.Operation,
)
```

Then:

```text
handler.Execute()
```

Unknown combinations must produce:

```text
validation-error
```

or:

```text
unsupported-operation
```

---

# 51. Command Permissions

The capability required by a command should be explicit.

Example:

| Resource    | Operation | Capability            |
| ----------- | --------- | --------------------- |
| Node        | update    | `nodes.modify`        |
| Job         | cancel    | `jobs.cancel`         |
| Job         | suspend   | `jobs.suspend`        |
| Job         | resume    | `jobs.resume`         |
| Account     | create    | `accounts.create`     |
| Account     | update    | `accounts.modify`     |
| Account     | delete    | `accounts.delete`     |
| User        | create    | `users.create`        |
| User        | update    | `users.modify`        |
| User        | delete    | `users.delete`        |
| Association | create    | `associations.create` |
| Association | update    | `associations.modify` |
| Association | delete    | `associations.delete` |

The capability resolver may later account for:

```text
AdminLevel
Coordinator
Linux identity
resource scope
```

---

# 52. Command and Coordinator Authorization

Coordinator authorization is resource-scoped.

Example:

```text
User:
    alice

AdminLevel:
    none

Coordinator:
    research
```

Command:

```text
update Account/research
```

may be allowed.

Command:

```text
update Account/finance
```

must be rejected.

Therefore:

```text
Capability
+
Resource Scope
```

are both required.

---

# 53. Command and Linux Identity

The Bridge establishes the authenticated Linux/Cockpit identity.

The command must not trust:

```json
{
  "username": "admin"
}
```

supplied by the frontend.

The Bridge obtains the authoritative UserContext and evaluates authorization against it.

The Channel is a transport layer and does not become an authorization authority.

---

# 54. Command Retry

The client may retry a command only using the same:

```text
commandId
```

Example:

```text
attempt 1:
    CMD001

connection lost

attempt 2:
    CMD001
```

The Bridge recognizes:

```text
CMD001
```

and returns the existing command status.

The client must not generate:

```text
CMD002
```

for a retry of the same logical operation unless it intentionally wants to execute a new command.

---

# 55. Command Cancellation

Command cancellation is different from cancellation of the Slurm resource.

For example:

```text
cancel command
```

does not necessarily mean:

```text
cancel Slurm job
```

These are separate concepts.

A future IPC operation may allow:

```text
cancel-command
```

which attempts to stop a still-running command execution.

The Slurm operation:

```text
operation = cancel
resource = Job
```

means:

```text
cancel the Job
```

These must not be confused.

---

# 56. Command Concurrency

Multiple commands may execute concurrently.

Example:

```text
CMD001 → Job/12345 cancel
CMD002 → Job/12346 cancel
CMD003 → Account/research update
```

Commands targeting unrelated resources may execute concurrently.

Commands targeting the same resource may require serialization or conflict detection.

Example:

```text
CMD010 → Account/research update description=A
CMD011 → Account/research update description=B
```

The implementation should use:

```text
resourceVersion
```

or another concurrency mechanism where appropriate.

---

# 57. Command Ordering

Commands are not automatically globally ordered.

The Bridge should preserve ordering where required by resource semantics.

For example:

```text
Job/12345
    suspend
    resume
```

should not result in an unexpected execution order.

The implementation may use per-resource serialization.

Global command serialization should be avoided because it unnecessarily reduces scalability.

---

# 58. Command Transaction Boundary

A command represents one logical requested operation.

It is not necessarily a distributed transaction.

For example:

```text
create Account
```

may involve:

```text
sacctmgr
```

followed by:

```text
resource refresh
```

The Bridge should not pretend that the entire sequence is an ACID transaction.

The command result indicates the status of command processing.

Resource events indicate the observed resulting state.

---

# 59. Command and Resource Refresh

After a successful state-changing command, the Bridge should ensure that the affected resource is refreshed.

Preferred flow:

```text
Command
   ↓
Slurm
   ↓
successful
   ↓
resource refresh
   ↓
canonical resource
   ↓
cache
   ↓
event
```

If the normal background synchronization detects the change, the explicit refresh may be unnecessary.

The implementation can therefore support:

```text
command-triggered refresh
```

and:

```text
normal reconciliation
```

without changing the public command model.

---

# 60. Command Success Does Not Guarantee Event Timing

A command may complete before the corresponding resource event is delivered.

For example:

```text
t0  command submitted
t1  Slurm accepts command
t2  command-response = completed
t3  resource refresh
t4  cache update
t5  resource event
```

The UI must therefore not assume that:

```text
command-response
```

and:

```text
event
```

arrive simultaneously.

---

# 61. Command Failure and Resource Events

A failed command normally should not produce a resource event caused by that command.

Example:

```text
cancel Job/12345
       ↓
Slurm rejects operation
       ↓
command-response = failed
```

No command-induced resource event should be generated.

However, an unrelated external Slurm change may still produce an event.

---

# 62. External Resource Changes

A resource can change without a cockpit-slurm command.

Examples:

```text
Slurm scheduler
slurmctld
another administrator
command-line user
external application
```

Therefore:

```text
resource event
```

does not necessarily imply:

```text
command
```

The event model may identify the cause as:

```text
scheduler
external-change
reconciliation
```

when no local command caused it.

---

# 63. Command and Event Causality

When a command causes a resource event:

```text
Command CMD001
       ↓
Slurm
       ↓
Resource Event EVT001
```

the event may contain:

```json
{
  "cause": {
    "type": "user-command",
    "commandId": "CMD001"
  }
}
```

This is optional in the initial implementation but strongly recommended for auditability.

---

# 64. Command Model and IPC Protocol

`ipc-protocol.md` defines:

```text
message envelope
messageId
message type
transport
framing
request/response
```

This document defines:

```text
command semantics
command lifecycle
operation
target
parameters
authorization
idempotency
execution
result
```

Therefore:

```text
ipc-protocol.md
       ↓
transport contract

command-model.md
       ↓
command semantic contract
```

A command is transported as:

```text
type = command
```

and its result as:

```text
type = command-response
```

---

# 65. Example IPC Command Envelope

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "MSG001",
  "type": "command",
  "resource": "jobs",
  "payload": {
    "commandId": "CMD001",
    "operation": "cancel",
    "target": {
      "kind": "Job",
      "name": "12345"
    }
  }
}
```

The exact envelope remains governed by `ipc-protocol.md`.

---

# 66. Command Response Envelope

```json
{
  "protocol": "cockpit-slurm",
  "version": "1.0",
  "messageId": "MSG002",
  "type": "command-response",
  "payload": {
    "commandId": "CMD001",
    "status": "completed"
  }
}
```

---

# 67. Command Model and Event Model

The relationship is:

```text
Command
   ↓
operation
   ↓
Slurm
   ↓
Resource change
   ↓
Cache
   ↓
Event
```

The Event Model defines:

```text
Created
Modified
Deleted
generation
sequence
eventId
snapshot
recovery
```

The Command Model defines:

```text
intent
execution
status
result
failure
idempotency
audit
```

Therefore the two models must remain separate.

---

# 68. Command Model and UserContext

`user-context.md` defines:

```text
username
UID
GID
AdminLevel
Coordinator
accounts
capabilities
resource scope
```

The Command Model uses this information during authorization.

Relationship:

```text
UserContext
     +
Command
     |
     v
Authorization
     |
     +---- denied
     |
     v
Command Execution
```

The Command Model must not duplicate UserContext semantics.

---

# 69. Command Model and Resource Model

`resource-model.md` defines:

```text
apiVersion
kind
metadata
spec
status
generation
resourceVersion
ResourceRef
```

The Command Model references resources using:

```text
ResourceRef
```

rather than embedding complete resources.

Example:

```json
{
  "target": {
    "kind": "Account",
    "name": "research"
  }
}
```

not:

```json
{
  "target": {
    "kind": "Account",
    "name": "research",
    "spec": {
      "... complete resource ..."
    }
  }
}
```

unless the operation specifically requires a new resource specification.

---

# 70. Command Model Invariants

The following invariants MUST be maintained.

## Invariant 1

Every command has a unique:

```text
commandId
```

## Invariant 2

The Bridge is authoritative for command authorization.

## Invariant 3

The Channel never performs command authorization.

## Invariant 4

Commands do not directly mutate the resource cache.

## Invariant 5

Resource events represent observed resource state changes.

## Invariant 6

Duplicate command IDs must not cause duplicate execution within the retention period.

## Invariant 7

Command responses and resource events are separate message types.

## Invariant 8

Command targets use canonical resource identity.

## Invariant 9

Slurm-specific models remain inside adapters.

## Invariant 10

AdminLevel and Coordinator remain separate concepts.

## Invariant 11

A command failure must not be represented as a successful resource event.

## Invariant 12

The frontend must never be the authoritative authorization mechanism.

---

# 71. Initial Command Registry

The initial implementation should focus on commands required by the first vertical slices.

| Resource    | Operation | Initial Priority |
| ----------- | --------- | ---------------: |
| Job         | cancel    |               P0 |
| Job         | suspend   |               P0 |
| Job         | resume    |               P0 |
| Account     | create    |               P1 |
| Account     | update    |               P1 |
| Account     | delete    |               P1 |
| User        | create    |               P1 |
| User        | update    |               P1 |
| User        | delete    |               P1 |
| Association | create    |               P2 |
| Association | update    |               P2 |
| Association | delete    |               P2 |
| QOS         | create    |               P2 |
| QOS         | update    |               P2 |
| QOS         | delete    |               P2 |
| Node        | update    |               P2 |
| Partition   | update    |               P2 |

The registry is an implementation capability matrix.

It is not a promise that every Slurm version supports every operation.

---

# 72. First Implementation Scope

Version 1.0 should implement:

```text
Command
CommandID
Operation
Resource
Target
Parameters
CommandStatus
CommandResult
CommandError
Authorization
Idempotency
Audit correlation
```

Initial operations:

```text
Job.cancel
Job.suspend
Job.resume
```

Then:

```text
Account.create
Account.update
Account.delete
```

Then:

```text
User.create
User.update
User.delete
```

---

# 73. Recommended Development Sequence

## Phase 1 — Command Domain Model

Implement:

```text
CommandID
CommandOperation
CommandStatus
Command
CommandResult
CommandError
CommandRecord
```

---

## Phase 2 — Command Registry

Implement:

```text
Register(resource, operation, handler)
Resolve(resource, operation)
```

Start with:

```text
Job.cancel
Job.suspend
Job.resume
```

---

## Phase 3 — Command Service

Implement:

```text
receive
validate
authorize
idempotency
dispatch
execute
result
audit
```

---

## Phase 4 — Mock Command Adapter

Before connecting to Slurm:

```text
Command
   ↓
Mock Adapter
   ↓
Command Result
```

This allows the complete IPC path to be tested without Slurm.

---

## Phase 5 — Real Job Command Adapter

Implement:

```text
Job.cancel
Job.suspend
Job.resume
```

using the selected Slurm adapter.

---

## Phase 6 — Cache/Event Integration

Verify:

```text
Command
   ↓
Slurm
   ↓
Resource refresh
   ↓
Cache
   ↓
Event
```

Do not allow the command adapter to directly modify the cache.

---

## Phase 7 — Idempotency

Test:

```text
CMD001
   ↓
execute
   ↓
connection lost
   ↓
CMD001 retry
   ↓
existing result
```

and verify:

```text
Slurm operation executed only once
```

---

## Phase 8 — Authorization

Test:

```text
Admin
Operator
Coordinator
User
Visitor / Not Set
```

against command capabilities and resource scope.

---

## Phase 9 — Audit

Verify that every state-changing command can be correlated through:

```text
connectionId
messageId
commandId
username
resource
operation
status
```

---

# 74. Recommended Go Package Structure

The command implementation should be separated from IPC transport.

Recommended:

```text
internal/
├── ipc/
│   ├── protocol.go
│   ├── message.go
│   └── server.go
│
├── protocol/
│   ├── command.go
│   └── error.go
│
├── command/
│   ├── command.go
│   ├── service.go
│   ├── registry.go
│   ├── handler.go
│   ├── record.go
│   ├── idempotency.go
│   └── errors.go
│
├── authorization/
│   └── authorizer.go
│
├── audit/
│   └── audit.go
│
├── resource/
├── cache/
├── event/
├── subscription/
└── adapter/
    ├── job/
    ├── account/
    ├── user/
    └── association/
```

The distinction is:

```text
ipc/
    transport

protocol/
    message definitions

command/
    command semantics

authorization/
    permission evaluation

adapter/
    Slurm-specific execution
```

---

# 75. Testing Requirements

Command tests must be executable without Slurm.

Unit tests should cover:

```text
✓ command encoding
✓ command decoding
✓ required fields
✓ invalid operation
✓ invalid target
✓ invalid parameters
✓ authorization denied
✓ authorization allowed
✓ command ID generation
✓ duplicate command ID
✓ command lifecycle
✓ command timeout
✓ Slurm error
✓ command result
✓ audit correlation
✓ resource-version conflict
```

---

# 76. Idempotency Tests

Test:

```text
CMD001
    ↓
execute
    ↓
completed
```

Then:

```text
CMD001
    ↓
retry
```

Expected:

```text
completed
```

and:

```text
Slurm execution count = 1
```

---

# 77. Authorization Tests

Example:

```text
User Alice
AdminLevel = none
Coordinator = research
```

Test:

```text
update Account/research
```

Expected:

```text
allowed
```

Test:

```text
update Account/finance
```

Expected:

```text
authorization-error
```

---

# 78. Command/Event Integration Test

A complete integration test should execute:

```text
1. connect
2. establish UserContext
3. submit command
4. receive command-response
5. simulate Slurm resource change
6. update cache
7. generate event
8. receive event
9. verify commandId/event correlation
```

Example:

```text
CMD001
cancel Job/12345
       ↓
command-response
       ↓
Job/12345 changed
       ↓
EVT001
       ↓
React
```

---

# 79. Complete Command Flow

The final architecture should be:

```text
                    React
                      │
                      │ command
                      ▼
             cockpit.channel()
                      │
                      ▼
          cockpit-slurm-channel
                      │
                      │ IPC
                      ▼
          cockpit-slurm-bridge
                      │
                      ▼
                UserContext
                      │
                      ▼
                Authorization
                      │
                      ▼
               Command Service
                      │
             ┌────────┴────────┐
             │                 │
             ▼                 ▼
        Idempotency       Validation
             │                 │
             └────────┬────────┘
                      ▼
               Command Handler
                      │
                      ▼
               Command Adapter
                      │
                      ▼
                    Slurm
                      │
                      ▼
              Resource Adapter
                      │
                      ▼
             Canonical Resource
                      │
                      ▼
                    Cache
                      │
                      ▼
                    Event
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

---

# 80. Architectural Summary

The key rule of the Command Model is:

```text
Command = requested operation
Resource = observed state
Event = observed state change
```

Therefore:

```text
             Command
                │
                │ intent
                ▼
              Slurm
                │
                │ actual state
                ▼
        Canonical Resource
                │
                ▼
              Cache
                │
                │ change
                ▼
              Event
```

The command system must never become a second source of truth for resource state.

The Bridge is responsible for:

```text
authentication
authorization
validation
idempotency
command execution
command status
audit
```

The Slurm Adapter is responsible for translating:

```text
Canonical Command
        ↓
Slurm operation
```

The Resource Adapter is responsible for translating:

```text
Slurm state
        ↓
Canonical Resource
```

The Cache is responsible for:

```text
authoritative observed state
```

The Event system is responsible for:

```text
propagating observed state changes
```

The Channel is responsible only for:

```text
transport
```

and React is responsible for:

```text
presentation
user interaction
command initiation
resource display
event consumption
```

---

# 81. Relationship to Other Architecture Documents

The v3.5 architecture should therefore be viewed as:

```text
                  user-context.md
                         │
                         ▼
                  Authorization
                         │
                         ▼
ipc-protocol.md ──► command-model.md
                         │
                         ▼
                  resource-model.md
                         │
                         ▼
                    cache
                         │
                         ▼
                   event-model.md
                         │
                         ▼
                    React UI
```

More precisely:

```text
ipc-protocol.md
    = HOW messages travel

user-context.md
    = WHO is making the request

command-model.md
    = WHAT operation is requested

resource-model.md
    = WHAT resource is being operated on

event-model.md
    = WHAT state change actually occurred
```

This separation should be kept as a core v3.5 architectural invariant.

````

### One recommendation before you commit it

I would make **`command-model.md` the fourth contract after `ipc-protocol.md`, `resource-model.md`, and `user-context.md`**, exactly as the implementation plan intends. The implementation plan explicitly identifies these contracts as the architecture that should be frozen before substantial coding. :contentReference[oaicite:4]{index=4}

I would also make one small adjustment to the overall documentation structure:

```text
docs/architecture/
├── ipc-protocol.md
├── resource-model.md
├── user-context.md
├── event-model.md
├── command-model.md       ← this document
└── error-model.md         ← next
````

The **next document I recommend writing is `error-model.md`**. It should be relatively short, because the command model above already establishes the main error categories. Then the six architecture contracts are complete:

```text
IPC
 │
 ├── Resource
 ├── UserContext
 ├── Command
 ├── Event
 └── Error
```

That will give you a clean foundation for implementing the Bridge's `internal/command`, `internal/authorization`, `internal/event`, and `internal/ipc` packages. The current v3.5 implementation plan specifically recommends this separation and then moving into the Bridge IPC skeleton, Channel transport, and the Node vertical slice. 

[1]: https://github.com/lingweicai/cockpit-slurm/tree/development-bridge-v3.5/docs/architecture "cockpit-slurm/docs/architecture at development-bridge-v3.5 · lingweicai/cockpit-slurm · GitHub"

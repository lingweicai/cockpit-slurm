# cockpit-slurm User Context

**Status:** Proposed  
**Version:** 1.0  
**Project:** cockpit-slurm

Related documents:

- `implementation-plan-v3.5.md`
- `ipc-protocol.md`
- `resource-model.md`
- `development-cockpit-slurm-bridge_v3.5.md`
- `development-cockpit-slurm-channel-v3.5.md`
- `development-cockpit-slurm-UI-v2.md`

---

# 1. Purpose

This document defines the User Context model for `cockpit-slurm`.

UserContext represents the identity and effective authorization context of the user connected to the cockpit-slurm service.

The model is used by:

- `cockpit-slurm-bridge`
- IPC authorization
- Query Service
- Command Service
- Resource Service
- Subscription Manager
- React UI
- audit logging
- role-based navigation
- role-based home pages

The most important architectural principle is:

> The Bridge is authoritative for UserContext and authorization.

The React frontend must never determine authoritative Slurm permissions by itself.

The overall architecture is:

```
    Cockpit authenticated user
              |
              v
    cockpit-slurm-channel
              |
              v
    cockpit-slurm-bridge
              |
              +-- Linux identity
              |
              +-- Slurm identity
              |
              +-- AdminLevel
              |
              +-- Coordinator relationships
              |
              +-- authorization policy
              |
              v
         UserContext
              |
              +-- Query authorization
              +-- Command authorization
              +-- Subscription authorization
              +-- UI capabilities
              +-- Audit identity
```

---

# 2. Architectural Principle

UserContext is a server-side security object.

The frontend may receive a representation of UserContext, but it must be treated as:

```
display / capability information
```

rather than:

```
authoritative authorization information
```

The authoritative decision is always made by the Bridge.

For example:

```
React
   |
   | cancel Job/12345
   v
Channel
   |
   v
Bridge
   |
   +-- UserContext
   |
   +-- Authorization
   |
   v
allowed / denied
```

The frontend may hide the Cancel button when:

```
jobs.cancel
```

is absent from the advertised capabilities.

However, the Bridge must still perform the authorization check when the command arrives.

---

# 3. Relationship with Cockpit Authentication

Cockpit authenticates the user before the cockpit-slurm channel is created.

The architecture is:

```
Browser
   |
   | Cockpit login
   v
Cockpit
   |
   | authenticated session
   v
cockpit.channel()
   |
   v
cockpit-slurm-channel
   |
   v
cockpit-slurm-bridge
```

The Channel is not responsible for deciding Slurm privileges.

The Bridge obtains the effective identity available to it through the Cockpit/channel execution context and the local operating-system identity.

The exact mechanism used to transfer the authenticated identity must be verified during implementation on the target Cockpit version.

The Bridge must never trust a username supplied only as an arbitrary field in a JSON request.

For example, this must NOT be considered authoritative:

```
{
  "username": "alice"
}
```

The authenticated operating-system/session identity is authoritative.

---

# 4. Identity Layers

UserContext consists of several identity layers.

```
┌─────────────────────────────┐
│ Cockpit identity            │
└──────────────┬──────────────┘
               |
               v
┌─────────────────────────────┐
│ Linux identity              │
│ uid / gid / groups          │
└──────────────┬──────────────┘
               |
               v
┌─────────────────────────────┐
│ Slurm User                  │
│ username / AdminLevel       │
└──────────────┬──────────────┘
               |
               v
┌─────────────────────────────┐
│ Slurm associations          │
│ account / cluster / user    │
│ / partition                 │
└──────────────┬──────────────┘
               |
               v
┌─────────────────────────────┐
│ Coordinator relationships   │
└──────────────┬──────────────┘
               |
               v
┌─────────────────────────────┐
│ Effective capabilities      │
└─────────────────────────────┘
```

These layers must not be collapsed into one role field.

---

# 5. Linux Identity

Linux identity represents the operating-system identity under which the Bridge request is authorized.

Example:

```json
{
  "uid": 1001,
  "gid": 1001,
  "username": "alice",
  "groups": [
    "users",
    "hpc"
  ]
}
```

The Linux identity may contain:

| Field      | Description          |
| ---------- | -------------------- |
| `username` | Linux login name     |
| `uid`      | numeric user ID      |
| `gid`      | primary group ID     |
| `groups`   | supplementary groups |

Linux groups may be relevant to local deployment policy.

For example:

```
users
hpc
wheel
```

However, Linux group membership must not automatically be interpreted as a Slurm AdminLevel.

---

# 6. Linux Groups vs Slurm AdminLevel

These are separate authorization domains.

For example:

```
Linux:
    group = wheel
```

does not automatically mean:

```
Slurm:
    AdminLevel = admin
```

Similarly:

```
Slurm:
    AdminLevel = admin
```

does not necessarily mean:

```
Linux:
    group = wheel
```

The Bridge may use both sources when implementing site-specific authorization policy, but the relationship must be explicit.

---

# 7. Slurm User Identity

A Slurm User is represented by the canonical `User` resource defined in `resource-model.md`.

The important distinction is:

```
Linux User
    |
    +-- operating-system identity

Slurm User
    |
    +-- Slurm accounting identity
```

These are normally related by username but are conceptually different.

Example:

```text
Linux username:
alice

Slurm username:
alice
```

The Bridge should not assume that every Linux user necessarily has a Slurm accounting record.

---

# 8. User Not Present in Slurm

A Linux user may exist without a corresponding Slurm User.

Example:

```text
Linux:
    alice

Slurm:
    User alice does not exist
```

The UserContext should therefore distinguish:

```
Linux identity exists
```

from:

```
Slurm accounting identity exists
```

Example:

```json
{
  "linux": {
    "username": "alice",
    "uid": 1001
  },
  "slurm": {
    "exists": false
  }
}
```

Such a user may still be able to access Cockpit, but their Slurm capabilities may be limited.

---

# 9. AdminLevel

Slurm AdminLevel is an attribute of the Slurm User.

The canonical values are:

```text
none
operator
admin
```

The Bridge should normalize Slurm's representation to these canonical lowercase values.

Example:

```json
{
  "slurm": {
    "username": "alice",
    "adminLevel": "operator"
  }
}
```

AdminLevel is global with respect to the Slurm accounting context.

It must not be confused with Coordinator.

---

# 10. AdminLevel Semantics

Conceptually:

```
none
   |
   v
operator
   |
   v
admin
```

However, the Bridge must not implement authorization merely as:

```text
adminLevel >= requiredLevel
```

because many permissions are resource- and operation-specific.

Instead:

```
UserContext
   +
resource
   +
operation
   +
resource scope
   |
   v
authorization decision
```

---

# 11. Coordinator

Coordinator is an account-scoped privilege.

It must not be modeled as:

```json
{
  "adminLevel": "coordinator"
}
```

Instead:

```json
{
  "adminLevel": "none",
  "coordinatorAccounts": [
    "research",
    "chemistry"
  ]
}
```

A user may therefore have:

```
AdminLevel = none
```

and still be:

```
Coordinator of research
```

This distinction is fundamental to the authorization architecture.

---

# 12. Coordinator Scope

Coordinator privileges must be evaluated against the account being operated on.

Example:

```text
User:
    alice

AdminLevel:
    none

Coordinator:
    research
```

Then:

```text
Account research
    |
    +-- coordinator privilege
```

while:

```text
Account teaching
    |
    +-- no coordinator privilege
```

Therefore:

```
alice + modify research account
```

may be allowed while:

```
alice + modify teaching account
```

may be denied.

---

# 13. Association Context

A user's Slurm associations provide additional authorization context.

An association is identified by:

```
cluster
account
user
partition
```

For example:

```text
cluster = cluster01
account = research
user = alice
partition = gpu
```

The UserContext should therefore be able to reference the user's associations.

Example:

```json
{
  "slurm": {
    "username": "alice",
    "associations": [
      {
        "cluster": "cluster01",
        "account": "research",
        "partition": "gpu"
      },
      {
        "cluster": "cluster01",
        "account": "research",
        "partition": "compute"
      }
    ]
  }
}
```

The complete Association resource remains a separate canonical resource.

UserContext should contain only the information needed for authorization and presentation.

---

# 14. Effective Role

The UI may use a derived role.

The role is not itself the source of truth.

Example:

```text
Admin
Operator
Coordinator
User
Visitor
```

The effective role is derived from:

```
authenticated identity
      +
Slurm User
      +
AdminLevel
      +
Coordinator relationships
      +
site policy
      +
resource authorization
```

---

# 15. Recommended Role Model

The initial UI role hierarchy should be:

```text
Not Set / Visitor
        |
        v
      User
        |
        +---- Coordinator
        |
        +---- Operator
        |
        +---- Admin
```

However, Coordinator is not strictly a higher global role than User.

It is better represented as:

```text
User
 |
 +-- Coordinator(account-scoped)
```

while:

```text
Operator
Admin
```

are global Slurm AdminLevel values.

Therefore the effective role should be represented as attributes rather than a single enum.

---

# 16. Why a Single Role Enum Is Insufficient

Avoid:

```typescript
type Role =
    | "visitor"
    | "user"
    | "coordinator"
    | "operator"
    | "admin";
```

as the authoritative backend model.

It loses information.

For example:

```text
alice
AdminLevel = none
Coordinator = research
```

cannot be represented accurately by a single global role.

Prefer:

```typescript
interface UserContext {
    identity: IdentityContext;
    slurm: SlurmIdentityContext;
    authorization: AuthorizationContext;
}
```

with:

```typescript
interface SlurmIdentityContext {
    adminLevel: "none" | "operator" | "admin";
    coordinatorAccounts: string[];
}
```

---

# 17. Capabilities

The Bridge should expose normalized capabilities.

Examples:

```text
nodes.view
nodes.modify

jobs.view
jobs.cancel
jobs.suspend
jobs.resume
jobs.submit

partitions.view
partitions.modify

accounts.view
accounts.create
accounts.modify
accounts.delete

users.view
users.create
users.modify
users.delete

associations.view
associations.create
associations.modify
associations.delete

qos.view
qos.modify

reservations.view
reservations.create
reservations.modify
reservations.delete
```

Capabilities are application-level authorization identifiers.

They are not direct copies of Slurm command names.

---

# 18. Capability Naming Convention

Use:

```text
<resource>.<action>
```

Examples:

```text
jobs.view
jobs.cancel
accounts.view
accounts.modify
users.modify
reservations.create
```

Avoid exposing command-specific authorization identifiers such as:

```text
scontrol.cancel
sacctmgr.modify
```

The UI should express intent rather than implementation.

---

# 19. Resource-Scoped Capabilities

Capabilities may be evaluated against a resource scope.

For example:

```text
accounts.modify
```

may mean:

```text
modify any account
```

for Admin.

For a Coordinator:

```text
accounts.modify
```

may mean:

```text
modify accounts for which this user is Coordinator
```

Therefore a capability alone is not sufficient.

The authorization function should be conceptually:

```text
Authorize(
    UserContext,
    action,
    resource
)
```

---

# 20. Authorization Decision

The Bridge should produce a structured decision.

Example:

```json
{
  "allowed": true,
  "capability": "jobs.cancel",
  "scope": {
    "job": "12345"
  }
}
```

For denial:

```json
{
  "allowed": false,
  "reason": {
    "code": "FORBIDDEN",
    "message": "User is not authorized to cancel this job."
  }
}
```

The client should not need to understand Slurm's internal permission calculation.

---

# 21. Authorization Layers

Authorization should be evaluated in layers.

```text
1. Connection authentication
        |
        v
2. UserContext construction
        |
        v
3. Capability check
        |
        v
4. Resource scope check
        |
        v
5. Slurm operation
        |
        v
6. Slurm authorization
```

The Bridge should perform its own authorization before issuing the Slurm operation.

Slurm remains the final enforcement layer where applicable.

---

# 22. Defense in Depth

The architecture should assume that the frontend can be manipulated.

Therefore:

```text
React UI
    |
    | "delete account"
    v
IPC
    |
    v
Bridge authorization
    |
    +-- allowed --> Slurm
    |
    +-- denied --> error
```

The following is NOT sufficient:

```text
React hides Delete button
```

UI visibility is convenience.

Bridge authorization is security.

---

# 23. Query Authorization

Queries must also be authorized.

For example:

```text
GET jobs
```

may be allowed for all users.

But:

```text
GET administrative configuration
```

may be restricted.

The Query Service should therefore receive:

```text
UserContext
```

and apply:

```text
resource visibility
+
field visibility
+
filter restrictions
```

when necessary.

---

# 24. Command Authorization

Commands require stronger authorization.

Example:

```text
cancel Job/12345
```

Flow:

```text
Command
    |
    v
UserContext
    |
    v
Authorize
    |
    +---- denied
    |
    +---- allowed
             |
             v
        Command Adapter
             |
             v
           Slurm
```

The Command Service must never assume that because a resource is visible, it can also be modified.

---

# 25. Subscription Authorization

Subscriptions are also authorization-controlled.

For example:

```text
subscribe jobs
```

may be permitted.

But an administrative event stream may not be.

The Subscription Manager should therefore associate:

```text
subscriptionId
connectionId
UserContext
resource
filter
generation
```

The existing Bridge design already models subscriptions around connection, resource kind, filter, and generation. UserContext should be part of the authorization context of that subscription.

---

# 26. UserContext and Subscription Lifecycle

The lifecycle becomes:

```text
Connect
   |
   v
Create UserContext
   |
   v
Authorize subscribe
   |
   v
Create Subscription
   |
   v
Snapshot
   |
   v
Events
   |
   v
Disconnect
   |
   v
Destroy UserContext-bound subscriptions
```

A subscription must never outlive its owning IPC connection.

---

# 27. Connection ID vs UserContext

`connectionId` identifies the IPC connection.

UserContext identifies the authenticated security context.

They are related but not identical.

```text
connectionId
    |
    +-- UserContext
    |
    +-- subscriptions
    |
    +-- messages
    |
    +-- commands
```

The IPC protocol already distinguishes:

```text
connectionId
messageId
commandId
subscriptionId
```

UserContext should be attached to the connection internally rather than repeated in every request.

---

# 28. Never Trust User Identity in Request Payload

A request such as:

```json
{
  "type": "command",
  "payload": {
    "username": "root",
    "operation": "delete-account"
  }
}
```

must not cause the Bridge to execute the command as root.

The identity comes from:

```text
authenticated connection
```

not:

```text
request.username
```

If a request contains a username for business purposes, it must be treated as data, not authentication.

---

# 29. Root

Root is a Linux identity.

Example:

```text
uid = 0
username = root
```

The Bridge may recognize root as a privileged operating-system identity.

However, root should not be modeled as a special Slurm resource kind.

Instead:

```text
Linux identity
    username = root
    uid = 0
```

is used by the authorization policy.

The exact mapping:

```text
root -> admin
```

should be an explicit deployment policy rather than an implicit assumption embedded throughout the application.

---

# 30. SlurmUser

`SlurmUser` is the Unix account under which Slurm services operate.

It is not automatically the Cockpit user.

The Bridge must distinguish:

```text
Cockpit user
    |
    +-- authenticated interactive user

SlurmUser
    |
    +-- service identity
```

The Bridge may use SlurmUser credentials/permissions to communicate with Slurm.

This does not mean that every Cockpit user becomes SlurmUser.

---

# 31. Privileged Backend vs User Authorization

A particularly important distinction is:

```text
process privilege
```

versus:

```text
request authorization
```

The Bridge may run with sufficient operating-system privileges to communicate with Slurm.

That does NOT mean every connected user is authorized to perform every operation.

The architecture must remain:

```text
Bridge process privilege
          |
          v
       Slurm API
          ^
          |
UserContext authorization
          ^
          |
     IPC request
```

---

# 32. UserContext Structure

Recommended conceptual structure:

```go
type UserContext struct {
    ConnectionID string

    Identity     IdentityContext
    Linux        LinuxIdentity
    Slurm        SlurmIdentity

    Authorization AuthorizationContext

    CreatedAt time.Time
}
```

Supporting types:

```go
type IdentityContext struct {
    Username string
}

type LinuxIdentity struct {
    Username string
    UID      uint32
    GID      uint32
    Groups   []string
}

type SlurmIdentity struct {
    Username            string
    Exists              bool
    AdminLevel          AdminLevel
    CoordinatorAccounts []string
    Associations        []AssociationRef
}

type AuthorizationContext struct {
    Capabilities []string
}
```

---

# 33. AdminLevel Type

Recommended Go type:

```go
type AdminLevel string

const (
    AdminLevelNone     AdminLevel = "none"
    AdminLevelOperator AdminLevel = "operator"
    AdminLevelAdmin    AdminLevel = "admin"
)
```

Unknown values must not silently become `admin`.

For example:

```text
unknown
```

should produce a safe failure or a non-privileged state.

---

# 34. Capability Type

Recommended:

```go
type Capability string
```

Example:

```go
const (
    CapabilityNodesView       Capability = "nodes.view"
    CapabilityJobsView        Capability = "jobs.view"
    CapabilityJobsCancel      Capability = "jobs.cancel"
    CapabilityJobsSuspend     Capability = "jobs.suspend"
    CapabilityJobsResume      Capability = "jobs.resume"

    CapabilityAccountsView    Capability = "accounts.view"
    CapabilityAccountsCreate  Capability = "accounts.create"
    CapabilityAccountsModify  Capability = "accounts.modify"
    CapabilityAccountsDelete  Capability = "accounts.delete"
)
```

The capability registry should be centralized.

---

# 35. UserContext Creation

UserContext should be created when the IPC connection is established.

Flow:

```text
Channel connects
      |
      v
Bridge accepts connection
      |
      v
Authenticate / identify peer
      |
      v
Load Linux identity
      |
      v
Resolve Slurm User
      |
      v
Load AdminLevel
      |
      v
Load Coordinator relationships
      |
      v
Calculate capabilities
      |
      v
Create UserContext
```

The context remains associated with the connection.

---

# 36. UserContext Refresh

Some identity attributes may change while a connection is active.

Examples:

```text
AdminLevel changed
Coordinator relationship changed
account association changed
```

The Bridge therefore needs a refresh strategy.

For the first implementation:

```text
UserContext created at connection
```

and:

```text
refresh when necessary
```

is sufficient.

A future implementation may support:

```text
UserContext generation
```

and dynamic authorization refresh.

---

# 37. UserContext Generation

Optionally:

```json
{
  "generation": 17
}
```

can identify the current authorization context.

Example:

```text
UserContext generation 17
       |
       | AdminLevel changed
       v
UserContext generation 18
```

This is useful for long-lived subscriptions and sessions.

It is separate from resource generation.

---

# 38. UserContext vs Resource Generation

Do not confuse:

```text
UserContext generation
```

with:

```text
Resource generation
```

For example:

```text
UserContext generation = 3

Node generation = 102

Job generation = 508

Account generation = 21
```

They have different purposes.

---

# 39. UserContext Snapshot

The Bridge may return a sanitized UserContext snapshot to the frontend.

Example:

```json
{
  "username": "alice",
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

The frontend can use this for:

* navigation
* home page selection
* button visibility
* explanatory messages
* capability-aware components

---

# 40. Do Not Expose Everything

The frontend does not necessarily need:

```text
uid
gid
all Linux groups
internal authorization rules
SlurmUser
backend credentials
policy implementation
```

The IPC response should expose only the sanitized information needed by the UI.

---

# 41. UI Role Selection

The UI can derive its home page from the sanitized UserContext.

Example:

```text
AdminLevel = admin
        |
        v
Admin Home

AdminLevel = operator
        |
        v
Operator Home

AdminLevel = none
Coordinator accounts != empty
        |
        v
Coordinator Home

AdminLevel = none
Slurm User exists
        |
        v
User Home

Slurm User does not exist
        |
        v
Not Set / Visitor Home
```

This directly supports the role-based home-page architecture.

---

# 42. Recommended Home Page Decision

The frontend should use a deterministic function:

```typescript
function getHomePage(context: UserContext): HomePage {
    if (context.slurm.adminLevel === "admin")
        return "admin";

    if (context.slurm.adminLevel === "operator")
        return "operator";

    if (context.slurm.coordinatorAccounts.length > 0)
        return "coordinator";

    if (context.slurm.exists)
        return "user";

    return "not-set";
}
```

This is a UI decision.

The Bridge still remains authoritative for actual operations.

---

# 43. Coordinator Home Page

A Coordinator home page should be account-scoped.

Example:

```text
Coordinator Home
    |
    +-- research
        |
        +-- Users
        +-- Associations
        +-- Jobs
        +-- Usage
        +-- Limits
```

The UI should not show unrestricted global administration simply because the user is a Coordinator.

---

# 44. Admin Home Page

Admin users may receive global administration navigation.

Example:

```text
Admin Home
    |
    +-- Cluster
    +-- Nodes
    +-- Partitions
    +-- Jobs
    +-- Accounts
    +-- Users
    +-- Associations
    +-- QOS
    +-- Reservations
    +-- Configuration
    +-- Audit
```

Actual access remains enforced by the Bridge.

---

# 45. Operator Home Page

Operator users should receive operational navigation.

Example:

```text
Operator Home
    |
    +-- Overview
    +-- Nodes
    +-- Jobs
    +-- Partitions
    +-- Reservations
    +-- Monitoring
```

Administrative account-management functions may remain unavailable.

---

# 46. User Home Page

A normal Slurm user may receive:

```text
User Home
    |
    +-- My Jobs
    +-- Submit Job
    +-- My Accounts
    +-- Usage
    +-- Job History
```

The exact permissions are capability-driven.

---

# 47. Not Set / Visitor Home Page

If the authenticated Cockpit user has no usable Slurm identity:

```text
Slurm User:
    not found
```

the UI can display:

```text
Not Set
```

or:

```text
Visitor
```

home page.

This page should not imply that the user has Slurm administrative permissions.

---

# 48. Capability-Driven Navigation

Navigation should ultimately be capability-driven.

Example:

```text
if capabilities includes:
    accounts.modify

show:
    Users & Accounts
        Accounts
        Associations
```

while:

```text
jobs.view
```

may enable:

```text
Workloads
    Jobs
```

This prevents the UI from hard-coding every role combination.

---

# 49. Resource Authorization

Authorization should be evaluated using the canonical resource model.

Example:

```text
UserContext
    |
    +-- adminLevel
    +-- coordinatorAccounts
    +-- capabilities
            |
            v
Canonical Resource
    |
    +-- Account
    +-- User
    +-- Job
    +-- Association
            |
            v
Operation
            |
            v
Authorization decision
```

This is why `resource-model.md` and `user-context.md` must be designed together.

---

# 50. Example: Cancel Own Job

Suppose:

```text
alice
AdminLevel = none
```

and:

```text
Job/12345
user = alice
```

Request:

```text
cancel Job/12345
```

The authorization service may evaluate:

```text
jobs.cancel
+
job.user == context.username
```

and allow the operation according to the deployment's policy.

---

# 51. Example: Cancel Another User's Job

Suppose:

```text
alice
AdminLevel = none
```

and:

```text
Job/12346
user = bob
```

Request:

```text
cancel Job/12346
```

The Bridge may deny:

```text
FORBIDDEN
```

unless the user's Slurm privileges permit the operation.

The UI must not be trusted to enforce this restriction.

---

# 52. Example: Operator

Suppose:

```text
alice
AdminLevel = operator
```

Then:

```text
alice
    |
    +-- jobs.view
    +-- jobs.cancel
    +-- jobs.suspend
    +-- jobs.resume
    +-- nodes.view
    +-- nodes.modify
```

depending on the project's policy mapping.

The exact capability matrix should be defined separately from the UserContext data model.

---

# 53. Example: Admin

Suppose:

```text
alice
AdminLevel = admin
```

The effective capability set may include:

```text
nodes.*
jobs.*
partitions.*
accounts.*
users.*
associations.*
qos.*
reservations.*
```

Again, this is a policy mapping rather than a hard-coded consequence of the resource model.

---

# 54. Example: Coordinator

Suppose:

```text
alice
AdminLevel = none
Coordinator = research
```

Then:

```text
alice
    |
    +-- accounts.view
    +-- accounts.modify
    +-- users.view
    +-- associations.modify
```

may be available only when:

```text
resource.account == research
```

This is the key distinction between global AdminLevel and account-scoped Coordinator authorization.

---

# 55. Authorization API

Recommended internal interface:

```go
type Authorizer interface {
    Authorize(
        ctx context.Context,
        user UserContext,
        action Action,
        resource ResourceRef,
    ) Decision
}
```

Example:

```go
decision := authorizer.Authorize(
    ctx,
    userContext,
    ActionJobsCancel,
    ResourceRef{
        Kind: "Job",
        Name: "12345",
    },
)

if !decision.Allowed {
    return ErrForbidden
}
```

---

# 56. Authorization Decision

Recommended:

```go
type Decision struct {
    Allowed    bool
    Capability Capability
    Reason     string
    Scope      *ResourceScope
}
```

For debugging/audit:

```go
type AuthorizationReason struct {
    Code   string
    Detail string
}
```

---

# 57. Policy Layer

The policy should be separate from UserContext.

Recommended:

```text
UserContext
      |
      v
Authorization Policy
      |
      v
Decision
```

Do not place policy rules directly inside:

```text
UserContext
```

UserContext describes the user.

Policy determines what that user may do.

---

# 58. Example Policy

Conceptually:

```go
func authorize(
    user UserContext,
    action Action,
    resource Resource,
) Decision {

    if user.Slurm.AdminLevel == AdminLevelAdmin {
        return Allow()
    }

    if user.Slurm.AdminLevel == AdminLevelOperator {
        return operatorPolicy(user, action, resource)
    }

    if isCoordinator(user, resource) {
        return coordinatorPolicy(user, action, resource)
    }

    return userPolicy(user, action, resource)
}
```

This is illustrative only.

The production implementation should avoid scattering authorization rules throughout controllers.

---

# 59. Authorization Centralization

Authorization should be centralized.

Avoid:

```text
NodeController
    own authorization

JobController
    different authorization

AccountController
    another authorization
```

Prefer:

```text
Controllers
    |
    v
Authorization Service
    |
    v
Policy
```

This makes the system easier to audit and test.

---

# 60. Audit Context

Every command should carry the effective UserContext into the audit layer.

Example:

```json
{
  "commandId": "cmd-123",
  "username": "alice",
  "uid": 1001,
  "adminLevel": "operator",
  "operation": "cancel",
  "resource": "Job/12345",
  "decision": "allowed"
}
```

The audit record should identify:

```text
who
what
when
which resource
authorization result
command result
```

---

# 61. Audit vs Authentication

Do not use:

```text
username
```

alone as the complete audit identity.

Prefer:

```text
connectionId
username
uid
adminLevel
commandId
timestamp
```

where appropriate.

This makes multiple concurrent Cockpit sessions distinguishable.

---

# 62. Multiple Sessions

The same user may open multiple Cockpit tabs.

Example:

```text
alice
 |
 +-- connection-001
 |
 +-- connection-002
 |
 +-- connection-003
```

Each connection has:

```text
connectionId
UserContext
subscriptions
```

but the underlying identity may be the same.

The Bridge must therefore distinguish:

```text
user identity
```

from:

```text
connection identity
```

---

# 63. UserContext Lifetime

Recommended lifetime:

```text
IPC connection established
        |
        v
UserContext created
        |
        v
Requests / subscriptions / commands
        |
        v
IPC connection closed
        |
        v
UserContext discarded
```

The persistent Bridge does not need a permanent UserContext for every possible Linux user.

It maintains contexts only for active connections.

---

# 64. Subscription Ownership

Each subscription belongs to a connection.

Therefore:

```text
Connection
    |
    +-- UserContext
    |
    +-- Subscription A
    +-- Subscription B
```

When the connection closes:

```text
Connection closed
      |
      +-- destroy UserContext-bound subscriptions
      +-- cancel pending operations if appropriate
      +-- release resources
```

---

# 65. Command Ownership

A command also belongs to the connection that submitted it.

Example:

```text
connection-001
      |
      +-- command-001
```

The command record should retain enough identity information for auditing even after the connection closes.

---

# 66. Command Idempotency

The command cache should use:

```text
commandId
```

for duplicate detection.

The command cache design in Bridge v3.5 already identifies:

```text
Pending
Running
Succeeded
Failed
```

states and recommends short retention for command tracking.

UserContext should be recorded with the command but should not be used as the command's unique identity.

---

# 67. Error Handling

Authorization failures should use stable error codes.

Recommended:

```text
UNAUTHENTICATED
FORBIDDEN
NO_SLURM_USER
INVALID_USER_CONTEXT
CAPABILITY_REQUIRED
RESOURCE_SCOPE_FORBIDDEN
```

Example:

```json
{
  "code": "FORBIDDEN",
  "message": "Operation is not permitted."
}
```

Do not expose unnecessary internal policy details.

---

# 68. UserContext in IPC

The IPC protocol should NOT require every message to contain the complete UserContext.

Bad:

```json
{
  "username": "alice",
  "uid": 1001,
  "adminLevel": "admin",
  "capabilities": [...]
}
```

in every request.

Instead:

```text
IPC connection
      |
      +-- connectionId
      +-- UserContext
```

The Bridge attaches the context internally.

---

# 69. Optional UserContext Response

The frontend may request:

```text
get-context
```

or receive UserContext during:

```text
hello-response
```

Example:

```json
{
  "type": "hello-response",
  "payload": {
    "connectionId": "01CONNECTION001",
    "userContext": {
      "username": "alice",
      "adminLevel": "operator",
      "coordinatorAccounts": [
        "research"
      ],
      "capabilities": [
        "nodes.view",
        "jobs.view",
        "jobs.cancel"
      ]
    }
  }
}
```

The existing IPC protocol already establishes UserContext during the connection handshake; this document makes its semantics explicit.

---

# 70. Sanitized Frontend Context

The frontend-facing context should be smaller than the internal context.

Internal:

```text
UserContext
 ├── Linux identity
 ├── Slurm identity
 ├── policy
 ├── capabilities
 └── connection metadata
```

Frontend:

```text
UserContextView
 ├── username
 ├── adminLevel
 ├── coordinatorAccounts
 └── capabilities
```

This reduces information leakage.

---

# 71. TypeScript Model

Recommended:

```typescript
export type AdminLevel =
    | "none"
    | "operator"
    | "admin";

export interface UserContextView {
    username: string;
    adminLevel: AdminLevel;
    slurmUserExists: boolean;
    coordinatorAccounts: string[];
    capabilities: string[];
}
```

Optional:

```typescript
export interface UserContextState {
    context?: UserContextView;
    loading: boolean;
    error?: Error;
}
```

---

# 72. React UserContext Provider

The UI should have a dedicated provider.

Example:

```text
UserContextProvider
        |
        +-- user identity
        +-- admin level
        +-- coordinator accounts
        +-- capabilities
        |
        +-- Navigation
        +-- Home page
        +-- Page authorization
        +-- Action buttons
```

It should consume the Bridge-provided context rather than reconstructing it from separate Slurm queries.

---

# 73. Example React Usage

Conceptually:

```tsx
const { context } = useUserContext();

if (context?.capabilities.includes("jobs.cancel")) {
    // display cancel action
}
```

But the corresponding backend command still performs:

```text
Authorize(
    UserContext,
    jobs.cancel,
    Job/12345
)
```

---

# 74. Capability Helper

The frontend may provide:

```typescript
function can(
    context: UserContextView,
    capability: string,
): boolean {
    return context.capabilities.includes(capability);
}
```

Usage:

```tsx
{can(context, "jobs.cancel") && (
    <Button>Cancel</Button>
)}
```

This is presentation logic, not security enforcement.

---

# 75. UserContext and Resource Visibility

Capabilities may determine which navigation sections are visible.

For example:

```text
jobs.view
    |
    v
Workloads

accounts.view
    |
    v
Users & Accounts

nodes.view
    |
    v
Compute
```

But resource filtering may also be required.

A Coordinator may have:

```text
accounts.view
```

but only for:

```text
research
```

---

# 76. Coordinator Resource Filtering

For Coordinator users, Query Service may need to restrict:

```text
accounts
users
associations
jobs
usage
```

to account scope.

Example:

```text
Coordinator:
    research
```

Query:

```text
jobs
```

may become internally:

```text
jobs where account in ["research"]
```

The frontend should not be responsible for applying this security filter.

---

# 77. Query Flow with UserContext

The complete flow is:

```text
React
  |
  | query jobs
  v
Channel
  |
  v
IPC
  |
  v
Query Controller
  |
  +-- UserContext
  |
  v
Authorization Service
  |
  v
Query Service
  |
  v
Cache
  |
  v
Filtered Snapshot
  |
  v
React
```

The cache remains authoritative.

---

# 78. Command Flow with UserContext

The complete flow is:

```text
React
  |
  | cancel Job/12345
  v
Channel
  |
  v
IPC
  |
  v
Command Controller
  |
  +-- UserContext
  |
  v
Authorization Service
  |
  v
Command Service
  |
  v
Command Adapter
  |
  v
Slurm
```

Afterward:

```text
Slurm
  |
  v
Resource Adapter
  |
  v
Canonical Job
  |
  v
Cache
  |
  v
Event
  |
  v
React
```

---

# 79. UserContext and Canonical Resource Model

UserContext and Resource Model have different responsibilities.

UserContext:

```text
Who is requesting?
What privileges do they have?
What account scopes apply?
```

Resource Model:

```text
What is the current Slurm state?
```

Authorization combines them:

```text
UserContext
      +
Canonical Resource
      +
Operation
      |
      v
Authorization Decision
```

---

# 80. UserContext and IPC Protocol

`ipc-protocol.md` defines how UserContext information crosses the Channel/Bridge boundary.

This document defines what UserContext means.

Therefore:

```text
ipc-protocol.md
    = transport/message contract

user-context.md
    = identity/authorization contract
```

The Channel does not interpret UserContext.

It transports messages.

---

# 81. UserContext and Channel

`cockpit-slurm-channel` should remain unaware of:

```text
AdminLevel
Coordinator
capabilities
Slurm User
authorization policy
```

It only forwards messages.

Architecture:

```text
Channel
    |
    +-- transport
    +-- framing
    +-- forwarding
    +-- lifecycle
```

not:

```text
Channel
    |
    +-- authorization
    +-- Slurm logic
```

This is consistent with the Channel v3.5 design.

---

# 82. UserContext and Bridge

The Bridge owns:

```text
UserContext
Authorization
Canonical Resources
Cache
Commands
Subscriptions
Events
```

Therefore UserContext belongs under the Bridge application/service layer.

Recommended package structure:

```text
internal/
├── auth/
│   ├── user_context.go
│   ├── identity.go
│   ├── capability.go
│   ├── authorizer.go
│   └── policy.go
│
├── resource/
├── cache/
├── command/
├── query/
├── subscription/
└── event/
```

---

# 83. Recommended Separation

I recommend separating:

```text
auth/
```

from:

```text
resource/
```

because:

```text
UserContext
```

is not a canonical Slurm resource.

The canonical:

```text
User
```

belongs under:

```text
resource/
```

while:

```text
UserContext
```

belongs under:

```text
auth/
```

This is an important distinction.

---

# 84. User Resource vs UserContext

Do not merge these two structures.

### User resource

Represents:

```text
Slurm accounting User
```

and belongs in:

```text
Canonical Resource Model
```

### UserContext

Represents:

```text
current authenticated connection
```

and belongs in:

```text
Authorization
```

Therefore:

```text
resource.User
```

and:

```text
auth.UserContext
```

are intentionally different types.

---

# 85. Example

Canonical User:

```json
{
  "kind": "User",
  "metadata": {
    "name": "alice"
  },
  "status": {
    "adminLevel": "operator"
  }
}
```

UserContext:

```json
{
  "username": "alice",
  "adminLevel": "operator",
  "coordinatorAccounts": [],
  "capabilities": [
    "nodes.view",
    "jobs.view",
    "jobs.cancel"
  ]
}
```

The first describes the Slurm resource.

The second describes the current authorization context.

---

# 86. Security Invariants

The implementation must enforce the following invariants.

### Invariant 1

The frontend cannot choose its own identity.

### Invariant 2

The frontend cannot choose its own AdminLevel.

### Invariant 3

The frontend cannot grant itself capabilities.

### Invariant 4

The Channel does not perform authorization.

### Invariant 5

The Bridge performs authorization.

### Invariant 6

UserContext is associated with an authenticated connection.

### Invariant 7

Coordinator privileges are account-scoped.

### Invariant 8

AdminLevel and Coordinator are separate concepts.

### Invariant 9

Linux groups and Slurm AdminLevel are separate concepts.

### Invariant 10

Commands are authorized before execution.

### Invariant 11

Queries and subscriptions are also authorization-controlled.

### Invariant 12

Audit records identify the effective user context.

---

# 87. Failure-Safe Behavior

If UserContext cannot be established reliably:

```text
DO NOT
    assume admin
    assume operator
    assume root
    trust request username
```

Instead:

```text
deny privileged operations
```

and return:

```text
UNAUTHENTICATED
```

or:

```text
INVALID_USER_CONTEXT
```

as appropriate.

---

# 88. Unknown AdminLevel

If Slurm returns an unknown AdminLevel:

```text
unknown
```

the Bridge should not map it to:

```text
admin
```

Safe behavior is:

```text
AdminLevel = none / unavailable
```

with an internal warning.

The policy implementation should explicitly handle unsupported values.

---

# 89. Slurm Unavailable

If the Bridge can identify the Linux user but cannot currently query Slurm:

```text
Linux identity:
    available

Slurm identity:
    temporarily unavailable
```

The UserContext should indicate the degraded state.

Example:

```json
{
  "linux": {
    "username": "alice"
  },
  "slurm": {
    "available": false
  }
}
```

The UI can display:

```text
Slurm unavailable
```

without incorrectly treating the user as an unauthorized user.

---

# 90. UserContext State

Recommended internal state:

```text
initializing
active
degraded
expired
```

Example:

```text
initializing
      |
      v
active
      |
      +---- Slurm unavailable
      |          |
      |          v
      |       degraded
      |
      v
connection closed
      |
      v
expired
```

---

# 91. Testing Strategy

UserContext should be tested independently of Slurm command execution.

Test cases:

```text
1. normal Linux user
2. user without Slurm account
3. AdminLevel = none
4. AdminLevel = operator
5. AdminLevel = admin
6. Coordinator of one account
7. Coordinator of multiple accounts
8. unknown AdminLevel
9. root
10. SlurmUser
11. multiple simultaneous connections
12. disconnected connection
```

---

# 92. Authorization Test Matrix

At minimum test:

| User          | Resource                 | Operation             | Expected                   |
| ------------- | ------------------------ | --------------------- | -------------------------- |
| User          | own Job                  | view                  | allow                      |
| User          | own Job                  | cancel                | policy-dependent           |
| User          | another Job              | cancel                | deny unless permitted      |
| Coordinator   | own account              | view                  | allow                      |
| Coordinator   | own account              | modify                | allow according to policy  |
| Coordinator   | other account            | modify                | deny                       |
| Operator      | Job                      | operational action    | allow according to policy  |
| Operator      | Account                  | administrative change | deny unless policy permits |
| Admin         | Account                  | modify                | allow                      |
| No Slurm User | Job                      | view                  | deny/limited               |
| No context    | any privileged operation | deny                  |                            |

---

# 93. Audit Test

Every privileged command should produce an audit record containing at least:

```text
timestamp
connectionId
username
uid
commandId
operation
resource
authorization decision
result
```

Example:

```json
{
  "timestamp": "2026-08-17T03:00:00Z",
  "connectionId": "conn-001",
  "username": "alice",
  "uid": 1001,
  "commandId": "cmd-001",
  "operation": "cancel",
  "resource": "Job/12345",
  "authorization": "allowed",
  "result": "succeeded"
}
```

---

# 94. Implementation Sequence

Implement UserContext in the following order.

## Step 1 — Linux Identity

Implement:

```text
username
uid
gid
groups
```

and verify the identity observed by the Bridge.

---

## Step 2 — Connection Context

Associate:

```text
connectionId
```

with:

```text
UserContext
```

---

## Step 3 — Slurm User Lookup

Resolve:

```text
Linux username
        |
        v
Slurm User
```

using the Slurm adapter.

---

## Step 4 — AdminLevel

Read:

```text
AdminLevel
```

from the Slurm User resource/accounting information.

Normalize to:

```text
none
operator
admin
```

---

## Step 5 — Coordinator

Resolve account-scoped Coordinator relationships.

Produce:

```text
coordinatorAccounts[]
```

---

## Step 6 — Capability Policy

Implement a centralized capability policy.

For example:

```text
Admin
Operator
Coordinator
User
```

should produce different capability sets.

---

## Step 7 — Authorization Service

Implement:

```go
Authorize(
    userContext,
    action,
    resource
)
```

---

## Step 8 — IPC Integration

Return sanitized UserContext information through the IPC handshake or a dedicated context query.

---

## Step 9 — Command Integration

Every Command Controller must call the Authorization Service.

---

## Step 10 — Query/Subscription Integration

Apply UserContext to:

```text
queries
subscriptions
resource visibility
```

---

## Step 11 — React UserContextProvider

Consume the sanitized UserContext.

Use it for:

```text
home page
navigation
action visibility
```

---

# 95. First Vertical Slice

UserContext should be introduced during the Node vertical slice, but authorization should initially be simple.

The first complete flow should be:

```text
Cockpit
   |
   v
Channel
   |
   v
Bridge
   |
   +-- UserContext
   |
   +-- Query Node
   |
   v
Node Snapshot
   |
   v
React
```

Then extend it to:

```text
UserContext
   |
   +-- Node query authorization
   |
   +-- Node subscription authorization
```

The implementation plan already recommends building Node as the first complete vertical slice before generalizing to Job, Account and User. This remains the right approach.

---

# 96. Second Vertical Slice — Job

Job introduces command authorization.

Flow:

```text
UserContext
      |
      v
jobs.cancel
      |
      v
Authorize Job/12345
      |
      v
Command Service
      |
      v
Slurm
```

This should be the first complete test of:

```text
UserContext
+
Authorization
+
Command
+
Audit
```

---

# 97. Third Vertical Slice — Account

Account introduces Coordinator authorization.

Flow:

```text
UserContext
      |
      +-- AdminLevel
      |
      +-- Coordinator Accounts
      |
      v
Account/Research
      |
      v
Authorize
```

This is the point at which the distinction between:

```text
AdminLevel
```

and:

```text
Coordinator
```

must become fully functional.

---

# 98. Fourth Vertical Slice — User

The User resource completes the identity/account model.

Flow:

```text
User
 |
 +-- AdminLevel
 |
 +-- Associations
 |
 +-- Coordinator
 |
 v
UserContext
```

This should allow the UI to display:

```text
Users
Accounts
Associations
AdminLevel
Coordinator
```

while the Bridge continues to derive authorization independently.

---

# 99. Recommended Directory Structure

Final recommended Bridge structure:

```text
internal/
├── auth/
│   ├── user_context.go
│   ├── identity.go
│   ├── linux_identity.go
│   ├── slurm_identity.go
│   ├── admin_level.go
│   ├── capability.go
│   ├── policy.go
│   ├── authorizer.go
│   └── decision.go
│
├── resource/
│   ├── resource.go
│   ├── metadata.go
│   ├── reference.go
│   ├── node.go
│   ├── job.go
│   ├── account.go
│   ├── user.go
│   ├── association.go
│   └── partition.go
│
├── cache/
├── query/
├── command/
├── subscription/
├── event/
├── audit/
├── ipc/
└── adapter/
```

---

# 100. Final Architecture

The complete security architecture is:

```text
                    Cockpit
                       |
                       | authenticated session
                       v
              cockpit-slurm-channel
                       |
                       | IPC
                       v
        ┌─────────────────────────────────┐
        │       cockpit-slurm-bridge      │
        │                                 │
        │  ┌───────────────────────────┐  │
        │  │       UserContext         │  │
        │  │                           │  │
        │  │ Linux identity            │  │
        │  │ Slurm User                │  │
        │  │ AdminLevel                │  │
        │  │ Coordinator accounts      │  │
        │  │ Capabilities              │  │
        │  └─────────────┬─────────────┘  │
        │                │                │
        │                v                │
        │       Authorization             │
        │          Service                │
        │                │                │
        │       ┌────────┼────────┐       │
        │       │        │        │       │
        │       v        v        v       │
        │     Query   Command  Subscribe  │
        │       │        │        │       │
        │       └────────┼────────┘       │
        │                v                │
        │       Canonical Resources       │
        │                │                │
        │              Cache              │
        │                                 │
        └────────────────┬────────────────┘
                         |
                         v
                       Slurm
```

The key rule is:

```text
                    USER CONTEXT
                         |
                         v
              AUTHORIZATION SERVICE
                         |
          ┌──────────────┼──────────────┐
          |              |              |
        QUERY          COMMAND       SUBSCRIBE
          |              |              |
          └──────────────┼──────────────┘
                         |
                         v
                CANONICAL RESOURCES
```

UserContext is therefore the **security context connecting the authenticated Cockpit session to the canonical Slurm resource model**.

---

# 101. Summary of Key Decisions

The implementation should follow these decisions:

1. **UserContext belongs to the Bridge.**
2. **The Channel is not an authorization component.**
3. **The frontend does not determine authoritative permissions.**
4. **Linux identity and Slurm identity are separate.**
5. **Linux groups and Slurm AdminLevel are separate.**
6. **AdminLevel is `none`, `operator`, or `admin`.**
7. **Coordinator is account-scoped and is not an AdminLevel.**
8. **User and UserContext are different concepts.**
9. **User is a canonical Slurm resource; UserContext is an authorization object.**
10. **Capabilities use `<resource>.<action>`.**
11. **Authorization is evaluated against UserContext + operation + resource.**
12. **Queries, commands, and subscriptions are all authorization-controlled.**
13. **The Bridge remains the authoritative enforcement point.**
14. **React receives a sanitized UserContext view.**
15. **React uses capabilities for navigation and UI visibility.**
16. **Commands must be authorized again by the Bridge even if the UI hides the action.**
17. **Audit records retain the effective user identity and command identity.**
18. **Connection ID, UserContext, command ID, and subscription ID remain distinct.**
19. **Root and SlurmUser are identities, not special canonical resource kinds.**
20. **The first implementation should be introduced through the Node → Job → Account → User vertical-slice progression.**

````

### One architectural change I strongly recommend

There is one subtle point I would add to your current v3.5 architecture: **make `UserContext` a first-class object inside the Bridge's connection/session state, but don't make it part of the canonical Resource Model.**

So the relationship should be:

```text
                    Bridge
                      │
             ┌────────┴────────┐
             │                 │
       Connection State    Resource State
             │                 │
             ▼                 ▼
        UserContext       Canonical Resources
             │                 │
             ▼                 ▼
       Authorization          Cache
             │                 │
       ┌─────┼─────┐           │
       ▼     ▼     ▼           │
     Query Command Subscribe   │
       │     │     │           │
       └─────┼─────┘           │
             ▼                 │
            IPC ◄──────────────┘
````

This fits very well with your existing v3.5 design: the Bridge owns state, the Channel is deliberately thin, queries read from cache, commands modify Slurm and then refresh the cache, and subscriptions deliver snapshots/events. ([GitHub][2])

It also gives your UI architecture a clean foundation for the **Not Set → User → Coordinator → Operator → Admin** home-page model without incorrectly treating those as a single Slurm role hierarchy. The existing UI plan already calls for role-based navigation and separate Users/Accounts/Associations areas. ([GitHub][3])

I would consequently place the four architecture documents in this dependency order:

```text
resource-model.md
       │
       ├──────────────┐
       ▼              ▼
user-context.md   command-model.md
       │              │
       └──────┬───────┘
              ▼
       authorization
              │
              ▼
       ipc-protocol.md
              │
              ▼
    channel + React UI
```

And, importantly, **`user-context.md` should be implemented before the Account/User/Coordinator vertical slices**, while only a minimal version is needed for the first Node slice. This matches the overall v3.5 recommendation to build the system as vertical slices rather than completing Bridge, Channel, and UI independently. ([GitHub][4])

[cockpit-slurm architecture directory](https://github.com/lingweicai/cockpit-slurm/tree/development-bridge-v3.5/docs/architecture?utm_source=chatgpt.com)

[1]: https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/docs/architecture/ipc-protocol.md "cockpit-slurm/docs/architecture/ipc-protocol.md at development-bridge-v3.5 · lingweicai/cockpit-slurm · GitHub"
[2]: https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/docs/architecture/development-cockpit-slurm-bridge_v3.5.md "cockpit-slurm/docs/architecture/development-cockpit-slurm-bridge_v3.5.md at development-bridge-v3.5 · lingweicai/cockpit-slurm · GitHub"
[3]: https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/docs/architecture/development-cockpit-slurm-UI-v2.md "cockpit-slurm/docs/architecture/development-cockpit-slurm-UI-v2.md at development-bridge-v3.5 · lingweicai/cockpit-slurm · GitHub"
[4]: https://github.com/lingweicai/cockpit-slurm/blob/development-bridge-v3.5/docs/architecture/implementation-plan-v3.5.md "cockpit-slurm/docs/architecture/implementation-plan-v3.5.md at development-bridge-v3.5 · lingweicai/cockpit-slurm · GitHub"

# Cockpit-Slurm Bridge and Channel Architecture v2

## Design Principles

1. cockpit-slurm-bridge is the only component that executes Slurm commands.
2. cockpit-slurm-bridge owns all caches.
3. cockpit-slurm-channel is a stateless proxy between cockpit.channel() and bridge.
4. Frontend receives initial snapshots followed by incremental updates.
5. Every cache update increments a generation number.
6. Every subscriber receives a unique connection ID.
7. All communication uses request IDs for correlation.

---

# Architecture

React + PatternFly

↓

cockpit.channel()

↓

cockpit-slurm-channel

↓

Unix Socket

↓

cockpit-slurm-bridge

↓

Cache Manager

↓

Slurm CLI Commands

* sacctmgr
* scontrol
* sinfo
* squeue
* sreport

---

# Entity Cache Structure

Each Slurm entity has an independent cache.

Examples:

* account
* user
* association
* cluster
* qos
* partition
* node
* reservation
* job

Example:

```go
type AccountCache struct {
    mu sync.RWMutex

    Generation uint64
    UpdatedAt time.Time

    Items map[string]*models.Account
}
```

Generation starts from:

```text
1
```

Every cache modification increments generation.

Example:

```text
generation 1
generation 2
generation 3
```

This includes:

* periodic refresh updates
* add operations
* modify operations
* delete operations

---

# Generation IDs

Generation IDs are maintained independently for every entity type.

Example:

```text
account generation = 15
user generation = 22
partition generation = 104
job generation = 5834
```

This avoids a high-frequency entity such as jobs causing unnecessary generation updates for accounts.

---

# Connection IDs

Every socket subscriber receives a unique connection ID.

Example:

```text
conn-1
conn-2
conn-3
```

Suggested implementation:

```go
uuid.NewString()
```

Example:

```text
7c3a72c7-66e0-48f8-8db5-d6d0b5774ca9
```

Bridge stores:

```go
type Subscriber struct {
    ConnectionID string
    User string

    Subscriptions map[string]uint64

    Conn net.Conn

    CreatedAt time.Time
}
```

Subscriptions contains:

```text
entity -> last generation seen
```

Example:

```text
account -> 17
user -> 22
node -> 104
```

---

# Request IDs

Every request includes a request ID.

Example:

```json
{
    "request_id": "f0d89f9d",
    "type": "list",
    "entity": "account"
}
```

Response:

```json
{
    "request_id": "f0d89f9d",
    "success": true
}
```

Request IDs allow:

* multiple concurrent requests
* duplicate request detection
* retry support

---

# Message Types

## List

Retrieve complete cache snapshot.

Request:

```json
{
    "request_id": "123",
    "type": "list",
    "entity": "account"
}
```

Response:

```json
{
    "request_id": "123",
    "type": "snapshot",
    "entity": "account",
    "generation": 15,
    "items": []
}
```

---

## Get

Retrieve one object.

Request:

```json
{
    "request_id": "124",
    "type": "get",
    "entity": "account",
    "id": "research"
}
```

---

## Subscribe

Register interest in an entity.

Request:

```json
{
    "request_id": "125",
    "type": "subscribe",
    "entity": "account",
    "generation": 15
}
```

The generation field indicates the last generation known by frontend.

---

## Unsubscribe

Request:

```json
{
    "request_id": "126",
    "type": "unsubscribe",
    "entity": "account"
}
```

---

## Create

Request:

```json
{
    "request_id": "127",
    "type": "create",
    "entity": "account",
    "payload": {}
}
```

---

## Update

Request:

```json
{
    "request_id": "128",
    "type": "update",
    "entity": "account",
    "payload": {}
}
```

---

## Delete

Request:

```json
{
    "request_id": "129",
    "type": "delete",
    "entity": "account",
    "payload": {}
}
```

---

# Event Messages

Bridge broadcasts diffs.

Example:

```json
{
    "type": "event",
    "entity": "account",
    "generation": 16,
    "added": [],
    "modified": [],
    "deleted": []
}
```

Only subscribers of account receive this message.

---

# Missing Event Detection

Frontend tracks generation numbers.

Example:

```text
received generation 16
received generation 17
received generation 19
```

Generation 18 is missing.

Frontend automatically performs:

```json
{
    "type": "list",
    "entity": "account"
}
```

Bridge returns a fresh snapshot.

---

# Subscriber Manager

Bridge owns subscriber state.

Example:

```go
type SubscriptionManager struct {
    mu sync.RWMutex

    subscribers map[string]*Subscriber
}
```

Index:

```text
connectionID -> subscriber
```

Optional reverse index:

```text
entity -> subscriber list
```

Example:

```go
map[string]map[string]*Subscriber
```

This allows efficient broadcasts.

---

# Channel Responsibilities

cockpit-slurm-channel must remain stateless.

Responsibilities:

* receive stdin messages from cockpit.channel()
* forward messages to socket
* forward responses to stdout
* reconnect to bridge if necessary

Must not contain:

* cache
* subscriptions
* business logic
* Slurm commands

---

# Bridge Responsibilities

cockpit-slurm-bridge owns:

* cache
* diff engine
* subscriber management
* request routing
* request deduplication
* command execution
* authorization

---

# Polling Intervals

Suggested defaults:

Account: 60 seconds

User: 60 seconds

Cluster: 60 seconds

QOS: 60 seconds

Partition: 30 seconds

Reservation: 30 seconds

Node: 10 seconds

Job: 5 seconds

---

# Development Order

Task 1

Implement request and response message structures.

Task 2

Implement AccountCache with generation support.

Task 3

Implement SubscriptionManager with connection IDs.

Task 4

Implement snapshot list operation.

Task 5

Implement subscribe and unsubscribe operations.

Task 6

Implement Account diff engine.

Task 7

Implement event broadcasting.

Task 8

Implement periodic cache refresh.

Task 9

Implement create operation.

Task 10

Implement update operation.

Task 11

Implement delete operation.

Task 12

Implement request deduplication cache.

Task 13

Generalize cache implementation using Go generics.

Task 14

Add User cache.

Task 15

Add Cluster cache.

Task 16

Add QOS cache.

Task 17

Add Partition cache.

Task 18

Add Node cache.

Task 19

Add Job cache.

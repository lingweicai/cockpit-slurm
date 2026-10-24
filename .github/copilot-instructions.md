# Architecture and Implementation Rules

## 1. Authoritative Documentation

Follow these documents in order:

1. [`docs/architecture/0.1 development-cockpit-slurm-_v3.6.md`](../docs/architecture/0.1%20development-cockpit-slurm-_v3.6.md) — normative v3.6 architecture and cross-cutting constraints.
2. [`docs/architecture/1.0 implementation-plan-v3.6.md`](../docs/architecture/1.0%20implementation-plan-v3.6.md) — phases, dependencies, and completion criteria.
3. The applicable phase specification under `docs/architecture/` — detailed requirements, interfaces, acceptance criteria, and tests.
4. Existing source and tests — evidence of implementation, not permission to violate approved architecture.

For protocol, error, and identity contracts, also follow [`2.1 ipc-protocol.md`](../docs/architecture/2.1%20ipc-protocol.md), [`2.6 error-model.md`](../docs/architecture/2.6%20error-model.md), and [`2.3 user-context.md`](../docs/architecture/2.3%20user-context.md).

Do not introduce architectural concepts absent from the approved documents. If sources conflict, do not silently invent a solution: identify the conflict and propose the smallest resolution. Implement only the requested phase or task; do not add future-phase functionality unless required by an approved dependency. Do not modify architecture documents unless explicitly requested.

## 2. Version 3.6 Architecture

Version 3.6 removes the custom `cockpit-slurm-channel` executable. The communication path is:

```text
React frontend
→ Cockpit cockpit.channel() API
→ native Cockpit transport
→ Unix-domain socket
→ cockpit-slurm daemon
→ backend IPC server and dispatcher
```

- Use Cockpit's native channel API; do not restore `cockpit-slurm-channel`, add another custom channel executable, forwarding process, or IPC socket.
- The persistent `cockpit-slurm` backend owns application logic, IPC, authorization, Slurm operations, canonical resources, cache, synchronization, subscriptions, events, and command lifecycle.
- Do not execute Slurm commands with frontend `cockpit.spawn()` or move backend business logic, authorization, or resource management into the frontend.
- Keep the application IPC protocol independent of Cockpit's transport. Treat Cockpit stream chunks as arbitrary byte chunks, not complete application messages.
- Preserve the documented four-byte unsigned big-endian length prefix, UTF-8 JSON payload, and message-size limit.
- Use the documented production socket path `/run/cockpit-slurm/cockpit-slurm.sock`; honor documented development socket configuration without inventing fallback paths or choosing a socket based on unrelated group membership.
- Socket access controls who can connect; it does not grant Slurm capabilities or administrative privileges.

## 3. Backend State and Authority

- The backend is authoritative for connection identity, authorization, Slurm resource state, cache and generations, command execution, and event publication.
- Follow the documented flow: Slurm observation or command result → backend synchronization → authoritative cache → event publication → frontend update.
- Frontend state and capability checks are for presentation; they are not authoritative. Do not optimistically treat a requested mutation as confirmed state.
- Frontend services are transport/client abstractions, not business-logic services.

## 4. Identity and Authorization

- Derive and validate requesting-user identity through the approved shared connection initialization and server-side Unix-socket peer-credential flow. Do not trust identity, role, account, or capability fields supplied by a client.
- Validate the mapping from the authenticated Cockpit user to observed peer UID/GID on the target installation before enabling protected command paths. `SO_PEERCRED` code alone does not prove this mapping.
- Ensure every protected connection path, including streams and commands, establishes the same authenticated connection context.
- Resolve Linux and Slurm identity using the approved process. Missing, unresolved, stale, mismatched, or failed identity/authorization data must deny protected operations.
- Enforce operation-specific authorization in the backend for protected reads, snapshots, subscriptions, event delivery, and commands. Revalidate when required, including fresh Slurm authorization for new commands.
- Use the approved capability policy and registry; frontend visibility is only a hint. Do not infer Slurm administration from membership in the `cockpit-slurm` Unix group.
- For Phase 5 Node drain/resume, require the specified fresh authorization: only a currently resolved Slurm `AdminLevel=admin` receives `nodes.drain` and `nodes.resume`. Operators, coordinators, and ordinary users do not receive Node mutation permission.

## 5. IPC and Command Contracts

- Treat the IPC specification as a contract shared by frontend and backend. Preserve its envelopes, fields, types, message lifecycle, correlation behavior, and canonical error structure.
- Keep message correlation IDs distinct from command execution IDs. Distinguish command acceptance from terminal execution status.
- Validate message structure, required parameters, operation IDs, and resource targets. Reject unknown operations and invalid input; never build arbitrary shell commands from client input.
- Resolve operations, handlers, validators, and required capabilities from the approved backend registry. Keep command execution separate from read/query handling.
- Follow documented deduplication and idempotency rules; a repeated command ID must not execute a command again.
- Preserve command-ledger ordering and durability semantics. Do not claim a denial or outcome is durably recorded if persistence failed.
- Distinguish definite rejection from an execution with unknown outcome. Do not automatically retry a potentially executed non-idempotent command after timeout or connection loss; reconcile using authoritative backend state and the documented status mechanism.
- Do not change the IPC contract for implementation convenience. Change its specification and both implementations together only when explicitly approved.

## 6. Frontend and Repository Conventions

- Follow existing React, TypeScript, and PatternFly 6 patterns and strict TypeScript checking.
- Do not introduce `any`, `react-router-dom`, Recharts, or Tailwind CSS.
- Reuse the existing IPC client, framing, types, and provider architecture rather than creating parallel implementations.
- Prefer small interfaces and add unit tests for new Go code.
- Handle connection loss, malformed messages, timeouts, subscription cleanup, and component lifecycle correctly.
- Display command success only from its documented execution result; use backend events and snapshots as the source of truth for resource state.

## 7. Implementation, Build, and Test Discipline

Before changing code, read the applicable architecture and phase specifications, inspect the implementation and related tests, identify affected contracts and compatibility requirements, and make the smallest complete change.

After changing code:

- Add or update tests for affected behavior, including success and failure cases and negative authorization tests for protected operations.
- Verify frontend/backend protocol compatibility and preserve existing behavior.
- Run relevant project checks. Common commands:
  - `make` — build the frontend and Go binaries.
  - `go test ./...` — run Go tests.
  - `npm run test:ipc` — run frontend IPC tests.
  - `npm run build` — build the frontend.
  - `npm run eslint`, `npm run stylelint`, or `make codecheck` — run applicable static checks.
- Report the exact checks performed and their results; never claim an unexecuted check passed.

Do not weaken security checks, remove failing tests, or make unrelated refactors to simplify a task. If a requirement is ambiguous, a dependency is missing, or implementation conflicts with approved architecture, explain the issue before making a consequential design change.

# Phase 6 Stage A Audit and Baseline

Date recorded: 2026-10-30.

## Decision

**In progress; not accepted.** No Phase 6 resource or command implementation is enabled by this record. The Job data source has not been selected or approved. Stage A's source and execution-identity findings require review before Stage D or new privileged command work. This record does not authorize Stage B or resolve the Job read policy.

## Node Implementation Audit

- `cmd/cockpit-slurm/main.go` wires one Node adapter, cache, synchronizer, stream registry, query handler, authenticated command service, and IPC server. The first Node refresh occurs before the listener starts; periodic synchronization is independent of clients.
- `internal/resource/node.go`, `node_adapter.go`, and `node_synchronizer.go` own the canonical Node model, `scontrol show nodes --json` mapping, and complete-snapshot validation. The synchronizer serializes refreshes and leaves the cache unchanged on acquisition or validation failure.
- `internal/resource/node_cache.go` implements a Node-specific map, identity lookup, collection generation, snapshot replacement, and publication/baseline locking. A changed complete snapshot advances the generation once and publishes a complete change batch; unchanged data does not advance it. The stream registry and cache cooperate on the snapshot boundary.
- `internal/stream/node_registry.go` and `internal/ipc/server.go` are coupled to Node payloads and `NodeStreamRegistry`. They provide per-connection snapshot registration, bounded fan-out, serialized IPC delivery, and disconnect cleanup. `internal/service/query.go` accepts the Node query resource `nodes` and reads the Node cache; Node stream events identify `node`.
- `internal/protocol` and `src/ipc` provide shared length-prefixed framing, envelopes, correlation, and channel transport. Event payload types and validators are Node-specific. `NodeProvider` currently applies all event messages to the Node validator; a Job event on that connection would cause recovery rather than be ignored. `IpcClient` exposes one unsolicited-message callback, so any shared frontend delivery must preserve both resource consumers and existing command correlation.
- `internal/authorization/registry.go` already registers `node.drain` and `node.resume` with distinct required capabilities, schemas, and handler metadata, but its validation intentionally permits only those two operations. `internal/command/service.go` is Node-specific and owns caller-scoped durable claims, fresh authorization, execution, result recording, and post-command refresh. Do not treat the command registry as an existing generic Job registry.

## Phase 5 Command Regression Baseline

The existing tests in `internal/command`, `internal/authorization`, `internal/ledger`, and `internal/ipc` cover Node operation/capability mappings, fresh authorization, denial and lookup-failure tombstones, requester mismatch, target validation, duplicate IDs and conflicts, uncertain persistence, unknown execution outcomes, IPC correlation, and command/event coexistence. Existing frontend IPC and Node UI tests cover command requests and presentation. These tests are the pre-refactor regression baseline; no duplicate tests or new mutation operations were added in Stage A.

The accepted Phase 5 Stage C/D/E records contain prior target-installation tests, including permitted and denied Node operations and authoritative events. Those prior results are historical acceptance evidence, not newly executed live tests in this Stage A run.

## Requester and Execution Identities

- IPC requester: derived from the accepted connection's Unix peer credentials and resolved server-side. The accepted Phase 5 Stage A record contains the target Cockpit-to-peer identity evidence; client payload claims are not authoritative.
- Daemon: `systemctl show cockpit-slurm.service` reports `User=cockpit-slurm`, `Group=cockpit-slurm`, and `ActiveState=active`. The local account is UID 201.
- Node mutation boundary: the daemon's `DBusExecutor` calls the restricted Node mutation helper, not a requester-selected executable or Unix identity. The helper unit is configured as `User=slurm`, `Group=slurm`; the local account is UID 202. Its inactive state at inspection is consistent with on-demand activation and is not a new execution test.
- The Phase 5 Stage C/D acceptance records document the helper's real-cluster Node drain/resume behavior and that direct Node mutation as the daemon account failed. The command ledger separately records requester, daemon, and helper execution identities. The helper's Slurm identity must not be interpreted as the IPC requester's identity.

## Job Source Investigation

- The installed Slurm client reports version `25.05.7`; `scontrol ping` reported the primary controller up. The read-only probes ran as `dev`, not the daemon or helper account.
- `scontrol show job --json` returned valid JSON with top-level `errors`, `jobs`, `last_backfill`, `last_update`, `meta`, and `warnings`; `jobs` was an empty array, `errors` was empty, and the reported parser plugin was `data_parser/v0.0.43`. `scontrol --all --local show job --json` likewise returned zero Jobs and zero errors. `scontrol -o show job` yielded no Job records. No Job content was retained in this record.
- `scontrol show config` reported `ClusterName=cluster`, `PrivateData=none`, and `SlurmUser=slurm(202)` for this installation. These settings and empty results do not prove that the daemon account can enumerate the full active population, including hidden partitions, on this or another target.
- At the initial baseline there were no active Jobs from which to verify required fields, Job states, individual or aggregated array identity, normalization, or truncation handling. The baseline probes did not submit or modify Jobs.
- The operator subsequently reported allocating an ordinary interactive Job (ID 11) as `dev`. Their `squeue` and `scontrol show job --json` output showed the Job running on `c21` and `c22`; the JSON reported Slurm `25.05.7`, `data_parser/v0.0.43`, an empty `errors` array, cluster `cluster`, and Job state `["RUNNING"]`. The output included `job_id`, `user_name`, `partition`, and allocated-resource fields.
- A separate read-only `scontrol show job --json` probe as `dev` confirmed Job 11 and its identity fields. For this non-array Job, `array_job_id` was `{ "set": true, "infinite": false, "number": 0 }`, `array_task_id` was `{ "set": false, "infinite": false, "number": 0 }`, and `array_task_string` was empty. This conflicts with Phase 6 section 6.1, which requires both array identifiers absent for normal Jobs and prohibits coercing a present zero to absence. Do not silently normalize or reject such records; review and approve source-specific identity normalization before implementation.
- The generated `V0043JobInfo` declares fields for the observed Job ID, state, username, cluster, and array data, but field-name and parser-version matches do not establish full JSON schema or semantic compatibility. Real array Jobs, other states and versions, malformed/truncated responses, and visibility as the backend execution identity remain unverified.
- Source approval remains open. Before selection, gather representative normal and array Job output, verify active-state and array semantics, parsing and truncated/error responses, and establish complete visibility under the actual approved backend execution identity and target configuration. A successful empty response under `dev` is insufficient evidence. No fallback source is approved by this record.

## Checks Run

| Check | Result |
| --- | --- |
| `go test ./...` | Passed. |
| `go vet ./...` | Passed. |
| `go test -race ./internal/resource ./internal/stream ./internal/ipc ./internal/command ./internal/ledger ./internal/authorization` | Passed. |
| `npm run test:ipc` | Passed. |
| `npm run test:node-ui` | Passed. |
| `npm run build` | Passed; this build is not a TypeScript type check. |
| `npm run stylelint` | Passed. |
| `npx tsc --noEmit` | Failed with four existing errors: `src/ipc/command-service.test.ts:27`, `src/ipc/command-service.ts:29`, `src/ipc/command-service.ts:117`, and `src/node-command-ui.ts:46`. |
| `npm run eslint` | Failed with 262 errors in `src/app.tsx`, `src/index.tsx`, `src/ipc/channel.ts`, `src/ipc/framing.test.ts`, `src/ipc/framing.ts`, and `src/ipc/node-query.ts`; this matches the prior Phase 5 Stage E count. |
| `go test ./internal/command ./internal/ipc ./internal/ledger ./internal/authorization -list .` | Passed; inventoried existing regression cases. |

No live mutation, service restart, production ledger modification, or service-account impersonation was performed.

## Outstanding Decisions and Evidence

1. Approve a Job read-visibility policy covering queries, snapshots, event delivery, fields, denial, and permission changes. The daemon's source visibility is not an authorization grant for IPC callers. Define filtered-stream generation and gap recovery before implementing Job delivery.
2. Resolve the observed `array_job_id` zero/set conflict with the approved canonical identity contract; define supported active states, Job/array representation and identity, canonical fields, and Job query/snapshot/change wire payloads. Validate representative normal and array source output against those decisions.
3. Verify source completeness and privacy/hidden-partition behavior under the actual approved backend execution identity; record parser, malformed/truncated response, and array evidence. Obtain explicit source approval at the Stage A gate.
4. Review the Node-to-shared extraction plan against the ownership and ordering boundaries above. Reuse existing transport, connection initialization, authorization, command registry, and ledger mechanisms; preserve Phase 5 command behavior. No generic command extraction is required unless existing code demonstrates a concrete common need.
5. Account for the existing TypeScript and ESLint failures when planning the required Phase 6 acceptance checks; do not report those checks as passing or weaken security/tests to clear them.
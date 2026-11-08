# Phase 6 Stage A Audit and Baseline

Date recorded: 2026-10-30.

## Decision

**In progress; not accepted.** No Phase 6 resource or command implementation is enabled by this record. The Job data source has not been selected or approved, and its identity normalization has not been verified using representative source output under the approved backend execution identity. The Phase 6 event-sequence and visibility-policy contracts are now specified in `docs/architecture/4.5 Implementation-phase6.md`, but the Stage A evidence and review gate remain incomplete. This record does not authorize Stage B.

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
- A separate read-only `scontrol show job --json` probe as `dev` confirmed Job 11 and its source identity fields. For this non-array Job, `array_job_id` was `{ "set": true, "infinite": false, "number": 0 }`, `array_task_id` was `{ "set": false, "infinite": false, "number": 0 }`, and `array_task_string` was empty. This raw source representation requires a verified source-specific normalization rule to produce the canonical normal-Job identity with both array identifiers absent. Do not infer absence from zero alone: array task ID zero may be valid. This probe was not run under the approved backend data-acquisition identity and does not establish the normalization rule.
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

1. Verify complete active-Job acquisition and privacy/hidden-partition behavior under the approved backend data-acquisition OS/Slurm identity. Record the selected source, supported Slurm version, parser behavior, required fields, malformed/truncated response behavior, and explicit source approval. Evidence from a different user is insufficient.
2. Obtain representative normal-Job and array-Job output from the approved source under the approved backend identity. Verify source-specific unset/value semantics, including a valid array task ID zero where available, and document the deterministic normalization rule that yields absent array identifiers for normal Jobs without erasing valid zero-valued array task IDs.
3. Review the Phase 6 stream contract: all resource events on one IPC connection use a monotonically increasing connection sequence; hidden changes emit nothing and consume no sequence; visibility revocation emits an identity-only removal; authorization-context changes reconcile the projection; and sequence gaps recover by opening a new connection and receiving fresh snapshots. Confirm the shared frontend consumer validates across resource kinds before provider dispatch.
4. Confirm the query-policy mapping is explicit in the Phase 6 spec: `node.list` references `node.cluster`, and `job.list` references `job.active`; the referenced policy scopes are defined. Stage B must add validation tests that reject missing references, unknown policy identifiers, resource-kind mismatches, and enabled record-returning queries without references.
5. Review the Node-to-shared extraction plan against the ownership and ordering boundaries above. Reuse existing transport, connection initialization, authorization, command registry, and ledger mechanisms; preserve Phase 5 command behavior. No generic command extraction is required unless existing code demonstrates a concrete common need.
6. Account for the existing TypeScript and ESLint failures when planning the required Phase 6 acceptance checks; do not report those checks as passing or weaken security/tests to clear them.

## Stage A Acceptance Criteria

Stage A remains unaccepted until a reviewer records approval after verifying all of the following:

- The per-connection event-sequence, visibility-projection, identity-only removal, authorization-context reconciliation, and fresh-snapshot recovery contract is documented and internally consistent with the accepted IPC transport and event envelope.
- The active-Job source is selected and approved using completeness, parser, failure/truncation, visibility, and supported-version evidence from the approved backend data-acquisition identity.
- Representative normal and array Jobs from that source establish the source-specific identity normalization rule. Normal canonical Jobs have both array identifiers absent; valid array task ID zero remains representable.
- The IPC requester, daemon data-acquisition identity, and any privileged-helper identity are separately identified and verified for their respective boundaries.
- The exact `node.list` → `node.cluster` and `job.list` → `job.active` visibility-policy references and role scopes are documented; Stage B includes required reference-validation tests.
- The extraction plan is mapped to the audited implementation without authorizing speculative abstractions or weakening Phase 5 behavior.

Until this acceptance is recorded, **Stage B MUST NOT start**. The decisions in Phase 6 define what must be implemented; they do not themselves approve implementation work.
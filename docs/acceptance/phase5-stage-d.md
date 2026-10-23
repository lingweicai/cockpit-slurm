# Phase 5 Stage D Acceptance Record

Date recorded: 2026-10-23.

## Gate Entry

Stage D implementation started after Stages A, B, and C were explicitly recorded as passed in their acceptance records. This entry records authorization to begin Stage D; it does not mark Stage D accepted.

## Implemented Progress

- Added a backend Command Service using the existing v1 command payload, operation registry, fresh Authorizer, canonical Node cache, durable SQLite ledger, and injected Node execution boundary.
- The service authenticates from the connection-derived `UserContext`, derives `node.drain` or `node.resume` from the validated state, checks the exact target in the authoritative Node cache, and rejects invalid requests before claiming or authorizing them.
- The command ledger schema is now version 2 and persists the requesting GID, original request `messageId`, and daemon service UID/username alongside the existing caller UID/username. The helper's effective execution identity remains separately stored. A v1-to-v2 migration preserves old records and marks newly introduced historical fields with explicit unknown defaults.
- New commands persist `authorization_pending` before invoking the Authorizer, then persist `accepted` and `running` before calling the executor. Denials and authorization lookup failures become durable non-executable tombstones.
- Caller UID plus `commandId` and a canonical request fingerprint scope deduplication. Matching retries return stored outcomes without reauthorization or redispatch; conflicting payloads are rejected.
- Added a fixed D-Bus client for the Stage C restricted helper. The command record keeps requester identity separate from the helper's configured Slurm execution identity and D-Bus boundary.
- Registered `command` as an authenticated IPC handler and wired the command service, registry, Authorizer, and helper client into daemon startup. No frontend mutation UI was added.
- After a durably recorded successful result, the service requests a bounded asynchronous refresh through the existing `NodeSynchronizer.Refresh`; it does not optimistically mutate the Node cache.
- An execution outcome persisted as `unknown` also requests that bounded refresh so normal synchronization can reconcile observed state; the refresh does not rewrite the command result. Definite failures do not trigger this unknown-outcome refresh.
- The D-Bus executor treats unclassified transport failures as `unknown` outcomes; only explicit helper validation/access/execution errors are considered definite failures.

## Verification

- `go test ./internal/command -count=1` passed for drain/resume lifecycle, durable success/deduplication, denied and lookup-failure tombstones, resolver identity mismatch, invalid target rejection before claiming, the fixed D-Bus operation boundary, unknown transport outcome classification, requesting the Node synchronization refresh, cross-Service duplicate/conflicting-fingerprint races, and injected initial-claim/accepted-transition errors. Commit-before-error claims and accepted transitions are quarantined as `authorization_uncertain`; confirmed-absent pre-commit claims do not invoke authorization; failed reconciliation keeps admission blocked.
- `go test ./internal/ipc -count=1` passed for connection-scoped `messageId` replay caching: equivalent request payloads with changed timestamps reuse the original response, conflicting payloads return `message-id-conflict` without dispatch, and the bounded per-connection cache closes rather than evicting replay history.
- A Unix-domain socket integration test verifies the live IPC loop dispatches an initial command once, returns the same response for an equivalent same-`messageId` replay, and returns `message-id-conflict` for a different payload without invoking the handler again.
- A second Unix-domain socket test sends a Node drain through authenticated dispatch, fresh capability resolution, the Command Service, the durable test ledger, and an injected executor. It verifies the stored requester identity and response correlation, then proves same-message replay and conflicting-message rejection do not invoke authorization or execution again.
- That authenticated IPC test also makes the fake executor update the adapter's observed Node snapshot and invokes the real `NodeSynchronizer.Refresh`; the client receives the committed generation-2 `DRAIN` event through the existing Node stream. The test passed 20 consecutive runs.
- An authenticated IPC test simulates an execution whose outcome is unknown after dispatch, verifies the canonical non-retryable `command-outcome-unknown`, observes the normal `DRAIN` synchronization event, then retries the same command ID and confirms no reauthorization or re-execution.
- An authenticated Unix IPC denial test resolves the peer as an operator, receives canonical `authorization-denied`, verifies the durable `denied` tombstone, and asserts zero executor calls.
- An authenticated Unix IPC lookup-failure test returns correlated `user-lookup-failed`, persists an `authorization_failed` tombstone, and asserts the executor is never called.
- An authenticated Unix IPC persistence-fault test injects an uncertain accepted transition and failed reconciliation; it verifies canonical non-retryable `ledger-persistence-failed`, a blocked admission gate, the retained non-executable claim, and zero executor calls.
- A same-connection authorization test changes the resolver from admin to operator and back: a new ID is denied after revocation, the denied ID stays denied after permission is restored, and a fresh ID is allowed. Resolver and executor call counts confirm each eligible new command is freshly authorized exactly once.
- `go test ./cmd/cockpit-slurm` passed after daemon startup wiring.
- Command Service tests verify unknown outcomes request Node synchronization while definite execution failures do not.
- Ledger, Command Service, and authenticated IPC tests verify requester GID, original request `messageId`, and daemon service identity survive persistence and reopen; the v1 migration test preserves existing records and initializes unavailable historical attribution fields to explicit defaults.
- On 2026-10-23, the installed daemon binary timestamp was 14:41:28 CST and `cockpit-slurm.service` started at 14:41:37 CST with `ActiveState=active`, `ExecMainStatus=0`, and `NRestarts=0`. Because startup opens/migrates the ledger before opening the listener, this records successful normal installed startup with schema v2; it does not substitute for systemd fault-injection or seeded v1 recovery evidence.
- On 2026-10-23, the compiled `internal/command` test binary passed with `-test.v -test.count=1` in a transient systemd unit running as `cockpit-slurm:cockpit-slurm`, with isolated `StateDirectory=cockpit-slurm-stage-d` and `RuntimeDirectory=cockpit-slurm-stage-d`, both mode `0750`, plus `NoNewPrivileges=true`, `ProtectSystem=strict`, and `ProtectHome=true`. The run passed all command-service tests, including authorization, idempotence, unknown outcomes, identity attribution, and claim/transition fault injection. `TMPDIR` was isolated under `/var/lib/cockpit-slurm-stage-d`; the production `/var/lib/cockpit-slurm/commands.db` was not used, and no Slurm/helper operation was invoked.
- A real-cluster IPC attempt as `slurm_admin` (UID 1008) against `c21` returned `ledger-persistence-failed` before a command response. Read-only follow-up reported `PRAGMA integrity_check=ok`, no row for command ID `stage-d-c21-drain-df09dd81c5924d559b8f9a32f982f673`, an active daemon with no reconciliation-failure journal entry, and `c21` still `IDLE`. Since dispatch requires the durable claim and running transition, this attempt did not reach the execution boundary and caused no Node mutation. The underlying ledger write failure is unresolved; this attempt is not passing real-cluster evidence.

## Remaining Stage D Gates

- Exercise SQLite commit-before-error and pre-commit failures through the production Command Service/ledger combination under the named production systemd unit and record the resulting recovery behavior. The complete command-package suite now passes under an isolated transient unit with matching service identity and hardening, but it does not inject faults into the named unit's production ledger.
- Verify the refreshed Node state and event on a real Slurm mutation; the complete cache/stream path is covered locally with an injected execution boundary.
- Run the dedicated real-cluster drain/resume test through authenticated Cockpit IPC, proving the original requester is authorized before the helper is invoked, and restore the test Node's original state.
- Diagnose the real-cluster attempt's `ledger-persistence-failed` response before retrying with a new command ID; the command ledger was intact and the node remained idle, but the write failure's underlying cause was not exposed in the response or journal.
- Record the target installation's exact command response, ledger record, helper execution identity, and subsequent Node event.

Stage D remains in progress and is not accepted. Stage E and mutation UI remain unimplemented and gated on Stage D acceptance.
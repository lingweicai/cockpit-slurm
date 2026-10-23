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

- `go test ./internal/command -count=1` passed for drain/resume lifecycle, durable success/deduplication, denied and lookup-failure tombstones, resolver identity mismatch, invalid target rejection before claiming, the fixed D-Bus operation boundary, unknown transport outcome classification, requesting the Node synchronization refresh, cross-Service duplicate/conflicting-fingerprint races, and injected accepted-transition errors. Commit-before-error and pre-commit failures are quarantined as `authorization_uncertain`; a failed reconciliation keeps admission blocked.
- `go test ./internal/ipc -count=1` passed for connection-scoped `messageId` replay caching: equivalent request payloads with changed timestamps reuse the original response, conflicting payloads return `message-id-conflict` without dispatch, and the bounded per-connection cache closes rather than evicting replay history.
- A Unix-domain socket integration test verifies the live IPC loop dispatches an initial command once, returns the same response for an equivalent same-`messageId` replay, and returns `message-id-conflict` for a different payload without invoking the handler again.
- A second Unix-domain socket test sends a Node drain through authenticated dispatch, fresh capability resolution, the Command Service, the durable test ledger, and an injected executor. It verifies the stored requester identity and response correlation, then proves same-message replay and conflicting-message rejection do not invoke authorization or execution again.
- That authenticated IPC test also makes the fake executor update the adapter's observed Node snapshot and invokes the real `NodeSynchronizer.Refresh`; the client receives the committed generation-2 `DRAIN` event through the existing Node stream. The test passed 20 consecutive runs.
- An authenticated Unix IPC denial test resolves the peer as an operator, receives canonical `authorization-denied`, verifies the durable `denied` tombstone, and asserts zero executor calls.
- An authenticated Unix IPC persistence-fault test injects an uncertain accepted transition and failed reconciliation; it verifies canonical non-retryable `ledger-persistence-failed`, a blocked admission gate, the retained non-executable claim, and zero executor calls.
- A same-connection authorization test changes the resolver from admin to operator and back: a new ID is denied after revocation, the denied ID stays denied after permission is restored, and a fresh ID is allowed. Resolver and executor call counts confirm each eligible new command is freshly authorized exactly once.
- `go test ./cmd/cockpit-slurm` passed after daemon startup wiring.
- Command Service tests verify unknown outcomes request Node synchronization while definite execution failures do not.
- Ledger, Command Service, and authenticated IPC tests verify requester GID, original request `messageId`, and daemon service identity survive persistence and reopen; the v1 migration test preserves existing records and initializes unavailable historical attribution fields to explicit defaults.
- On 2026-10-23, an attempt to run the compiled command-package tests as `cockpit-slurm` in an isolated transient systemd unit with `StateDirectory=cockpit-slurm-stage-d`, `NoNewPrivileges=true`, `ProtectSystem=strict`, and `ProtectHome=true` reported test failure without per-test diagnostics. A verbose follow-up could not run because non-interactive sudo required a password. This attempt is not counted as passing evidence; the production ledger was not accessed and no Slurm/helper operation was invoked. The temporary test binary was absent on follow-up inspection.

## Remaining Stage D Gates

- Diagnose and rerun the isolated hardened systemd command-package suite, then exercise SQLite commit-before-error and pre-commit failures through the production Command Service/ledger combination under the production systemd unit. The injected service-level reconciliation contract is covered locally.
- Verify the refreshed Node state and event on a real Slurm mutation; the complete cache/stream path is covered locally with an injected execution boundary.
- Run the dedicated real-cluster drain/resume test through authenticated Cockpit IPC, proving the original requester is authorized before the helper is invoked, and restore the test Node's original state.
- Record the target installation's exact command response, ledger record, helper execution identity, and subsequent Node event.

Stage D remains in progress and is not accepted. Stage E and mutation UI remain unimplemented and gated on Stage D acceptance.
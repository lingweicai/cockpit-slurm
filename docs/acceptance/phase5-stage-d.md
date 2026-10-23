# Phase 5 Stage D Acceptance Record

Date recorded: 2026-10-23.

## Gate Entry

Stage D implementation started after Stages A, B, and C were explicitly recorded as passed in their acceptance records. This entry records authorization to begin Stage D; it does not mark Stage D accepted.

## Implemented Progress

- Added a backend Command Service using the existing v1 command payload, operation registry, fresh Authorizer, canonical Node cache, durable SQLite ledger, and injected Node execution boundary.
- The service authenticates from the connection-derived `UserContext`, derives `node.drain` or `node.resume` from the validated state, checks the exact target in the authoritative Node cache, and rejects invalid requests before claiming or authorizing them.
- New commands persist `authorization_pending` before invoking the Authorizer, then persist `accepted` and `running` before calling the executor. Denials and authorization lookup failures become durable non-executable tombstones.
- Caller UID plus `commandId` and a canonical request fingerprint scope deduplication. Matching retries return stored outcomes without reauthorization or redispatch; conflicting payloads are rejected.
- Added a fixed D-Bus client for the Stage C restricted helper. The command record keeps requester identity separate from the helper's configured Slurm execution identity and D-Bus boundary.
- Registered `command` as an authenticated IPC handler and wired the command service, registry, Authorizer, and helper client into daemon startup. No frontend mutation UI was added.
- After a durably recorded successful result, the service requests a bounded asynchronous refresh through the existing `NodeSynchronizer.Refresh`; it does not optimistically mutate the Node cache.
- The D-Bus executor treats unclassified transport failures as `unknown` outcomes; only explicit helper validation/access/execution errors are considered definite failures.

## Verification

- `go test ./internal/command -count=1` passed for command lifecycle, durable success/deduplication, denied and lookup-failure tombstones, invalid target rejection before claiming, the fixed D-Bus operation boundary, unknown transport outcome classification, and requesting the Node synchronization refresh.
- `go test ./cmd/cockpit-slurm` passed after daemon startup wiring.

## Remaining Stage D Gates

- Add and verify complete message-ID retransmission behavior and concurrent/ledger-race handling, including uncertain persistence outcomes through the Command Service.
- Add command-service integration coverage for resume, execution failure/unknown outcomes, identity mismatch, and dispatch prevention for every invalid/unauthorized case.
- Verify the refreshed authoritative Node state arrives through the existing event stream after a real mutation; the refresh callback itself has focused unit coverage.
- Run the dedicated real-cluster drain/resume test through authenticated Cockpit IPC, proving the original requester is authorized before the helper is invoked, and restore the test Node's original state.
- Record the target installation's exact command response, ledger record, helper execution identity, and subsequent Node event.

Stage D remains in progress and is not accepted. Stage E and mutation UI remain unimplemented and gated on Stage D acceptance.
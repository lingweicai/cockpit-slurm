# Phase 5 Stage B Acceptance Record

Date recorded: 2026-10-20.

## Local implementation and evidence

- Added typed v1 command and capability payloads, strict unknown/duplicate-key validation, canonical error codes, and existing four-byte framing tests.
- Added the `node.drain` → `nodes.drain` and `node.resume` → `nodes.resume` registry with consistency validation.
- Added fresh Slurm user capability resolution and fail-closed authorization tests for `none`, `operator`, `admin`, coordinator-only, missing users, lookup errors, unknown AdminLevel, stale permissions, and identity mismatch.
- Added authenticated capability request dispatch using the connection's server-derived `UserContext`; capability responses have explicit booleans and echo `messageId`.
- Added same-connection tests for concurrent outstanding capability requests and independently correlated responses.
- Added read-only `sacctmgr --json` user lookup and wired it into daemon startup. Its output shape was manually confirmed against the generated Slurm v0.0.43 user model as the development account.
- On 2026-10-20, the operator ran the documented read-only query as `cockpit-slurm`. SlurmDB returned exactly one `slurm_admin` record with `administrator_level=["Administrator"]`, the `devel` association, empty coordinators, and no errors or warnings. Metadata identified client user/group `cockpit-slurm:cockpit-slurm` and Slurm 25.05.7.

## Verification

- `go test ./...` passed on 2026-10-20.
- `go test -race ./internal/authorization ./internal/identity ./internal/dispatcher ./internal/service ./internal/ipc ./internal/protocol` passed.
- `git diff --check` passed.

## Remaining deployment evidence

The service-account lookup is verified for `slurm_admin` only. The other permitted users' records have not been queried as `cockpit-slurm`; resolver role behavior for `none`, `operator`, and coordinator-only users is currently covered by unit tests. Capability lookup fails closed as `capability-unavailable` if the runtime query fails.

No command execution or mutation UI is enabled by this stage record.

## Stage B gate decision

Stage B is explicitly marked passed based on the recorded protocol and authorization-contract evidence in this file.

- Typed v1 command/capability protocol, strict validation behavior, and canonical error handling were implemented and tested.
- Required operation-to-capability mappings (`node.drain` -> `nodes.drain`, `node.resume` -> `nodes.resume`) were implemented with registry consistency checks.
- Fresh authorization resolution and fail-closed behavior were validated by tests across role and lookup-failure scenarios.
- Capability request/response dispatch and correlation on the authenticated connection path were implemented and tested.

These results satisfy the Stage B exit criteria for protocol and authorization contracts, while Stage D command execution remains gated by Stage C/D sequencing.
# Phase 5 Stage B Acceptance Record

Date recorded: 2026-10-20.

## Local implementation and evidence

- Added typed v1 command and capability payloads, strict unknown/duplicate-key validation, canonical error codes, and existing four-byte framing tests.
- Added the `node.drain` → `nodes.drain` and `node.resume` → `nodes.resume` registry with consistency validation.
- Added fresh Slurm user capability resolution and fail-closed authorization tests for `none`, `operator`, `admin`, coordinator-only, missing users, lookup errors, unknown AdminLevel, stale permissions, and identity mismatch.
- Added authenticated capability request dispatch using the connection's server-derived `UserContext`; capability responses have explicit booleans and echo `messageId`.
- Added same-connection tests for concurrent outstanding capability requests and independently correlated responses.
- Added read-only `sacctmgr --json` user lookup and wired it into daemon startup. Its output shape was manually confirmed against the generated Slurm v0.0.43 user model as the development account.

## Verification

- `go test ./...` passed on 2026-10-20.
- `go test -race ./internal/authorization ./internal/identity ./internal/dispatcher ./internal/service ./internal/ipc ./internal/protocol` passed.
- `git diff --check` passed.

## Remaining deployment check

The SlurmDB query has not been run as the `cockpit-slurm` systemd account in this environment; non-interactive sudo was unavailable. Before marking Stage B fully accepted, verify that the installed daemon account can read user, association, and coordinator records using the documented read-only `sacctmgr` query. Capability lookup fails closed as `capability-unavailable` if this permission is absent.

No command execution or mutation UI is enabled by this stage record.
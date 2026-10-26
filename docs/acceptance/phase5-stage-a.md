# Phase 5 Stage A Acceptance Record

Date recorded: 2026-10-19. Trace timestamps are host-local; timezone was not included in the excerpts.

## Cockpit peer identity observations

The operator reported running the captures from Cockpit sessions for the named accounts. `SO_PEERCRED` returned:

| Cockpit account reported for session | Peer UID | Peer GID | Capture time |
|---|---:|---:|---|
| `slurm_none` | 1006 | 978 | 17:14:06 |
| `slurm_admin` | 1008 | 978 | 17:17:36 |
| `slurm_operator` | 1007 | 978 | 17:20:18 |
| `slurm_coord` | 1009 | 978 | 17:22:36 |

These values match the supplied Linux account records. The daemon trace showed successful `accept4` followed by successful `getsockopt(..., SOL_SOCKET, SO_PEERCRED, ...)` for each captured connection.

`visitor` is intentionally not permitted to access the socket. Its socket preflight failed before a connection was accepted, so there is no `SO_PEERCRED` observation for that account; this is expected access policy, not an identity mismatch.

The evidence consists of operator-provided `strace` excerpts and account records. The association between each authenticated Cockpit session and its capture is operator-reported; the raw peer-credential trace alone does not identify the browser login.

## Stage A gate decision

Stage A target-installation identity mapping is recorded as passed for this installation evidence set.

- Recorded Cockpit sessions for `slurm_none`, `slurm_operator`, `slurm_admin`, and `slurm_coord` each showed a distinct `SO_PEERCRED` UID with a matching Linux account record.
- The daemon trace showed successful `accept4` and successful `SO_PEERCRED` retrieval for each captured connection.
- `visitor` remained blocked at socket access preflight by policy, so no accepted connection and no peer-credential observation were expected.

Based on this recorded evidence, the Stage A exit criterion requiring target-installation authenticated-user to peer-credential mapping validation is closed.

## Local Stage A verification

- `go test ./...` passed.
- `go test -race ./internal/ipc ./internal/connection ./internal/stream` passed.
- `git diff --check` passed.

The transport regression covers framed command dispatch and Node event delivery on one connection, correlated responses, stream cleanup, and bounded-queue overflow. No command execution or mutation UI is enabled by this record.

Stage A is explicitly marked passed.
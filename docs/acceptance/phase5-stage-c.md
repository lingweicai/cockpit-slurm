# Phase 5 Stage C Acceptance Record

Date recorded: 2026-10-20.

## Progress

- Added `StateDirectory=cockpit-slurm` and `StateDirectoryMode=0750` to the production unit. `ProtectSystem=strict` remains enabled.
- Added pure-Go `modernc.org/sqlite` v1.60.1, compatible with Go 1.26, through `GOPROXY=https://goproxy.cn,direct`.
- The daemon initializes `/var/lib/cockpit-slurm/commands.db` before opening the IPC listener. Database files are mode `0600`; the state directory is mode `0750`.
- SQLite uses WAL journaling, `synchronous=FULL`, a 5-second busy timeout, foreign-key enforcement, and in-memory temporary storage. WAL and shared-memory sidecars are created with no group/other access.
- Schema version 1 is stored in `PRAGMA user_version`; migrations run transactionally, and newer unsupported versions fail startup.
- Startup integrity-checks the database and quarantines interrupted `authorization_pending`/`accepted` records as `authorization_uncertain`; `running` records become `unknown`.
- Added durable caller-UID/command-ID claims, request-fingerprint conflict detection, 30-day expiry, explicit lifecycle transitions, and a process admission gate that closes on persistence uncertainty.
- Reconciliation integrity-checks, quarantines unresolved states, checkpoints WAL, verifies no executable intermediate state remains, and only then reopens admission.
- `systemd-analyze verify packaging/cockpit-slurm.service` passed.
- After the operator restarted the service on 2026-10-20, `systemctl status cockpit-slurm` reported `active (running)`.
- Post-restart `stat` showed `/var/lib/cockpit-slurm` as `0750 cockpit-slurm:cockpit-slurm` and `commands.db`, `commands.db-wal`, and `commands.db-shm` as `0600 cockpit-slurm:cockpit-slurm`.
- Latest install verification: `/usr/local/bin/cockpit-slurm` modified at 12:05:36 CST; systemd launched PID 26663 at 12:05:48 CST and reported it active. The ledger files retained the expected service ownership and modes.
- Follow-up on 2026-10-20: systemd still reports PID 26663 active with `StateDirectory=cockpit-slurm`, `StateDirectoryMode=0750`, `ProtectSystem=strict`, and `NoNewPrivileges=yes`.
- Read-only Slurm snapshots reported `c21`-`c24` as `IDLE`; `squeue -w c21,c22,c23,c24` returned no jobs. No node was identified as the dedicated mutation-test node, and no drain/resume was issued.
- The operator subsequently designated `c21` as the test node. Direct drain and resume attempts as `cockpit-slurm` both returned `slurm_update error: Invalid user id`; read-only checks showed c21 remained `IDLE`, and no jobs were allocated. Direct service-account execution therefore failed and is not approved.
- A read-only SQLite status query as `cockpit-slurm` could not be run because `sudo -n` required a password. Ledger contents after restart and recovery of a seeded interrupted record remain unverified.

## Verification

- `go test ./...` passed on 2026-10-20.
- `go test -race ./internal/ledger ./internal/authorization ./internal/identity ./internal/service ./internal/dispatcher ./internal/ipc ./internal/protocol` passed.
- `git diff --check` passed.

## Remaining Stage C Gates

- No command service consumes the ledger yet; command execution remains out of scope for this slice.
- Fault-injection tests cover commit-before-error for claims, accepted transitions, running intent, denied tombstones, and authorization-failure tombstones. True pre-commit failures for claim, denial, accepted reservation, and running intent are reconciled from the last durable state; claim absence is confirmed before admission reopens, and unresolved/corrupt recovery keeps admission blocked.
- Tests cover reopen persistence, startup quarantine (`authorization_pending`/`accepted` → `authorization_uncertain`, `running` → `unknown`), denied tombstone immutability, schema-version rejection, corrupt database rejection, expiry behavior, independent-handle claim races, and database lock timeout/reconciliation.
- Commit-before-error and true pre-commit fault injection cover claims, accepted/running intent, denial tombstones, and authorization-failure tombstones. Reconciliation checkpoints WAL and only reopens admission after integrity and unresolved-state checks pass.
- Restart confirmed the daemon starts with the ledger present and correct filesystem ownership/modes, but persistent record recovery still needs a seeded interrupted-record restart test under the production unit.
- Direct execution failed on the approved test node: both `sudo -u cockpit-slurm /usr/bin/scontrol update NodeName=c21 State=DRAIN Reason=phase5-direct-execution-test` and `... State=RESUME` returned `slurm_update error: Invalid user id`; c21 remained `IDLE` and `squeue` showed no jobs.
- Added a separately activated D-Bus helper whose executable/unit are root-owned but whose process and fixed `scontrol` child run as the configured SlurmUser (`slurm`, UID 202 on this host). Bus calls are limited to the `cockpit-slurm` user and the helper independently verifies that caller UID. It maps only fixed drain/resume operations to argument-array execution. Positive/negative unit tests, full Go tests, race tests, and `go vet ./...` pass; policy XML parses and staged install paths include the helper artifacts.
- An earlier host check confirmed `/usr/local/libexec/cockpit-slurm-node-helper` was root-owned mode `0755` and a `dev` caller (UID 1002) received D-Bus `AccessDenied`; no `scontrol update` process ran and c21 remained `IDLE`. Latest inspection shows the helper service is inactive and the installed unit still has `User=root`, so the current SlurmUser source configuration is not yet installed; repeat the negative-call check after installation.
- A fresh read-only check shows c21 `IDLE` with no allocated jobs. Positive c21 drain/resume through the helper, verification/restoration, and confirmation that Slurm observes UID 202 remain unverified. The helper is not wired to the Command Service. Stage D remains gated.
- No Command Service consumes the ledger yet, so absence of unintended execution or duplicate mutations across restart has not been exercised. Command execution remains gated.

No mutation endpoint or execution is enabled. Stage C is not passed.
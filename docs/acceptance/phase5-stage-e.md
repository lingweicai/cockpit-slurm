# Phase 5 Stage E Acceptance Record

Date recorded: 2026-10-25.

## Gate Entry

The Stage D acceptance record marks Stages A-C and Stage D as passed. Stage A records the target-installation peer-identity mapping as passed based on operator-reported Cockpit sessions and daemon `SO_PEERCRED` traces. This record does not replace or extend that evidence.

## Implemented Progress

- `NodeProvider` shares one IPC client between the Node stream and capability/command requests, requests capabilities after the snapshot, exposes drain/resume methods, and tracks command status separately from authoritative Node state.
- The provider rejects commands when capability information, connection state, or the latest observed Node state is unavailable or inapplicable. Pending, running, and unknown outcomes prevent duplicate submissions.
- `NodeTable` uses a rightmost PatternFly `ActionsColumn`, with actions gated by current capability and observed state. Drain requires a validated reason and confirmation; resume identifies the target and requires confirmation. Command progress and errors are displayed without optimistic Node-state updates.
- The State cell formats the primary Slurm state together with its state flags, so composite states such as `IDLE+DRAIN` are displayed rather than only `IDLE`.
- `GenericTable` supports the accessible screen-reader heading and action-cell semantics required by the menu column.

## Verification

- `npm run test:ipc` passed, including framing, client, native channel, Node stream, and command-service checks.
- After the composite-state display fix, `npm run build`, `npm run test:ipc`, `npx eslint src/node-table.tsx`, and `git diff --check` passed.
- ESLint passed for `src/node-provider.tsx`, `src/node-table.tsx`, and `src/generic-table.tsx`.
- `git diff --check` passed.
- Repository-wide ESLint still reports 262 errors in six untouched files: `src/app.tsx`, `src/index.tsx`, `src/ipc/channel.ts`, `src/ipc/framing.test.ts`, `src/ipc/framing.ts`, and `src/ipc/node-query.ts`.
- The live Cockpit route returned HTTP 200 when inspected with TLS verification disabled. After deploying the composite-state fix, the installed bundle at `/usr/local/share/cockpit/cockpit-slurm/index.js` matches `dist/index.js`, SHA-256 `f2d32e862302afafe8fbbc53de0671833c7874477b464f91bd0b887faad19f6c`.

## Browser Acceptance Status

Initially, the integrated browser rejected the host's HTTPS certificate. After the user installed the Cockpit CA in the local Windows trust store, a trusted browser session reached Cockpit and the user authenticated as `slurm_admin`.

On 2026-10-25, the user authenticated as `slurm_admin` in the trusted Windows Edge browser. The deployed UI showed `c21` with reason `test v4`, command `drain: succeeded` (`cmd-edcc8ebc-a0c6-439d-a4bf-faf5fad52304`), and state `IDLE+DRAIN`. The user-provided `sinfo` output independently reported `c21` in `drain`. The installed bundle hash was subsequently confirmed to match the rebuilt bundle. This confirms the composite-state display fix and authoritative state agreement for the drain path. The reported node remains drained; this evidence does not include a resume result.

The repository browser bootstrap was not run: it changes root/admin credentials, adds SSH keys, changes the core-dump setting, and installs a VM image. The existing browser test only checks the general application page and does not cover Stage E command workflows.

## Stage E Decision

Stage E is implemented and its local frontend checks pass, but it is **not accepted**. The trusted admin browser report verifies the drain command result and composite state against `sinfo`. Acceptance remains pending:

- admin capability refresh and correlated command response;
- admin resume confirmation, visible progress/result, and authoritative Node update through the stream;
- non-admin controls unavailable, and controls unavailable when capabilities or observed state are missing;
- command rejection/error display and duplicate prevention while pending or unknown;
- the authenticated browser account corresponds to the daemon-observed peer UID/GID for the command connection.

Use the dedicated test Node, verify and restore its initial state, and record the browser identity, command results, and stream generations before marking Stage E accepted.
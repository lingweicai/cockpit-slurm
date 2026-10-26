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
- `npm run test:node-ui` passed synthetic policy checks for missing capabilities, disconnected/unavailable Node state, non-admin capabilities, `IDLE+DRAIN` resume eligibility, inapplicable states, duplicate blocking for pending/running/unknown outcomes, and error classification.
- The same `test:node-ui` run bundles the actual React command-status and PatternFly actions-cell components, server-renders pending, failed, denied, and unknown status feedback, checks command IDs/errors are visible, and verifies the rendered action menu is disabled/empty for unavailable data and active command outcomes. These checks do not open IPC or invoke Slurm.
- After the composite-state display fix, `npm run build`, `npm run test:ipc`, `npx eslint src/node-table.tsx`, and `git diff --check` passed.
- After adding the rendered-component tests, the production build, IPC suite, UI tests, focused ESLint, and `git diff --check` passed.
- ESLint passed for `src/node-provider.tsx`, `src/node-table.tsx`, and `src/generic-table.tsx`.
- `git diff --check` passed.
- Repository-wide ESLint still reports 262 errors in six untouched files: `src/app.tsx`, `src/index.tsx`, `src/ipc/channel.ts`, `src/ipc/framing.test.ts`, `src/ipc/framing.ts`, and `src/ipc/node-query.ts`.
- The live Cockpit route returned HTTP 200 when inspected with TLS verification disabled. After deploying the composite-state fix, the installed bundle at `/usr/local/share/cockpit/cockpit-slurm/index.js` matches `dist/index.js`, SHA-256 `f2d32e862302afafe8fbbc53de0671833c7874477b464f91bd0b887faad19f6c`.

## Browser Acceptance Status

Initially, the integrated browser rejected the host's HTTPS certificate. After the user installed the Cockpit CA in the local Windows trust store, a trusted browser session reached Cockpit and the user authenticated as `slurm_admin`.

On 2026-10-25, the user authenticated as `slurm_admin` in the trusted Windows Edge browser. The deployed UI showed `c21` with reason `test v4`, command `drain: succeeded` (`cmd-ca66a8f8-1a14-49fa-b121-f14fc7b4b60d`), and state `IDLE+DRAIN`. The user-provided `sinfo` output independently reported `c21` in `drain`.

The same browser session then submitted resume for `c21`; the UI reported `resume: succeeded` (`cmd-7ced7294-8fde-455f-bde4-f92b0fe63e1c`) and showed `c21` state `IDLE` with empty reason. The user-provided `sinfo` output independently reported all nodes idle, including `c21`.

The user then logged in as `slurm_none`; the Node table rendered without mutation actions for all rows. This is consistent with the expected non-admin capability-denied presentation.

The installed bundle hash was subsequently confirmed to match the rebuilt bundle.

The repository browser bootstrap was not run: it changes root/admin credentials, adds SSH keys, changes the core-dump setting, and installs a VM image. The existing browser test only checks the general application page and does not cover Stage E command workflows.

## Stage E Decision

Stage E frontend integration is **accepted** as of 2026-10-26. The operator-reported Windows Edge tests verify admin drain/resume responses against authoritative `sinfo` state and verify that `slurm_none` sees no mutation actions. The local policy and rendered-component tests verify pending/error/unknown feedback, correlation-ID presentation, duplicate blocking, and disabled/empty actions when capabilities or observed state are unavailable. The Stage A acceptance record remains the evidence for the authenticated Cockpit-user-to-peer-credential mapping prerequisite.

Pending/error rendering was verified with React server rendering of the actual presentation component, not with a live delayed or disconnected Cockpit command. No Slurm command was issued by the synthetic UI tests. The repository browser bootstrap remains intentionally unrun because it changes host credentials and settings.
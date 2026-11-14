# Phase 6 Stage A Audit and Baseline

Date recorded: 2026-10-30.

## Decision

**In progress; not accepted.** No Phase 6 resource or command implementation is enabled by this record. The owner selected the Job source for the tested installation and approved normalization for the observed source representations, independent visibility branches, the stream direction, and the extraction direction on 2026-11-09. The active-state classification is a proposal for review; transition-flag behavior, source completeness, and deployment limitations remain open. The updated documentation and remaining evidence limitations require the owner's explicit review and approval before Stage A acceptance can be recorded. This record does not authorize Stage B.

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
- At the initial investigation, source approval remained open. The owner later selected the candidate for the tested installation; see the subsequent samples and Owner Decisions below. Active-state and array semantics, failure handling, and complete visibility still require the documented evidence and review. A successful empty response under `dev` is insufficient evidence. No fallback source is approved by this record.

## Stage A Recheck (2026-11-08)

This recheck is read-only and does not change the acceptance decision.

### Runtime and Job-source evidence

- `systemctl show cockpit-slurm.service` reports `User=cockpit-slurm`, `Group=cockpit-slurm`, and `ActiveState=active`; `getent passwd cockpit-slurm` resolves UID 201.
- The installed client reports Slurm `25.05.7`; `scontrol ping` reports the primary controller at `sms10` is UP.
- `scontrol show config`, run as the invoking user `dev`, reports `ClusterName=cluster`, `PrivateData=none`, and `SlurmUser=slurm(202)`. This does not establish the daemon's source visibility.
- A current `scontrol show job --json` probe as `dev` returned `errors: []`, warning `Zero jobs to dump`, and zero Jobs. The queried parser field was null in this empty response. This provides no normal-Job or array-Job sample and does not establish complete enumeration.
- The initial non-interactive `sudo -n -u cockpit-slurm id` probe failed because sudo required a password. The operator subsequently supplied an interactive, read-only `sudo -u cockpit-slurm scontrol show job --json` transcript, recorded below. This provides daemon-context samples but does not by itself establish source completeness or approve the source.
- In the operator-provided daemon-context transcript, `scontrol show job --json` returned `errors: []`, `warnings: []`, and three records: Job 12 (`dev`, UID 1002, `COMPLETED`), Job 13 (`slurm_admin`, UID 1008, `RUNNING`), and Job 14 (`visitor`, UID 1010, `RUNNING`). All three reported cluster `cluster`, partition `normal`, `array_job_id` set to number `0`, and `array_task_id` unset. This confirms the backend identity can observe these sampled owners in the normal partition and provides normal-Job source representation candidates. It does not show any array elements, prove visibility for other partitions, or establish complete cluster-wide enumeration.
- The transcript's projected `.meta.data_parser` value was `null`, so parser-plugin/version evidence remains unavailable from this query. The included completed Job also means the selected source's active-state filtering rule must be defined and tested before it can populate the active-Job collection.
- The accompanying `sinfo` output showed the `normal` partition up, with `c22` and `c23` allocated and `c21` and `c24` idle. This is an operator-provided point-in-time scheduler view, not evidence that the Job snapshot covers all configured partitions or all active states.

The current candidate support scope is limited to the tested Slurm 25.05.7
configuration and its enumerated non-hidden partitions. The supplied inventory
reports only `normal`, with `Hidden=NO`. Deployments with hidden partitions are
not supported by this evidence; support for them requires a separate complete
enumeration comparison under the actual data-acquisition identity and explicit
review. Re-run the partition inventory below on `sms10` before treating this
restriction as verified for the target installation.

### Stream and policy implementation audit

- `internal/stream/node_registry.go` still registers a Node-only stream, sends the complete unfiltered Node snapshot, and fans out global `NodeChangeBatch` payloads. The payload has collection generation but no per-connection sequence or visibility projection.
- `internal/resource/node_cache.go` retains authoritative collection generation and emits a full Node value for removals. The agreed wire contract requires per-connection projection and identity-only removals at the delivery boundary.
- `src/ipc/types.ts`, `src/ipc/framing.ts`, and `src/ipc/node-stream.ts` still validate Node-only messages, require a full Node for every change, and detect gaps using consecutive collection generations. There is no shared sequence validator across resource kinds. These are Stage B implementation gaps, not completed Stage A behavior.
- The architecture now defines `node.list` → `node.cluster` and `job.list` → `job.active`. Policy-loader implementation and reference-validation tests remain deferred to Stage B; no runtime policy configuration was changed in this recheck.

### Recheck commands

The checks below are historical Stage A run results, not checks rerun for this
documentation update. They are automated repository checks, not live
Slurm/service-runtime verification:

| Check | Result |
| --- | --- |
| `go test ./...` | Passed. |
| `go vet ./...` | Passed. |
| `go test -race ./internal/resource ./internal/stream ./internal/ipc ./internal/command ./internal/ledger ./internal/authorization` | Passed. |
| `npm run test:ipc` | Passed. |
| `npm run test:node-ui` | Passed. |
| `npm run build` | Passed. |
| `npm run stylelint` | Passed. |
| `npx tsc --noEmit` | Failed with the same four existing diagnostics recorded above. |
| `npm run eslint` | Failed with 262 errors in `src/app.tsx`, `src/index.tsx`, `src/ipc/channel.ts`, `src/ipc/framing.test.ts`, `src/ipc/framing.ts`, and `src/ipc/node-query.ts`. |
| `git diff --check` | Passed for the documentation changes. |

Live Slurm/service-runtime evidence is recorded separately in the source
investigation sections above and below. Those read-only point-in-time samples
do not establish complete enumeration or runtime failure behavior. No live
integration test, service restart, production configuration change, or
service-account execution was performed as part of the automated checks listed
above.

No Slurm job was submitted, modified, or cancelled, and no service restart or runtime configuration change was performed as part of the automated checks above. The service-account source probes recorded elsewhere were read-only operator-provided evidence.

## Held Array Sample (2026-11-09)

- The operator supplied read-only `sudo -u cockpit-slurm scontrol --all --local show job --json` output. Its `meta.client.user` was `cockpit-slurm`, its `meta.plugin.data_parser` was `data_parser/v0.0.43`, and its `errors` and `warnings` arrays were empty.
- The output included pending held Job 15 in cluster `cluster`, partition `normal`, for UID 1002 and account `easyhpc`. Its `job_id` and set `array_job_id` were both 15; `array_task_id` was unset and `array_task_string` was `1-10`. The operator's `squeue` showed `15_[1-10]` held by the user. This is a representative aggregated array placeholder, not an individual array element.
- The same output contained cancelled Jobs 13 and 14, both with `array_job_id` set to zero, `array_task_id` unset, and empty `array_task_string`. One record had an empty account. These observations do not establish the normalization rule, the set of active states, or how missing account data affects visibility.
- This sample verifies the reported parser path and daemon-context visibility for these records only. At the time of this sample, individual array-element output was not yet available; see the subsequent running array sample below. Complete cluster-wide visibility and failure/truncation behavior remain open. The subsequent source selection is scoped to the tested installation; Stage A remains unaccepted.

## Running Array Sample (2026-11-09)

- The operator submitted array Job 16 with task IDs 0 and 1. In the supplied `scontrol --all --local show job --json` output, `meta.client.user` was `cockpit-slurm`, `meta.plugin.data_parser` was `data_parser/v0.0.43`, and `errors` and `warnings` were empty.
- Both elements were `RUNNING` in cluster `cluster`, partition `normal`, owned by UID 1002 with account `easyhpc`. The element for task 0 had `job_id` 17, `array_job_id` set to 16, and `array_task_id` set to 0; the element for task 1 had `job_id` 16, `array_job_id` set to 16, and `array_task_id` set to 1. Both had an empty `array_task_string`. Thus, for these observed source records, task ID zero is explicitly set and must not be normalized to absence; their canonical tuple candidates are `(cluster, 17, 16, 0)` and `(cluster, 16, 16, 1)` respectively.
- The same snapshot contained cancelled held-array Job 15, with `array_job_id` 15 set, `array_task_id` unset, and task string `1-10`. This confirms that terminal records can appear alongside running elements; the complete active-state selection rule remains to be defined and verified.
- Operator-provided `squeue --all --local --array` output as `dev` and as `cockpit-slurm` showed the same two running tasks, `16_0` and `16_1`, for this point-in-time comparison. The operator's partition/configuration output reported only `normal`, with `Hidden=NO`, `ClusterName=cluster`, `PrivateData=none`, and `SlurmUser=slurm(202)`. This sample does not verify active jobs owned by different users simultaneously, hidden partitions, or complete visibility for all states and deployments.
- These observations provide representative element and placeholder evidence, not proof of completeness. The owner subsequently selected the tested-installation candidate and approved rules for the observed representations; completeness, malformed/truncated response handling, active-state semantics, final contract review, and explicit Stage A acceptance remain open.

## Array Expression Length Sample (2026-11-09)

- The operator submitted held array Job 18 with an irregular set of task IDs from 0 to 630, then supplied two read-only `scontrol --all --local show job --json` projections run via `sudo -u cockpit-slurm`. The commands differed only in `SLURM_BITSTR_LEN`: unset in the first, `0` in the second.
- Both projected records showed `job_id` 18, `array_job_id` set to 18, and `array_task_id` unset. With the variable unset, `array_task_string` ended in `...,153,171,19...`, truncating the expression inside task ID 190. With `SLURM_BITSTR_LEN=0`, the expression ended in `...,153,171,190,210,231,253,276,300,325,351,378,406,435,465,496,528,561,595,630`, consistent with the submitted task IDs.
- The default output was valid JSON despite containing an incomplete array expression. The operator's `jq` projection did not include `meta`, `errors`, or `warnings`, so this comparison does not establish their values for either response. The sample demonstrates the limit's effect for this source and installation; it does not prove that setting the variable eliminates all possible truncation or guarantees complete cluster-wide enumeration.
- The selected tested-installation candidate explicitly sets `SLURM_BITSTR_LEN=0` in the backend acquisition process; it must validate command success and the full response and reject malformed, abbreviated, or otherwise incomplete array expressions rather than committing a partial snapshot. Verify the execution environment, parser/version behavior, and large-expression/frame-size limits. Do not treat successful JSON decoding as proof of completeness.

## Multi-User Samples (2026-11-09)

- The operator supplied `squeue --json` output whose `meta.client.user` was `dev`. It showed pending held arrays 18 (`dev`, UID 1002), 19 (`slurm_none`, UID 1006), and 20 (`slurm_admin`, UID 1008), as well as running normal Jobs 21 (`visitor`, UID 1010) and 22 (`slurm_admin`, UID 1008). The three array expressions were abbreviated in that default-environment JSON. This establishes only the invoking user's view at that time, not the daemon's view.
- The operator also supplied submission transcripts for Jobs 22 (`slurm_admin`), 23 (`slurm_operator`), 24 (`slurm_coord`), and 25 (`slurm_none`). The pasted `squeue --json` snapshot included Job 22 but not Jobs 23-25; submission does not establish that they ran successfully or were visible to the daemon.
- A subsequent read-only `SLURM_BITSTR_LEN=0 scontrol --all --local show job --json` query run as `dev` returned the complete displayed array expressions for held Jobs 18-20, with empty `errors` and `warnings` and parser `data_parser/v0.0.43`. It reported Jobs 21 and 22 as `COMPLETED` and Jobs 23-25 as `FAILED`; no failure cause was investigated. A contemporaneous `squeue --all --local --array` as `dev` listed the held arrays 18-20, not the short Jobs 21-25.
- The operator subsequently supplied `sudo -u cockpit-slurm env SLURM_BITSTR_LEN=0 scontrol --all --local show job --json` output, projected to Job IDs 18-25. It reported client `cockpit-slurm`, parser `data_parser/v0.0.43`, and empty `errors` and `warnings`. It contained all three pending held arrays 18-20 with the same respective owners and full displayed expressions as the `dev` sample; the accompanying `dev` queue showed the same three held array IDs.
- The daemon projection also contained completed Jobs 21 and 22; Job 21 had an empty account. Jobs 23-25 were absent from this later projection, despite appearing as failed in the earlier `dev` query; the reason for their absence is unknown. These point-in-time samples establish daemon visibility of the three held arrays, not complete enumeration, daemon visibility across all submitted users or states, or frontend authorization. Stage A remains unaccepted.

## Stage A Review Preparation (2026-11-09)

### Owner Decisions

- The owner selected `scontrol --all --local show job --json` under the `cockpit-slurm` identity with `SLURM_BITSTR_LEN=0` as the candidate for this tested Slurm 25.05.7 installation and observed `data_parser/v0.0.43`. This is not a completeness certification or approval of other versions or configurations; hidden-partition behavior remains unverified.
- The owner approved normalization of the observed normal-Job zero/set combination to absent canonical array identifiers, preservation of individual array IDs including task zero, and preservation of array Job ID and nonempty task expression for aggregated placeholders. Retain cluster and source Job ID. Unknown, inconsistent, or ambiguous combinations reject the entire refresh without removals.
- The owner approved independent fail-closed direct-owner and coordinator-account visibility branches: a missing account does not invalidate a valid direct-owner match, and a missing owner does not invalidate a valid coordinated-account match. Missing attributes never grant access; if neither applicable branch establishes access, deny. Privileged roles remain governed by their existing policy.
- The owner approved preserving the accepted Phase 4 stream contract: one connection-wide sequence across Node and Job events, independent authoritative collection generations, visibility filtering before sequence allocation, no sequence use for hidden changes, identity-only visibility removals, authorization-context reconciliation, reconnect/fresh-snapshot recovery, and shared frontend sequence validation before provider dispatch. Phase 6 brings the Node implementation into conformance and extends the same mechanism to Job events; it does not change the accepted Phase 4 architecture.
- The owner approved the extraction direction below: resource-specific models, adapters, and collection generations; only demonstrated shared resource and stream mechanisms; separate query authorization and record visibility; and preservation of the Phase 5 command service, authorization, helper, ledger, and lifecycle without speculative generic command abstractions.
- The owner requested an active-state proposal and review of the updated documents and remaining evidence limitations before any Stage A acceptance or Stage B work. No acceptance or Stage B authorization has been recorded.

### Source and normalization proposal

- Selected candidate: `scontrol --all --local show job --json` with `SLURM_BITSTR_LEN=0` under the daemon's `cockpit-slurm` identity on the tested Slurm 25.05.7 installation. The owner approved this source selection, not universal completeness. The supplied snapshots cover normal-Job candidates, aggregated array placeholders, and individual running array elements including task zero; they do not prove complete enumeration across users, states, hidden partitions, or other configurations.
- The installed `scontrol` manual says `--all` displays jobs in hidden and group-unavailable partitions, `--local` restricts the query to the local cluster, and `--json` uses a data-parser plugin. The manual's stated options are not evidence that the actual execution identity can retrieve every required record or that a successful response is complete under all cluster configurations.
- For the observed v0.0.43 samples, the approved source-specific normalization retains `cluster` and source `job_id`; preserves a set array task ID, including zero, together with its array job ID; represents the observed nonzero array-job ID with an unset task ID and nonempty task expression as an array placeholder; and maps the observed normal-Job combination of zero array-job ID, unset task ID, and empty task expression to absent canonical array identifiers. Unknown, inconsistent, or ambiguous combinations reject the entire refresh; these approvals do not establish a general classifier for other parser versions.
- The installed `scontrol` manual describes `SLURM_BITSTR_LEN` as limiting the job-array task-ID expression to 64 bytes by default, with zero requesting the full expression. The Job 18 sample above confirms that the default limit affects `array_task_string` in JSON on this installation. The zero setting recovered the submitted expression in that sample; detection of other incomplete responses and behavior across configurations remain unverified despite the owner's scoped source selection.
- The installed `squeue --helpstate` lists states beyond `PENDING` and `RUNNING`, including `SUSPENDED`, `COMPLETING`, `CONFIGURING`, and requeue states. The exact supported active-state and flag combinations, including transitions and unknown values, need a reviewed rule and representative evidence. Unknown or invalid state must not silently cause active-cache removals. Do not equate the default `squeue` view with the complete active population.
- Stage B acceptance tests must simulate command failure, invalid/truncated JSON, abbreviated array expressions like the observed valid-JSON sample, invalid or ambiguous records, duplicate identities, oversized snapshots, and unchanged snapshots, and verify that failed refreshes leave the cache unchanged and emit no removals. Those are planned implementation tests, not tests performed in Stage A; source-side completeness still requires separate investigation.

### Read-only verification procedure for `sms10`

Run these probes on `sms10` while representative Jobs from the required owners
and states are present. They read Slurm configuration and queue state; they do
not submit, modify, hold, release, or cancel Jobs. Record the timestamp,
command, exit status, stderr, Slurm version, and the complete unmodified output
in the approved evidence location. Keep raw owner/account data protected; the
Stage A record may summarize owner coverage by numeric UID and redact display
names.

1. **Verify the source execution identity and installation scope.**

   ```sh
   date -Is
   systemctl show cockpit-slurm.service -p MainPID -p User -p Group -p ExecStart -p Environment -p EnvironmentFiles
   systemctl cat cockpit-slurm.service
   getent passwd cockpit-slurm
   sudo -u cockpit-slurm id
   sudo -u cockpit-slurm scontrol --version
   sudo -u cockpit-slurm scontrol ping
   sudo -u cockpit-slurm scontrol show config
   sudo -u cockpit-slurm scontrol show partition
   ```

   Record the daemon's actual OS UID/GID, Slurm client and controller
   versions, configured cluster name, `PrivateData`, `SlurmUser`, every
   partition and its `Hidden`/state attributes, and any service environment
   that affects Slurm access. Redact secret environment values from recorded
   evidence. Ensure the source probes below use the same Slurm configuration
   and relevant environment as the service; a bare `sudo -u` command does not
   prove equivalence if the unit supplies additional configuration. A
   partition inventory containing any hidden partition means this candidate
   remains unsupported until hidden-partition enumeration is independently
   verified and approved.

2. **Capture the candidate source under the actual acquisition identity.**

   ```sh
   sudo -u cockpit-slurm env SLURM_BITSTR_LEN=0 \
     scontrol --all --local show job --json
   ```

   Preserve the full output, command exit status, stderr, parser/plugin
   version, and top-level `errors` and `warnings`. For every record, summarize
   the configured cluster, `job_id`, numeric owner UID, account, partition,
   source state/base-state/flags, array job/task set/value fields, and
   `array_task_string`. Count records by owner UID, partition, and state. Note
   malformed records, duplicate canonical identity tuples, missing required
   fields, absent partitions, and any response-size or parser limits. Empty
   `errors`/`warnings` or successful JSON parsing alone do not prove
   completeness.

3. **Compare against an independent Slurm view at the same time.**

   Run the following queue query once as `cockpit-slurm` and once as an
   independently authorized Slurm operator. Record which identity ran each
   query; do not treat two commands run under the same identity as independent
   authorization evidence.

   ```sh
   sudo -u cockpit-slurm squeue --all --local --array --states=all \
     -h -o '%i|%A|%a|%u|%U|%T|%P|%R'
   ```

   ```sh
   squeue --all --local --array --states=all \
     -h -o '%i|%A|%a|%u|%U|%T|%P|%R'
   ```

   Reconcile the candidate source's active-record identity set with the
   independent queue view using Job/array identity, owner UID, partition, and
   state. Explain every mismatch, including ordinary Jobs, array elements,
   placeholders, owners, partitions, and transition states. Confirm that each
   required owner and each supported partition has representative records;
   if the live queue lacks a required owner/state, record the gap rather than
   claiming it was tested. If the installed `squeue` rejects `--states=all`,
   retain that failure and obtain the exact supported equivalent from the
   installed manual before repeating; do not silently narrow the comparison.

4. **Check expression truncation and command failure behavior.**

   With a currently visible long-expression array Job, compare these read-only
   queries under the acquisition identity:

   ```sh
   sudo -u cockpit-slurm scontrol --all --local show job --json
   sudo -u cockpit-slurm env SLURM_BITSTR_LEN=0 \
     scontrol --all --local show job --json
   ```

   Compare the same record's complete array expression, not just JSON validity;
   record exit status, stderr, `errors`, `warnings`, parser version, and
   expression length for both. If no suitable array is present, this probe
   remains open. Also record the response to a read-only invalid-ID probe:

   ```sh
   sudo -u cockpit-slurm scontrol show job not-a-job --json
   ```

   This checks invalid-identifier failure only. It does not simulate controller
   failure, parser truncation, or service-cache behavior. Those failure paths
   require separate Stage B tests; every failed, malformed, truncated,
   ambiguous, or oversized snapshot must preserve the cache and emit no
   removals.

5. **Record the support boundary.**

   The proposed support scope is this tested Slurm 25.05.7/parser
   configuration and enumerated non-hidden partitions only. Explicitly record
   the full partition inventory and confirm `PrivateData` does not prevent
   complete acquisition. Hidden-partition deployments remain unsupported
   unless separately verified and approved. Do not generalize this evidence
   to other Slurm versions, parser versions, clusters, or authorization
   configurations.

### Slurm 25.05 Documentation Review

- The [Slurm 25.05.7 array documentation](https://slurm.schedmd.com/archive/slurm-25.05.7/job_array.html) states that zero is a valid task index; individual elements have a job ID and array-job/task IDs, and pending tasks can be represented by an aggregated record. These statements support the observed task-zero and placeholder examples, but do not by themselves define the JSON parser's zero/set sentinel semantics for normal Jobs.
- The [Slurm 25.05.7 job-state documentation](https://slurm.schedmd.com/archive/slurm-25.05.7/job_state_codes.html) distinguishes base states from flags. It describes `PENDING`, `RUNNING`, and `SUSPENDED` as nonterminal base states, while flags such as `COMPLETING` and `REQUEUED` describe transitions. The repository's v0.0.43 OpenAPI `job_info.job_state` schema is an array whose enum includes both base-state and flag names. This schema is source-model evidence, not proof that the CLI JSON encodes every state/flag combination the same way.
- The proposal below defines conservative behavior for base states and transition flags, subject to owner review and source verification. The supplied CLI output establishes only the observed `PENDING`, `RUNNING`, `COMPLETED`, `CANCELLED`, and `FAILED` forms; no transition-flag sample has been verified. Unknown, unsupported, or ambiguous combinations must not be interpreted as authoritative removals.
- The owner resolved the restricted-visibility ambiguity by approving independent, branch-specific fail-closed evaluation. The earlier running `visitor` Job 21 had a valid numeric owner but an empty account in the operator's `squeue` JSON; Stage B tests must verify that a valid direct-owner match remains visible while an empty account cannot establish coordinator-account access. They must also cover the inverse missing-owner/valid-coordinated-account case and denial when neither branch succeeds.
- The [Slurm 25.05.7 scontrol manual](https://slurm.schedmd.com/archive/slurm-25.05.7/scontrol.html) documents `SLURM_BITSTR_LEN=0` and warns that larger expressions may affect performance. The Job 18 comparison corroborates this specific behavior for the installed source; it does not certify every possible incomplete or oversized response.
- A read-only `scontrol --all --local show job not-a-job --json` probe as `dev` printed `scontrol_print_job error: Invalid job id specified` rather than a JSON document. This tests invalid identifier handling only; it does not establish the daemon identity's behavior on controller failure, the command's exit status in this record, or all acquisition-failure modes.

### Stream contract review

- The accepted Phase 4 §§8–10 already define one connection-wide sequence shared across resource types; collection generation as separate authoritative cache metadata; visibility filtering before sequence allocation; no sequence consumption for hidden changes; identity-only visibility removal events; and reconnect with fresh snapshots as recovery. Phase 6 preserves this contract, brings the Node implementation into conformance, and applies the same mechanism to Job events. The current Node implementation's use of collection generation and full-Node removal payloads is an implementation gap, not an accepted-contract change.
- Do not add application-level `subscribe`/`unsubscribe` messages, `subscriptionId`, or another recovery protocol. The historical [2.1 ipc-protocol.md](../architecture/2.1%20ipc-protocol.md) gives accepted Phase 4/5 specifications precedence, but its §22 snapshot example omits the Phase 4 sequence field; §37 and §§54–55 retain per-resource generation conflict/gap detection and a `resync-required`/query recovery example; and §§40, 66, and 67 retain subscription lifecycle/package/test concepts. These passages require separately approved documentation cleanup; they do not authorize another recovery mechanism. Phase 4 §8 also retains a protocol-vocabulary list containing `subscribe`, despite the automatic-stream contract. Include that wording in the separately approved consistency cleanup, not as an application message contract.

### Active-state proposal for Slurm 25.05.7

This is the proposed active-collection policy for the selected
`data_parser/v0.0.43` source, not a claim that each state/flag combination has
been observed or is ready for implementation.

| Source combination | Proposed behavior | Evidence and status |
| --- | --- | --- |
| `PENDING` base, no transition flag | Include. | Observed in the supplied daemon-context held-array sample; Slurm documentation classifies it as nonterminal. |
| `RUNNING` base, no transition flag | Include. | Observed in supplied daemon-context normal-Job and array-element samples; Slurm documentation classifies it as nonterminal. |
| `SUSPENDED` base, no transition flag | Include. | Slurm documentation classifies it as nonterminal; not observed in supplied CLI output. |
| Active base plus `CONFIGURING` | Include only if source output explicitly represents an unambiguous active base and recognized flag together. | Transition described by Slurm documentation, but no source combination observed; reject until verified. |
| Active or terminal base plus `COMPLETING` | Include during cleanup only if source output explicitly represents an unambiguous base and recognized flag together. | Cleanup flag described by Slurm documentation, but no source combination observed; reject until verified. |
| `COMPLETED`, `CANCELLED`, or `FAILED` base, with no recognized active transition | Exclude after a complete valid snapshot. | `COMPLETED` and `CANCELLED` were observed in daemon-context output; `FAILED` was observed only in a supplied `dev` query, not the daemon-context sample. Slurm documents all three as terminal. |
| Active base plus `REQUEUED`, `REQUEUE_HOLD`, or `REQUEUE_FED` | Proposed include only when the base is explicitly active and the flag combination is recognized and unambiguous. | State/flag names appear in Slurm documentation/help, but no combination was observed; reject until verified. |
| `SPECIAL_EXIT`, a requeue-related flag without an active base, or an unclassified flag on a terminal/cleanup base | Reject the entire refresh. | No supplied source output establishes these combinations or their encoding. |
| Unknown base/flag, missing base needed to classify a flag, unsupported combination, or ambiguous representation | Reject the entire refresh; preserve cache and emit no removals. | Required fail-closed behavior. |

Other unclassified flags include `STOPPED`, `RESIZING`, `SIGNALING`,
`STAGE_OUT`, `POWER_UP_NODE`, `RESV_DEL_HOLD`, `REVOKED`, `LAUNCH_FAILED`,
and `RECONFIG_FAIL`. Their treatment must not be inferred from their names.

### Proposed Extraction Plan

The owner approved the following extraction direction, subject to the documented boundaries; this is not authorization to begin Stage B:

1. Retain `internal/resource/node.go`, `node_adapter.go`, and `node_synchronizer.go` as the Node-specific model, acquisition, and normalization boundary. Introduce a separate Job model and adapter only after the source and normalization decisions are approved.
2. Extract only demonstrated cache/snapshot and publication-ordering mechanisms from `internal/resource/node_cache.go`; retain independent collection generations and complete-snapshot validation. Preserve the atomic baseline/publication boundary and race tests.
3. Evolve `internal/stream/node_registry.go` and the existing IPC connection writer into one per-connection projection/sequence path. Validate sequencing across resource kinds before provider dispatch, preserve bounded queues and reconnect cleanup, and test revocation without disclosing now-hidden fields.
4. Adapt `internal/service/query.go` and the existing dispatcher to route resource-specific query handlers through the central query-policy and separate visibility evaluators. Load and validate `node.list` → `node.cluster` and `job.list` → `job.active` references before readiness; keep the authoritative cache shared.
5. Reuse the frontend IPC client and framing in `src/ipc`; add shared connection-level sequence validation before resource-specific providers, retaining typed Node and Job state. Keep the existing Phase 5 command service, authorization, helper, ledger, and execution lifecycle separate and covered by their regression tests.

Stage A remains **in progress; not accepted**. The owner must review and explicitly approve the updated documentation and remaining evidence limitations before acceptance is recorded; Stage B MUST NOT start before that acceptance and authorization.

## Checks Run

These are historical automated repository-check results from the prior Stage A
run; they were not rerun for this documentation update. They do not represent
live Slurm/service-runtime verification.

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

No live mutation, service restart, production ledger modification, or
service-account impersonation was performed as part of the automated checks
listed here. The read-only service-account source probes recorded above are
separate operator-provided evidence.

## Outstanding Decisions and Evidence

1. Execute the read-only `sms10` procedure above under the actual `cockpit-slurm` acquisition identity. Existing samples show held arrays 18–20 across three owners in the `normal` partition and a prior two-element array comparison; they do not prove complete enumeration across all required owners, supported partitions, or active states.
2. Verify the approved source-specific normalization against the daemon-context normal-Job, array-placeholder, and individual-element samples, including task ID zero. The owner approved the observed zero/set classifier for v0.0.43; unknown, inconsistent, and ambiguous representations must reject the refresh. Do not generalize this approval to other parser versions.
3. Confirm that Phase 6 preserves, rather than changes, the accepted Phase 4 stream contract. It already specifies the shared connection sequence, independent collection generation, pre-sequence visibility filtering, identity-only removals, and fresh-snapshot recovery. The current Node implementation is not yet conformant.
4. Confirm the query-policy mapping is explicit in the Phase 6 spec: `node.list` references `node.cluster`, and `job.list` references `job.active`; the referenced policy scopes are defined. Stage B must add validation tests that reject missing references, unknown policy identifiers, resource-kind mismatches, and enabled record-returning queries without references.
5. Review the owner-approved Node-to-shared extraction direction against the ownership and ordering boundaries above. Reuse existing transport, connection initialization, authorization, command registry, and ledger mechanisms; preserve Phase 5 command behavior without speculative generic command abstractions.
6. Account for the existing TypeScript and ESLint failures when planning the required Phase 6 acceptance checks; do not report those checks as passing or weaken security/tests to clear them.
7. Test the approved independent missing-account/owner visibility branches in Stage B, including the observed empty-account case. Do not silently interpret an unresolved attribute as granting access.
8. Review the explicit active-state table in Phase 6 §6.1 and gather source evidence for each proposed transition flag. The supplied CLI output establishes only the listed `PENDING`, `RUNNING`, `COMPLETED`, `CANCELLED`, and `FAILED` forms; it does not establish `SUSPENDED`, `CONFIGURING`, `COMPLETING`, requeue, or other flag combinations. Until verified and explicitly approved, unsupported or ambiguous combinations must reject the entire refresh without removals.

## Stage A Acceptance Criteria

Stage A remains unaccepted until a reviewer records approval after verifying all of the following:

- The per-connection event-sequence, visibility-projection, identity-only removal, authorization-context reconciliation, and fresh-snapshot recovery contract is documented and internally consistent with the accepted IPC transport and event envelope.
- The Phase 4 stream contract is preserved; Phase 6 does not introduce a new sequence or recovery contract.
- The active-Job source is selected and approved using completeness, independent-view comparison, parser, failure/truncation, visibility, and supported-version/partition-scope evidence from the approved backend data-acquisition identity. Hidden-partition deployments are explicitly excluded unless separately verified and approved.
- Automated repository test results are distinguished from live Slurm/service-runtime evidence; failed checks are not reported as passed.
- Representative normal and array Jobs from that source establish the source-specific identity normalization rule. Normal canonical Jobs have both array identifiers absent; valid array task ID zero remains representable.
- The IPC requester, daemon data-acquisition identity, and any privileged-helper identity are separately identified and verified for their respective boundaries.
- The exact `node.list` → `node.cluster` and `job.list` → `job.active` visibility-policy references and role scopes are documented; Stage B includes required reference-validation tests.
- The extraction plan is mapped to the audited implementation without authorizing speculative abstractions or weakening Phase 5 behavior.

Until this acceptance is recorded, **Stage B MUST NOT start**. The decisions in Phase 6 define what must be implemented; they do not themselves approve implementation work.
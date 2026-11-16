1. Follow docs/architecture as the authoritative architecture.

2. Do not introduce architectural concepts not documented there.

3. Do not put Slurm logic into cockpit-slurm-channel.

4. cockpit-slurm daemon owns application logic.

5. IPC protocol must conform to ipc-protocol.md.

6. Error semantics must conform to error-model.md.

7. User identity semantics must conform to user-context.md.

8. New Go code must have unit tests.

9. Prefer small interfaces.

10. Do not prematurely implement future phases.

11. Do not modify architecture documents unless explicitly requested.
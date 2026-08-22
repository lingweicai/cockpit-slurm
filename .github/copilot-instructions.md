```text
1. Follow docs/architecture as the authoritative architecture.

2. Do not introduce architectural concepts not documented there.

3. Do not put Slurm logic into cockpit-slurm-channel.

4. Do not put cache, authorization, resource adapters,
   or canonical resources into the channel.

5. Bridge owns application logic.

6. IPC protocol must conform to ipc-protocol.md.

7. Error semantics must conform to error-model.md.

8. User identity semantics must conform to user-context.md.

9. New Go code must have unit tests.

10. Prefer small interfaces.

11. Do not prematurely implement future phases.

12. Do not modify architecture documents unless explicitly requested.
```
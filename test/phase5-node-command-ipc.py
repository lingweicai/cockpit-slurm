#!/usr/bin/env python3
import argparse
import datetime
import grp
import json
import os
import pwd
import socket
import struct
import subprocess
import time
import uuid


SOCKET_PATH = "/run/cockpit-slurm/cockpit-slurm.sock"
NODE_NAME = "c21"
EXPECTED_USER = "slurm_admin"
TIMEOUT_SECONDS = 90
MAX_FRAME_SIZE = 4 * 1024 * 1024


class TestFailure(Exception):
    pass


class IPCClient:
    def __init__(self, connection):
        self.connection = connection
        self.pending_events = []
        self.generation = 0

    def send(self, message):
        encoded = json.dumps(message, separators=(",", ":")).encode()
        if not encoded or len(encoded) > MAX_FRAME_SIZE:
            raise TestFailure("IPC message size is invalid")
        self.connection.sendall(struct.pack(">I", len(encoded)) + encoded)

    def receive(self):
        header = self._read_exact(4)
        size = struct.unpack(">I", header)[0]
        if size == 0 or size > MAX_FRAME_SIZE:
            raise TestFailure(f"IPC frame size is invalid: {size}")
        message = json.loads(self._read_exact(size))
        if message.get("protocol") != "cockpit-slurm" or message.get("version") != "1.0":
            raise TestFailure("server returned an unexpected protocol envelope")
        if message.get("type") == "event":
            self.pending_events.append(message)
            payload = message.get("payload", {})
            self.generation = max(self.generation, payload.get("generation", 0))
        return message

    def _read_exact(self, size):
        data = bytearray()
        while len(data) < size:
            chunk = self.connection.recv(size - len(data))
            if not chunk:
                raise TestFailure("IPC connection closed before the response arrived")
            data.extend(chunk)
        return bytes(data)

    def wait_for_snapshot(self):
        deadline = time.monotonic() + TIMEOUT_SECONDS
        while time.monotonic() < deadline:
            message = self.receive()
            if message.get("type") != "event":
                continue
            payload = message.get("payload", {})
            if payload.get("resource") != "node" or payload.get("event") != "snapshot":
                continue
            nodes = payload.get("nodes", [])
            node = next(
                (
                    item
                    for item in nodes
                    if item.get("spec", {}).get("nodeName") == NODE_NAME
                    or item.get("metadata", {}).get("name") == NODE_NAME
                ),
                None,
            )
            if node is None:
                raise TestFailure(f"initial Node snapshot does not contain {NODE_NAME}")
            state = node.get("status", {}).get("state")
            flags = node.get("status", {}).get("stateFlags", [])
            if state != "IDLE" or flags:
                raise TestFailure(
                    f"refusing mutation: initial {NODE_NAME} state is {state!r} "
                    f"with flags {flags!r}, expected unflagged 'IDLE'"
                )
            self.generation = payload.get("generation", 0)
            return self.generation
        raise TestFailure("timed out waiting for the initial Node snapshot")

    def command(self, state, reason=None):
        suffix = uuid.uuid4().hex
        message_id = f"stage-d-{state.lower()}-{suffix}"
        command_id = f"stage-d-c21-{state.lower()}-{suffix}"
        parameters = {"state": state}
        if reason is not None:
            parameters["reason"] = reason
        payload = {
            "commandId": command_id,
            "resource": "node",
            "command": "update",
            "target": {"nodeName": NODE_NAME},
            "parameters": parameters,
        }
        self.send(
            {
                "protocol": "cockpit-slurm",
                "version": "1.0",
                "messageId": message_id,
                "type": "command",
                "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                "payload": payload,
            }
        )

        deadline = time.monotonic() + TIMEOUT_SECONDS
        while time.monotonic() < deadline:
            response = self.receive()
            if response.get("messageId") != message_id:
                continue
            if response.get("type") == "error":
                return command_id, None, response.get("payload", {})
            if response.get("type") == "command-response":
                result = response.get("payload", {})
                if result.get("commandId") != command_id:
                    raise TestFailure("command response has an unexpected commandId")
                return command_id, result, None
            raise TestFailure(f"unexpected response type: {response.get('type')!r}")
        raise TestFailure(f"timed out waiting for {state} command response")

    def wait_for_state(self, state, after_generation):
        deadline = time.monotonic() + TIMEOUT_SECONDS
        while time.monotonic() < deadline:
            for message in self.pending_events:
                payload = message.get("payload", {})
                if (
                    payload.get("resource") != "node"
                    or payload.get("event") != "changes"
                    or payload.get("generation", 0) <= after_generation
                ):
                    continue
                for change in payload.get("changes", []):
                    node = change.get("node", {})
                    name = node.get("spec", {}).get("nodeName") or node.get("metadata", {}).get("name")
                    if name == NODE_NAME and node.get("status", {}).get("state") == state:
                        self.pending_events.remove(message)
                        return payload["generation"]
            self.receive()
        raise TestFailure(f"timed out waiting for authoritative {NODE_NAME} state {state}")


def require_socket_access():
    user = pwd.getpwuid(os.geteuid())
    if user.pw_name != EXPECTED_USER:
        raise TestFailure(
            f"run as {EXPECTED_USER}; current IPC peer would be {user.pw_name} (uid {user.pw_uid})"
        )
    group_ids = set(os.getgroups())
    group_ids.add(os.getegid())
    socket_group = grp.getgrnam("cockpit-slurm")
    if socket_group.gr_gid not in group_ids:
        raise TestFailure("slurm_admin is not in the cockpit-slurm socket group")


def current_slurm_state():
    result = subprocess.run(
        ["sinfo", "-h", "-N", "-n", NODE_NAME, "-o", "%T"],
        check=True,
        capture_output=True,
        text=True,
        timeout=20,
    )
    states = [line.strip().upper() for line in result.stdout.splitlines() if line.strip()]
    if len(states) != 1:
        raise TestFailure(f"could not determine a single Slurm state for {NODE_NAME}: {states!r}")
    return states[0]


def main():
    parser = argparse.ArgumentParser(
        description="Drain and resume c21 through authenticated cockpit-slurm IPC."
    )
    parser.add_argument(
        "--confirm-node",
        required=True,
        choices=[NODE_NAME],
        help="explicitly confirm the dedicated test Node",
    )
    args = parser.parse_args()
    if args.confirm_node != NODE_NAME:
        raise TestFailure("unexpected confirmation target")
    require_socket_access()

    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as connection:
        connection.settimeout(TIMEOUT_SECONDS)
        connection.connect(SOCKET_PATH)
        client = IPCClient(connection)
        initial_generation = client.wait_for_snapshot()
        print(f"peer={EXPECTED_USER} uid={os.geteuid()} node={NODE_NAME} initial=IDLE")

        drain_may_have_changed = False
        try:
            drain_may_have_changed = True
            command_id, result, error = client.command("DRAIN", "Stage D real-cluster validation")
            if error is not None:
                print(f"drain commandId={command_id} error={json.dumps(error, sort_keys=True)}")
                if error.get("code") in {
                    "invalid-command",
                    "invalid-payload",
                    "identity-unavailable",
                    "user-lookup-failed",
                    "authorization-denied",
                    "command-id-conflict",
                }:
                    drain_may_have_changed = False
                raise TestFailure("drain command did not return success")
            if result.get("status") != "succeeded":
                drain_may_have_changed = result.get("status") in {"running", "unknown"}
                raise TestFailure(f"drain command status was {result.get('status')!r}")
            print(f"drain commandId={command_id} status=succeeded")
            drain_generation = client.wait_for_state("DRAIN", initial_generation)
            print(f"event node={NODE_NAME} state=DRAIN generation={drain_generation}")

            resume_id, resume_result, resume_error = client.command("RESUME")
            if resume_error is not None:
                print(f"resume commandId={resume_id} error={json.dumps(resume_error, sort_keys=True)}")
                raise TestFailure("resume command did not return success")
            if resume_result.get("status") != "succeeded":
                raise TestFailure(f"resume command status was {resume_result.get('status')!r}")
            print(f"resume commandId={resume_id} status=succeeded")
            idle_generation = client.wait_for_state("IDLE", drain_generation)
            drain_may_have_changed = False
            print(f"event node={NODE_NAME} state=IDLE generation={idle_generation}")
            print("PASS: c21 drained and restored to its original IDLE state")
        except Exception:
            if drain_may_have_changed:
                try:
                    observed_state = current_slurm_state()
                    if observed_state == "IDLE":
                        print(f"RESTORED: {NODE_NAME} is already IDLE; no resume command needed")
                    elif not observed_state.startswith(("DRAIN", "DRNG")):
                        print(
                            f"RESTORE NOT ATTEMPTED: {NODE_NAME} is {observed_state}; "
                            "inspect the node before taking further action"
                        )
                    else:
                        recovery_id, recovery_result, recovery_error = client.command("RESUME")
                        if recovery_error is not None:
                            print(
                                "RESTORE FAILED: "
                                f"commandId={recovery_id} error={json.dumps(recovery_error, sort_keys=True)}"
                            )
                        elif recovery_result.get("status") == "succeeded":
                            generation = client.wait_for_state("IDLE", client.generation)
                            print(
                                f"RESTORED: node={NODE_NAME} state=IDLE generation={generation} "
                                f"commandId={recovery_id}"
                            )
                        else:
                            print(f"RESTORE UNCERTAIN: commandId={recovery_id} result={recovery_result}")
                except Exception as restore_error:
                    print(f"RESTORE FAILED: {restore_error}")
            raise


if __name__ == "__main__":
    try:
        main()
    except (OSError, TestFailure, ValueError, json.JSONDecodeError) as error:
        raise SystemExit(f"FAIL: {error}")
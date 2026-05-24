# Sinfo Data Model

This model is based on the Slurm `sinfo` command and its JSON output. The official manual is the source of truth for field semantics:

- https://slurm.schedmd.com/sinfo.html

## Source Commands

- `sinfo --json`
- `sinfo`

## Default table columns

The frontend default table should show:

- `partitionName` / `PARTITION`
- `availability` / `AVAIL`
- `timeLimit` / `TIMELIMIT`
- `nodesTotal` / `NODES`
- `nodeState` / `STATE`
- `nodeList` / `NODELIST`

## Expanded row details

When a row is expanded or clicked, display additional fields from the same `sinfo` record:

- `cpusTotal`, `cpusAllocated`, `cpusIdle`, `cpusOther`
- `memoryFreeMin`, `memoryFreeMax`, `memoryAllocated`
- `reasonDescription`, `reasonUser`, `reasonTime`
- `comment`
- `reservation`
- `partitionState`
- `partitionTRES`
- `featuresActive`, `gresUsed`
- raw JSON details (`raw`) for deeper inspection

## Field definitions

| Field | Type | Source | Description |
| --- | --- | --- | --- |
| `partition_name` | string | `partition.name` | Partition name |
| `availability` | string | `partition.partition.state` | Partition availability state (UP/DOWN) |
| `time_limit` | string | `partition.maximums.time` | Maximum wallclock time for the partition |
| `nodes_total` | integer | `nodes.total` | Total nodes in this partition row |
| `nodes_allocated` | integer | `nodes.allocated` | Allocated nodes |
| `nodes_idle` | integer | `nodes.idle` | Idle nodes |
| `nodes_other` | integer | `nodes.other` | Nodes in a non-idle/non-alloc state |
| `node_state` | string | `node.state` | Node-level state for the partition row |
| `node_list` | string | `partition.nodes.configured` | Node list expression |
| `cpus_total` | integer | `cpus.total` | Total CPUs in this row |
| `cpus_allocated` | integer | `cpus.allocated` | Allocated CPUs |
| `cpus_idle` | integer | `cpus.idle` | Idle CPUs |
| `cpus_other` | integer | `cpus.other` | Other CPUs |
| `memory_free_min` | integer | `memory.free.minimum.number` | Minimum free memory across nodes |
| `memory_free_max` | integer | `memory.free.maximum.number` | Maximum free memory across nodes |
| `memory_allocated` | integer | `memory.allocated` | Allocated memory |
| `reason_description` | string | `reason.description` | Reason text for partition/node state |
| `reason_user` | string | `reason.user` | User who triggered the reason |
| `reason_time` | integer | `reason.time` | Epoch timestamp for the reason |
| `comment` | string | `comment` | Partition comment |
| `reservation` | string | `reservation` | Reservation name if set |
| `partition_state` | string[] | `partition.partition.state` | Partition state array |
| `partition_tres` | string | `partition.tres.configured` | TRES configuration string |
| `features_active` | string | `features.active` | Active features string |
| `gres_used` | string | `gres.used` | GRES usage string |

## Cache contract

The backend cache should store a list of canonical `SinfoPartitionRow` values and a timestamp.

- `rows`: array of partition rows
- `updatedAt`: cache timestamp

This makes the frontend request simple and avoids re-running `sinfo --json` for each UI refresh.

## Frontend example using cockpit.channel()

The frontend can open a channel to the bridge socket and request the canonical cache payload from the backend.

```ts
import type { SinfoPartitionRow } from "../types/sinfo";

declare const cockpit: any;

const SOCKET_PATH = "/run/cockpit-slurm/bridge.sock";

type SinfoCachePayload = {
  rows: SinfoPartitionRow[];
  updated_at: string;
};

function openSinfoChannel() {
  return cockpit.channel(SOCKET_PATH);
}

export async function fetchSinfo(): Promise<SinfoCachePayload> {
  const channel = openSinfoChannel();
  return new Promise((resolve, reject) => {
    function onMessage(data: any) {
      const payload = typeof data === "string" ? JSON.parse(data) : data;
      if (payload.type === "sinfo.response") {
        cleanup();
        resolve(payload.data as SinfoCachePayload);
      }
    }

    function onError(err: any) {
      cleanup();
      reject(err);
    }

    function cleanup() {
      channel.removeListener?.("message", onMessage);
      channel.removeListener?.("error", onError);
      channel.close?.();
    }

    channel.on("message", onMessage);
    channel.on("error", onError);
    channel.send(JSON.stringify({ action: "get_sinfo" }));
  });
}

export function subscribeSinfoUpdates(callback: (event: any) => void) {
  const channel = openSinfoChannel();
  channel.on("message", (data: any) => {
    const payload = typeof data === "string" ? JSON.parse(data) : data;
    if (payload.type?.startsWith("sinfo.")) {
      callback(payload);
    }
  });
  channel.send(JSON.stringify({ action: "subscribe" }));

  return () => {
    channel.close?.();
  };
}
```

This example uses the backend socket service at `/run/cockpit-slurm/bridge.sock` and expects the bridge server to return `sinfo.response` payloads and broadcast `sinfo.updated` events when the cached state changes.

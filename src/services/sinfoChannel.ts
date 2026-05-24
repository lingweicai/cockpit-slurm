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

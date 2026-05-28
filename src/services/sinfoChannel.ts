import type { SinfoPartitionRow } from "../types/sinfo";

declare const cockpit: any;

const SOCKET_PATH = "/run/cockpit-slurm/bridge.sock";

type SinfoCachePayload = {
  rows: SinfoPartitionRow[];
  updated_at: string;
};

function openSinfoChannel() {
  return cockpit.channel({ payload: SOCKET_PATH });
}

function addChannelListener(channel: any, event: string, listener: (...args: any[]) => void) {
  if (typeof channel.addEventListener === 'function') {
    channel.addEventListener(event, listener);
  } else if (typeof channel.on === 'function') {
    channel.on(event, listener);
  } else if (typeof channel.addListener === 'function') {
    channel.addListener(event, listener);
  } else {
    // best-effort: some shims expose different shapes; ignore if unsupported
    // eslint-disable-next-line no-console
    console.warn('channel does not support addEventListener/on/addListener', event);
  }
}

function removeChannelListener(channel: any, event: string, listener: (...args: any[]) => void) {
  if (typeof channel.removeEventListener === 'function') {
    channel.removeEventListener(event, listener);
  } else if (typeof channel.removeListener === 'function') {
    channel.removeListener(event, listener);
  } else if (typeof channel.off === 'function') {
    channel.off(event, listener);
  } else {
    // eslint-disable-next-line no-console
    console.warn('channel does not support removeEventListener/removeListener/off', event);
  }
}

export async function fetchSinfo(): Promise<SinfoCachePayload> {
  const channel = openSinfoChannel();
  return new Promise((resolve, reject) => {
    // Debug info to help diagnose channel lifecycle in browser
    // eslint-disable-next-line no-console
    console.debug('sinfo: opened channel', { id: channel?.id, options: channel?.options });
    function onMessage(data: any) {
      const payload = typeof data === "string" ? JSON.parse(data) : data;
      if (payload.type === "sinfo.response") {
        cleanup();
        resolve(payload.data as SinfoCachePayload);
      }
    }

    function onClose(err: any) {
      // eslint-disable-next-line no-console
      console.warn('sinfo: channel closed', err);
      cleanup();
      const msg = err instanceof Error ? err.message : (err && err.message) || 'Sinfo channel closed';
      reject(new Error(`Sinfo channel closed: ${msg}`));
    }

    function cleanup() {
      removeChannelListener(channel, "message", onMessage);
      removeChannelListener(channel, "close", onClose);
      channel.close?.();
    }

    addChannelListener(channel, "message", onMessage);
    addChannelListener(channel, "close", onClose);

    // Prefer sending after the channel signals `ready` if available.
    const sendRequest = () => {
      try {
        channel.send(JSON.stringify({ action: "get_sinfo" }));
      } catch (e) {
        // eslint-disable-next-line no-console
        console.error('sinfo: failed to send get_sinfo', e);
      }
    };

    if (typeof channel.addEventListener === 'function' || typeof channel.on === 'function') {
      const onReady = () => {
        // eslint-disable-next-line no-console
        console.debug('sinfo: channel ready');
        removeChannelListener(channel, 'ready', onReady);
        sendRequest();
      };

      addChannelListener(channel, 'ready', onReady);
      // fallback: send immediately if ready flag is present
      if (channel.ready) {
        removeChannelListener(channel, 'ready', onReady);
        sendRequest();
      }
    } else {
      sendRequest();
    }
  });
}

export function subscribeSinfoUpdates(callback: (event: any) => void) {
  const channel = openSinfoChannel();
  const onMessage = (data: any) => {
    const payload = typeof data === "string" ? JSON.parse(data) : data;
    if (payload.type?.startsWith("sinfo.")) {
      callback(payload);
    }
  };

  // Debug
  // eslint-disable-next-line no-console
  console.debug('sinfo: subscribe opening channel', { id: channel?.id, options: channel?.options });

  addChannelListener(channel, "message", onMessage);

  const performSubscribe = () => {
    try {
      channel.send(JSON.stringify({ action: "subscribe" }));
    } catch (e) {
      // eslint-disable-next-line no-console
      console.error('sinfo: subscribe send failed', e);
    }
  };

  const onReadySub = () => {
    // eslint-disable-next-line no-console
    console.debug('sinfo: subscribe channel ready');
    removeChannelListener(channel, 'ready', onReadySub);
    performSubscribe();
  };

  addChannelListener(channel, 'ready', onReadySub);
  if (channel.ready) {
    removeChannelListener(channel, 'ready', onReadySub);
    performSubscribe();
  }

  return () => {
    removeChannelListener(channel, "message", onMessage);
    channel.close?.();
  };
}

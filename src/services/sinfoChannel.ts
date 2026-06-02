import type { SinfoPartitionRow } from "../types/sinfo";

declare const cockpit: any;

const SOCKET_PATH = "/run/cockpit-slurm/bridge.sock";
const CHANNEL_HELPERS = [
  "/usr/libexec/cockpit-slurm/cockpit-slurm-channel",
  "/usr/local/libexec/cockpit-slurm/cockpit-slurm-channel",
];

type SinfoCachePayload = {
  rows: SinfoPartitionRow[];
  updated_at: string;
};

function openSinfoChannel() {
  for (const helperPath of CHANNEL_HELPERS) {
    try {
      const ch = cockpit.channel({ payload: "stream", spawn: [helperPath] });
      // If the returned object looks like a real channel, use it.
      if (ch && (typeof ch.send === 'function' || typeof ch.on === 'function' || typeof ch.addEventListener === 'function')) {
        return ch;
      }
      // Otherwise try the next helper path.
    } catch (err) {
      // eslint-disable-next-line no-console
      console.debug('sinfo: spawn helper failed', helperPath, err);
    }
  }

  try {
    return cockpit.channel(SOCKET_PATH);
  } catch (err) {
    // eslint-disable-next-line no-console
    console.error('sinfo: failed to open channel to socket', SOCKET_PATH, err);
    throw err;
  }
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

function parseChannelEvent(data: any): any[] {
  let raw = data;

  if (raw && typeof raw === 'object') {
    if (typeof raw.data === 'string') {
      raw = raw.data;
    } else if (typeof raw.detail === 'string') {
      raw = raw.detail;
    } else if (raw.detail && typeof raw.detail === 'object') {
      raw = raw.detail;
    }
  }

  if (typeof raw === 'string') {
    return raw
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => JSON.parse(line));
  }

  return [raw];
}

export async function fetchSinfo(): Promise<SinfoCachePayload> {
  const channel = openSinfoChannel();
  return new Promise((resolve, reject) => {
    const hasSend = typeof channel?.send === 'function';
    const hasOn = typeof channel?.on === 'function';
    const hasAddEventListener = typeof channel?.addEventListener === 'function';
    const hasReadySignal = typeof channel?.ready !== 'undefined';

    // Debug info to help diagnose channel lifecycle in browser
    // eslint-disable-next-line no-console
    console.debug('sinfo: opened channel', {
      channel,
      id: channel?.id,
      options: channel?.options,
      hasSend,
      hasOn,
      hasAddEventListener,
      hasReadySignal,
    });

    if (!hasSend) {
      reject(new Error('Sinfo channel is not writable: missing send()')); 
      return;
    }

    function onMessage(data: any) {
      let raw = data;
      try {
        const payloads = parseChannelEvent(raw);

        for (const payload of payloads) {
          // eslint-disable-next-line no-console
          console.debug('sinfo: channel message', { raw, payload });

          if (payload.type === "sinfo.response") {
            cleanup();
            resolve(payload.data as SinfoCachePayload);
            return;
          }
        }
      } catch (err) {
        // eslint-disable-next-line no-console
        console.warn('sinfo: failed to parse channel message', raw, err);
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

    const sendRequest = () => {
      try {
        const message = JSON.stringify({ action: "get_sinfo" }) + "\n";
        // eslint-disable-next-line no-console
        console.debug('sinfo: sending get_sinfo', { message });
        channel.send(message);
      } catch (e) {
        // eslint-disable-next-line no-console
        console.error('sinfo: failed to send get_sinfo', e);
      }
    };

    let requestSent = false;
    const sendRequestOnce = () => {
      if (!requestSent) {
        requestSent = true;
        sendRequest();
      }
    };

    if (hasOn || hasAddEventListener) {
      let fallbackTimeout: ReturnType<typeof setTimeout> | undefined;

      const onReady = () => {
        // eslint-disable-next-line no-console
        console.debug('sinfo: channel ready');
        removeChannelListener(channel, 'ready', onReady);
        if (fallbackTimeout !== undefined) {
          clearTimeout(fallbackTimeout);
        }
        sendRequestOnce();
      };

      addChannelListener(channel, 'ready', onReady);

      if (channel.ready) {
        removeChannelListener(channel, 'ready', onReady);
        sendRequestOnce();
      }

      fallbackTimeout = setTimeout(() => {
        // If ready never fires, still send once after a short delay.
        // eslint-disable-next-line no-console
        console.debug('sinfo: ready event did not fire; sending request anyway');
        sendRequestOnce();
      }, 150);
    } else {
      sendRequestOnce();
    }
  });
}

export function subscribeSinfoUpdates(callback: (event: any) => void) {
  const channel = openSinfoChannel();
  const onMessage = (data: any) => {
    let raw = data;
    try {
      const payloads = parseChannelEvent(raw);

      for (const payload of payloads) {
        // eslint-disable-next-line no-console
        console.debug('sinfo: subscribe message', { raw, payload });

        if (payload.type?.startsWith("sinfo.")) {
          callback(payload);
        }
      }
    } catch (err) {
      // eslint-disable-next-line no-console
      console.warn('sinfo: failed to parse subscribe message', raw, err);
    }
  };

  const hasSend = typeof channel?.send === 'function';
  const hasOn = typeof channel?.on === 'function';
  const hasAddEventListener = typeof channel?.addEventListener === 'function';
  const hasReadySignal = typeof channel?.ready !== 'undefined';

  // Debug
  // eslint-disable-next-line no-console
  console.debug('sinfo: subscribe opening channel', {
    channel,
    id: channel?.id,
    options: channel?.options,
    hasSend,
    hasOn,
    hasAddEventListener,
    hasReadySignal,
  });

  if (!hasSend) {
    // eslint-disable-next-line no-console
    console.error('sinfo: subscribe channel is not writable: missing send()');
    return () => {};
  }

  addChannelListener(channel, "message", onMessage);

  const performSubscribe = () => {
    try {
      const message = JSON.stringify({ action: "subscribe" }) + "\n";
      // eslint-disable-next-line no-console
      console.debug('sinfo: sending subscribe', { message });
      channel.send(message);
    } catch (e) {
      // eslint-disable-next-line no-console
      console.error('sinfo: subscribe send failed', e);
    }
  };

  let subscribeSent = false;
  const performSubscribeOnce = () => {
    if (!subscribeSent) {
      subscribeSent = true;
      performSubscribe();
    }
  };

  let fallbackTimeout: ReturnType<typeof setTimeout> | undefined;
  const onReadySub = () => {
    // eslint-disable-next-line no-console
    console.debug('sinfo: subscribe channel ready');
    removeChannelListener(channel, 'ready', onReadySub);
    if (fallbackTimeout !== undefined) {
      clearTimeout(fallbackTimeout);
    }
    performSubscribeOnce();
  };

  if (hasOn || hasAddEventListener) {
    addChannelListener(channel, 'ready', onReadySub);

    if (channel.ready) {
      removeChannelListener(channel, 'ready', onReadySub);
      performSubscribeOnce();
    }

    fallbackTimeout = setTimeout(() => {
      // eslint-disable-next-line no-console
      console.debug('sinfo: subscribe ready event did not fire; sending request anyway');
      performSubscribeOnce();
    }, 150);
  } else {
    performSubscribeOnce();
  }

  return () => {
    removeChannelListener(channel, "message", onMessage);
    channel.close?.();
  };
}

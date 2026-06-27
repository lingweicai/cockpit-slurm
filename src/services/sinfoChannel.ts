import cockpit from 'cockpit';

import type { BridgeEnvelope } from '../types/bridge';
import type { SinfoPartitionRow } from '../types/sinfo';

import {
  addChannelListener,
  openBridgeChannel,
  removeChannelListener,
  sendJsonLine,
} from '../lib/cockpit';
import { parseBridgeMessages } from '../lib/cockpit/parser';

const _ = cockpit.gettext;

type SinfoCachePayload = {
  rows: SinfoPartitionRow[];
  updated_at: string;
};

function normalizeSinfoPayload(value: unknown): SinfoCachePayload | null {
  if (!value || typeof value !== 'object') {
    return null;
  }

  const record = value as Record<string, unknown>;
  const rows = Array.isArray(record.rows) ? record.rows as SinfoPartitionRow[] : null;
  const updatedAt = typeof record.updated_at === 'string'
    ? record.updated_at
    : typeof record.updatedAt === 'string'
      ? record.updatedAt
      : null;

  if (!rows || !updatedAt) {
    return null;
  }

  return {
    rows,
    updated_at: updatedAt,
  };
}

function extractSinfoPayload(message: BridgeEnvelope): SinfoCachePayload | null {
  if (message && typeof message === 'object') {
    const record = message as Record<string, unknown>;

    if (record.type === 'sinfo.response' || record.type === 'sinfo.snapshot' || record.type === 'snapshot') {
      return normalizeSinfoPayload(record.data ?? record.payload ?? record);
    }

    if (record.type === 'event' && record.entity === 'sinfo') {
      return normalizeSinfoPayload(record.payload ?? record);
    }
  }

  return normalizeSinfoPayload(message);
}

function isInitializedSinfoPayload(payload: SinfoCachePayload | null) {
  if (!payload?.updated_at) {
    return false;
  }

  const timestamp = Date.parse(payload.updated_at);
  return Number.isFinite(timestamp) && timestamp > 0;
}

function waitForChannelReady(channel: ReturnType<typeof openBridgeChannel>, onReady: () => void) {
  const readyListener = () => {
    removeChannelListener(channel, 'ready', readyListener);
    onReady();
  };

  addChannelListener(channel, 'ready', readyListener);

  if (channel.ready) {
    removeChannelListener(channel, 'ready', readyListener);
    onReady();
  }

  return () => removeChannelListener(channel, 'ready', readyListener);
}

export async function fetchSinfo(): Promise<SinfoCachePayload> {
  const channel = openBridgeChannel();

  return new Promise((resolve, reject) => {
    let settled = false;
    let readyCleanup = () => {};

    const cleanup = () => {
      readyCleanup();
      removeChannelListener(channel, 'message', onMessage);
      removeChannelListener(channel, 'close', onClose);
      channel.close();
    };

    const finish = (payload: SinfoCachePayload) => {
      if (settled) {
        return;
      }

      settled = true;
      cleanup();
      resolve(payload);
    };

    const fail = (message: string) => {
      if (settled) {
        return;
      }

      settled = true;
      cleanup();
      reject(new Error(message));
    };

    const onMessage = (data: unknown) => {
      const messages = parseBridgeMessages(data);
      for (const message of messages) {
        const payload = extractSinfoPayload(message);
        if (payload) {
          finish(payload);
          return;
        }
      }
    };

    const onClose = (event: unknown) => {
      if (settled) {
        return;
      }

      if (event instanceof Error) {
        fail(event.message);
        return;
      }

      if (event && typeof event === 'object' && 'message' in event && typeof (event as { message?: unknown }).message === 'string') {
        fail((event as { message: string }).message);
        return;
      }

      fail(_('Sinfo channel closed before a response was received.'));
    };

    addChannelListener(channel, 'message', onMessage);
    addChannelListener(channel, 'close', onClose);

    readyCleanup = waitForChannelReady(channel, () => {
      sendJsonLine(channel, { action: 'get_sinfo' });
    });
  });
}

export function subscribeSinfoUpdates(callback: (event: BridgeEnvelope) => void) {
  const channel = openBridgeChannel();

  const onMessage = (data: unknown) => {
    const messages = parseBridgeMessages(data);
    for (const message of messages) {
      if (message && typeof message === 'object') {
        const record = message as Record<string, unknown>;
        if (record.type === 'sinfo.updated' || record.type === 'sinfo.event' || record.entity === 'sinfo') {
          callback(message);
        }
      }
    }
  };

  const startSubscription = () => {
    sendJsonLine(channel, { action: 'subscribe' });
  };

  const readyCleanup = waitForChannelReady(channel, startSubscription);
  addChannelListener(channel, 'message', onMessage);

  return () => {
    readyCleanup();
    removeChannelListener(channel, 'message', onMessage);
    channel.close();
  };
}

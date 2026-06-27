import cockpit from 'cockpit';

import type { BridgeRequest } from '../../types/bridge';
import type { CockpitChannelLike } from '../../types/cockpit';

export const DEFAULT_BRIDGE_SOCKET_PATH = '/run/cockpit-slurm/bridge.sock';

const CHANNEL_HELPER_CANDIDATES = [
  (userHome?: string) => (userHome ? `${userHome}/.local/libexec/cockpit-slurm/cockpit-slurm-channel` : null),
  () => '/usr/local/libexec/cockpit-slurm/cockpit-slurm-channel',
  () => '/usr/libexec/cockpit-slurm/cockpit-slurm-channel',
];

function getChannelHelpers(): string[] {
  const userHome = cockpit.info?.user?.home;
  return CHANNEL_HELPER_CANDIDATES.map((candidate) => candidate(userHome)).filter((helperPath): helperPath is string => Boolean(helperPath));
}

export function getBridgeSocketPath(): string {
  const globalAny = globalThis as { COCKPIT_SLURM_BRIDGE_SOCKET_PATH?: unknown };
  const overridePath = typeof globalAny.COCKPIT_SLURM_BRIDGE_SOCKET_PATH === 'string'
    ? globalAny.COCKPIT_SLURM_BRIDGE_SOCKET_PATH.trim()
    : '';

  if (overridePath) {
    return overridePath;
  }

  const cockpitUser = cockpit.info?.user;
  if (cockpitUser && typeof cockpitUser.uid === 'number') {
    return `/run/user/${cockpitUser.uid}/cockpit-slurm/bridge.sock`;
  }

  return DEFAULT_BRIDGE_SOCKET_PATH;
}

export function isCockpitChannelLike(channel: unknown): channel is CockpitChannelLike {
  return Boolean(channel)
    && typeof (channel as CockpitChannelLike).send === 'function'
    && typeof (channel as CockpitChannelLike).close === 'function';
}

export function addChannelListener(channel: CockpitChannelLike, event: string, listener: (...args: unknown[]) => void) {
  if (typeof channel.addEventListener === 'function') {
    channel.addEventListener(event, listener as (event: unknown) => void);
    return;
  }

  if (typeof channel.on === 'function') {
    channel.on(event, listener);
    return;
  }

  if (typeof channel.addListener === 'function') {
    channel.addListener(event, listener);
  }
}

export function removeChannelListener(channel: CockpitChannelLike, event: string, listener: (...args: unknown[]) => void) {
  if (typeof channel.removeEventListener === 'function') {
    channel.removeEventListener(event, listener as (event: unknown) => void);
    return;
  }

  if (typeof channel.removeListener === 'function') {
    channel.removeListener(event, listener);
    return;
  }

  if (typeof channel.off === 'function') {
    channel.off(event, listener);
  }
}

export function readChannelMessage(data: unknown): unknown[] {
  let raw = data;

  if (raw && typeof raw === 'object') {
    const eventLike = raw as { data?: unknown; detail?: unknown };
    if (typeof eventLike.data === 'string') {
      raw = eventLike.data;
    } else if (typeof eventLike.detail === 'string') {
      raw = eventLike.detail;
    } else if (eventLike.detail !== undefined) {
      raw = eventLike.detail;
    }
  }

  if (Array.isArray(raw)) {
    return raw;
  }

  if (typeof raw === 'string') {
    return raw
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        try {
          return JSON.parse(line) as unknown;
        } catch {
          return line;
        }
      });
  }

  return [raw];
}

export function sendJsonLine<T extends object>(channel: CockpitChannelLike, payload: T) {
  channel.send(`${JSON.stringify(payload)}\n`);
}

export function createRequestId(prefix = 'req'): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `${prefix}-${crypto.randomUUID()}`;
  }

  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

export function openBridgeChannel(): CockpitChannelLike {
  const socketPath = getBridgeSocketPath();

  for (const helperPath of getChannelHelpers()) {
    try {
      const channel = cockpit.channel({ payload: 'stream', spawn: [helperPath] });
      if (isCockpitChannelLike(channel)) {
        return channel;
      }
    } catch {
      // Try the next helper path.
    }
  }

  const channel = cockpit.channel(socketPath);
  if (!isCockpitChannelLike(channel)) {
    throw new Error(`Unable to open Cockpit channel for ${socketPath}`);
  }

  return channel;
}

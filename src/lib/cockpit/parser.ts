import type { BridgeEnvelope, BridgeError, BridgeEvent, BridgeSnapshot } from '../../types/bridge';

import { readChannelMessage } from './channel';

export function parseBridgeMessages(data: unknown): BridgeEnvelope[] {
  return readChannelMessage(data).map((item) => {
    if (item && typeof item === 'object') {
      return item as BridgeEnvelope;
    }

    return { type: 'error', message: String(item) } satisfies BridgeError;
  });
}

export function isBridgeSnapshot(message: BridgeEnvelope): message is BridgeSnapshot {
  return Boolean(message)
    && typeof message === 'object'
    && (message as BridgeSnapshot).type === 'snapshot'
    && typeof (message as BridgeSnapshot).generation === 'number';
}

export function isBridgeEvent(message: BridgeEnvelope): message is BridgeEvent {
  return Boolean(message)
    && typeof message === 'object'
    && (message as BridgeEvent).type === 'event'
    && typeof (message as BridgeEvent).generation === 'number';
}

export function isBridgeError(message: BridgeEnvelope): message is BridgeError {
  return Boolean(message)
    && typeof message === 'object'
    && (message as BridgeError).type === 'error'
    && typeof (message as BridgeError).message === 'string';
}

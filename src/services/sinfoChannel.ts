import cockpit from 'cockpit';

import type { BridgeEnvelope } from '../types/bridge';
import type { SinfoPartitionRow } from '../types/sinfo';
import { fetchEntitySnapshot, subscribeEntityUpdates } from './entityChannel';

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

        if (record.type === 'snapshot' && record.entity === 'sinfo') {
            return normalizeSinfoPayload(record.payload ?? record.data ?? record);
        }

        if (record.type === 'event' && record.entity === 'sinfo') {
            return normalizeSinfoPayload(record.payload ?? record.data ?? record);
        }
    }

    return normalizeSinfoPayload(message);
}

export async function fetchSinfo(): Promise<SinfoCachePayload> {
    return fetchEntitySnapshot({
        entity: 'sinfo',
        extractPayload: extractSinfoPayload,
        closedMessage: _('Sinfo channel closed before a response was received.'),
    });
}

export function subscribeSinfoUpdates(callback: (event: BridgeEnvelope) => void) {
    return subscribeEntityUpdates({
        entity: 'sinfo',
        extractDelta: () => null,
        callback: (event) => callback(event),
        shouldHandleMessage: (message) => {
            if (!message || typeof message !== 'object') {
                return false;
            }

            const record = message as Record<string, unknown>;
            return record.type === 'event' && record.entity === 'sinfo';
        },
    });
}

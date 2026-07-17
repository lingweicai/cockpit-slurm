import type { BridgeEnvelope } from '../types/bridge';
import type { SlurmPartition } from '../types/slurm-api';
import type { SlurmPartitionsDelta } from '../features/partitions/partitionsData';

export type PartitionsCachePayload = {
    partitions: SlurmPartition[];
};

export function normalizePartitionsDelta(value: unknown): SlurmPartitionsDelta | null {
    if (!value || typeof value !== 'object') {
        return null;
    }

    const record = value as Record<string, unknown>;
    const added = Array.isArray(record.added) ? record.added as SlurmPartition[] : [];
    const modified = Array.isArray(record.modified) ? record.modified as SlurmPartition[] : [];
    const deleted = Array.isArray(record.deleted) ? record.deleted as SlurmPartition[] : [];

    if (added.length === 0 && modified.length === 0 && deleted.length === 0) {
        return null;
    }

    return { added, modified, deleted };
}

export function extractPartitionsDelta(message: BridgeEnvelope): SlurmPartitionsDelta | null {
    if (!message || typeof message !== 'object') {
        return null;
    }

    const record = message as Record<string, unknown>;
    if (record.type !== 'event' || (record.entity !== 'partition' && record.entity !== 'partitions')) {
        return null;
    }

    return normalizePartitionsDelta(record.data ?? record.payload ?? record);
}

export function normalizePartitionsPayload(value: unknown): PartitionsCachePayload | null {
    if (!value || typeof value !== 'object') {
        return null;
    }

    const record = value as Record<string, unknown>;
    const partitions = Array.isArray(record.partitions)
        ? record.partitions as SlurmPartition[]
        : Array.isArray(record.items)
            ? record.items as SlurmPartition[]
            : null;
    if (!partitions) {
        return null;
    }

    return { partitions };
}

export function extractPartitionsPayload(message: BridgeEnvelope): PartitionsCachePayload | null {
    if (message && typeof message === 'object') {
        const record = message as Record<string, unknown>;

        if (record.type === 'snapshot' && (record.entity === 'partition' || record.entity === 'partitions')) {
            return normalizePartitionsPayload(record.payload ?? record.data ?? record);
        }

        if (record.type === 'event' && (record.entity === 'partition' || record.entity === 'partitions')) {
            return normalizePartitionsPayload(record.payload ?? record.data ?? record);
        }
    }

    return normalizePartitionsPayload(message);
}

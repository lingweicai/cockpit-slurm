import type { PartitionSummary } from '../cluster/clusterData';
import type { SlurmPartition, SlurmPartitionsResponse } from '../../types/slurm-api';

export type SlurmPartitionsPayload = Pick<SlurmPartitionsResponse, 'partitions'>;

export type SlurmPartitionsDelta = {
    added?: SlurmPartition[];
    modified?: SlurmPartition[];
    deleted?: SlurmPartition[];
};

function normalizeInteger(value: unknown) {
    if (typeof value === 'number' && Number.isFinite(value)) {
        return value;
    }

    if (value && typeof value === 'object' && 'number' in value) {
        const next = Number((value as { number?: unknown }).number);
        if (Number.isFinite(next)) {
            return next;
        }
    }

    return 0;
}

function normalizePartitionState(partition: SlurmPartition) {
    return Array.isArray(partition.partition?.state) && partition.partition.state.length > 0
        ? partition.partition.state.join(', ')
        : 'Unknown';
}

function normalizeLimit(value: unknown) {
    const normalized = normalizeInteger(value);
    return normalized > 0 ? String(normalized) : 'N/A';
}

function partitionIdentity(partition: SlurmPartition) {
    return partition.name ?? 'unknown';
}

export function mapSlurmPartitionToSummary(partition: SlurmPartition): PartitionSummary {
    return {
        partitionName: partitionIdentity(partition),
        state: normalizePartitionState(partition),
        nodes: normalizeInteger(partition.nodes?.total),
        cpus: normalizeInteger(partition.cpus?.total),
        jobs: 'N/A',
        availability: normalizePartitionState(partition),
        features: partition.node_sets ?? 'N/A',
        limits: normalizeLimit(partition.defaults?.time ?? partition.maximums?.time),
        comment: partition.defaults?.job ?? 'None',
        reservation: partition.alternate ?? 'N/A',
        partitionTRES: partition.tres?.configured ?? 'N/A',
    };
}

export function mapSlurmPartitionsPayloadToSummaries(payload: SlurmPartitionsPayload): PartitionSummary[] {
    return payload.partitions
            .map((partition) => mapSlurmPartitionToSummary(partition))
            .sort((left, right) => left.partitionName.localeCompare(right.partitionName));
}

export function resolvePartitionSummaries(payload?: SlurmPartitionsPayload | null): PartitionSummary[] {
    if (!payload || !Array.isArray(payload.partitions) || payload.partitions.length === 0) {
        return [];
    }

    return mapSlurmPartitionsPayloadToSummaries(payload);
}

export function applySlurmPartitionsDelta(
    current: SlurmPartitionsPayload | null,
    delta: SlurmPartitionsDelta,
): SlurmPartitionsPayload | null {
    const added = Array.isArray(delta.added) ? delta.added : [];
    const modified = Array.isArray(delta.modified) ? delta.modified : [];
    const deleted = Array.isArray(delta.deleted) ? delta.deleted : [];

    if (!current || !Array.isArray(current.partitions) || current.partitions.length === 0) {
        if (added.length > 0 || modified.length > 0) {
            return { partitions: [...added, ...modified] };
        }

        return current;
    }

    const nextById = new Map(current.partitions.map((partition) => [partitionIdentity(partition), partition]));

    for (const partition of added) {
        nextById.set(partitionIdentity(partition), partition);
    }

    for (const partition of modified) {
        nextById.set(partitionIdentity(partition), partition);
    }

    for (const partition of deleted) {
        nextById.delete(partitionIdentity(partition));
    }

    return { partitions: Array.from(nextById.values()) };
}

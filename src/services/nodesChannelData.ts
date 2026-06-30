import type { BridgeEnvelope } from '../types/bridge';
import type { SlurmNode } from '../types/slurm-api';
import type { SlurmNodesDelta } from '../features/nodes/nodesData';

export type NodesCachePayload = {
    nodes: SlurmNode[];
};

export function normalizeNodesDelta(value: unknown): SlurmNodesDelta | null {
    if (!value || typeof value !== 'object') {
        return null;
    }

    const record = value as Record<string, unknown>;
    const added = Array.isArray(record.added) ? record.added as SlurmNode[] : [];
    const modified = Array.isArray(record.modified) ? record.modified as SlurmNode[] : [];
    const deleted = Array.isArray(record.deleted) ? record.deleted as SlurmNode[] : [];

    if (added.length === 0 && modified.length === 0 && deleted.length === 0) {
        return null;
    }

    return { added, modified, deleted };
}

export function extractNodesDelta(message: BridgeEnvelope): SlurmNodesDelta | null {
    if (!message || typeof message !== 'object') {
        return null;
    }

    const record = message as Record<string, unknown>;
    if (record.type !== 'event' || (record.entity !== 'node' && record.entity !== 'nodes')) {
        return null;
    }

    return normalizeNodesDelta(record.data ?? record.payload ?? record);
}

export function normalizeNodesPayload(value: unknown): NodesCachePayload | null {
    if (Array.isArray(value)) {
        return { nodes: value as SlurmNode[] };
    }

    if (!value || typeof value !== 'object') {
        return null;
    }

    const record = value as Record<string, unknown>;
    const nodes = Array.isArray(record.nodes)
        ? record.nodes as SlurmNode[]
        : Array.isArray(record.items)
            ? record.items as SlurmNode[]
            : null;
    if (!nodes) {
        return null;
    }

    return { nodes };
}

export function extractNodesPayload(message: BridgeEnvelope): NodesCachePayload | null {
    if (message && typeof message === 'object') {
        const record = message as Record<string, unknown>;

        if (record.type === 'snapshot' && (record.entity === 'node' || record.entity === 'nodes')) {
            return normalizeNodesPayload(record.payload ?? record.data ?? record);
        }

        if (record.type === 'event' && (record.entity === 'node' || record.entity === 'nodes')) {
            return normalizeNodesPayload(record.payload ?? record.data ?? record);
        }
    }

    return normalizeNodesPayload(message);
}

import type { SlurmNode, SlurmNodesResponse } from '../../types/slurm-api';
import type { NodeSummary } from '../cluster/clusterData';

export type SlurmNodesPayload = Pick<SlurmNodesResponse, 'nodes'>;

export type SlurmNodesDelta = {
    added?: SlurmNode[];
    modified?: SlurmNode[];
    deleted?: SlurmNode[];
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

function normalizeNodeState(node: SlurmNode) {
    return Array.isArray(node.state) && node.state.length > 0
        ? node.state.join(', ')
        : 'Unknown';
}

function normalizeFeatures(node: SlurmNode) {
    const source = Array.isArray(node.active_features) && node.active_features.length > 0
        ? node.active_features
        : Array.isArray(node.features)
            ? node.features
            : [];

    return Array.from(new Set(source.filter(Boolean))).sort();
}

function normalizePartitions(node: SlurmNode) {
    return Array.isArray(node.partitions)
        ? Array.from(new Set(node.partitions.filter(Boolean))).sort()
        : [];
}

function nodeIdentity(node: SlurmNode) {
    return node.name ?? node.hostname ?? 'unknown';
}

export function mapSlurmNodeToSummary(node: SlurmNode): NodeSummary {
    const partitions = normalizePartitions(node);
    const features = normalizeFeatures(node);
    const nodeState = normalizeNodeState(node);

    return {
        name: nodeIdentity(node),
        partitions,
        state: nodeState,
        nodeState,
        availability: nodeState,
        cpus: normalizeInteger(node.cpus),
        memory: normalizeInteger(node.real_memory),
        features,
    };
}

export function mapSlurmNodesPayloadToSummaries(payload: SlurmNodesPayload): NodeSummary[] {
    return payload.nodes
            .map((node) => mapSlurmNodeToSummary(node))
            .sort((left, right) => left.name.localeCompare(right.name));
}

export function resolveNodeSummaries(payload?: SlurmNodesPayload | null): NodeSummary[] {
    if (!payload || !Array.isArray(payload.nodes) || payload.nodes.length === 0) {
        return [];
    }

    return mapSlurmNodesPayloadToSummaries(payload);
}

export function applySlurmNodesDelta(
    current: SlurmNodesPayload | null,
    delta: SlurmNodesDelta,
): SlurmNodesPayload | null {
    const added = Array.isArray(delta.added) ? delta.added : [];
    const modified = Array.isArray(delta.modified) ? delta.modified : [];
    const deleted = Array.isArray(delta.deleted) ? delta.deleted : [];

    if (!current || !Array.isArray(current.nodes) || current.nodes.length === 0) {
        if (added.length > 0 || modified.length > 0) {
            return { nodes: [...added, ...modified] };
        }

        return current;
    }

    const nextById = new Map(current.nodes.map((node) => [nodeIdentity(node), node]));

    for (const node of added) {
        nextById.set(nodeIdentity(node), node);
    }

    for (const node of modified) {
        nextById.set(nodeIdentity(node), node);
    }

    for (const node of deleted) {
        nextById.delete(nodeIdentity(node));
    }

    return { nodes: Array.from(nextById.values()) };
}

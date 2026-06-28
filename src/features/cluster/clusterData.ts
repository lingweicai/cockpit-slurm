import type { SinfoPartitionRow } from '../../types/sinfo';

export type PartitionSummary = {
    partitionName: string;
    state: string;
    nodes: number;
    cpus: number;
    jobs: string;
    availability: string;
    features: string;
    limits: string;
    comment: string;
    reservation: string;
};

export type NodeSummary = {
    name: string;
    partitions: string[];
    state: string;
    nodeState: string;
    availability: string;
    cpus: number;
    memory: number;
    features: string[];
};

export type NodeGroup = {
    name: string;
    nodes: NodeSummary[];
};

function splitNames(value: string) {
    return value
            .split(/[\s,]+/)
            .map((entry) => entry.trim())
            .filter(Boolean);
}

function expandBracketExpression(prefix: string, expression: string) {
    return expression.split(',').flatMap((part) => {
        const range = part.trim().match(/^(\d+)-(\d+)$/);
        if (!range) {
            return [`${prefix}${part.trim()}`];
        }

        const start = Number(range[1]);
        const end = Number(range[2]);
        const width = range[1].length;
        const result: string[] = [];

        for (let index = start; index <= end; index += 1) {
            result.push(`${prefix}${String(index).padStart(width, '0')}`);
        }

        return result;
    });
}

export function expandNodeList(nodeList: string): string[] {
    const names = splitNames(nodeList);
    const expanded: string[] = [];

    for (const name of names) {
        const match = name.match(/^([^[]+)\[([^]]+)\]$/);
        if (match) {
            expanded.push(...expandBracketExpression(match[1], match[2]));
            continue;
        }

        expanded.push(name);
    }

    return Array.from(new Set(expanded));
}

function getPartitionStateLabel(row: SinfoPartitionRow) {
    const state = row.partitionState?.join(', ') || row.availability || 'Unknown';
    return state;
}

function getNodeStateLabel(row: SinfoPartitionRow) {
    return row.nodeState || row.availability || 'Unknown';
}

function getNodeFeatures(row: SinfoPartitionRow) {
    return row.featuresActive
        ? row.featuresActive.split(/[,\s]+/).map((entry) => entry.trim())
                .filter(Boolean)
        : [];
}

export function buildPartitionSummaries(rows: SinfoPartitionRow[]): PartitionSummary[] {
    return rows.map((row) => ({
        partitionName: row.partitionName,
        state: getPartitionStateLabel(row),
        nodes: row.nodesTotal,
        cpus: row.cpusTotal,
        jobs: row.nodesAllocated > 0 ? `${row.nodesAllocated} allocated nodes` : 'No active nodes',
        availability: row.availability,
        features: row.featuresActive || 'N/A',
        limits: row.timeLimit || 'N/A',
        comment: row.comment || 'None',
        reservation: row.reservation || 'N/A',
    }));
}

export function buildNodeSummaries(rows: SinfoPartitionRow[]): NodeSummary[] {
    const nodes = new Map<string, NodeSummary>();

    for (const row of rows) {
        for (const nodeName of expandNodeList(row.nodeList)) {
            const current = nodes.get(nodeName);
            const nextPartitions = new Set(current?.partitions ?? []);
            nextPartitions.add(row.partitionName);

            const nextFeatures = new Set(current?.features ?? []);
            for (const feature of getNodeFeatures(row)) {
                nextFeatures.add(feature);
            }

            nodes.set(nodeName, {
                name: nodeName,
                partitions: Array.from(nextPartitions).sort(),
                state: getPartitionStateLabel(row),
                nodeState: getNodeStateLabel(row),
                availability: row.availability,
                cpus: row.cpusTotal,
                memory: row.memoryFreeMax,
                features: Array.from(nextFeatures).sort(),
            });
        }
    }

    return Array.from(nodes.values()).sort((left, right) => left.name.localeCompare(right.name));
}

export function groupNodesByPrefix(nodes: NodeSummary[]): NodeGroup[] {
    const groups = new Map<string, NodeSummary[]>();

    for (const node of nodes) {
        const prefixMatch = node.name.match(/^[A-Za-z_-]+/);
        const groupName = prefixMatch?.[0] ?? 'Ungrouped';
        const current = groups.get(groupName) ?? [];
        current.push(node);
        groups.set(groupName, current);
    }

    return Array.from(groups.entries())
            .sort(([left], [right]) => left.localeCompare(right))
            .map(([name, groupNodes]) => ({
                name,
                nodes: groupNodes.sort((left, right) => left.name.localeCompare(right.name)),
            }));
}

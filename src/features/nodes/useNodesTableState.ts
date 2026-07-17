import { useMemo, useState } from 'react';

import { buildTableResetHandler } from '../../lib/tableState';
import type { NodeSummary } from '../cluster/clusterData';

export type NodeSortKey = 'name' | 'state' | 'cpus' | 'memory';
export type NodeSortDirection = 'asc' | 'desc';

type NodeFilterable = Pick<NodeSummary, 'name' | 'nodeState' | 'partitions' | 'features' | 'cpus' | 'memory'>;

export function normalizeStateToken(state: string) {
    return state
            .split(/[,+]/)
            .map((token) => token.trim().toUpperCase())
            .find(Boolean) ?? 'UNKNOWN';
}

export function matchesNodeFilter(node: Pick<NodeFilterable, 'name' | 'nodeState' | 'partitions' | 'features'>, query: string) {
    if (!query.trim()) {
        return true;
    }

    const haystack = [
        node.name,
        node.nodeState,
        node.partitions.join(' '),
        node.features.join(' '),
    ].join(' ').toLowerCase();

    return haystack.includes(query.trim().toLowerCase());
}

export function useNodesTableState(nodes: NodeSummary[]) {
    const [query, setQuery] = useState('');
    const [stateFilter, setStateFilter] = useState('ALL');
    const [sortKey, setSortKey] = useState<NodeSortKey>('name');
    const [sortDirection, setSortDirection] = useState<NodeSortDirection>('asc');

    const stateOptions = useMemo(() => {
        return ['ALL', ...Array.from(new Set(nodes.map((node) => normalizeStateToken(node.nodeState)))).sort()];
    }, [nodes]);

    const filteredNodes = useMemo(() => {
        return nodes.filter((node) => {
            const matchesState = stateFilter === 'ALL' || normalizeStateToken(node.nodeState) === stateFilter;
            return matchesState && matchesNodeFilter(node, query);
        });
    }, [nodes, query, stateFilter]);

    const sortedNodes = useMemo(() => {
        const directionFactor = sortDirection === 'asc' ? 1 : -1;

        return filteredNodes.slice().sort((left, right) => {
            if (sortKey === 'cpus') {
                return (left.cpus - right.cpus) * directionFactor;
            }

            if (sortKey === 'memory') {
                return (left.memory - right.memory) * directionFactor;
            }

            if (sortKey === 'state') {
                return left.nodeState.localeCompare(right.nodeState) * directionFactor;
            }

            return left.name.localeCompare(right.name) * directionFactor;
        });
    }, [filteredNodes, sortDirection, sortKey]);

    const handleSortChange = (nextSortKey: NodeSortKey) => {
        if (sortKey === nextSortKey) {
            setSortDirection((current) => (current === 'asc' ? 'desc' : 'asc'));
            return;
        }

        setSortKey(nextSortKey);
        setSortDirection('asc');
    };

    const resetFilters = buildTableResetHandler(
        () => setQuery(''),
        () => setStateFilter('ALL'),
        () => setSortKey('name'),
        () => setSortDirection('asc'),
    );

    return {
        query,
        setQuery,
        stateFilter,
        setStateFilter,
        sortKey,
        sortDirection,
        stateOptions,
        filteredNodes,
        sortedNodes,
        handleSortChange,
        resetFilters,
    };
}

import { useMemo, useState } from 'react';

import { buildTableResetHandler } from '../../lib/tableState';
import type { PartitionSummary } from '../cluster/clusterData';

export type PartitionSortKey = 'partitionName' | 'state' | 'nodes' | 'cpus';
export type PartitionSortDirection = 'asc' | 'desc';

export function matchesPartitionFilter(summary: PartitionSummary, query: string) {
    if (!query.trim()) {
        return true;
    }

    const normalizedQuery = query.trim().toLowerCase();
    const haystack = [
        summary.partitionName,
        summary.state,
        String(summary.nodes),
        String(summary.cpus),
        summary.availability,
        summary.features,
        summary.limits,
        summary.comment,
        summary.reservation,
        summary.partitionTRES,
    ].join(' ').toLowerCase();

    return haystack.includes(normalizedQuery);
}

export function usePartitionsTableState(summaries: PartitionSummary[]) {
    const [query, setQuery] = useState('');
    const [sortKey, setSortKey] = useState<PartitionSortKey>('partitionName');
    const [sortDirection, setSortDirection] = useState<PartitionSortDirection>('asc');

    const filteredSummaries = useMemo(() => {
        return summaries.filter((summary) => matchesPartitionFilter(summary, query));
    }, [query, summaries]);

    const sortedSummaries = useMemo(() => {
        const directionFactor = sortDirection === 'asc' ? 1 : -1;

        return filteredSummaries.slice().sort((left, right) => {
            if (sortKey === 'nodes') {
                return (left.nodes - right.nodes) * directionFactor;
            }

            if (sortKey === 'cpus') {
                return (left.cpus - right.cpus) * directionFactor;
            }

            if (sortKey === 'state') {
                return left.state.localeCompare(right.state) * directionFactor;
            }

            return left.partitionName.localeCompare(right.partitionName) * directionFactor;
        });
    }, [filteredSummaries, sortDirection, sortKey]);

    const handleSortChange = (nextSortKey: PartitionSortKey) => {
        if (sortKey === nextSortKey) {
            setSortDirection((current) => (current === 'asc' ? 'desc' : 'asc'));
            return;
        }

        setSortKey(nextSortKey);
        setSortDirection('asc');
    };

    const resetFilters = buildTableResetHandler(
        () => setQuery(''),
        () => setSortKey('partitionName'),
        () => setSortDirection('asc'),
    );

    return {
        query,
        setQuery,
        sortKey,
        sortDirection,
        sortedSummaries,
        handleSortChange,
        resetFilters,
    };
}

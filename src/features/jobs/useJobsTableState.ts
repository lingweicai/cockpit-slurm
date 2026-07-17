import { useMemo, useState } from 'react';

import { buildTableResetHandler } from '../../lib/tableState';
import type { JobRecord, JobState } from '../../types/job';

export type JobsSortKey = 'jobId' | 'user' | 'partition' | 'state' | 'runtime';
export type JobsSortDirection = 'asc' | 'desc';

const STATE_ORDER: Record<JobState, number> = {
    RUNNING: 0,
    PENDING: 1,
    FAILED: 2,
    COMPLETED: 3,
    CANCELLED: 4,
};

function runtimeToSeconds(runtime: string) {
    const parts = runtime.split(':').map((part) => Number(part));
    if (parts.length !== 3 || parts.some((part) => Number.isNaN(part))) {
        return 0;
    }

    return parts[0] * 3600 + parts[1] * 60 + parts[2];
}

export function matchesJobFilter(job: JobRecord, query: string) {
    if (!query.trim()) {
        return true;
    }

    const haystack = [
        job.jobId,
        job.name,
        job.user,
        job.account,
        job.partition,
        job.state,
        job.command,
        job.nodeList,
        job.qos,
    ].join(' ').toLowerCase();

    return haystack.includes(query.toLowerCase());
}

function compareJobs(left: JobRecord, right: JobRecord, sortKey: JobsSortKey, direction: JobsSortDirection) {
    const directionFactor = direction === 'asc' ? 1 : -1;

    if (sortKey === 'runtime') {
        return (runtimeToSeconds(left.runtime) - runtimeToSeconds(right.runtime)) * directionFactor;
    }

    if (sortKey === 'state') {
        return (STATE_ORDER[left.state] - STATE_ORDER[right.state]) * directionFactor;
    }

    const leftValue = String(left[sortKey]).toLowerCase();
    const rightValue = String(right[sortKey]).toLowerCase();
    return leftValue.localeCompare(rightValue) * directionFactor;
}

export function useJobsTableState(jobs: JobRecord[]) {
    const [query, setQuery] = useState('');
    const [stateFilter, setStateFilter] = useState<'ALL' | JobState>('ALL');
    const [sortKey, setSortKey] = useState<JobsSortKey>('state');
    const [sortDirection, setSortDirection] = useState<JobsSortDirection>('asc');

    const filteredJobs = useMemo(() => {
        return jobs
                .slice()
                .filter((job) => (stateFilter === 'ALL' ? true : job.state === stateFilter))
                .sort((left, right) => compareJobs(left, right, sortKey, sortDirection));
    }, [jobs, sortDirection, sortKey, stateFilter]);

    const handleSortChange = (nextSortKey: JobsSortKey) => {
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
        () => setSortKey('state'),
        () => setSortDirection('asc'),
    );

    return {
        query,
        setQuery,
        stateFilter,
        setStateFilter,
        sortKey,
        sortDirection,
        filteredJobs,
        handleSortChange,
        resetFilters,
    };
}

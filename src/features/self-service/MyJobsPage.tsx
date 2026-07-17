import React, { useMemo, useState } from 'react';
import { Alert, Card, CardBody, CardTitle } from '@patternfly/react-core';

import cockpit from 'cockpit';

import { EntityTable, type EntityTableColumn } from '../../components/EntityTable';
import { SummaryMetricsGallery } from '../../components/SummaryMetricsGallery';
import { getCurrentUserName } from '../../lib/cockpit/session';
import type { JobRecord } from '../../types/job';
import { JOB_FIXTURES } from '../jobs/jobsData';

const _ = cockpit.gettext;

type MyJobsSortKey = 'jobId' | 'name' | 'partition' | 'state' | 'runtime';
type MyJobsSortDirection = 'asc' | 'desc';

function formatCount(value: number) {
    return value.toLocaleString();
}

export const MyJobsPage = () => {
    const currentUser = getCurrentUserName();
    const [sortKey, setSortKey] = useState<MyJobsSortKey>('jobId');
    const [sortDirection, setSortDirection] = useState<MyJobsSortDirection>('asc');
    const jobs = useMemo(() => JOB_FIXTURES.filter((job) => job.user === currentUser), [currentUser]);
    const running = jobs.filter((job) => job.state === 'RUNNING').length;
    const pending = jobs.filter((job) => job.state === 'PENDING').length;
    const completed = jobs.filter((job) => job.state === 'COMPLETED').length;
    const summaryMetrics = [
        { title: _('My jobs'), value: formatCount(jobs.length) },
        { title: _('Running'), value: formatCount(running) },
        { title: _('Pending'), value: formatCount(pending) },
        { title: _('Completed'), value: formatCount(completed) },
    ];

    const sortedJobs = useMemo(() => {
        const directionFactor = sortDirection === 'asc' ? 1 : -1;

        return jobs.slice().sort((left, right) => {
            if (sortKey === 'runtime') {
                return left.runtime.localeCompare(right.runtime) * directionFactor;
            }

            if (sortKey === 'jobId') {
                return left.jobId.localeCompare(right.jobId, undefined, { numeric: true }) * directionFactor;
            }

            return String(left[sortKey]).localeCompare(String(right[sortKey])) * directionFactor;
        });
    }, [jobs, sortDirection, sortKey]);

    const handleSortChange = (nextSortKey: MyJobsSortKey) => {
        if (sortKey === nextSortKey) {
            setSortDirection((current) => (current === 'asc' ? 'desc' : 'asc'));
            return;
        }

        setSortKey(nextSortKey);
        setSortDirection('asc');
    };

    const columns: EntityTableColumn<JobRecord>[] = [
        {
            header: _('JobID'),
            dataLabel: _('JobID'),
            cell: (job) => job.jobId,
            sortable: {
                isActive: sortKey === 'jobId',
                direction: sortDirection,
                onSort: () => handleSortChange('jobId'),
            },
        },
        {
            header: _('Name'),
            dataLabel: _('Name'),
            cell: (job) => job.name,
            sortable: {
                isActive: sortKey === 'name',
                direction: sortDirection,
                onSort: () => handleSortChange('name'),
            },
        },
        {
            header: _('Partition'),
            dataLabel: _('Partition'),
            cell: (job) => job.partition,
            sortable: {
                isActive: sortKey === 'partition',
                direction: sortDirection,
                onSort: () => handleSortChange('partition'),
            },
        },
        {
            header: _('State'),
            dataLabel: _('State'),
            cell: (job) => job.state,
            sortable: {
                isActive: sortKey === 'state',
                direction: sortDirection,
                onSort: () => handleSortChange('state'),
            },
        },
        {
            header: _('Runtime'),
            dataLabel: _('Runtime'),
            cell: (job) => job.runtime,
            sortable: {
                isActive: sortKey === 'runtime',
                direction: sortDirection,
                onSort: () => handleSortChange('runtime'),
            },
        },
    ];

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <SummaryMetricsGallery metrics={summaryMetrics} />

            <Card>
                <CardTitle>{cockpit.format(_('Jobs for $0'), currentUser)}</CardTitle>
                <CardBody>
                    <EntityTable
                        ariaLabel={_('My jobs table')}
                        columns={columns}
                        rows={sortedJobs}
                        rowKey={(job) => job.jobId}
                        pagination={{
                            defaultPerPage: 10,
                            perPageOptions: [10, 20, 50],
                        }}
                        emptyState={<Alert variant="info" title={_('No jobs found for the current user.')} />}
                    />
                </CardBody>
            </Card>
        </div>
    );
};

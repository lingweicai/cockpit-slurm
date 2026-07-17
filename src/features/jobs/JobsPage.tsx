import React, { useEffect, useMemo, useState } from 'react';
import {
    Badge,
    Button,
    Card,
    CardBody,
    CardTitle,
    Drawer,
    DrawerContent,
    DrawerPanelBody,
    DrawerPanelContent,
    Gallery,
    GalleryItem,
    Tab,
    Tabs,
    TabTitleText,
} from '@patternfly/react-core';

import cockpit from 'cockpit';

import type { AppRole } from '../../app/navigation';
import { ActionFeedbackAlert } from '../../components/ActionFeedbackAlert';
import { EntityTable, type EntityTableColumn, type EntityTableRowAction } from '../../components/EntityTable';
import { ResetTableFiltersButton } from '../../components/ResetTableFiltersButton';
import { SummaryMetricsGallery } from '../../components/SummaryMetricsGallery';
import { TableEmptyMatchState } from '../../components/TableEmptyMatchState';
import { TableToolbarActions } from '../../components/TableToolbarActions';
import { TableToolbarField } from '../../components/TableToolbarField';
import { useTransientAlert } from '../../hooks/useTransientAlert';
import { buildCopyNameRowAction, buildDetailsRowAction } from '../../lib/rowActions';
import type { JobRecord, JobState } from '../../types/job';
import type { SlurmJob } from '../../types/slurm-api';
import { fetchJobs, subscribeJobsUpdates } from '../../services/jobsChannel';
import { applySlurmJobsDelta, resolveJobRows } from './jobsData';
import { type JobsSortKey, matchesJobFilter, useJobsTableState } from './useJobsTableState';

const _ = cockpit.gettext;

type JobsPageProps = {
    role: AppRole;
};

type DrawerTabKey = 'general' | 'resources' | 'environment' | 'stdout' | 'stderr' | 'history';

function formatCount(value: number) {
    return value.toLocaleString();
}

function renderStateBadge(state: JobState) {
    switch (state) {
    case 'RUNNING':
        return <Badge isRead>{state}</Badge>;
    case 'PENDING':
        return <Badge isRead>{state}</Badge>;
    case 'FAILED':
        return <Badge isRead>{state}</Badge>;
    case 'COMPLETED':
        return <Badge isRead>{state}</Badge>;
    case 'CANCELLED':
        return <Badge isRead>{state}</Badge>;
    default:
        return <Badge isRead>{state}</Badge>;
    }
}

function buildSummary(jobRows: JobRecord[]) {
    const counts = jobRows.reduce((acc, job) => {
        acc[job.state] += 1;
        return acc;
    }, {
        RUNNING: 0,
        PENDING: 0,
        FAILED: 0,
        COMPLETED: 0,
        CANCELLED: 0,
    });

    return [
        { title: _('Running'), value: formatCount(counts.RUNNING) },
        { title: _('Pending'), value: formatCount(counts.PENDING) },
        { title: _('Failed'), value: formatCount(counts.FAILED) },
        { title: _('Completed'), value: formatCount(counts.COMPLETED) },
    ];
}

function DrawerDetails({ job, tabKey, setTabKey }: { job: JobRecord; tabKey: DrawerTabKey; setTabKey: (key: DrawerTabKey) => void; }) {
    return (
        <DrawerPanelContent id={`job-${job.jobId}`}>
            <DrawerPanelBody>
                <Card isPlain>
                    <CardTitle>{cockpit.format(_('Job $0'), job.jobId)}</CardTitle>
                    <CardBody>
                        <div style={{ display: 'grid', gap: '1rem' }}>
                            <Tabs activeKey={tabKey} onSelect={(_event, key) => setTabKey(key as DrawerTabKey)}>
                                <Tab eventKey="general" title={<TabTitleText>{_('General')}</TabTitleText>}>
                                    <div style={{ display: 'grid', gap: '0.5rem' }}>
                                        <div><strong>{_('Name')}:</strong> {job.name}</div>
                                        <div><strong>{_('User')}:</strong> {job.user}</div>
                                        <div><strong>{_('Account')}:</strong> {job.account}</div>
                                        <div><strong>{_('Partition')}:</strong> {job.partition}</div>
                                        <div><strong>{_('State')}:</strong> {job.state}</div>
                                        <div><strong>{_('QOS')}:</strong> {job.qos}</div>
                                        <div><strong>{_('Command')}:</strong> {job.command}</div>
                                        <div><strong>{_('Work directory')}:</strong> {job.workDir}</div>
                                    </div>
                                </Tab>
                                <Tab eventKey="resources" title={<TabTitleText>{_('Resources')}</TabTitleText>}>
                                    <div style={{ display: 'grid', gap: '0.5rem' }}>
                                        <div><strong>{_('Nodes')}:</strong> {job.nodes}</div>
                                        <div><strong>{_('CPUs')}:</strong> {job.cpus}</div>
                                        <div><strong>{_('Node list')}:</strong> {job.nodeList || _('Pending allocation')}</div>
                                        <div><strong>{_('Runtime')}:</strong> {job.runtime}</div>
                                    </div>
                                </Tab>
                                <Tab eventKey="environment" title={<TabTitleText>{_('Environment')}</TabTitleText>}>
                                    <div style={{ display: 'grid', gap: '0.5rem' }}>
                                        {Object.entries(job.environment).map(([key, value]) => (
                                            <div key={key}><strong>{key}:</strong> {value}</div>
                                        ))}
                                    </div>
                                </Tab>
                                <Tab eventKey="stdout" title={<TabTitleText>{_('Stdout')}</TabTitleText>}>
                                    <div>{job.stdout}</div>
                                </Tab>
                                <Tab eventKey="stderr" title={<TabTitleText>{_('Stderr')}</TabTitleText>}>
                                    <div>{job.stderr}</div>
                                </Tab>
                                <Tab eventKey="history" title={<TabTitleText>{_('History')}</TabTitleText>}>
                                    <div style={{ display: 'grid', gap: '0.75rem' }}>
                                        {job.history.map((entry) => (
                                            <div key={`${entry.timestamp}-${entry.event}`}>
                                                <strong>{entry.event}</strong> · {new Date(entry.timestamp).toLocaleString()}<br />
                                                {entry.detail}
                                            </div>
                                        ))}
                                    </div>
                                </Tab>
                            </Tabs>
                        </div>
                    </CardBody>
                </Card>
            </DrawerPanelBody>
        </DrawerPanelContent>
    );
}

function buildJobRowActionItems(
    job: JobRecord,
    onSelectJob: (jobId: string) => void,
    showActionMessage: (alert: { variant: 'success' | 'danger' | 'warning' | 'info'; title: string }) => void,
): EntityTableRowAction<JobRecord>[] {
    return [
        buildDetailsRowAction({
            onClick: () => onSelectJob(job.jobId),
        }),
        buildCopyNameRowAction({
            id: 'copy-job-id',
            label: _('Copy Job ID'),
            value: job.jobId,
            successTitle: cockpit.format(_('Copied Job ID $0 to clipboard.'), job.jobId),
            failureTitle: cockpit.format(_('Unable to copy Job ID $0.'), job.jobId),
            showAlert: showActionMessage,
        }),
        buildCopyNameRowAction({
            id: 'copy-job-command',
            label: _('Copy Job Command'),
            value: job.command,
            successTitle: cockpit.format(_('Copied command for Job $0 to clipboard.'), job.jobId),
            failureTitle: cockpit.format(_('Unable to copy command for Job $0.'), job.jobId),
            showAlert: showActionMessage,
        }),
    ];
}

export const JobsPage = ({ role }: JobsPageProps) => {
    const [jobsPayload, setJobsPayload] = useState<{ jobs: SlurmJob[] } | null>(null);
    const jobs = useMemo(() => resolveJobRows(jobsPayload), [jobsPayload]);
    const {
        query,
        setQuery,
        stateFilter,
        setStateFilter,
        sortKey,
        sortDirection,
        filteredJobs,
        handleSortChange,
        resetFilters,
    } = useJobsTableState(jobs);
    const [selectedJobId, setSelectedJobId] = useState<string | null>(jobs[0]?.jobId ?? null);
    const [drawerTab, setDrawerTab] = useState<DrawerTabKey>('general');
    const { alert: actionMessage, showAlert: showActionMessage } = useTransientAlert();

    useEffect(() => {
        let isMounted = true;

        const loadJobs = async () => {
            try {
                const payload = await fetchJobs();
                if (!isMounted) {
                    return;
                }

                setJobsPayload(payload);
            } catch {
                // Keep fixtures if live fetch fails.
            }
        };

        loadJobs();

        const unsubscribe = subscribeJobsUpdates((_event, delta) => {
            if (delta) {
                setJobsPayload((current) => applySlurmJobsDelta(current, delta));
                return;
            }

            loadJobs().catch(() => {
                // Keep current data if refresh fails.
            });
        });

        return () => {
            isMounted = false;
            unsubscribe();
        };
    }, []);

    useEffect(() => {
        if (!selectedJobId || !jobs.some((job) => job.jobId === selectedJobId)) {
            setSelectedJobId(jobs[0]?.jobId ?? null);
        }
    }, [jobs, selectedJobId]);

    const selectedJob = filteredJobs.find((job) => job.jobId === selectedJobId) ?? filteredJobs[0] ?? null;
    const summary = useMemo(() => buildSummary(filteredJobs), [filteredJobs]);
    const tableColumns: EntityTableColumn<JobRecord>[] = [
        {
            header: _('JobID'),
            dataLabel: _('JobID'),
            cell: (job) => job.jobId,
            sortable: {
                isActive: sortKey === 'jobId',
                direction: sortDirection,
                onSort: () => handleSortChange('jobId' as JobsSortKey),
            },
        },
        {
            header: _('User'),
            dataLabel: _('User'),
            cell: (job) => job.user,
            sortable: {
                isActive: sortKey === 'user',
                direction: sortDirection,
                onSort: () => handleSortChange('user' as JobsSortKey),
            },
        },
        {
            header: _('Account'),
            dataLabel: _('Account'),
            cell: (job) => job.account,
        },
        {
            header: _('Partition'),
            dataLabel: _('Partition'),
            cell: (job) => job.partition,
            sortable: {
                isActive: sortKey === 'partition',
                direction: sortDirection,
                onSort: () => handleSortChange('partition' as JobsSortKey),
            },
        },
        {
            header: _('State'),
            dataLabel: _('State'),
            cell: (job) => renderStateBadge(job.state),
            sortable: {
                isActive: sortKey === 'state',
                direction: sortDirection,
                onSort: () => handleSortChange('state' as JobsSortKey),
            },
        },
        {
            header: _('Runtime'),
            dataLabel: _('Runtime'),
            cell: (job) => job.runtime,
            sortable: {
                isActive: sortKey === 'runtime',
                direction: sortDirection,
                onSort: () => handleSortChange('runtime' as JobsSortKey),
            },
        },
        {
            header: _('Nodes'),
            dataLabel: _('Nodes'),
            cell: (job) => job.nodes,
        },
    ];

    return (
        <Drawer isExpanded={Boolean(selectedJob)} isInline>
            <DrawerContent panelContent={selectedJob
                ? (
                    <DrawerDetails
                    job={selectedJob}
                    tabKey={drawerTab}
                    setTabKey={setDrawerTab}
                    />
                )
                : null}
            >
                <div style={{ display: 'grid', gap: '1rem' }}>
                    <SummaryMetricsGallery metrics={summary} />

                    <Card>
                        <CardTitle>{role === 'user' ? _('My jobs') : _('Jobs queue')}</CardTitle>
                        <CardBody>
                            <ActionFeedbackAlert alert={actionMessage} />

                            {filteredJobs.length === 0 && (
                                <TableEmptyMatchState title={_('No jobs match the current filters.')} />
                            )}
                            {filteredJobs.length > 0 && (
                                <EntityTable
                                    ariaLabel={_('Jobs queue table')}
                                    columns={tableColumns}
                                    rows={filteredJobs}
                                    rowKey={(job) => job.jobId}
                                    onRowClick={(job) => setSelectedJobId(job.jobId)}
                                    selectedRowKey={selectedJob?.jobId ?? null}
                                    rowActionsVariant="menu"
                                    rowActionItems={(job) => buildJobRowActionItems(job, setSelectedJobId, showActionMessage)}
                                    pagination={{
                                        defaultPerPage: 10,
                                        perPageOptions: [10, 20, 50],
                                    }}
                                    toolbar={(
                                        <TableToolbarActions>
                                            <TableToolbarField label={_('State')}>
                                                <select value={stateFilter} onChange={(event) => setStateFilter(event.target.value as 'ALL' | JobState)}>
                                                    <option value="ALL">{_('All')}</option>
                                                    <option value="RUNNING">{_('Running')}</option>
                                                    <option value="PENDING">{_('Pending')}</option>
                                                    <option value="FAILED">{_('Failed')}</option>
                                                    <option value="COMPLETED">{_('Completed')}</option>
                                                    <option value="CANCELLED">{_('Cancelled')}</option>
                                                </select>
                                            </TableToolbarField>
                                            <ResetTableFiltersButton onReset={resetFilters} />
                                        </TableToolbarActions>
                                    )}
                                    filter={{
                                        placeholder: _('Search by job, user, account, partition, or state'),
                                        query,
                                        onQueryChange: setQuery,
                                        matches: matchesJobFilter,
                                        emptyState: <TableEmptyMatchState title={_('No jobs match the current filters.')} />,
                                    }}
                                />
                            )}
                        </CardBody>
                    </Card>
                </div>
            </DrawerContent>
        </Drawer>
    );
};

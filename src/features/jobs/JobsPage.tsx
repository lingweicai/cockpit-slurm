import React, { useMemo, useState } from 'react';
import {
    Alert,
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
    TextInput,
} from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';

import cockpit from 'cockpit';

import type { AppRole } from '../../app/navigation';
import type { JobRecord, JobState } from '../../types/job';
import { JOB_FIXTURES } from './jobsData';

const _ = cockpit.gettext;

type JobsPageProps = {
    role: AppRole;
};

type SortKey = 'jobId' | 'user' | 'partition' | 'state' | 'runtime';
type SortDirection = 'asc' | 'desc';
type DrawerTabKey = 'general' | 'resources' | 'environment' | 'stdout' | 'stderr' | 'history';

const STATE_ORDER: Record<JobState, number> = {
    RUNNING: 0,
    PENDING: 1,
    FAILED: 2,
    COMPLETED: 3,
    CANCELLED: 4,
};

function formatCount(value: number) {
    return value.toLocaleString();
}

function runtimeToSeconds(runtime: string) {
    const parts = runtime.split(':').map((part) => Number(part));
    if (parts.length !== 3 || parts.some((part) => Number.isNaN(part))) {
        return 0;
    }

    return parts[0] * 3600 + parts[1] * 60 + parts[2];
}

function compareJobs(left: JobRecord, right: JobRecord, sortKey: SortKey, direction: SortDirection) {
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

function matchesFilter(job: JobRecord, query: string) {
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

export const JobsPage = ({ role }: JobsPageProps) => {
    const [query, setQuery] = useState('');
    const [stateFilter, setStateFilter] = useState<'ALL' | JobState>('ALL');
    const [sortKey, setSortKey] = useState<SortKey>('state');
    const [sortDirection, setSortDirection] = useState<SortDirection>('asc');
    const [selectedJobId, setSelectedJobId] = useState<string | null>(JOB_FIXTURES[0]?.jobId ?? null);
    const [drawerTab, setDrawerTab] = useState<DrawerTabKey>('general');

    const filteredJobs = useMemo(() => {
        return JOB_FIXTURES
                .slice()
                .filter((job) => (stateFilter === 'ALL' ? true : job.state === stateFilter))
                .filter((job) => matchesFilter(job, query))
                .sort((left, right) => compareJobs(left, right, sortKey, sortDirection));
    }, [query, sortDirection, sortKey, stateFilter]);

    const selectedJob = filteredJobs.find((job) => job.jobId === selectedJobId) ?? filteredJobs[0] ?? null;
    const summary = useMemo(() => buildSummary(filteredJobs), [filteredJobs]);

    const tableRows = filteredJobs.map((job) => (
        <Tr key={job.jobId} onClick={() => setSelectedJobId(job.jobId)}>
            <Td dataLabel={_('JobID')}>{job.jobId}</Td>
            <Td dataLabel={_('User')}>{job.user}</Td>
            <Td dataLabel={_('Account')}>{job.account}</Td>
            <Td dataLabel={_('Partition')}>{job.partition}</Td>
            <Td dataLabel={_('State')}>{renderStateBadge(job.state)}</Td>
            <Td dataLabel={_('Runtime')}>{job.runtime}</Td>
            <Td dataLabel={_('Nodes')}>{job.nodes}</Td>
        </Tr>
    ));

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
                    <Gallery hasGutter>
                        {summary.map((metric) => (
                            <GalleryItem key={metric.title}>
                                <Card>
                                    <CardTitle>{metric.title}</CardTitle>
                                    <CardBody>
                                        <strong>{metric.value}</strong>
                                    </CardBody>
                                </Card>
                            </GalleryItem>
                        ))}
                    </Gallery>

                    <Card>
                        <CardTitle>{role === 'user' ? _('My jobs') : _('Jobs queue')}</CardTitle>
                        <CardBody>
                            <div style={{ display: 'grid', gap: '0.75rem', marginBottom: '1rem' }}>
                                <TextInput
                                    value={query}
                                    onChange={(_event, value) => setQuery(value)}
                                    aria-label={_('Search jobs')}
                                    placeholder={_('Search by job, user, account, partition, or state')}
                                />
                                <div style={{ display: 'grid', gap: '0.5rem', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))' }}>
                                    <label>
                                        <div>{_('State')}</div>
                                        <select value={stateFilter} onChange={(event) => setStateFilter(event.target.value as 'ALL' | JobState)}>
                                            <option value="ALL">{_('All')}</option>
                                            <option value="RUNNING">{_('Running')}</option>
                                            <option value="PENDING">{_('Pending')}</option>
                                            <option value="FAILED">{_('Failed')}</option>
                                            <option value="COMPLETED">{_('Completed')}</option>
                                            <option value="CANCELLED">{_('Cancelled')}</option>
                                        </select>
                                    </label>
                                    <label>
                                        <div>{_('Sort by')}</div>
                                        <select value={sortKey} onChange={(event) => setSortKey(event.target.value as SortKey)}>
                                            <option value="state">{_('State')}</option>
                                            <option value="jobId">{_('JobID')}</option>
                                            <option value="user">{_('User')}</option>
                                            <option value="partition">{_('Partition')}</option>
                                            <option value="runtime">{_('Runtime')}</option>
                                        </select>
                                    </label>
                                    <label>
                                        <div>{_('Direction')}</div>
                                        <select value={sortDirection} onChange={(event) => setSortDirection(event.target.value as SortDirection)}>
                                            <option value="asc">{_('Ascending')}</option>
                                            <option value="desc">{_('Descending')}</option>
                                        </select>
                                    </label>
                                </div>
                                <Button
                                    variant="link"
                                    onClick={() => {
                                        setQuery('');
                                        setStateFilter('ALL');
                                        setSortKey('state');
                                        setSortDirection('asc');
                                    }}
                                >
                                    {_('Reset filters')}
                                </Button>
                            </div>

                            {filteredJobs.length === 0 && (
                                <Alert variant="info" title={_('No jobs match the current filters.')} />
                            )}
                            {filteredJobs.length > 0 && (
                                <Table aria-label={_('Jobs queue table')} variant="compact">
                                    <Thead>
                                        <Tr>
                                            <Th>{_('JobID')}</Th>
                                            <Th>{_('User')}</Th>
                                            <Th>{_('Account')}</Th>
                                            <Th>{_('Partition')}</Th>
                                            <Th>{_('State')}</Th>
                                            <Th>{_('Runtime')}</Th>
                                            <Th>{_('Nodes')}</Th>
                                        </Tr>
                                    </Thead>
                                    <Tbody>{tableRows}</Tbody>
                                </Table>
                            )}
                        </CardBody>
                    </Card>
                </div>
            </DrawerContent>
        </Drawer>
    );
};

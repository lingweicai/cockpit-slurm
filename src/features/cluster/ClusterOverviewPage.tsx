import React, { useMemo, useState } from 'react';
import {
    Badge,
    Card,
    CardBody,
    CardTitle,
    Gallery,
    GalleryItem,
    Progress,
    FormSelect,
    FormSelectOption,
} from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';

import cockpit from 'cockpit';

import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { LoadingState } from '../../components/LoadingState';
import type { SinfoPartitionRow } from '../../types/sinfo';
import { buildNodeSummaries, groupNodesByPrefix, type NodeSummary } from './clusterData';

const _ = cockpit.gettext;

type ClusterOverviewPageProps = {
    loading: boolean;
    rows: SinfoPartitionRow[];
    updatedAt: string | null;
    waitMessage: string | null;
    error: string | null;
    onSelectNode: (nodeName: string) => void;
};

type ClusterProfile = {
    name: string;
    description: string;
    focus: string;
};

const CLUSTER_PROFILES: ClusterProfile[] = [
    { name: 'production', description: _('Production cluster for shared workloads'), focus: _('Stable workloads and accounting visibility') },
    { name: 'gpu', description: _('GPU cluster for accelerated jobs'), focus: _('Accelerated workloads and reservations') },
    { name: 'testing', description: _('Testing cluster for validation runs'), focus: _('Short jobs and platform verification') },
];

function formatCount(value: number) {
    return value.toLocaleString();
}

function formatPercent(numerator: number, denominator: number) {
    if (denominator <= 0) {
        return 0;
    }

    return Math.round((numerator / denominator) * 100);
}

function isDegradedPartition(row: SinfoPartitionRow) {
    const states = row.partitionState?.map((state) => state.toLowerCase()) ?? [];
    return states.some((state) => state.includes('down') || state.includes('drain') || state.includes('error'));
}

function badgeVariantForNodeState(state: string) {
    const normalizedState = state.toLowerCase();

    if (normalizedState.includes('down') || normalizedState.includes('error') || normalizedState.includes('fail')) {
        return 'danger';
    }

    if (normalizedState.includes('drain') || normalizedState.includes('maint') || normalizedState.includes('mixed')) {
        return 'warning';
    }

    return 'success';
}

function buildTopologyGroups(rows: SinfoPartitionRow[]) {
    return groupNodesByPrefix(buildNodeSummaries(rows));
}

function renderNodeChip(node: NodeSummary) {
    return (
        <Badge key={node.name} isRead variant={badgeVariantForNodeState(node.nodeState)}>
            {node.nodeState}
        </Badge>
    );
}

export const ClusterOverviewPage = ({ loading, rows, updatedAt, waitMessage, error, onSelectNode }: ClusterOverviewPageProps) => {
    const [clusterName, setClusterName] = useState(CLUSTER_PROFILES[0].name);
    const selectedCluster = CLUSTER_PROFILES.find((profile) => profile.name === clusterName) ?? CLUSTER_PROFILES[0];

    const metrics = useMemo(() => {
        const partitionCount = rows.length;
        const nodesTotal = rows.reduce((sum, row) => sum + row.nodesTotal, 0);
        const nodesAllocated = rows.reduce((sum, row) => sum + row.nodesAllocated, 0);
        const cpusTotal = rows.reduce((sum, row) => sum + row.cpusTotal, 0);
        const cpusAllocated = rows.reduce((sum, row) => sum + row.cpusAllocated, 0);
        const degraded = rows.filter(isDegradedPartition).length;

        return [
            { title: _('Partitions'), value: formatCount(partitionCount), description: cockpit.format(_('Degraded: $0'), String(degraded)) },
            { title: _('Nodes'), value: formatCount(nodesTotal), description: cockpit.format(_('Allocated: $0'), String(nodesAllocated)), progress: formatPercent(nodesAllocated, nodesTotal) },
            { title: _('CPUs'), value: formatCount(cpusTotal), description: cockpit.format(_('Allocated: $0'), String(cpusAllocated)), progress: formatPercent(cpusAllocated, cpusTotal) },
            { title: _('Snapshot'), value: rows.length > 0 ? _('Ready') : _('Empty'), description: updatedAt ? new Date(updatedAt).toLocaleString() : _('Waiting for data') },
        ];
    }, [rows, updatedAt]);

    const topologyGroups = useMemo(() => buildTopologyGroups(rows), [rows]);

    if (error) {
        return <ErrorState title={_('Unable to load cluster overview')} message={error} />;
    }

    if (loading && !rows.length) {
        return <LoadingState title={_('Loading cluster overview...')} message={waitMessage ?? _('Waiting for bridge cache...')} />;
    }

    if (!loading && rows.length === 0) {
        return <EmptyState title={_('No cluster overview data')} message={_('The bridge cache has not provided partition data yet.')} />;
    }

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <Card>
                <CardTitle>{_('Cluster selector')}</CardTitle>
                <CardBody>
                    <div style={{ display: 'grid', gap: '0.75rem', maxWidth: '24rem' }}>
                        <label>
                            <div>{_('Active cluster')}</div>
                            <FormSelect
                                value={clusterName}
                                onChange={(_event, value) => setClusterName(value)}
                                aria-label={_('Active cluster')}
                            >
                                {CLUSTER_PROFILES.map((profile) => (
                                    <FormSelectOption key={profile.name} value={profile.name} label={profile.name} />
                                ))}
                            </FormSelect>
                        </label>
                        <div>{selectedCluster.description}</div>
                        <Badge isRead>{selectedCluster.focus}</Badge>
                    </div>
                </CardBody>
            </Card>

            <Gallery hasGutter>
                {metrics.map((metric) => (
                    <GalleryItem key={metric.title}>
                        <Card>
                            <CardTitle>{metric.title}</CardTitle>
                            <CardBody>
                                <strong>{metric.value}</strong>
                                <div>{metric.description}</div>
                                {typeof metric.progress === 'number' && (
                                    <Progress value={metric.progress} aria-label={metric.title} />
                                )}
                            </CardBody>
                        </Card>
                    </GalleryItem>
                ))}
            </Gallery>

            <Card>
                <CardTitle>{_('Partition focus')}</CardTitle>
                <CardBody>
                    <Table aria-label={_('Cluster overview table')} variant="compact">
                        <Thead>
                            <Tr>
                                <Th>{_('Partition')}</Th>
                                <Th>{_('State')}</Th>
                                <Th>{_('Nodes')}</Th>
                                <Th>{_('CPUs')}</Th>
                                <Th>{_('Availability')}</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {rows.slice(0, 5).map((row) => (
                                <Tr key={row.partitionName}>
                                    <Td dataLabel={_('Partition')}>{row.partitionName}</Td>
                                    <Td dataLabel={_('State')}>{row.partitionState?.join(', ') || row.availability}</Td>
                                    <Td dataLabel={_('Nodes')}>{row.nodesTotal}</Td>
                                    <Td dataLabel={_('CPUs')}>{row.cpusTotal}</Td>
                                    <Td dataLabel={_('Availability')}>{row.availability}</Td>
                                </Tr>
                            ))}
                        </Tbody>
                    </Table>
                </CardBody>
            </Card>

            <Card>
                <CardTitle>{_('Cluster topology')}</CardTitle>
                <CardBody>
                    <div style={{ display: 'grid', gap: '1rem' }}>
                        <div>{_('Nodes are grouped by name prefix so operators can scan racks or host families quickly.')}</div>
                        <div style={{ display: 'grid', gap: '0.75rem' }}>
                            {topologyGroups.map((group) => (
                                <Card key={group.name} isPlain>
                                    <CardBody>
                                        <div style={{ display: 'grid', gap: '0.5rem' }}>
                                            <div style={{ display: 'flex', justifyContent: 'space-between', gap: '1rem', flexWrap: 'wrap' }}>
                                                <strong>{group.name}</strong>
                                                <span>{cockpit.format(_('$0 node(s)'), String(group.nodes.length))}</span>
                                            </div>
                                            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
                                                {group.nodes.map((node) => (
                                                    <button
                                                        key={node.name}
                                                        type="button"
                                                        onClick={() => onSelectNode(node.name)}
                                                        style={{
                                                            display: 'inline-flex',
                                                            alignItems: 'center',
                                                            gap: '0.5rem',
                                                            border: 'none',
                                                            background: 'transparent',
                                                            padding: 0,
                                                            cursor: 'pointer',
                                                        }}
                                                        aria-label={cockpit.format(_('Open node $0'), node.name)}
                                                    >
                                                        <span style={{ textDecoration: 'underline' }}>{node.name}</span>
                                                        {renderNodeChip(node)}
                                                    </button>
                                                ))}
                                            </div>
                                        </div>
                                    </CardBody>
                                </Card>
                            ))}
                        </div>
                    </div>
                </CardBody>
            </Card>
        </div>
    );
};

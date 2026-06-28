import React, { useMemo, useState } from 'react';
import {
    Card,
    CardBody,
    CardTitle,
    Gallery,
    GalleryItem,
    Progress,
    TreeView,
} from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';

import cockpit from 'cockpit';

import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { LoadingState } from '../../components/LoadingState';
import type { SinfoPartitionRow } from '../../types/sinfo';
import { buildNodeSummaries, groupNodesByPrefix } from '../cluster/clusterData';

const _ = cockpit.gettext;

type NodesPageProps = {
    loading: boolean;
    rows: SinfoPartitionRow[];
    updatedAt: string | null;
    waitMessage: string | null;
    error: string | null;
};

function formatCount(value: number) {
    return value.toLocaleString();
}

function formatPercent(numerator: number, denominator: number) {
    if (denominator <= 0) {
        return 0;
    }

    return Math.round((numerator / denominator) * 100);
}

export const NodesPage = ({ loading, rows, updatedAt, waitMessage, error }: NodesPageProps) => {
    const [selectedNode, setSelectedNode] = useState<string | null>(null);
    const nodes = useMemo(() => buildNodeSummaries(rows), [rows]);
    const groups = useMemo(() => groupNodesByPrefix(nodes), [nodes]);

    const selected = useMemo(() => nodes.find((node) => node.name === selectedNode) ?? null, [nodes, selectedNode]);
    const totalNodes = nodes.length;
    const activeNodes = nodes.filter((node) => !node.nodeState.toLowerCase().includes('down')).length;
    const degradedNodes = nodes.filter((node) => node.nodeState.toLowerCase().includes('down') || node.nodeState.toLowerCase().includes('drain')).length;

    const treeData = useMemo(() => groups.map((group) => ({
        id: group.name,
        name: group.name,
        defaultExpanded: true,
        children: group.nodes.map((node) => ({
            id: node.name,
            name: node.name,
        })),
    })), [groups]);

    const summaryMetrics = [
        { title: _('Nodes'), value: formatCount(totalNodes), description: _('Aggregated from partition node lists') },
        { title: _('Healthy'), value: formatCount(activeNodes), description: _('Nodes not marked down/drained') },
        { title: _('Degraded'), value: formatCount(degradedNodes), description: _('Nodes marked down/drained') },
        { title: _('Groups'), value: formatCount(groups.length), description: _('Topology groups by name prefix') },
    ];

    if (error) {
        return <ErrorState title={_('Unable to load nodes')} message={error} />;
    }

    if (loading && !rows.length) {
        return <LoadingState title={_('Loading nodes...')} message={waitMessage ?? _('Waiting for bridge cache...')} />;
    }

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <Gallery hasGutter>
                {summaryMetrics.map((metric) => (
                    <GalleryItem key={metric.title}>
                        <Card>
                            <CardTitle>{metric.title}</CardTitle>
                            <CardBody>
                                <strong>{metric.value}</strong>
                                <div>{metric.description}</div>
                            </CardBody>
                        </Card>
                    </GalleryItem>
                ))}
            </Gallery>

            <Card>
                <CardTitle>{_('Topology')}</CardTitle>
                <CardBody>
                    {nodes.length === 0 && (
                        <EmptyState title={_('No node inventory is available.')} message={_('The bridge cache has not produced node data yet.')} />
                    )}
                    {nodes.length > 0 && (
                        <TreeView
                            data={treeData}
                            aria-label={_('Node topology tree')}
                            hasGuides
                            onSelect={(_event, item) => {
                                if (item.id) {
                                    setSelectedNode(item.id);
                                }
                            }}
                        />
                    )}
                </CardBody>
            </Card>

            <Card>
                <CardTitle>{_('Node inventory')}</CardTitle>
                <CardBody>
                    {nodes.length === 0 && (
                        <EmptyState title={_('No node inventory is available.')} message={_('The bridge cache has not produced node data yet.')} />
                    )}
                    {nodes.length > 0 && (
                        <Table aria-label={_('Node inventory table')} variant="compact">
                            <Thead>
                                <Tr>
                                    <Th>{_('Node')}</Th>
                                    <Th>{_('State')}</Th>
                                    <Th>{_('Partitions')}</Th>
                                    <Th>{_('CPUs')}</Th>
                                    <Th>{_('Memory')}</Th>
                                    <Th>{_('Availability')}</Th>
                                </Tr>
                            </Thead>
                            <Tbody>
                                {nodes.map((node) => {
                                    return (
                                        <Tr key={node.name}>
                                            <Td dataLabel={_('Node')}>{node.name}</Td>
                                            <Td dataLabel={_('State')}>{node.nodeState}</Td>
                                            <Td dataLabel={_('Partitions')}>{node.partitions.join(', ')}</Td>
                                            <Td dataLabel={_('CPUs')}>{node.cpus}</Td>
                                            <Td dataLabel={_('Memory')}>{node.memory}</Td>
                                            <Td dataLabel={_('Availability')}>
                                                <Progress
                                                    value={formatPercent(node.partitions.length, Math.max(totalNodes, 1))}
                                                    title={node.availability}
                                                    label={node.availability}
                                                    measureLocation="none"
                                                    aria-label={node.availability}
                                                />
                                            </Td>
                                        </Tr>
                                    );
                                })}
                            </Tbody>
                        </Table>
                    )}
                </CardBody>
            </Card>

            {selected && (
                <Card>
                    <CardTitle>{cockpit.format(_('Selected node: $0'), selected.name)}</CardTitle>
                    <CardBody>
                        <div style={{ display: 'grid', gap: '0.5rem' }}>
                            <div><strong>{_('State')}:</strong> {selected.nodeState}</div>
                            <div><strong>{_('Partitions')}:</strong> {selected.partitions.join(', ')}</div>
                            <div><strong>{_('Availability')}:</strong> {selected.availability}</div>
                            <div><strong>{_('Features')}:</strong> {selected.features.join(', ') || _('N/A')}</div>
                            <div><strong>{_('Logical CPUs')}:</strong> {selected.cpus}</div>
                            <div><strong>{_('Memory')}:</strong> {selected.memory}</div>
                        </div>
                    </CardBody>
                </Card>
            )}

            {!loading && rows.length > 0 && (
                <p>{cockpit.format(_('Last update: $0'), updatedAt ? new Date(updatedAt).toLocaleString() : _('Unknown'))}</p>
            )}
        </div>
    );
};

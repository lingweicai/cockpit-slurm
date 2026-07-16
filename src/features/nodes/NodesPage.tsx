import React, { useEffect, useMemo, useState } from 'react';
import {
    Card,
    CardBody,
    CardTitle,
    Drawer,
    DrawerContent,
    DrawerContentBody,
    Gallery,
    GalleryItem,
    FormSelect,
    FormSelectOption,
    Progress,
    TextInput,
    TreeView,
} from '@patternfly/react-core';

import cockpit from 'cockpit';

import { EmptyState } from '../../components/EmptyState';
import { EntityTable, type EntityTableColumn } from '../../components/EntityTable';
import { EntityDrawer } from '../../components/EntityDrawer';
import { ErrorState } from '../../components/ErrorState';
import { LoadingState } from '../../components/LoadingState';
import { fetchNodes, subscribeNodeUpdates } from '../../services/nodesChannel';
import type { SinfoPartitionRow } from '../../types/sinfo';
import type { SlurmNode } from '../../types/slurm-api';
import { buildNodeSummaries, groupNodesByPrefix } from '../cluster/clusterData';
import { applySlurmNodesDelta, resolveNodeSummaries, type SlurmNodesPayload } from './nodesData';

const _ = cockpit.gettext;

type NodesPageProps = {
    loading: boolean;
    rows: SinfoPartitionRow[];
    updatedAt: string | null;
    waitMessage: string | null;
    error: string | null;
};

type NodeSortKey = 'name' | 'state' | 'cpus' | 'memory';
type NodeSortDirection = 'asc' | 'desc';

function formatCount(value: number) {
    return value.toLocaleString();
}

function formatPercent(numerator: number, denominator: number) {
    if (denominator <= 0) {
        return 0;
    }

    return Math.round((numerator / denominator) * 100);
}

function normalizeStateToken(state: string) {
    return state
            .split(/[,+]/)
            .map((token) => token.trim().toUpperCase())
            .find(Boolean) ?? 'UNKNOWN';
}

function matchesNodeQuery(node: { name: string; nodeState: string; partitions: string[]; features: string[] }, query: string) {
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

type NodesTableRow = {
    name: string;
    nodeState: string;
    partitions: string[];
    cpus: number;
    memory: number;
    availability: string;
};

export const NodesPage = ({ loading, rows, updatedAt, waitMessage, error }: NodesPageProps) => {
    const [nodesPayload, setNodesPayload] = useState<SlurmNodesPayload | null>(null);
    const [selectedNode, setSelectedNode] = useState<string | null>(null);
    const [query, setQuery] = useState('');
    const [stateFilter, setStateFilter] = useState('ALL');
    const [sortKey, setSortKey] = useState<NodeSortKey>('name');
    const [sortDirection, setSortDirection] = useState<NodeSortDirection>('asc');
    const nodes = useMemo(() => {
        const liveNodes = resolveNodeSummaries(nodesPayload);
        if (liveNodes.length > 0) {
            return liveNodes;
        }

        return buildNodeSummaries(rows);
    }, [nodesPayload, rows]);

    const stateOptions = useMemo(() => {
        return ['ALL', ...Array.from(new Set(nodes.map((node) => normalizeStateToken(node.nodeState)))).sort()];
    }, [nodes]);

    const filteredNodes = useMemo(() => {
        return nodes.filter((node) => {
            const matchesState = stateFilter === 'ALL' || normalizeStateToken(node.nodeState) === stateFilter;
            return matchesState && matchesNodeQuery(node, query);
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

    const groups = useMemo(() => groupNodesByPrefix(sortedNodes), [sortedNodes]);

    const handleSortChange = (nextSortKey: NodeSortKey) => {
        if (sortKey === nextSortKey) {
            setSortDirection((current) => (current === 'asc' ? 'desc' : 'asc'));
            return;
        }

        setSortKey(nextSortKey);
        setSortDirection('asc');
    };

    useEffect(() => {
        let isMounted = true;

        const loadNodes = async () => {
            try {
                const payload = await fetchNodes();
                if (!isMounted) {
                    return;
                }

                setNodesPayload(payload);
            } catch {
                // Keep sinfo-derived fallback if live node fetch fails.
            }
        };

        loadNodes();

        const unsubscribe = subscribeNodeUpdates((_event, delta) => {
            if (delta) {
                setNodesPayload((current) => applySlurmNodesDelta(current, delta));
                return;
            }

            loadNodes().catch(() => {
                // Keep current node view if refresh fails.
            });
        });

        return () => {
            isMounted = false;
            unsubscribe();
        };
    }, []);

    useEffect(() => {
        if (!selectedNode || !sortedNodes.some((node) => node.name === selectedNode)) {
            setSelectedNode(sortedNodes[0]?.name ?? null);
        }
    }, [selectedNode, sortedNodes]);

    const selected = useMemo(() => sortedNodes.find((node) => node.name === selectedNode) ?? null, [selectedNode, sortedNodes]);
    const totalNodes = sortedNodes.length;
    const activeNodes = sortedNodes.filter((node) => !node.nodeState.toLowerCase().includes('down')).length;
    const degradedNodes = sortedNodes.filter((node) => node.nodeState.toLowerCase().includes('down') || node.nodeState.toLowerCase().includes('drain')).length;

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

    const tableColumns: EntityTableColumn<NodesTableRow>[] = [
        {
            header: _('Node'),
            dataLabel: _('Node'),
            cell: (node) => node.name,
            sortable: {
                isActive: sortKey === 'name',
                direction: sortDirection,
                onSort: () => handleSortChange('name'),
            },
        },
        {
            header: _('State'),
            dataLabel: _('State'),
            cell: (node) => node.nodeState,
            sortable: {
                isActive: sortKey === 'state',
                direction: sortDirection,
                onSort: () => handleSortChange('state'),
            },
        },
        {
            header: _('Partitions'),
            dataLabel: _('Partitions'),
            cell: (node) => node.partitions.join(', '),
        },
        {
            header: _('CPUs'),
            dataLabel: _('CPUs'),
            cell: (node) => node.cpus,
            sortable: {
                isActive: sortKey === 'cpus',
                direction: sortDirection,
                onSort: () => handleSortChange('cpus'),
            },
        },
        {
            header: _('Memory'),
            dataLabel: _('Memory'),
            cell: (node) => node.memory,
            sortable: {
                isActive: sortKey === 'memory',
                direction: sortDirection,
                onSort: () => handleSortChange('memory'),
            },
        },
        {
            header: _('Availability'),
            dataLabel: _('Availability'),
            cell: (node) => (
                <Progress
                    value={formatPercent(node.partitions.length, Math.max(totalNodes, 1))}
                    title={node.availability}
                    label={node.availability}
                    measureLocation="none"
                    aria-label={node.availability}
                />
            ),
        },
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
                    {filteredNodes.length === 0 && (
                        <EmptyState title={_('No node inventory is available.')} message={_('The bridge cache has not produced node data yet.')} />
                    )}
                    {sortedNodes.length > 0 && (
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

            <Drawer isExpanded={Boolean(selected)}>
                <DrawerContent
                    panelContent={selected ? (
                        <EntityDrawer
                            title={cockpit.format(_('Selected node: $0'), selected.name)}
                            onClose={() => setSelectedNode(null)}
                        >
                            <div style={{ display: 'grid', gap: '0.5rem' }}>
                                <div><strong>{_('State')}:</strong> {selected.nodeState}</div>
                                <div><strong>{_('Partitions')}:</strong> {selected.partitions.join(', ')}</div>
                                <div><strong>{_('Availability')}:</strong> {selected.availability}</div>
                                <div><strong>{_('Features')}:</strong> {selected.features.join(', ') || _('N/A')}</div>
                                <div><strong>{_('Logical CPUs')}:</strong> {selected.cpus}</div>
                                <div><strong>{_('Memory')}:</strong> {selected.memory}</div>
                            </div>
                        </EntityDrawer>
                    ) : undefined}
                >
                    <DrawerContentBody>
                        <Card>
                            <CardTitle>{_('Node inventory')}</CardTitle>
                            <CardBody>
                                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.75rem', marginBottom: '1rem' }}>
                                    <label>
                                        <div>{_('Search nodes')}</div>
                                        <TextInput
                                            id="node-search"
                                            value={query}
                                            onChange={(_event, value) => setQuery(value)}
                                            placeholder={_('Filter by node, partition, feature, or state')}
                                        />
                                    </label>
                                    <label>
                                        <div>{_('State')}</div>
                                        <FormSelect id="node-state-filter" value={stateFilter} onChange={(_event, value) => setStateFilter(value)}>
                                            {stateOptions.map((option) => (
                                                <FormSelectOption key={option} value={option} label={option} />
                                            ))}
                                        </FormSelect>
                                    </label>
                                </div>

                                {filteredNodes.length === 0 && (
                                    <EmptyState title={_('No matching nodes')} message={_('Adjust the search or state filter to find nodes.')} />
                                )}
                                {sortedNodes.length > 0 && (
                                    <EntityTable
                                        ariaLabel={_('Node inventory table')}
                                        columns={tableColumns}
                                        rows={sortedNodes}
                                        rowKey={(node) => node.name}
                                        onRowClick={(node) => setSelectedNode(node.name)}
                                        selectedRowKey={selected?.name ?? null}
                                        pagination={{
                                            defaultPerPage: 10,
                                            perPageOptions: [10, 20, 50],
                                        }}
                                    />
                                )}
                            </CardBody>
                        </Card>
                    </DrawerContentBody>
                </DrawerContent>
            </Drawer>

            {!loading && rows.length > 0 && (
                <p>{cockpit.format(_('Last update: $0'), updatedAt ? new Date(updatedAt).toLocaleString() : _('Unknown'))}</p>
            )}
        </div>
    );
};

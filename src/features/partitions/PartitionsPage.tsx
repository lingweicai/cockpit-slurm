import React, { useEffect, useMemo, useState } from 'react';
import {
    Card,
    CardBody,
    CardTitle,
} from '@patternfly/react-core';

import cockpit from 'cockpit';

import { ActionFeedbackAlert } from '../../components/ActionFeedbackAlert';
import { EmptyState } from '../../components/EmptyState';
import { EntityTable, type EntityTableColumn, type EntityTableRowAction } from '../../components/EntityTable';
import { ErrorState } from '../../components/ErrorState';
import { ResetTableFiltersButton } from '../../components/ResetTableFiltersButton';
import { SummaryMetricsGallery } from '../../components/SummaryMetricsGallery';
import { TableEmptyMatchState } from '../../components/TableEmptyMatchState';
import { TableToolbarActions } from '../../components/TableToolbarActions';
import { useTransientAlert } from '../../hooks/useTransientAlert';
import { buildCopyNameRowAction, buildToggleDetailsRowAction } from '../../lib/rowActions';
import { LoadingState } from '../../components/LoadingState';
import { fetchPartitions, subscribePartitionUpdates } from '../../services/partitionsChannel';
import type { SinfoPartitionRow } from '../../types/sinfo';
import { buildPartitionSummaries, type PartitionSummary } from '../cluster/clusterData';
import { applySlurmPartitionsDelta, resolvePartitionSummaries, type SlurmPartitionsPayload } from './partitionsData';
import { type PartitionSortKey, matchesPartitionFilter, usePartitionsTableState } from './usePartitionsTableState';

const _ = cockpit.gettext;

type PartitionsPageProps = {
    loading: boolean;
    rows: SinfoPartitionRow[];
    updatedAt: string | null;
    waitMessage: string | null;
    error: string | null;
    initialSelectedPartition: string | null;
};

function buildPartitionRowActionItems(
    summary: PartitionSummary,
    onSelectPartition: (partitionName: string) => void,
    onToggleExpanded: (partitionName: string) => void,
    showActionMessage: (alert: { variant: 'success' | 'danger' | 'warning' | 'info'; title: string }) => void,
): EntityTableRowAction<PartitionSummary>[] {
    return [
        buildToggleDetailsRowAction({
            onSelect: () => onSelectPartition(summary.partitionName),
            onToggle: () => onToggleExpanded(summary.partitionName),
        }),
        buildCopyNameRowAction({
            id: 'copy-partition-name',
            label: _('Copy Partition Name'),
            value: summary.partitionName,
            successTitle: cockpit.format(_('Copied partition name $0 to clipboard.'), summary.partitionName),
            failureTitle: cockpit.format(_('Unable to copy partition name $0.'), summary.partitionName),
            showAlert: showActionMessage,
        }),
    ];
}

function formatCount(value: number) {
    return value.toLocaleString();
}

function isDegradedPartition(row: SinfoPartitionRow) {
    const states = row.partitionState?.map((state) => state.toLowerCase()) ?? [];
    return states.some((state) => state.includes('down') || state.includes('drain') || state.includes('error'));
}

export const PartitionsPage = ({ loading, rows, updatedAt, waitMessage, error, initialSelectedPartition }: PartitionsPageProps) => {
    const [partitionsPayload, setPartitionsPayload] = useState<SlurmPartitionsPayload | null>(null);
    const [expandedPartition, setExpandedPartition] = useState<string | null>(null);
    const [selectedPartition, setSelectedPartition] = useState<string | null>(initialSelectedPartition ?? null);
    const { alert: actionMessage, showAlert: showActionMessage } = useTransientAlert();
    const summaries = useMemo(() => {
        const liveSummaries = resolvePartitionSummaries(partitionsPayload);
        if (liveSummaries.length > 0) {
            return liveSummaries;
        }

        return buildPartitionSummaries(rows);
    }, [partitionsPayload, rows]);
    const {
        query,
        setQuery,
        sortKey,
        sortDirection,
        sortedSummaries,
        handleSortChange,
        resetFilters,
    } = usePartitionsTableState(summaries);

    useEffect(() => {
        let isMounted = true;

        const loadPartitions = async () => {
            try {
                const payload = await fetchPartitions();
                if (!isMounted) {
                    return;
                }

                setPartitionsPayload(payload);
            } catch {
                // Keep sinfo-derived fallback if live partition fetch fails.
            }
        };

        loadPartitions();

        const unsubscribe = subscribePartitionUpdates((_event, delta) => {
            if (delta) {
                setPartitionsPayload((current) => applySlurmPartitionsDelta(current, delta));
                return;
            }

            loadPartitions().catch(() => {
                // Keep current partition view if refresh fails.
            });
        });

        return () => {
            isMounted = false;
            unsubscribe();
        };
    }, []);

    const metrics = useMemo(() => {
        const degraded = rows.filter(isDegradedPartition).length;
        return [
            { title: _('Partitions'), value: formatCount(rows.length), description: cockpit.format(_('Degraded: $0'), String(degraded)) },
            { title: _('Nodes'), value: formatCount(rows.reduce((sum, row) => sum + row.nodesTotal, 0)), description: _('Across all partitions') },
            { title: _('CPUs'), value: formatCount(rows.reduce((sum, row) => sum + row.cpusTotal, 0)), description: _('Across all partitions') },
            { title: _('Reservations'), value: formatCount(rows.filter((row) => Boolean(row.reservation)).length), description: _('Partitions with active reservations') },
        ];
    }, [rows]);

    useEffect(() => {
        if (initialSelectedPartition && sortedSummaries.some((summary) => summary.partitionName === initialSelectedPartition)) {
            setSelectedPartition(initialSelectedPartition);
            setExpandedPartition(initialSelectedPartition);
            return;
        }

        if (!selectedPartition || !sortedSummaries.some((summary) => summary.partitionName === selectedPartition)) {
            const nextPartition = sortedSummaries[0]?.partitionName ?? null;
            setSelectedPartition(nextPartition);
            setExpandedPartition(nextPartition);
        }
    }, [initialSelectedPartition, selectedPartition, sortedSummaries]);

    const selectPartition = (partitionName: string) => {
        const nextHash = `#partitions?partition=${encodeURIComponent(partitionName)}`;
        if (window.location.hash !== nextHash) {
            window.location.hash = nextHash;
        }

        setSelectedPartition(partitionName);
        setExpandedPartition(partitionName);
    };

    const tableColumns: EntityTableColumn<PartitionSummary>[] = [
        {
            header: _('Partition'),
            dataLabel: _('Partition'),
            cell: (summary) => summary.partitionName,
            sortable: {
                isActive: sortKey === 'partitionName',
                direction: sortDirection,
                onSort: () => handleSortChange('partitionName' as PartitionSortKey),
            },
        },
        {
            header: _('State'),
            dataLabel: _('State'),
            cell: (summary) => summary.state,
            sortable: {
                isActive: sortKey === 'state',
                direction: sortDirection,
                onSort: () => handleSortChange('state' as PartitionSortKey),
            },
        },
        {
            header: _('Nodes'),
            dataLabel: _('Nodes'),
            cell: (summary) => summary.nodes,
            sortable: {
                isActive: sortKey === 'nodes',
                direction: sortDirection,
                onSort: () => handleSortChange('nodes' as PartitionSortKey),
            },
        },
        {
            header: _('CPUs'),
            dataLabel: _('CPUs'),
            cell: (summary) => summary.cpus,
            sortable: {
                isActive: sortKey === 'cpus',
                direction: sortDirection,
                onSort: () => handleSortChange('cpus' as PartitionSortKey),
            },
        },
        {
            header: _('Availability'),
            dataLabel: _('Availability'),
            cell: (summary) => summary.availability,
        },
    ];

    if (error) {
        return <ErrorState title={_('Unable to load partitions')} message={error} />;
    }

    if (loading && !rows.length) {
        return <LoadingState title={_('Loading partitions...')} message={waitMessage ?? _('Waiting for bridge cache...')} />;
    }

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <SummaryMetricsGallery metrics={metrics} />

            <Card>
                <CardTitle>{_('Partition summary')}</CardTitle>
                <CardBody>
                    <ActionFeedbackAlert alert={actionMessage} />
                    {summaries.length === 0 && (
                        <EmptyState title={_('No partition rows are available.')} message={_('The bridge cache has not produced partition data yet.')} />
                    )}
                    {summaries.length > 0 && (
                        <EntityTable
                            ariaLabel={_('Partition summary table')}
                            columns={tableColumns}
                            rows={sortedSummaries}
                            rowKey={(summary) => summary.partitionName}
                            onRowClick={(summary) => selectPartition(summary.partitionName)}
                            selectedRowKey={selectedPartition}
                            rowActionsVariant="menu"
                            rowActionItems={(summary) => buildPartitionRowActionItems(summary, selectPartition, (partitionName) => setExpandedPartition((current) => (current === partitionName ? null : partitionName)), showActionMessage)}
                            expandable={{
                                expandedRowKey: expandedPartition,
                                onToggle: (_summary, rowKey) => {
                                    setExpandedPartition((current) => (current === rowKey ? null : rowKey));
                                },
                                renderExpandedContent: (summary) => (
                                    <div style={{ display: 'grid', gap: '0.5rem' }}>
                                        <div><strong>{_('Features')}:</strong> {summary.features}</div>
                                        <div><strong>{_('Limits')}:</strong> {summary.limits}</div>
                                        <div><strong>{_('Reservation')}:</strong> {summary.reservation}</div>
                                        <div><strong>{_('Comment')}:</strong> {summary.comment}</div>
                                        <div><strong>{_('Partition TRES')}:</strong> {summary.partitionTRES || _('N/A')}</div>
                                    </div>
                                ),
                            }}
                            toolbar={(
                                <TableToolbarActions>
                                    <ResetTableFiltersButton onReset={resetFilters} />
                                </TableToolbarActions>
                            )}
                            filter={{
                                placeholder: _('Filter partitions by name, state, limits, reservation, or TRES'),
                                query,
                                onQueryChange: setQuery,
                                matches: matchesPartitionFilter,
                                emptyState: <TableEmptyMatchState title={_('No matching partitions')} message={_('Adjust the filter to find partitions.')} />,
                            }}
                        />
                    )}
                    {!loading && summaries.length > 0 && (
                        <p>{cockpit.format(_('Last update: $0'), updatedAt ? new Date(updatedAt).toLocaleString() : _('Unknown'))}</p>
                    )}
                </CardBody>
            </Card>
        </div>
    );
};

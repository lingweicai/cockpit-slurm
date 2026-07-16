import React, { useEffect, useMemo, useState } from 'react';
import {
    Card,
    CardBody,
    CardTitle,
    Gallery,
    GalleryItem,
} from '@patternfly/react-core';

import cockpit from 'cockpit';

import { EmptyState } from '../../components/EmptyState';
import { EntityTable, type EntityTableColumn } from '../../components/EntityTable';
import { ErrorState } from '../../components/ErrorState';
import { LoadingState } from '../../components/LoadingState';
import { fetchPartitions, subscribePartitionUpdates } from '../../services/partitionsChannel';
import type { SinfoPartitionRow } from '../../types/sinfo';
import type { SlurmPartition } from '../../types/slurm-api';
import { buildPartitionSummaries, type PartitionSummary } from '../cluster/clusterData';
import { applySlurmPartitionsDelta, resolvePartitionSummaries } from './partitionsData';

const _ = cockpit.gettext;

type PartitionsPageProps = {
    loading: boolean;
    rows: SinfoPartitionRow[];
    updatedAt: string | null;
    waitMessage: string | null;
    error: string | null;
};

function formatCount(value: number) {
    return value.toLocaleString();
}

function isDegradedPartition(row: SinfoPartitionRow) {
    const states = row.partitionState?.map((state) => state.toLowerCase()) ?? [];
    return states.some((state) => state.includes('down') || state.includes('drain') || state.includes('error'));
}

export const PartitionsPage = ({ loading, rows, updatedAt, waitMessage, error }: PartitionsPageProps) => {
    const [partitionsPayload, setPartitionsPayload] = useState<{ partitions: SlurmPartition[] } | null>(null);
    const [expandedPartition, setExpandedPartition] = useState<string | null>(null);
    const summaries = useMemo(() => {
        const liveSummaries = resolvePartitionSummaries(partitionsPayload);
        if (liveSummaries.length > 0) {
            return liveSummaries;
        }

        return buildPartitionSummaries(rows);
    }, [partitionsPayload, rows]);

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

    const tableColumns: EntityTableColumn<PartitionSummary>[] = [
        {
            header: _('Partition'),
            dataLabel: _('Partition'),
            cell: (summary) => summary.partitionName,
        },
        {
            header: _('State'),
            dataLabel: _('State'),
            cell: (summary) => summary.state,
        },
        {
            header: _('Nodes'),
            dataLabel: _('Nodes'),
            cell: (summary) => summary.nodes,
        },
        {
            header: _('CPUs'),
            dataLabel: _('CPUs'),
            cell: (summary) => summary.cpus,
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
            <Gallery hasGutter>
                {metrics.map((metric) => (
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
                <CardTitle>{_('Partition summary')}</CardTitle>
                <CardBody>
                    {summaries.length === 0 && (
                        <EmptyState title={_('No partition rows are available.')} message={_('The bridge cache has not produced partition data yet.')} />
                    )}
                    {summaries.length > 0 && (
                        <EntityTable
                            ariaLabel={_('Partition summary table')}
                            columns={tableColumns}
                            rows={summaries}
                            rowKey={(summary) => summary.partitionName}
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

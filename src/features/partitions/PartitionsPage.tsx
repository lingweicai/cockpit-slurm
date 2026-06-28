import React, { useMemo, useState } from 'react';
import {
    Card,
    CardBody,
    CardTitle,
    Gallery,
    GalleryItem,
} from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';

import cockpit from 'cockpit';

import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { LoadingState } from '../../components/LoadingState';
import type { SinfoPartitionRow } from '../../types/sinfo';
import { buildPartitionSummaries } from '../cluster/clusterData';

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
    const [expandedPartition, setExpandedPartition] = useState<string | null>(null);
    const summaries = useMemo(() => buildPartitionSummaries(rows), [rows]);

    const metrics = useMemo(() => {
        const degraded = rows.filter(isDegradedPartition).length;
        return [
            { title: _('Partitions'), value: formatCount(rows.length), description: cockpit.format(_('Degraded: $0'), String(degraded)) },
            { title: _('Nodes'), value: formatCount(rows.reduce((sum, row) => sum + row.nodesTotal, 0)), description: _('Across all partitions') },
            { title: _('CPUs'), value: formatCount(rows.reduce((sum, row) => sum + row.cpusTotal, 0)), description: _('Across all partitions') },
            { title: _('Reservations'), value: formatCount(rows.filter((row) => Boolean(row.reservation)).length), description: _('Partitions with active reservations') },
        ];
    }, [rows]);

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
                    {rows.length === 0 && (
                        <EmptyState title={_('No partition rows are available.')} message={_('The bridge cache has not produced partition data yet.')} />
                    )}
                    {rows.length > 0 && (
                        <Table aria-label={_('Partition summary table')} variant="compact">
                            <Thead>
                                <Tr>
                                    <Th screenReaderText={_('Expand row')} />
                                    <Th>{_('Partition')}</Th>
                                    <Th>{_('State')}</Th>
                                    <Th>{_('Nodes')}</Th>
                                    <Th>{_('CPUs')}</Th>
                                    <Th>{_('Availability')}</Th>
                                </Tr>
                            </Thead>
                            {summaries.map((summary, rowIndex) => {
                                const row = rows.find((item) => item.partitionName === summary.partitionName);
                                const isExpanded = expandedPartition === summary.partitionName;

                                return (
                                    <Tbody key={summary.partitionName} isExpanded={isExpanded}>
                                        <Tr>
                                            <Td
                                                expand={{
                                                    isExpanded,
                                                    rowIndex,
                                                    onToggle: () => setExpandedPartition(isExpanded ? null : summary.partitionName),
                                                }}
                                            />
                                            <Td dataLabel={_('Partition')}>{summary.partitionName}</Td>
                                            <Td dataLabel={_('State')}>{summary.state}</Td>
                                            <Td dataLabel={_('Nodes')}>{summary.nodes}</Td>
                                            <Td dataLabel={_('CPUs')}>{summary.cpus}</Td>
                                            <Td dataLabel={_('Availability')}>{summary.availability}</Td>
                                        </Tr>
                                        {isExpanded && row && (
                                            <Tr isExpanded>
                                                <Td colSpan={6}>
                                                    <div style={{ display: 'grid', gap: '0.5rem' }}>
                                                        <div><strong>{_('Features')}:</strong> {summary.features}</div>
                                                        <div><strong>{_('Limits')}:</strong> {summary.limits}</div>
                                                        <div><strong>{_('Reservation')}:</strong> {summary.reservation}</div>
                                                        <div><strong>{_('Comment')}:</strong> {summary.comment}</div>
                                                        <div><strong>{_('Partition TRES')}:</strong> {row.partitionTRES || _('N/A')}</div>
                                                    </div>
                                                </Td>
                                            </Tr>
                                        )}
                                    </Tbody>
                                );
                            })}
                        </Table>
                    )}
                    {!loading && rows.length > 0 && (
                        <p>{cockpit.format(_('Last update: $0'), updatedAt ? new Date(updatedAt).toLocaleString() : _('Unknown'))}</p>
                    )}
                </CardBody>
            </Card>
        </div>
    );
};

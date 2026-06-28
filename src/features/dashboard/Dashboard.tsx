import React, { useMemo } from 'react';
import {
    Card,
    CardBody,
    CardTitle,
    Gallery,
    GalleryItem,
    Progress,
} from '@patternfly/react-core';

import cockpit from 'cockpit';

import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { LoadingState } from '../../components/LoadingState';
import type { SinfoPartitionRow } from '../../types/sinfo';

const _ = cockpit.gettext;

type DashboardProps = {
    loading: boolean;
    rows: SinfoPartitionRow[];
    updatedAt: string | null;
    waitMessage: string | null;
    error: string | null;
};

type DashboardMetric = {
    title: string;
    value: string;
    description: string;
    progress?: number;
    progressLabel?: string;
    progressVariant?: 'danger' | 'success' | 'warning';
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

function formatUpdatedAt(value: string | null) {
    if (!value) {
        return _('Unknown');
    }

    const timestamp = new Date(value);
    if (Number.isNaN(timestamp.getTime())) {
        return value;
    }

    return timestamp.toLocaleString();
}

function formatSnapshotAge(value: string | null) {
    if (!value) {
        return _('Waiting for the first bridge snapshot.');
    }

    const timestamp = new Date(value);
    if (Number.isNaN(timestamp.getTime())) {
        return _('Snapshot timestamp is unavailable.');
    }

    const ageMs = Date.now() - timestamp.getTime();
    if (ageMs < 30_000) {
        return _('Fresh');
    }

    if (ageMs < 60_000) {
        return _('Less than 1 minute old');
    }

    const minutes = Math.round(ageMs / 60_000);
    return cockpit.format(_('About $0 minute(s) old'), String(minutes));
}

function isDegradedPartition(row: SinfoPartitionRow) {
    const states = row.partitionState?.map((state) => state.toLowerCase()) ?? [];
    return states.some((state) => state.includes('down') || state.includes('drain') || state.includes('error'));
}

function buildMetrics(rows: SinfoPartitionRow[]): DashboardMetric[] {
    const partitionCount = rows.length;
    const degradedPartitions = rows.filter(isDegradedPartition).length;
    const healthyPartitions = partitionCount - degradedPartitions;

    const nodesTotal = rows.reduce((sum, row) => sum + row.nodesTotal, 0);
    const nodesAllocated = rows.reduce((sum, row) => sum + row.nodesAllocated, 0);
    const cpusTotal = rows.reduce((sum, row) => sum + row.cpusTotal, 0);
    const cpusAllocated = rows.reduce((sum, row) => sum + row.cpusAllocated, 0);

    return [
        {
            title: _('Partitions'),
            value: formatCount(partitionCount),
            description: cockpit.format(_('Healthy: $0 · Degraded: $1'), String(healthyPartitions), String(degradedPartitions)),
        },
        {
            title: _('Nodes'),
            value: formatCount(nodesTotal),
            description: cockpit.format(_('Allocated: $0 · Idle/other: $1'), String(nodesAllocated), String(nodesTotal - nodesAllocated)),
            progress: formatPercent(nodesAllocated, nodesTotal),
            progressLabel: cockpit.format(_('$0 allocated'), `${formatPercent(nodesAllocated, nodesTotal)}%`),
            progressVariant: degradedPartitions > 0 ? 'warning' : 'success',
        },
        {
            title: _('CPUs'),
            value: formatCount(cpusTotal),
            description: cockpit.format(_('Allocated: $0 · Idle/other: $1'), String(cpusAllocated), String(cpusTotal - cpusAllocated)),
            progress: formatPercent(cpusAllocated, cpusTotal),
            progressLabel: cockpit.format(_('$0 allocated'), `${formatPercent(cpusAllocated, cpusTotal)}%`),
            progressVariant: degradedPartitions > 0 ? 'warning' : 'success',
        },
        {
            title: _('Bridge snapshot'),
            value: rows.length > 0 ? _('Available') : _('Empty'),
            description: _('Live data is driven by the bridge cache.'),
        },
    ];
}

function MetricCard({ title, value, description, progress, progressLabel, progressVariant }: DashboardMetric) {
    return (
        <Card>
            <CardTitle>{title}</CardTitle>
            <CardBody>
                <div style={{ display: 'grid', gap: '0.5rem' }}>
                    <strong>{value}</strong>
                    <div>{description}</div>
                    {typeof progress === 'number' && (
                        <Progress
                            value={progress}
                            title={title}
                            measureLocation="top"
                            label={progressLabel}
                            variant={progressVariant}
                            aria-label={title}
                        />
                    )}
                </div>
            </CardBody>
        </Card>
    );
}

export const Dashboard = ({ loading, rows, updatedAt, waitMessage, error }: DashboardProps) => {
    const metrics = useMemo(() => buildMetrics(rows), [rows]);
    const nodeTotals = useMemo(() => ({
        total: rows.reduce((sum, row) => sum + row.nodesTotal, 0),
        allocated: rows.reduce((sum, row) => sum + row.nodesAllocated, 0),
        idle: rows.reduce((sum, row) => sum + row.nodesIdle, 0),
        other: rows.reduce((sum, row) => sum + row.nodesOther, 0),
    }), [rows]);
    const cpuTotals = useMemo(() => ({
        total: rows.reduce((sum, row) => sum + row.cpusTotal, 0),
        allocated: rows.reduce((sum, row) => sum + row.cpusAllocated, 0),
        idle: rows.reduce((sum, row) => sum + row.cpusIdle, 0),
        other: rows.reduce((sum, row) => sum + row.cpusOther, 0),
    }), [rows]);
    const degradedCount = rows.filter(isDegradedPartition).length;

    if (loading && !rows.length) {
        return <LoadingState title={_('Loading cluster dashboard...')} message={waitMessage ?? _('Waiting for bridge cache...')} />;
    }

    if (error) {
        return <ErrorState title={_('Unable to load dashboard')} message={error} />;
    }

    if (!loading && rows.length === 0) {
        return <EmptyState title={_('No cluster data is currently available.')} message={_('The bridge cache has not produced a snapshot yet.')} />;
    }

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <Gallery hasGutter>
                {metrics.map((metric) => (
                    <GalleryItem key={metric.title}>
                        <MetricCard {...metric} />
                    </GalleryItem>
                ))}
            </Gallery>

            <Card>
                <CardTitle>{_('Live utilization')}</CardTitle>
                <CardBody>
                    <div style={{ display: 'grid', gap: '1rem' }}>
                        <Progress
                            value={formatPercent(nodeTotals.allocated, nodeTotals.total)}
                            title={_('Nodes')}
                            label={cockpit.format(_('Allocated nodes: $0 / $1'), formatCount(nodeTotals.allocated), formatCount(nodeTotals.total))}
                            measureLocation="top"
                            variant={degradedCount > 0 ? 'warning' : 'success'}
                            aria-label={_('Allocated nodes')}
                        />
                        <Progress
                            value={formatPercent(cpuTotals.allocated, cpuTotals.total)}
                            title={_('CPUs')}
                            label={cockpit.format(_('Allocated CPUs: $0 / $1'), formatCount(cpuTotals.allocated), formatCount(cpuTotals.total))}
                            measureLocation="top"
                            variant={degradedCount > 0 ? 'warning' : 'success'}
                            aria-label={_('Allocated CPUs')}
                        />
                        <div>
                            {cockpit.format(_('Node balance: allocated $0 · idle $1 · other $2'), formatCount(nodeTotals.allocated), formatCount(nodeTotals.idle), formatCount(nodeTotals.other))}
                        </div>
                        <div>
                            {cockpit.format(_('CPU balance: allocated $0 · idle $1 · other $2'), formatCount(cpuTotals.allocated), formatCount(cpuTotals.idle), formatCount(cpuTotals.other))}
                        </div>
                    </div>
                </CardBody>
            </Card>

            <Card>
                <CardTitle>{_('Snapshot freshness')}</CardTitle>
                <CardBody>
                    <div style={{ display: 'grid', gap: '0.5rem' }}>
                        <div>{cockpit.format(_('Last update: $0'), formatUpdatedAt(updatedAt))}</div>
                        <div>{formatSnapshotAge(updatedAt)}</div>
                    </div>
                </CardBody>
            </Card>
        </div>
    );
};

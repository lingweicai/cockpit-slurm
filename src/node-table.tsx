import React from 'react';

import { GenericTable, type GenericTableColumn } from './generic-table';
import { type NodeResource, useNodes } from './node-provider';

function formatMemory(value: number | undefined): string {
    return value === undefined ? '—' : `${value} MiB`;
}

export const NodeTable = () => {
    const { nodes, loading, error } = useNodes();

    const columns: GenericTableColumn<NodeResource>[] = [
        { key: 'name', header: 'Node', render: node => node.metadata.name },
        { key: 'state', header: 'State', render: node => node.status.state || 'Unknown' },
        { key: 'reason', header: 'Reason', render: node => node.status.reason || '—' },
        { key: 'cpu-load', header: 'CPU Load', render: node => node.spec.cpuLoad ?? '—' },
        { key: 'real-memory', header: 'Real Memory', render: node => formatMemory(node.spec.realMemory) },
        { key: 'allocated-memory', header: 'Allocated Memory', render: node => formatMemory(node.spec.allocMemory) },
        { key: 'free-memory', header: 'Free Memory', render: node => formatMemory(node.spec.freeMemory) },
    ];

    return (
        <GenericTable
            items={ nodes }
            columns={ columns }
            rowKey={ node => node.metadata.name }
            ariaLabel='Slurm nodes'
            loading={ loading }
            error={ error }
            emptyMessage='The backend returned an empty Node snapshot.'
        />
    );
};
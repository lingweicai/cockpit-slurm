import React from 'react';
import { Alert } from '@patternfly/react-core/dist/esm/components/Alert/index.js';
import { Spinner } from '@patternfly/react-core/dist/esm/components/Spinner/index.js';

import { type NodeResource, useNodes } from './node-provider';

function formatMemory(value: number | undefined): string {
    return value === undefined ? '—' : `${value} MiB`;
}

function NodeRow({ node }: { node: NodeResource }) {
    return (
        <tr>
            <th scope='row'>{ node.metadata.name }</th>
            <td>{ node.status.state || 'Unknown' }</td>
            <td>{ node.status.reason || '—' }</td>
            <td>{ node.spec.cpuLoad ?? '—' }</td>
            <td>{ formatMemory(node.spec.realMemory) }</td>
            <td>{ formatMemory(node.spec.allocMemory) }</td>
            <td>{ formatMemory(node.spec.freeMemory) }</td>
        </tr>
    );
}

export const NodeTable = () => {
    const { nodes, loading, error } = useNodes();

    if (loading && nodes.length === 0) {
        return <Alert variant='info' title='Loading nodes'><Spinner size='md' aria-label='Loading nodes' /></Alert>;
    }

    if (error && nodes.length === 0) {
        return <Alert variant='danger' title='Unable to load nodes'>{ error.message }</Alert>;
    }

    if (nodes.length === 0) {
        return <Alert variant='info' title='No nodes found'>The backend returned an empty Node snapshot.</Alert>;
    }

    return (
        <div className='pf-v6-c-table pf-m-compact' role='region' aria-label='Slurm nodes' tabIndex={0}>
            <table>
                <thead>
                    <tr>
                        <th scope='col'>Node</th>
                        <th scope='col'>State</th>
                        <th scope='col'>Reason</th>
                        <th scope='col'>CPU Load</th>
                        <th scope='col'>Real Memory</th>
                        <th scope='col'>Allocated Memory</th>
                        <th scope='col'>Free Memory</th>
                    </tr>
                </thead>
                <tbody>
                    { nodes.map(node => <NodeRow key={ node.metadata.name } node={ node } />) }
                </tbody>
            </table>
        </div>
    );
};
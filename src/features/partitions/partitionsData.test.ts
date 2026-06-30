import assert from 'node:assert/strict';
import test from 'node:test';

import { applySlurmPartitionsDelta, mapSlurmPartitionToSummary, resolvePartitionSummaries } from './partitionsData';

function partition(name: string, overrides: Record<string, unknown> = {}) {
    return {
        name,
        partition: { state: ['UP'] },
        nodes: { total: 8 },
        cpus: { total: 128 },
        defaults: { time: { number: 60 } },
        tres: { configured: 'cpu=128,mem=512000M' },
        ...overrides,
    };
}

test('mapSlurmPartitionToSummary maps generated partition shape to UI summary', () => {
    const summary = mapSlurmPartitionToSummary(partition('compute') as never);

    assert.equal(summary.partitionName, 'compute');
    assert.equal(summary.state, 'UP');
    assert.equal(summary.nodes, 8);
    assert.equal(summary.cpus, 128);
    assert.equal(summary.partitionTRES, 'cpu=128,mem=512000M');
});

test('resolvePartitionSummaries sorts partitions by name', () => {
    const summaries = resolvePartitionSummaries({
        partitions: [partition('gpu') as never, partition('cpu') as never],
    });

    assert.equal(summaries[0]?.partitionName, 'cpu');
    assert.equal(summaries[1]?.partitionName, 'gpu');
});

test('applySlurmPartitionsDelta adds, modifies, and deletes partitions', () => {
    const next = applySlurmPartitionsDelta({
        partitions: [
            partition('cpu', { cpus: { total: 64 } }) as never,
            partition('gpu') as never,
        ],
    }, {
        added: [partition('debug') as never],
        modified: [partition('cpu', { cpus: { total: 96 } }) as never],
        deleted: [partition('gpu') as never],
    });

    assert.ok(next);
    assert.equal(next?.partitions.length, 2);
    assert.equal(next?.partitions.find((entry) => entry.name === 'cpu')?.cpus?.total, 96);
    assert.equal(next?.partitions.some((entry) => entry.name === 'gpu'), false);
    assert.equal(next?.partitions.some((entry) => entry.name === 'debug'), true);
});

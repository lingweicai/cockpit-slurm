import assert from 'node:assert/strict';
import test from 'node:test';

import { applySlurmNodesDelta, mapSlurmNodeToSummary, resolveNodeSummaries } from './nodesData';

function node(name: string, overrides: Record<string, unknown> = {}) {
    return {
        name,
        state: ['IDLE'],
        partitions: ['compute'],
        cpus: 64,
        real_memory: 256000,
        active_features: ['gpu'],
        ...overrides,
    };
}

test('mapSlurmNodeToSummary maps generated node shape to UI summary', () => {
    const summary = mapSlurmNodeToSummary(node('node001') as never);

    assert.equal(summary.name, 'node001');
    assert.deepEqual(summary.partitions, ['compute']);
    assert.equal(summary.nodeState, 'IDLE');
    assert.equal(summary.cpus, 64);
    assert.equal(summary.memory, 256000);
    assert.deepEqual(summary.features, ['gpu']);
});

test('resolveNodeSummaries sorts nodes by name', () => {
    const summaries = resolveNodeSummaries({
        nodes: [
            node('node002') as never,
            node('node001') as never,
        ],
    });

    assert.equal(summaries[0]?.name, 'node001');
    assert.equal(summaries[1]?.name, 'node002');
});

test('applySlurmNodesDelta adds, modifies, and deletes nodes', () => {
    const next = applySlurmNodesDelta({
        nodes: [
            node('node001', { cpus: 32 }) as never,
            node('node002') as never,
        ],
    }, {
        added: [node('node003') as never],
        modified: [node('node001', { cpus: 64 }) as never],
        deleted: [node('node002') as never],
    });

    assert.ok(next);
    assert.equal(next?.nodes.length, 2);
    assert.equal(next?.nodes.find((entry) => entry.name === 'node001')?.cpus, 64);
    assert.equal(next?.nodes.some((entry) => entry.name === 'node002'), false);
    assert.equal(next?.nodes.some((entry) => entry.name === 'node003'), true);
});

test('applySlurmNodesDelta seeds initial nodes when current is null', () => {
    const next = applySlurmNodesDelta(null, {
        added: [node('node100') as never],
    });

    assert.ok(next);
    assert.equal(next?.nodes.length, 1);
    assert.equal(next?.nodes[0]?.name, 'node100');
});

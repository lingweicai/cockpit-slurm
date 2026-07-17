import assert from 'node:assert/strict';
import test from 'node:test';

import { extractNodesDelta, extractNodesPayload } from './nodesChannelData';

test('extractNodesPayload reads snapshot node payload', () => {
    const payload = extractNodesPayload({
        type: 'snapshot',
        entity: 'node',
        data: {
            nodes: [
                { name: 'node001', state: ['IDLE'] },
            ],
        },
    });

    assert.ok(payload);
    assert.equal(payload?.nodes.length, 1);
    assert.equal(payload?.nodes[0].name, 'node001');
});

test('extractNodesDelta reads node event delta payload', () => {
    const delta = extractNodesDelta({
        type: 'event',
        entity: 'node',
        data: {
            added: [{ name: 'node002' }],
            modified: [{ name: 'node003' }],
            deleted: [{ name: 'node004' }],
        },
    });

    assert.ok(delta);
    assert.equal(delta?.added?.[0].name, 'node002');
    assert.equal(delta?.modified?.[0].name, 'node003');
    assert.equal(delta?.deleted?.[0].name, 'node004');
});

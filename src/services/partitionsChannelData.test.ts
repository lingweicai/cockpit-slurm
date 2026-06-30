import assert from 'node:assert/strict';
import test from 'node:test';

import { extractPartitionsDelta, extractPartitionsPayload } from './partitionsChannelData';

test('extractPartitionsPayload reads snapshot partition payload', () => {
    const payload = extractPartitionsPayload({
        type: 'snapshot',
        entity: 'partition',
        data: {
            partitions: [
                { name: 'compute' },
            ],
        },
    });

    assert.ok(payload);
    assert.equal(payload?.partitions.length, 1);
    assert.equal(payload?.partitions[0].name, 'compute');
});

test('extractPartitionsDelta reads partition event delta payload', () => {
    const delta = extractPartitionsDelta({
        type: 'event',
        entity: 'partition',
        data: {
            added: [{ name: 'debug' }],
            modified: [{ name: 'gpu' }],
            deleted: [{ name: 'cpu' }],
        },
    });

    assert.ok(delta);
    assert.equal(delta?.added?.[0].name, 'debug');
    assert.equal(delta?.modified?.[0].name, 'gpu');
    assert.equal(delta?.deleted?.[0].name, 'cpu');
});

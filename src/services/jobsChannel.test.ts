import assert from 'node:assert/strict';
import test from 'node:test';

import { extractJobsDelta, extractJobsPayload } from './jobsChannelData';

test('extractJobsPayload reads snapshot jobs payload', () => {
    const payload = extractJobsPayload({
        type: 'snapshot',
        entity: 'job',
        data: {
            jobs: [
                { job_id: 101, name: 'train-a' },
            ],
        },
    });

    assert.ok(payload);
    assert.equal(payload?.jobs.length, 1);
    assert.equal(payload?.jobs[0].job_id, 101);
});

test('extractJobsDelta reads event delta payload', () => {
    const delta = extractJobsDelta({
        type: 'event',
        entity: 'job',
        data: {
            added: [
                { job_id: 201, name: 'new-job' },
            ],
            modified: [
                { job_id: 202, name: 'updated-job' },
            ],
            deleted: [
                { job_id: 203, name: 'old-job' },
            ],
        },
    });

    assert.ok(delta);
    assert.equal(delta?.added?.[0].job_id, 201);
    assert.equal(delta?.modified?.[0].job_id, 202);
    assert.equal(delta?.deleted?.[0].job_id, 203);
});

test('extractJobsDelta ignores non-job events', () => {
    const delta = extractJobsDelta({
        type: 'event',
        entity: 'account',
        data: {
            added: [{ job_id: 1 }],
        },
    });

    assert.equal(delta, null);
});

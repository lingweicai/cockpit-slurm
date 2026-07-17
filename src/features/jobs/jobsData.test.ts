import assert from 'node:assert/strict';
import test from 'node:test';

import { applySlurmJobsDelta } from './jobsData';

type MinimalJob = {
    job_id: number;
    name: string;
};

function job(jobId: number, name: string): MinimalJob {
    return { job_id: jobId, name };
}

test('applySlurmJobsDelta adds and modifies jobs', () => {
    const current = {
        jobs: [
            job(1, 'old-a'),
            job(2, 'old-b'),
        ],
    };

    const next = applySlurmJobsDelta(current as never, {
        added: [job(3, 'new-c')] as never,
        modified: [job(1, 'new-a')] as never,
    });

    assert.ok(next);
    assert.equal(next?.jobs.length, 3);
    assert.equal(next?.jobs.find((entry) => entry.job_id === 1)?.name, 'new-a');
    assert.equal(next?.jobs.find((entry) => entry.job_id === 3)?.name, 'new-c');
});

test('applySlurmJobsDelta deletes jobs', () => {
    const current = {
        jobs: [
            job(1, 'keep'),
            job(2, 'delete'),
        ],
    };

    const next = applySlurmJobsDelta(current as never, {
        deleted: [job(2, 'delete')] as never,
    });

    assert.ok(next);
    assert.equal(next?.jobs.length, 1);
    assert.equal(next?.jobs[0].job_id, 1);
});

test('applySlurmJobsDelta seeds list when current is null', () => {
    const next = applySlurmJobsDelta(null, {
        added: [job(11, 'seed')] as never,
    });

    assert.ok(next);
    assert.equal(next?.jobs.length, 1);
    assert.equal(next?.jobs[0].job_id, 11);
});

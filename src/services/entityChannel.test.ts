import assert from 'node:assert/strict';
import test from 'node:test';

import { shouldResyncForGenerationGap } from './entityGeneration';

test('shouldResyncForGenerationGap returns false when previous generation is zero', () => {
    assert.equal(shouldResyncForGenerationGap(0, 3), false);
});

test('shouldResyncForGenerationGap returns false for contiguous generation updates', () => {
    assert.equal(shouldResyncForGenerationGap(10, 11), false);
});

test('shouldResyncForGenerationGap returns true when one or more events are missed', () => {
    assert.equal(shouldResyncForGenerationGap(10, 12), true);
    assert.equal(shouldResyncForGenerationGap(10, 15), true);
});

test('shouldResyncForGenerationGap returns false for out-of-order or duplicate generation values', () => {
    assert.equal(shouldResyncForGenerationGap(10, 10), false);
    assert.equal(shouldResyncForGenerationGap(10, 9), false);
});

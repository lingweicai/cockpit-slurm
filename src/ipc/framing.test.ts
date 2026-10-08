import { decodeFrameChunk, FrameDecoder, encodeFrame, validateEnvelope, validateNodeStreamMessage } from './framing.ts';
import { type Envelope } from './types.ts';

function assertEqual<T>(actual: T, expected: T, message: string): void {
  if (actual !== expected) {
    throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
  }
}

function assertThrows(callback: () => void, expectedMessage: string): void {
  try {
    callback();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (message.includes(expectedMessage)) {
      return;
    }
    throw new Error(`expected error containing ${expectedMessage}, got ${message}`);
  }
  throw new Error(`expected error containing ${expectedMessage}`);
}

function joinChunks(...chunks: Uint8Array[]): Uint8Array {
  const length = chunks.reduce((total, chunk) => total + chunk.length, 0);
  const joined = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    joined.set(chunk, offset);
    offset += chunk.length;
  }
  return joined;
}

function sampleMessage(messageId: string) {
  return {
    protocol: 'cockpit-slurm',
    version: '1.0',
    messageId,
    type: 'ping',
    payload: { status: 'ok' },
  };
}

const sample = sampleMessage('MSG0001');

const encoded = encodeFrame(sample);
assertEqual(encoded.length > 4, true, 'encoded frame should include a length header and JSON payload');

const frames = decodeFrameChunk(encoded);
assertEqual(frames.length, 1, 'one complete frame should decode');
assertEqual(frames[0].messageId, 'MSG0001', 'messageId should be preserved');
assertEqual(frames[0].type, 'ping', 'type should be preserved');
assertEqual(validateEnvelope(frames[0]), true, 'decoded envelope should validate');

function nodeStreamMessage(messageId: string, payload: unknown): Envelope<unknown> {
  return {
    protocol: 'cockpit-slurm',
    version: '1.0',
    messageId,
    type: 'event',
    timestamp: '2026-10-08T00:00:00Z',
    payload,
  };
}

const sampleNode = {
  metadata: {
    name: 'node001',
    kind: 'Node',
    generation: 3,
    observedAt: '2026-10-08T00:00:00Z',
    source: 'cockpit-slurm',
  },
  spec: { nodeName: 'node001', realMemory: 1024 },
  status: { state: 'IDLE', stateFlags: [] },
};

const snapshotMessage = nodeStreamMessage('NODE-1', {
  resource: 'node',
  event: 'snapshot',
  generation: 3,
  nodes: [sampleNode],
});
assertEqual(validateNodeStreamMessage(snapshotMessage), true, 'valid Node snapshot should validate');
assertEqual(validateNodeStreamMessage(decodeFrameChunk(encodeFrame(snapshotMessage))[0]), true, 'framed Node snapshot should validate');

const changeMessage = nodeStreamMessage('NODE-2', {
  resource: 'node',
  event: 'changes',
  generation: 4,
  changes: [
    { kind: 'updated', node: { ...sampleNode, status: { state: 'DOWN' } } },
    { kind: 'added', node: { ...sampleNode, metadata: { ...sampleNode.metadata, name: 'node002' }, spec: { nodeName: 'node002' } } },
    { kind: 'removed', node: { ...sampleNode, metadata: { ...sampleNode.metadata, name: 'node003' }, spec: { nodeName: 'node003' } } },
  ],
});
assertEqual(validateNodeStreamMessage(changeMessage), true, 'valid complete Node changes batch should validate');
assertEqual(validateNodeStreamMessage(nodeStreamMessage('NODE-3', {
  resource: 'node',
  event: 'changes',
  generation: 4,
  changes: [],
})), false, 'empty Node changes batch should be rejected');
assertEqual(validateNodeStreamMessage(nodeStreamMessage('NODE-4', {
  resource: 'node',
  event: 'snapshot',
  generation: 4,
  nodes: [sampleNode, sampleNode],
})), false, 'duplicate Node identities in a snapshot should be rejected');
assertEqual(validateNodeStreamMessage({
  ...changeMessage,
  payload: {
    ...changeMessage.payload as object,
    changes: [{ kind: 'renamed', node: sampleNode }],
  },
}), false, 'unknown Node change kinds should be rejected');

const decoder = new FrameDecoder();
const partialHeaderFrames = decoder.push(encoded.slice(0, 2));
assertEqual(partialHeaderFrames.length, 0, 'partial header should not produce a frame');
assertEqual(decoder.remaining().length, 2, 'partial header bytes should be retained');
const partialPayloadFrames = decoder.push(encoded.slice(2, 6));
assertEqual(partialPayloadFrames.length, 0, 'partial payload should not produce a frame');
assertEqual(decoder.remaining().length, 6, 'partial payload bytes should be retained');
const fragmentedFrames = decoder.push(encoded.slice(6));
assertEqual(fragmentedFrames.length, 1, 'split frame should decode after both chunks are received');
assertEqual(fragmentedFrames[0].messageId, 'MSG0001', 'messageId should survive chunk splitting');

const second = encodeFrame(sampleMessage('MSG0002'));
const third = encodeFrame(sampleMessage('MSG0003'));
const aggregatedFrames = decodeFrameChunk(joinChunks(encoded, second, third));
assertEqual(aggregatedFrames.length, 3, 'multiple frames in one chunk should decode');
assertEqual(aggregatedFrames[2].messageId, 'MSG0003', 'aggregated frames should preserve order');

const fourth = encodeFrame(sampleMessage('MSG0004'));
const mixedDecoder = new FrameDecoder();
const mixedFrames = [
  ...mixedDecoder.push(joinChunks(encoded, second.slice(0, 7))),
  ...mixedDecoder.push(joinChunks(second.slice(7), third, fourth.slice(0, 6))),
  ...mixedDecoder.push(fourth.slice(6)),
];
assertEqual(mixedFrames.length, 4, 'mixed fragmented and aggregated frames should decode');
assertEqual(mixedFrames.map(frame => frame.messageId).join(','), 'MSG0001,MSG0002,MSG0003,MSG0004', 'mixed frames should preserve order');
assertEqual(mixedDecoder.remaining().length, 0, 'complete mixed frames should not retain bytes');

assertThrows(() => new FrameDecoder().push(new Uint8Array([0, 0, 0, 0])), 'invalid frame length');
assertThrows(() => new FrameDecoder().push(new Uint8Array([0x01, 0x00, 0x00, 0x01])), 'invalid frame length');
assertThrows(() => new FrameDecoder().push(new Uint8Array([0, 0, 0, 1, 0x7b])), 'invalid JSON frame payload');
assertThrows(() => new FrameDecoder().push(new Uint8Array([0, 0, 0, 2, 0x7b, 0x7d])), 'invalid application envelope');

console.log('ipc framing checks passed');

import { decodeFrameChunk, FrameDecoder, encodeFrame, validateEnvelope } from './framing.ts';

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

const decoder = new FrameDecoder();
const split1 = encoded.slice(0, 5);
const split2 = encoded.slice(5);
const fragmentedFrames = [...decoder.push(split1), ...decoder.push(split2)];
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
assertThrows(() => new FrameDecoder().push(new Uint8Array([0, 0, 0, 1, 0x7b])), 'invalid JSON frame payload');
assertThrows(() => new FrameDecoder().push(new Uint8Array([0, 0, 0, 2, 0x7b, 0x7d])), 'invalid application envelope');

console.log('ipc framing checks passed');

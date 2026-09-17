import { decodeFrameChunk, FrameDecoder, encodeFrame, validateEnvelope } from './framing';

function assertEqual<T>(actual: T, expected: T, message: string): void {
  if (actual !== expected) {
    throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
  }
}

const sample = {
  protocol: 'cockpit-slurm',
  version: '1.0',
  messageId: 'MSG0001',
  type: 'ping',
  payload: { status: 'ok' },
};

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

console.log('ipc framing checks passed');

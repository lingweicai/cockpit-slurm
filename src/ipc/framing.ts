import { MAX_FRAME_SIZE, PROTOCOL_NAME, PROTOCOL_VERSION, type ApplicationEnvelope } from './types';

const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder();

function asUint8Array(chunk: Uint8Array | ArrayBuffer | number[] | string): Uint8Array {
  if (typeof chunk === 'string') {
    return textEncoder.encode(chunk);
  }

  if (chunk instanceof ArrayBuffer) {
    return new Uint8Array(chunk);
  }

  if (ArrayBuffer.isView(chunk)) {
    return new Uint8Array(chunk.buffer, chunk.byteOffset, chunk.byteLength);
  }

  return new Uint8Array(chunk);
}

export function validateEnvelope(value: unknown): value is ApplicationEnvelope {
  if (typeof value !== 'object' || value === null) {
    return false;
  }

  const envelope = value as Record<string, unknown>;

  return (
    envelope.protocol === PROTOCOL_NAME &&
    envelope.version === PROTOCOL_VERSION &&
    typeof envelope.messageId === 'string' &&
    envelope.messageId.length > 0 &&
    typeof envelope.type === 'string' &&
    envelope.type.length > 0
  );
}

export function encodeFrame(message: ApplicationEnvelope): Uint8Array {
  const json = JSON.stringify(message);
  if (json === undefined) {
    throw new Error('failed to encode message as JSON');
  }

  const payload = textEncoder.encode(json);
  if (payload.length === 0 || payload.length > MAX_FRAME_SIZE) {
    throw new Error(`frame payload size ${payload.length} is invalid`);
  }

  const header = new Uint8Array(4);
  const view = new DataView(header.buffer);
  view.setUint32(0, payload.length, false);

  const frame = new Uint8Array(4 + payload.length);
  frame.set(header, 0);
  frame.set(payload, 4);

  return frame;
}

export class FrameDecoder {
  private buffer = new Uint8Array(0);

  push(chunk: Uint8Array | ArrayBuffer | number[] | string): ApplicationEnvelope[] {
    const incoming = asUint8Array(chunk);
    const data = new Uint8Array(this.buffer.length + incoming.length);
    data.set(this.buffer, 0);
    data.set(incoming, this.buffer.length);
    this.buffer = data;

    const frames: ApplicationEnvelope[] = [];

    while (this.buffer.length >= 4) {
      const view = new DataView(this.buffer.buffer, this.buffer.byteOffset, this.buffer.byteLength);
      const length = view.getUint32(0, false);

      if (length === 0 || length > MAX_FRAME_SIZE) {
        throw new Error(`invalid frame length ${length}`);
      }

      const totalLength = 4 + length;
      if (this.buffer.length < totalLength) {
        break;
      }

      const payload = this.buffer.slice(4, totalLength);
      this.buffer = this.buffer.slice(totalLength);

      let parsed: unknown;
      try {
        parsed = JSON.parse(textDecoder.decode(payload));
      } catch {
        throw new Error('invalid JSON frame payload');
      }

      if (!validateEnvelope(parsed)) {
        throw new Error('invalid application envelope');
      }

      frames.push(parsed);
    }

    return frames;
  }

  remaining(): Uint8Array {
    return this.buffer;
  }
}

export function decodeFrameChunk(chunk: Uint8Array | ArrayBuffer | number[] | string): ApplicationEnvelope[] {
  const decoder = new FrameDecoder();
  return decoder.push(chunk);
}

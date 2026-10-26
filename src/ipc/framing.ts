import {
  MAX_FRAME_SIZE,
  PROTOCOL_NAME,
  PROTOCOL_VERSION,
  type Envelope,
  type NodeChange,
  type NodeChangePayload,
  type NodeSnapshotPayload,
  type NodeStreamMessage,
  type NodeStreamNode,
} from './types.ts';

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

export function validateEnvelope(value: unknown): value is Envelope {
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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0;
}

function isGeneration(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}

function isOptionalString(value: unknown): boolean {
  return value === undefined || typeof value === 'string';
}

function isNodeStreamNode(value: unknown): value is NodeStreamNode {
  if (!isRecord(value) || !isRecord(value.metadata) || !isRecord(value.spec) || !isRecord(value.status)) {
    return false;
  }

  const metadata = value.metadata;
  const spec = value.spec;
  const status = value.status;
  const optionalNumbers = ['cpuLoad', 'realMemory', 'allocMemory', 'freeMemory'] as const;

  return isNonEmptyString(metadata.name) &&
    isNonEmptyString(metadata.kind) &&
    isGeneration(metadata.generation) &&
    typeof metadata.observedAt === 'string' &&
    !Number.isNaN(Date.parse(metadata.observedAt)) &&
    isNonEmptyString(metadata.source) &&
    isNonEmptyString(spec.nodeName) &&
    isOptionalString(spec.address) &&
    isOptionalString(spec.hostname) &&
    optionalNumbers.every(key => spec[key] === undefined || (typeof spec[key] === 'number' && Number.isFinite(spec[key]))) &&
    isOptionalString(status.state) &&
    (status.stateFlags === undefined || (Array.isArray(status.stateFlags) && status.stateFlags.every(flag => typeof flag === 'string'))) &&
    isOptionalString(status.reason);
}

function isNodeSnapshotPayload(value: unknown): value is NodeSnapshotPayload {
  if (!isRecord(value) || value.resource !== 'node' || value.event !== 'snapshot' ||
      !isGeneration(value.generation) || !Array.isArray(value.nodes) ||
      !value.nodes.every(isNodeStreamNode)) {
    return false;
  }

  const identities = value.nodes.map(node => node.spec.nodeName);
  return new Set(identities).size === identities.length;
}

function isNodeChange(value: unknown): value is NodeChange {
  return isRecord(value) &&
    (value.kind === 'added' || value.kind === 'updated' || value.kind === 'removed') &&
    isNodeStreamNode(value.node);
}

function isNodeChangePayload(value: unknown): value is NodeChangePayload {
  if (!isRecord(value) || value.resource !== 'node' || value.event !== 'changes' ||
      !isGeneration(value.generation) || !Array.isArray(value.changes) ||
      value.changes.length === 0 || !value.changes.every(isNodeChange)) {
    return false;
  }

  const identities = value.changes.map(change => change.node.spec.nodeName);
  return new Set(identities).size === identities.length;
}

export function validateNodeStreamMessage(value: unknown): value is NodeStreamMessage {
  if (!validateEnvelope(value) || value.type !== 'event' || !isRecord(value.payload)) {
    return false;
  }

  return isNodeSnapshotPayload(value.payload) || isNodeChangePayload(value.payload);
}

export function encodeFrame<TPayload>(message: Envelope<TPayload>): Uint8Array {
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

  push(chunk: Uint8Array | ArrayBuffer | number[] | string): Envelope[] {
    const incoming = asUint8Array(chunk);
    const data = new Uint8Array(this.buffer.length + incoming.length);
    data.set(this.buffer, 0);
    data.set(incoming, this.buffer.length);
    this.buffer = data;

    const frames: Envelope[] = [];

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

export function decodeFrameChunk(chunk: Uint8Array | ArrayBuffer | number[] | string): Envelope[] {
  const decoder = new FrameDecoder();
  return decoder.push(chunk);
}

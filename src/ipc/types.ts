export const PROTOCOL_NAME = 'cockpit-slurm';
export const PROTOCOL_VERSION = '1.0';
export const MAX_FRAME_SIZE = 4 * 1024 * 1024;

export type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [key: string]: JsonValue };

export interface Envelope<TPayload = JsonValue | null | undefined> {
  protocol: string;
  version: string;
  messageId: string;
  type: string;
  timestamp?: string;
  payload?: TPayload;
  [key: string]: unknown;
}

export interface NodeStreamNode {
  metadata: {
    name: string;
    kind: string;
    generation: number;
    observedAt: string;
    source: string;
  };
  spec: {
    nodeName: string;
    address?: string;
    hostname?: string;
    cpuLoad?: number;
    realMemory?: number;
    allocMemory?: number;
    freeMemory?: number;
  };
  status: {
    state?: string;
    stateFlags?: string[];
    reason?: string;
  };
}

export interface NodeSnapshotPayload {
  resource: 'node';
  event: 'snapshot';
  generation: number;
  nodes: NodeStreamNode[];
}

export interface NodeChange {
  kind: 'added' | 'updated' | 'removed';
  node: NodeStreamNode;
}

export interface NodeChangePayload {
  resource: 'node';
  event: 'changes';
  generation: number;
  changes: NodeChange[];
}

export type NodeStreamEnvelope<TPayload extends NodeSnapshotPayload | NodeChangePayload> =
  Omit<Envelope<TPayload>, 'type' | 'payload'> & {
    type: 'event';
    payload: TPayload;
  };

export type NodeStreamMessage =
  | NodeStreamEnvelope<NodeSnapshotPayload>
  | NodeStreamEnvelope<NodeChangePayload>;

export interface ChannelOptions {
  socketPath: string;
  binary?: boolean;
  payload?: string;
}

export type MessageHandler = (message: Uint8Array | ArrayBuffer) => void;

export interface IpcChannel {
  send(data: Uint8Array | ArrayBuffer | number[]): void;
  close(): void;
  onmessage: MessageHandler | null;
  onerror: ((error: Error) => void) | null;
  onclose: (() => void) | null;
}

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

export interface ApplicationEnvelope {
  protocol: string;
  version: string;
  messageId: string;
  type: string;
  timestamp?: string;
  payload?: JsonValue | null;
  [key: string]: unknown;
}

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

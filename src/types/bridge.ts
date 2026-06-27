export type BridgeEntityName =
  | 'account'
  | 'association'
  | 'cluster'
  | 'job'
  | 'node'
  | 'partition'
  | 'qos'
  | 'reservation'
  | 'user'
  | 'sinfo'
  | (string & {});

export type BridgeRequestType = 'list' | 'get' | 'subscribe' | 'unsubscribe' | 'create' | 'update' | 'delete';
export type BridgeResponseType = 'snapshot' | 'event' | 'error';
export type BridgeConnectionStatus = 'connecting' | 'ready' | 'closed' | 'error';

export interface BridgeRequest<TEntity extends BridgeEntityName = BridgeEntityName, TPayload = unknown> {
  request_id: string;
  type: BridgeRequestType;
  entity: TEntity;
  generation?: number;
  id?: string;
  payload?: TPayload;
}

export interface BridgeSnapshot<TEntity extends BridgeEntityName = BridgeEntityName, TPayload = unknown> {
  request_id: string;
  type: 'snapshot';
  entity: TEntity;
  generation: number;
  timestamp: string;
  payload: TPayload;
}

export interface BridgeEvent<TEntity extends BridgeEntityName = BridgeEntityName, TPayload = unknown> {
  request_id?: string;
  type: 'event';
  entity: TEntity;
  generation: number;
  timestamp?: string;
  added?: TPayload[];
  modified?: TPayload[];
  deleted?: TPayload[];
  payload?: TPayload;
}

export interface BridgeError {
  request_id?: string;
  type: 'error';
  entity?: BridgeEntityName;
  message: string;
  code?: string;
  details?: unknown;
}

export type BridgeResponse<TEntity extends BridgeEntityName = BridgeEntityName, TPayload = unknown> =
  | BridgeSnapshot<TEntity, TPayload>
  | BridgeEvent<TEntity, TPayload>
  | BridgeError;

export type BridgeEnvelope = BridgeRequest | BridgeResponse | Record<string, unknown>;

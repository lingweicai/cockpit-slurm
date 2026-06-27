import type { BridgeEntityName, BridgeRequest, BridgeRequestType } from '../../types/bridge';

import { createRequestId } from './channel';

export function createBridgeRequest<TEntity extends BridgeEntityName, TPayload = unknown>(
  type: BridgeRequestType,
  entity: TEntity,
  payload?: TPayload,
): BridgeRequest<TEntity, TPayload> {
  const request: BridgeRequest<TEntity, TPayload> = {
    request_id: createRequestId(),
    type,
    entity,
  };

  if (payload !== undefined) {
    request.payload = payload;
  }

  return request;
}

export function createEntitySubscribeRequest<TEntity extends BridgeEntityName>(entity: TEntity, generation?: number) {
  const request = createBridgeRequest('subscribe', entity);
  if (generation !== undefined) {
    request.generation = generation;
  }
  return request;
}

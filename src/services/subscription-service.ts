import { type IpcClient } from '../ipc/client.ts';
import { type ApplicationEnvelope, type JsonValue } from '../ipc/types.ts';

export interface SubscriptionSnapshot {
  resource: string;
  subscriptionId: string;
  generation: number;
  snapshot: JsonValue;
}

export interface ResourceEvent {
  subscriptionId: string;
  resource: string;
  event: string;
  generation: number;
  eventSequence: number;
  [key: string]: JsonValue;
}

function isObject(value: JsonValue | undefined | null): value is { [key: string]: JsonValue } {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value));
}

export class SubscriptionService {
  constructor(private readonly client: IpcClient) {}

  async subscribe(resource: string): Promise<SubscriptionSnapshot> {
    const response = await this.client.send({
      type: 'subscribe',
      payload: { resource },
    });
    if (response.type !== 'subscribed' || !isObject(response.payload)) {
      throw new Error(`unexpected subscribe response ${response.type ?? 'unknown'}`);
    }

    const payload = response.payload;
    if (payload.resource !== resource || typeof payload.subscriptionId !== 'string' || typeof payload.generation !== 'number' || !('snapshot' in payload)) {
      throw new Error('invalid subscribed response');
    }
    return {
      resource,
      subscriptionId: payload.subscriptionId,
      generation: payload.generation,
      snapshot: payload.snapshot,
    };
  }

  async unsubscribe(subscriptionId: string): Promise<void> {
    const response = await this.client.send({
      type: 'unsubscribe',
      payload: { subscriptionId },
    });
    if (response.type !== 'unsubscribed') {
      throw new Error(`unexpected unsubscribe response ${response.type ?? 'unknown'}`);
    }
  }

  onEvent(subscriptionId: string, handler: (event: ResourceEvent) => void): () => void {
    return this.client.onEvent(subscriptionId, (message: ApplicationEnvelope) => {
      if (!isObject(message.payload)) {
        return;
      }
      const payload = message.payload;
      if (payload.resource !== 'nodes' || typeof payload.subscriptionId !== 'string' || typeof payload.event !== 'string' || typeof payload.generation !== 'number' || typeof payload.eventSequence !== 'number') {
        return;
      }
      handler(payload as unknown as ResourceEvent);
    });
  }
}
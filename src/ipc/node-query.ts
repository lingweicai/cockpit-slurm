import { IpcClient } from './client.ts';
import { type ApplicationEnvelope, type IpcChannel } from './types.ts';

export interface NodeResourceSnapshot {
  resource: string;
  generation: number;
  nodes: Array<{
    metadata?: { name?: string; generation?: number };
    spec?: { nodeName?: string; hostname?: string; address?: string; cpuLoad?: number; realMemory?: number; allocMemory?: number; freeMemory?: number };
    status?: { state?: string; stateFlags?: string[]; reason?: string };
    name?: string;
    state?: string;
  }>;
}

export async function queryNodes(client: IpcClient): Promise<NodeResourceSnapshot> {
  const response: ApplicationEnvelope = await client.send({
    type: 'query',
    messageId: `query-nodes-${Date.now()}`,
    payload: { resource: 'nodes' },
  });

  if (response.type !== 'query-response') {
    throw new Error(`unexpected response type ${response.type ?? 'unknown'}`);
  }

  const payload = response.payload as NodeResourceSnapshot | null | undefined;
  if (!payload || typeof payload !== 'object' || payload.resource !== 'nodes') {
    throw new Error('invalid nodes query response');
  }

  return payload;
}

export function createNodeQueryClient(channel?: IpcChannel): IpcClient {
  return new IpcClient(channel);
}

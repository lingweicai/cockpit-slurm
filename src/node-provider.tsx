import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';

import { type IpcClient } from './ipc/client';
import { queryNodes, type NodeResourceSnapshot } from './ipc/node-query';

export interface NodeResource {
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

export interface NodeProviderValue {
    nodes: NodeResource[];
    generation: number;
    loading: boolean;
    error: Error | null;
    refresh: () => Promise<void>;
}

interface NodeProviderProps {
    createClient: () => IpcClient;
    children: React.ReactNode;
}

const NodeContext = createContext<NodeProviderValue | null>(null);

function isNodeResource(value: unknown): value is NodeResource {
    if (!value || typeof value !== 'object') {
        return false;
    }
    const node = value as Partial<NodeResource>;
    return Boolean(node.metadata?.name && node.spec?.nodeName && node.status);
}

function decodeSnapshot(snapshot: NodeResourceSnapshot): NodeResource[] {
    if (!Array.isArray(snapshot.nodes) || !snapshot.nodes.every(isNodeResource)) {
        throw new Error('invalid nodes in query response');
    }
    return snapshot.nodes;
}

export const NodeProvider = ({ createClient, children }: NodeProviderProps) => {
    const [nodes, setNodes] = useState<NodeResource[]>([]);
    const [generation, setGeneration] = useState(0);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<Error | null>(null);

    const refresh = useCallback(async () => {
        setLoading(true);
        setError(null);
        const client = createClient();
        try {
            const snapshot = await queryNodes(client);
            setNodes(decodeSnapshot(snapshot));
            setGeneration(snapshot.generation);
        } catch (cause) {
            setError(cause instanceof Error ? cause : new Error(String(cause)));
        } finally {
            client.close();
            setLoading(false);
        }
    }, [createClient]);

    useEffect(() => {
        void refresh();
    }, [refresh]);

    const value = useMemo(() => ({ nodes, generation, loading, error, refresh }), [nodes, generation, loading, error, refresh]);
    return <NodeContext.Provider value={value}>{children}</NodeContext.Provider>;
};

export function useNodes(): NodeProviderValue {
    const value = useContext(NodeContext);
    if (!value) {
        throw new Error('useNodes must be used inside NodeProvider');
    }
    return value;
}
import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';

import { type IpcClient } from './ipc/client';
import { SubscriptionService, type ResourceEvent } from './services/subscription-service';

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

function decodeSnapshot(snapshot: { nodes?: unknown }): NodeResource[] {
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
    const generationRef = useRef(0);
    const clientRef = useRef<IpcClient | null>(null);
    const removeEventHandlerRef = useRef<(() => void) | null>(null);
    const subscriptionIdRef = useRef<string | null>(null);
    const eventSequenceRef = useRef(0);
    const connectRef = useRef<() => Promise<void>>();

    const applyEvent = useCallback((event: ResourceEvent) => {
        if (event.subscriptionId !== subscriptionIdRef.current || event.eventSequence <= eventSequenceRef.current) {
            return;
        }
        if (event.generation > generationRef.current + 1) {
            connectRef.current?.();
            return;
        }
        eventSequenceRef.current = event.eventSequence;
        if (event.event === 'removed') {
            const node = event.node as { metadata?: { name?: string } } | undefined;
            const name = node?.metadata?.name;
            if (name) {
                setNodes(current => current.filter(item => item.metadata.name !== name));
            }
        } else if (event.node && typeof event.node === 'object' && !Array.isArray(event.node) && isNodeResource(event.node)) {
            const nextNode = event.node;
            setNodes(current => {
                const index = current.findIndex(item => item.metadata.name === nextNode.metadata.name);
                if (index < 0) {
                    return [...current, nextNode];
                }
                return current.map((item, itemIndex) => itemIndex === index ? nextNode : item);
            });
        }
        generationRef.current = Math.max(generationRef.current, event.generation);
        setGeneration(generationRef.current);
    }, []);

    const connect = useCallback(async () => {
        setLoading(true);
        setError(null);
        removeEventHandlerRef.current?.();
        clientRef.current?.close();
        const client = createClient();
        clientRef.current = client;
        const subscriptions = new SubscriptionService(client);
        try {
            const subscription = await subscriptions.subscribe('nodes');
            const snapshot = subscription.snapshot;
            if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) {
                throw new Error('invalid nodes subscription snapshot');
            }
            setNodes(decodeSnapshot(snapshot as { nodes?: unknown }));
            generationRef.current = subscription.generation;
            setGeneration(subscription.generation);
            eventSequenceRef.current = 0;
            subscriptionIdRef.current = subscription.subscriptionId;
            removeEventHandlerRef.current = subscriptions.onEvent(subscription.subscriptionId, (event) => {
                applyEvent(event);
            });
        } catch (cause) {
            setError(cause instanceof Error ? cause : new Error(String(cause)));
        } finally {
            setLoading(false);
        }
    }, [applyEvent, createClient]);

    connectRef.current = connect;

    const refresh = useCallback(async () => {
        await connect();
    }, [connect]);

    useEffect(() => {
        refresh();
        return () => {
            removeEventHandlerRef.current?.();
            clientRef.current?.close();
        };
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
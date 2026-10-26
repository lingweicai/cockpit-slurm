import {
    type Envelope,
    type IpcSendOptions,
    type NodeCapabilities,
    type NodeCommandRequestPayload,
    type NodeCommandResponsePayload,
    type ProtocolErrorPayload,
} from './types.ts';

export const CAPABILITY_REQUEST_TIMEOUT_MS = 10_000;
export const COMMAND_REQUEST_TIMEOUT_MS = 90_000;

export interface IpcRequestClient {
    send(request: Partial<Envelope> & { type: string }, options?: IpcSendOptions): Promise<Envelope>;
}

export class IpcCommandError extends Error {
    readonly code: string;
    readonly category: string;
    readonly retryable: boolean;
    readonly commandId?: string;

    constructor(payload: ProtocolErrorPayload) {
        super(payload.message);
        this.name = 'IpcCommandError';
        this.code = payload.code;
        this.category = payload.category;
        this.retryable = payload.retryable;
        this.commandId = payload.commandId;
    }
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function decodeError(response: Envelope): IpcCommandError {
    if (!isRecord(response.payload) ||
        typeof response.payload.code !== 'string' ||
        typeof response.payload.category !== 'string' ||
        typeof response.payload.message !== 'string' ||
        typeof response.payload.retryable !== 'boolean') {
        throw new Error('backend returned a malformed error response');
    }
    const payload: ProtocolErrorPayload = {
        code: response.payload.code,
        category: response.payload.category,
        message: response.payload.message,
        retryable: response.payload.retryable,
    };
    if (typeof response.payload.commandId === 'string') {
        payload.commandId = response.payload.commandId;
    }
    return new IpcCommandError(payload);
}

function requireRecord(value: unknown, message: string): Record<string, unknown> {
    if (!isRecord(value)) {
        throw new Error(message);
    }
    return value;
}

function newCommandId(): string {
    if (!globalThis.crypto?.randomUUID) {
        throw new Error('secure command ID generation is unavailable');
    }
    return `cmd-${globalThis.crypto.randomUUID()}`;
}

export class CommandService {
    private readonly client: IpcRequestClient;

    constructor(client: IpcRequestClient) {
        this.client = client;
    }

    async requestCapabilities(): Promise<NodeCapabilities> {
        const response = await this.client.send(
            { type: 'capabilities-request', payload: {} },
            { timeoutMs: CAPABILITY_REQUEST_TIMEOUT_MS },
        );
        if (response.type === 'error') {
            throw decodeError(response);
        }
        if (response.type !== 'capabilities-response') {
            throw new Error(`unexpected capabilities response type: ${response.type}`);
        }

        const payload = requireRecord(response.payload, 'backend returned a malformed capabilities response');
        const capabilities = requireRecord(payload.capabilities, 'backend returned a malformed capability set');
        if (typeof capabilities['nodes.drain'] !== 'boolean' || typeof capabilities['nodes.resume'] !== 'boolean') {
            throw new Error('backend capability set must contain explicit Boolean values');
        }
        return {
            'nodes.drain': capabilities['nodes.drain'],
            'nodes.resume': capabilities['nodes.resume'],
        };
    }

    async updateNode(
        nodeName: string,
        parameters: NodeCommandRequestPayload['parameters'],
    ): Promise<NodeCommandResponsePayload> {
        validateCommand(nodeName, parameters);
        const commandId = newCommandId();
        const payload: NodeCommandRequestPayload = {
            commandId,
            resource: 'node',
            command: 'update',
            target: { nodeName },
            parameters,
        };
        let response: Envelope;
        try {
            response = await this.client.send(
                { type: 'command', payload },
                { timeoutMs: COMMAND_REQUEST_TIMEOUT_MS },
            );
        } catch {
            throw new IpcCommandError({
                code: 'command-outcome-unknown',
                category: 'timeout',
                message: 'The command outcome is unknown; do not retry automatically',
                retryable: false,
                commandId,
            });
        }
        if (response.type === 'error') {
            throw decodeError(response);
        }
        if (response.type !== 'command-response') {
            throw new Error(`unexpected command response type: ${response.type}`);
        }

        const result = requireRecord(response.payload, 'backend returned a malformed command response');
        if (result.commandId !== commandId || result.resource !== 'node' || result.command !== 'update' ||
            !isCommandStatus(result.status)) {
            throw new Error('backend returned a mismatched command response');
        }
        return result as unknown as NodeCommandResponsePayload;
    }
}

function validateCommand(nodeName: string, parameters: NodeCommandRequestPayload['parameters']): void {
    if (nodeName.trim() === '' || new TextEncoder().encode(nodeName).length > 255) {
        throw new Error('Node name must be non-empty and at most 255 UTF-8 bytes');
    }
    if (parameters.state === 'DRAIN') {
        const reason = parameters.reason ?? '';
        if (reason.trim() === '' || new TextEncoder().encode(reason).length > 256 || hasControlCharacters(reason)) {
            throw new Error('Drain reason must be non-empty, at most 256 UTF-8 bytes, and contain no control characters');
        }
    } else if (parameters.reason !== undefined) {
        throw new Error('Resume does not accept a reason');
    }
}

function hasControlCharacters(value: string): boolean {
    for (const character of value) {
        const codePoint = character.codePointAt(0) ?? 0;
        if (codePoint <= 0x1f || (codePoint >= 0x7f && codePoint <= 0x9f)) {
            return true;
        }
    }
    return false;
}

function isCommandStatus(value: unknown): value is NodeCommandResponsePayload['status'] {
    return value === 'running' || value === 'succeeded' || value === 'failed' || value === 'denied' || value === 'unknown';
}

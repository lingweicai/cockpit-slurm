import { CommandService, IpcCommandError } from './command-service.ts';
import { type Envelope } from './types.ts';

function assertEqual<T>(actual: T, expected: T, message: string): void {
    if (actual !== expected) {
        throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
    }
}

async function assertRejects(promise: Promise<unknown>, expected: (error: unknown) => boolean, message: string): Promise<void> {
    try {
        await promise;
    } catch (error) {
        if (expected(error)) {
            return;
        }
        throw new Error(`${message}: unexpected rejection ${String(error)}`);
    }
    throw new Error(`${message}: expected rejection`);
}

class FakeRequestClient {
    readonly requests: Array<{ request: Partial<Envelope> & { type: string }; options?: { timeoutMs?: number } }> = [];
    readonly responses: Envelope[] = [];

    async send(request: Partial<Envelope> & { type: string }, options?: { timeoutMs?: number }): Promise<Envelope> {
        this.requests.push({ request, options });
        const response = this.responses.shift();
        if (!response) {
            throw new Error('missing fake response');
        }
        return response;
    }
}

const capabilityResponse: Envelope = {
    protocol: 'cockpit-slurm',
    version: '1.0',
    messageId: 'cap-1',
    type: 'capabilities-response',
    payload: { capabilities: { 'nodes.drain': true, 'nodes.resume': false } },
};

async function run(): Promise<void> {
    const capabilitiesClient = new FakeRequestClient();
    capabilitiesClient.responses.push(capabilityResponse);
    const capabilities = await new CommandService(capabilitiesClient).requestCapabilities();
    assertEqual(capabilitiesClient.requests[0].request.type, 'capabilities-request', 'capability request type');
    assertEqual(capabilitiesClient.requests[0].options?.timeoutMs, 10_000, 'capability timeout');
    assertEqual(capabilities['nodes.drain'], true, 'drain capability');
    assertEqual(capabilities['nodes.resume'], false, 'resume capability');

    const commandClient = new FakeRequestClient();
    const commandResponse: Envelope = {
        protocol: 'cockpit-slurm',
        version: '1.0',
        messageId: 'response-1',
        type: 'command-response',
        payload: {
            commandId: 'placeholder',
            resource: 'node',
            command: 'update',
            status: 'succeeded',
        },
    };
    commandClient.responses.push(commandResponse);
    const commandService = new CommandService(commandClient);
    const commandPromise = commandService.updateNode('c21', {
        state: 'DRAIN',
        reason: 'scheduled maintenance',
    });
    const sentPayload = commandClient.requests[0].request.payload as {
        commandId: string;
        resource: string;
        command: string;
        target: { nodeName: string };
        parameters: { state: string; reason?: string };
    };
    commandResponse.payload = {
        commandId: sentPayload.commandId,
        resource: 'node',
        command: 'update',
        status: 'succeeded',
    };
    const commandResult = await commandPromise;
    assertEqual(commandClient.requests[0].request.type, 'command', 'command envelope type');
    assertEqual(commandClient.requests[0].options?.timeoutMs, 90_000, 'command timeout');
    assertEqual(sentPayload.resource, 'node', 'command resource');
    assertEqual(sentPayload.command, 'update', 'generic Node command');
    assertEqual(sentPayload.target.nodeName, 'c21', 'target node');
    assertEqual(sentPayload.parameters.state, 'DRAIN', 'drain state');
    assertEqual(sentPayload.parameters.reason, 'scheduled maintenance', 'drain reason');
    assertEqual(commandResult.commandId, sentPayload.commandId, 'response command ID');

    const invalidClient = new FakeRequestClient();
    await assertRejects(
        new CommandService(invalidClient).updateNode('c21', { state: 'DRAIN', reason: '  ' }),
        error => error instanceof Error && error.message.includes('Drain reason'),
        'empty drain reason should be rejected locally',
    );
    assertEqual(invalidClient.requests.length, 0, 'invalid command should not be sent');

    const deniedClient = new FakeRequestClient();
    deniedClient.responses.push({
        protocol: 'cockpit-slurm',
        version: '1.0',
        messageId: 'response-2',
        type: 'error',
        payload: {
            code: 'authorization-denied',
            category: 'authorization-error',
            message: 'Not authorized',
            retryable: false,
            commandId: 'cmd-denied',
        },
    });
    const denied = new CommandService(deniedClient).updateNode('c21', { state: 'RESUME' });
    deniedClient.requests[0].request.payload = {
        ...(deniedClient.requests[0].request.payload as Record<string, unknown>),
        commandId: 'cmd-denied',
    };
    await assertRejects(
        denied,
        error => error instanceof IpcCommandError && error.code === 'authorization-denied' && error.retryable === false,
        'canonical authorization denial should remain typed',
    );

    console.log('IPC command service checks passed');
}

run().catch(error => {
    throw error;
});

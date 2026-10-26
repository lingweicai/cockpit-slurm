import { IpcCommandError } from './ipc/command-service.ts';
import { type NodeCapabilities, type NodeCommandStatus, type NodeStreamNode } from './ipc/types.ts';

export type NodeAction = 'DRAIN' | 'RESUME';

export interface NodeCommandDisplayState {
    action: NodeAction;
    status: 'pending' | NodeCommandStatus;
    commandId?: string;
    error?: string;
}

export interface NodeCommandAvailability {
    node?: NodeStreamNode | undefined;
    action: NodeAction;
    capabilities: NodeCapabilities | null;
    capabilitiesLoading: boolean;
    connected: boolean;
    command?: NodeCommandDisplayState;
    commandInFlight?: boolean;
}

export function isDrained(node: NodeStreamNode): boolean {
    return node.status.state === 'DRAIN' || node.status.stateFlags?.includes('DRAIN') === true;
}

export function isCommandActive(status: NodeCommandDisplayState['status'] | undefined): boolean {
    return status === 'pending' || status === 'running' || status === 'unknown';
}

export function commandStatusText(command: NodeCommandDisplayState): string {
    if (command.status === 'pending') {
        return `Submitting ${command.action.toLowerCase()}`;
    }
    return `${command.action.toLowerCase()}: ${command.status}`;
}

export function commandFailureStatus(error: Error): NodeCommandDisplayState['status'] {
    if (!(error instanceof IpcCommandError)) {
        return 'unknown';
    }
    if (error.code === 'authorization-denied') {
        return 'denied';
    }
    if (error.code === 'command-outcome-unknown' ||
        error.code === 'command-timeout' ||
        error.code === 'ledger-persistence-failed') {
        return 'unknown';
    }
    return 'failed';
}

export function nodeCommandBlockReason(availability: NodeCommandAvailability): string | null {
    const { node, action, capabilities, capabilitiesLoading, connected, command, commandInFlight } = availability;
    if (!connected || !capabilities || capabilitiesLoading) {
        return 'Node commands are unavailable until connection capabilities are loaded';
    }
    const capability = action === 'DRAIN' ? capabilities['nodes.drain'] : capabilities['nodes.resume'];
    if (!capability) {
        return `The current user cannot ${action.toLowerCase()} nodes`;
    }
    if (!node || !node.status.state) {
        return 'The latest Node state is unavailable';
    }
    if (commandInFlight || isCommandActive(command?.status)) {
        return `A command for ${node.spec.nodeName} is already pending or has an unknown outcome`;
    }
    const applicable = action === 'DRAIN'
        ? node.status.state === 'IDLE' && !isDrained(node)
        : isDrained(node);
    if (!applicable) {
        return `${action} is not applicable to the latest observed state of ${node.spec.nodeName}`;
    }
    return null;
}

export function availableNodeActions(availability: Omit<NodeCommandAvailability, 'action'>): NodeAction[] {
    const actions: NodeAction[] = [];
    for (const action of ['DRAIN', 'RESUME'] as const) {
        if (nodeCommandBlockReason({ ...availability, action }) === null) {
            actions.push(action);
        }
    }
    return actions;
}

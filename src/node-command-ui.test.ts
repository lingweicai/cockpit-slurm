import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import { IpcCommandError } from './ipc/command-service.ts';
import { type NodeCapabilities, type NodeStreamNode } from './ipc/types.ts';
import { NodeActionsCell, NodeCommandStatus } from './node-command-ui.ts';
import type { NodeCommandDisplayState } from './node-command-state.ts';

function assertEqual<T>(actual: T, expected: T, message: string): void {
    if (actual !== expected) {
        throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
    }
}

function assertContains(value: string, expected: string, message: string): void {
    if (!value.includes(expected)) {
        throw new Error(`${message}: expected output to contain ${JSON.stringify(expected)}`);
    }
}

function makeNode(state: string | undefined = 'IDLE', stateFlags: string[] = []): NodeStreamNode {
    return {
        metadata: {
            name: 'c21',
            kind: 'Node',
            generation: 1,
            observedAt: '2026-10-25T00:00:00Z',
            source: 'cockpit-slurm',
        },
        spec: { nodeName: 'c21' },
        status: { state, stateFlags },
    };
}

function actionColumnProps(element: React.ReactElement): {
    isDisabled?: boolean;
    items: Array<{ itemKey?: string; title?: React.ReactNode }>;
} {
    return element.props as { isDisabled?: boolean; items: Array<{ itemKey?: string; title?: React.ReactNode }> };
}

const adminCapabilities: NodeCapabilities = { 'nodes.drain': true, 'nodes.resume': true };
const callbacks = { onDrain: () => undefined, onResume: () => undefined };
const baseActionProps = {
    node: makeNode(),
    capabilities: adminCapabilities,
    capabilitiesLoading: false,
    connected: true,
    ...callbacks,
};

const unavailableCapabilityProps = actionColumnProps(NodeActionsCell({
    ...baseActionProps,
    capabilities: null,
}));
assertEqual(unavailableCapabilityProps.isDisabled, true, 'unknown capabilities disable the rendered menu');
assertEqual(unavailableCapabilityProps.items.length, 0, 'unknown capabilities expose no actions');

const unavailableStateProps = actionColumnProps(NodeActionsCell({
    ...baseActionProps,
    node: makeNode(''),
}));
assertEqual(unavailableStateProps.isDisabled, true, 'unknown observed state disables the rendered menu');
assertEqual(unavailableStateProps.items.length, 0, 'unknown observed state exposes no actions');

const adminIdleProps = actionColumnProps(NodeActionsCell(baseActionProps));
assertEqual(adminIdleProps.isDisabled, false, 'admin menu is enabled when data is available');
assertEqual(adminIdleProps.items.map(item => item.itemKey).join(','), 'drain', 'idle node exposes drain only');

const drainedProps = actionColumnProps(NodeActionsCell({
    ...baseActionProps,
    node: makeNode('IDLE', ['DRAIN']),
}));
assertEqual(drainedProps.items.map(item => item.itemKey).join(','), 'resume', 'drained node exposes resume only');

for (const status of ['pending', 'running', 'unknown'] as const) {
    const blockedProps = actionColumnProps(NodeActionsCell({
        ...baseActionProps,
        command: { action: 'DRAIN', status, commandId: `cmd-${status}` },
    }));
    assertEqual(blockedProps.isDisabled, true, `${status} disables the rendered menu`);
    assertEqual(blockedProps.items.length, 0, `${status} removes duplicate actions`);
}

const pendingState: NodeCommandDisplayState = { action: 'DRAIN', status: 'pending' };
const pendingMarkup = renderToStaticMarkup(React.createElement(NodeCommandStatus, { command: pendingState }));
assertContains(pendingMarkup, 'Submitting drain', 'pending feedback is rendered');
assertContains(pendingMarkup, 'pf-v6-c-spinner', 'pending feedback includes the PatternFly spinner');

const failedState: NodeCommandDisplayState = {
    action: 'DRAIN',
    status: 'failed',
    commandId: 'cmd-failed',
    error: 'Slurm rejected the request',
};
const failedMarkup = renderToStaticMarkup(React.createElement(NodeCommandStatus, { command: failedState }));
assertContains(failedMarkup, 'drain: failed', 'failure status is rendered');
assertContains(failedMarkup, 'Slurm rejected the request', 'inline error is rendered');
assertContains(failedMarkup, 'cmd-failed', 'command correlation ID is rendered');

const unknownState: NodeCommandDisplayState = {
    action: 'RESUME',
    status: 'unknown',
    commandId: 'cmd-unknown',
    error: 'The command outcome is unknown',
};
const unknownMarkup = renderToStaticMarkup(React.createElement(NodeCommandStatus, { command: unknownState }));
assertContains(unknownMarkup, 'resume: unknown', 'unknown outcome is rendered');
assertContains(unknownMarkup, 'The command outcome is unknown', 'unknown outcome error is rendered');
assertContains(unknownMarkup, 'cmd-unknown', 'unknown command correlation ID is rendered');

const deniedState: NodeCommandDisplayState = {
    action: 'DRAIN',
    status: 'denied',
    error: new IpcCommandError({
        code: 'authorization-denied',
        category: 'authorization-error',
        message: 'Not authorized to drain nodes',
        retryable: false,
    }).message,
};
const deniedMarkup = renderToStaticMarkup(React.createElement(NodeCommandStatus, { command: deniedState }));
assertContains(deniedMarkup, 'drain: denied', 'authorization denial is rendered distinctly');
assertContains(deniedMarkup, 'Not authorized to drain nodes', 'authorization message is rendered');

console.log('Node command UI rendering checks passed');

import React from 'react';
import { Alert } from '@patternfly/react-core/dist/esm/components/Alert/index.js';
import { Spinner } from '@patternfly/react-core/dist/esm/components/Spinner/index.js';
import { ActionsColumn } from '@patternfly/react-table';

import {
    availableNodeActions,
    commandStatusText,
    type NodeCommandDisplayState,
} from './node-command-state.ts';
import { type NodeCapabilities, type NodeStreamNode } from './ipc/types.ts';

export interface NodeCommandStatusProps {
    command: NodeCommandDisplayState;
}

export function NodeCommandStatus({ command }: NodeCommandStatusProps): React.ReactElement {
    const active = command.status === 'pending' || command.status === 'running';
    return React.createElement('div', null,
                               active ? React.createElement(Spinner, { isInline: true, size: 'sm', 'aria-label': commandStatusText(command) }) : null,
                               React.createElement('span', null, commandStatusText(command)),
                               command.error ? React.createElement(Alert, { isInline: true, variant: 'danger', title: command.error }) : null,
                               command.commandId ? React.createElement('small', null, `Command ID: ${command.commandId}`) : null,
    );
}

export interface NodeActionsCellProps {
    node: NodeStreamNode;
    capabilities: NodeCapabilities | null;
    capabilitiesLoading: boolean;
    connected: boolean;
    command?: NodeCommandDisplayState;
    onDrain: () => void;
    onResume: () => void;
}

export function NodeActionsCell({
    node,
    capabilities,
    capabilitiesLoading,
    connected,
    command,
    onDrain,
    onResume,
}: NodeActionsCellProps): React.ReactElement {
    const availableActions = availableNodeActions({ node, capabilities, capabilitiesLoading, connected, command });
    const items = [];
    if (availableActions.includes('DRAIN')) {
        items.push({ itemKey: 'drain', title: 'Drain node', onClick: onDrain });
    }
    if (availableActions.includes('RESUME')) {
        items.push({ itemKey: 'resume', title: 'Resume node', onClick: onResume });
    }
    return React.createElement(ActionsColumn, {
        items,
        isDisabled: !connected || capabilitiesLoading || items.length === 0,
    });
}

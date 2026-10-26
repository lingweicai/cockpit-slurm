import React, { useState } from 'react';
import { Alert } from '@patternfly/react-core/dist/esm/components/Alert/index.js';
import { Button } from '@patternfly/react-core/dist/esm/components/Button/index.js';
import { FormGroup } from '@patternfly/react-core/dist/esm/components/Form/FormGroup.js';
import { Modal } from '@patternfly/react-core/dist/esm/components/Modal/Modal.js';
import { ModalBody } from '@patternfly/react-core/dist/esm/components/Modal/ModalBody.js';
import { ModalFooter } from '@patternfly/react-core/dist/esm/components/Modal/ModalFooter.js';
import { ModalHeader } from '@patternfly/react-core/dist/esm/components/Modal/ModalHeader.js';
import { TextArea } from '@patternfly/react-core/dist/esm/components/TextArea/index.js';

import { GenericTable, type GenericTableColumn } from './generic-table';
import { NodeActionsCell, NodeCommandStatus } from './node-command-ui';
import { type NodeResource, useNodes } from './node-provider';

type NodeAction = 'DRAIN' | 'RESUME';

interface ConfirmationDialog {
    nodeName: string;
    action: NodeAction;
}

function formatMemory(value: number | undefined): string {
    return value === undefined ? '—' : `${value} MiB`;
}

function formatNodeState(node: NodeResource): string {
    const states = [node.status.state, ...(node.status.stateFlags ?? [])].filter(Boolean);
    return states.length > 0 ? states.join('+') : 'Unknown';
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

export const NodeTable = () => {
    const {
        nodes,
        loading,
        error,
        connected,
        capabilities,
        capabilitiesLoading,
        capabilitiesError,
        commandStates,
        refreshCapabilities,
        drainNode,
        resumeNode,
    } = useNodes();
    const [dialog, setDialog] = useState<ConfirmationDialog | null>(null);
    const [drainReason, setDrainReason] = useState('');
    const [submitting, setSubmitting] = useState(false);
    const [submissionError, setSubmissionError] = useState<string | null>(null);

    const closeDialog = () => {
        if (submitting) {
            return;
        }
        setDialog(null);
        setDrainReason('');
        setSubmissionError(null);
    };

    const submitDialog = async () => {
        if (!dialog) {
            return;
        }
        setSubmitting(true);
        setSubmissionError(null);
        try {
            if (dialog.action === 'DRAIN') {
                await drainNode(dialog.nodeName, drainReason);
            } else {
                await resumeNode(dialog.nodeName);
            }
            setDialog(null);
            setDrainReason('');
        } catch (cause) {
            setSubmissionError(cause instanceof Error ? cause.message : String(cause));
        } finally {
            setSubmitting(false);
        }
    };

    const reasonValid = drainReason.trim() !== '' &&
        new TextEncoder().encode(drainReason).length <= 256 &&
        !hasControlCharacters(drainReason);

    const columns: GenericTableColumn<NodeResource>[] = [
        { key: 'name', header: 'Node', render: node => node.metadata.name },
        { key: 'state', header: 'State', render: formatNodeState },
        { key: 'reason', header: 'Reason', render: node => node.status.reason || '—' },
        { key: 'cpu-load', header: 'CPU Load', render: node => node.spec.cpuLoad ?? '—' },
        { key: 'real-memory', header: 'Real Memory', render: node => formatMemory(node.spec.realMemory) },
        { key: 'allocated-memory', header: 'Allocated Memory', render: node => formatMemory(node.spec.allocMemory) },
        { key: 'free-memory', header: 'Free Memory', render: node => formatMemory(node.spec.freeMemory) },
        {
            key: 'command-status',
            header: 'Command',
            render: node => {
                const command = commandStates[node.spec.nodeName];
                if (!command) {
                    return '—';
                }
                return <NodeCommandStatus command={ command } />;
            },
        },
        {
            key: 'actions',
            header: '',
            screenReaderText: 'Node actions',
            isAction: true,
            render: node => {
                const nodeName = node.spec.nodeName;
                const command = commandStates[nodeName];
                return (
                    <NodeActionsCell
                    node={ node }
                    capabilities={ capabilities }
                    capabilitiesLoading={ capabilitiesLoading }
                    connected={ connected }
                    command={ command }
                    onDrain={ () => {
                        setDrainReason('');
                        setSubmissionError(null);
                        setDialog({ nodeName, action: 'DRAIN' });
                    } }
                    onResume={ () => {
                        setSubmissionError(null);
                        setDialog({ nodeName, action: 'RESUME' });
                    } }
                    />
                );
            },
        },
    ];

    return (
        <>
            { capabilitiesError && (
                <Alert isInline variant='warning' title='Node command capabilities unavailable'>
                    { capabilitiesError.message }
                    <Button variant='link' onClick={ refreshCapabilities } isDisabled={ capabilitiesLoading }>
                        Retry
                    </Button>
                </Alert>
            ) }
            <GenericTable
                items={ nodes }
                columns={ columns }
                rowKey={ node => node.metadata.name }
                ariaLabel='Slurm nodes'
                loading={ loading }
                error={ error }
                emptyMessage='The backend returned an empty Node snapshot.'
            />
            <Modal isOpen={ dialog !== null } onClose={ closeDialog } variant='small'>
                <ModalHeader title={ dialog?.action === 'DRAIN' ? 'Drain node' : 'Resume node' } />
                <ModalBody>
                    { dialog ? <p>Confirm { dialog.action.toLowerCase() } for node { dialog.nodeName }.</p> : null }
                    { dialog?.action === 'DRAIN'
                        ? (
                            <FormGroup label='Reason' isRequired fieldId='node-drain-reason'>
                                <TextArea
                                    id='node-drain-reason'
                                    value={ drainReason }
                                    onChange={ (_event, value) => setDrainReason(value) }
                                    isRequired
                                    maxLength={ 256 }
                                    aria-label='Drain reason'
                                />
                            </FormGroup>
                        )
                        : null }
                    { submissionError
                        ? (
                            <Alert isInline variant='danger' title='Command was not confirmed'>
                                { submissionError }
                            </Alert>
                        )
                        : null }
                </ModalBody>
                <ModalFooter>
                    <Button
                        variant='primary'
                        onClick={ submitDialog }
                        isDisabled={ submitting || (dialog?.action === 'DRAIN' && !reasonValid) }
                    >
                        { submitting ? 'Submitting…' : dialog?.action === 'DRAIN' ? 'Drain node' : 'Resume node' }
                    </Button>
                    <Button variant='link' onClick={ closeDialog } isDisabled={ submitting }>Cancel</Button>
                </ModalFooter>
            </Modal>
        </>
    );
};

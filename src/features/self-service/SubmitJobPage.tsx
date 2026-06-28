import React, { useMemo, useState } from 'react';
import {
    Alert,
    Button,
    Card,
    CardBody,
    CardTitle,
    Checkbox,
    FileUpload,
    Form,
    FormGroup,
    FormSelect,
    FormSelectOption,
    Progress,
    TextInput,
    Wizard,
    WizardStep,
} from '@patternfly/react-core';

import cockpit from 'cockpit';

import { getCurrentUserName } from '../../lib/cockpit/session';

const _ = cockpit.gettext;

type SubmissionState = {
    scriptText: string;
    scriptName: string;
    partition: string;
    qos: string;
    nodes: string;
    cpus: string;
    memory: string;
    email: string;
    emailOnFinish: boolean;
};

const INITIAL_STATE: SubmissionState = {
    scriptText: '#!/bin/bash\n#SBATCH --job-name=demo\n',
    scriptName: 'job.sh',
    partition: 'compute',
    qos: 'normal',
    nodes: '1',
    cpus: '32',
    memory: '64G',
    email: '',
    emailOnFinish: true,
};

export const SubmitJobPage = () => {
    const user = getCurrentUserName();
    const [state, setState] = useState<SubmissionState>(INITIAL_STATE);
    const [submitted, setSubmitted] = useState(false);
    const [submissionId] = useState(() => `sub-${Date.now().toString(36)}`);

    const reviewText = useMemo(() => [
        `User: ${user}`,
        `Partition: ${state.partition}`,
        `QOS: ${state.qos}`,
        `Nodes: ${state.nodes}`,
        `CPUs: ${state.cpus}`,
        `Memory: ${state.memory}`,
        `Email: ${state.email || 'n/a'}`,
    ].join('\n'), [state, user]);

    return (
        <Card>
            <CardTitle>{_('Submit job')}</CardTitle>
            <CardBody>
                {submitted && (
                    <Alert variant="success" title={_('Job submission prepared')}>
                        {cockpit.format(_('Submission $0 is ready to send to the bridge.'), submissionId)}
                    </Alert>
                )}

                <Wizard header={_('Job submission wizard')} isProgressive startIndex={1} onSave={() => setSubmitted(true)} onClose={() => setSubmitted(false)}>
                    <WizardStep id="script" name={_('Select script')}>
                        <Form isHorizontal>
                            <FormGroup label={_('Script name')} isRequired fieldId="script-name">
                                <TextInput id="script-name" value={state.scriptName} onChange={(_event, value) => setState((current) => ({ ...current, scriptName: value }))} />
                            </FormGroup>
                            <FormGroup label={_('Script contents')} isRequired fieldId="script-text">
                                <FileUpload
                                    id="submit-script-upload"
                                    type="text"
                                    allowEditingUploadedText
                                    filename={state.scriptName}
                                    value={state.scriptText}
                                    onTextChange={(_event, text) => setState((current) => ({ ...current, scriptText: text }))}
                                    onFileInputChange={(_event, file) => setState((current) => ({ ...current, scriptName: file.name }))}
                                    onClearClick={() => setState((current) => ({ ...current, scriptText: '', scriptName: '' }))}
                                />
                            </FormGroup>
                        </Form>
                    </WizardStep>

                    <WizardStep id="resources" name={_('Resources')}>
                        <Form isHorizontal>
                            <FormGroup label={_('Partition')} fieldId="partition">
                                <FormSelect id="partition" value={state.partition} onChange={(_event, value) => setState((current) => ({ ...current, partition: value }))}>
                                    <FormSelectOption value="compute" label={_('compute')} />
                                    <FormSelectOption value="gpu" label={_('gpu')} />
                                    <FormSelectOption value="debug" label={_('debug')} />
                                </FormSelect>
                            </FormGroup>
                            <FormGroup label={_('QOS')} fieldId="qos">
                                <FormSelect id="qos" value={state.qos} onChange={(_event, value) => setState((current) => ({ ...current, qos: value }))}>
                                    <FormSelectOption value="normal" label={_('normal')} />
                                    <FormSelectOption value="high" label={_('high')} />
                                    <FormSelectOption value="debug" label={_('debug')} />
                                </FormSelect>
                            </FormGroup>
                            <FormGroup label={_('Nodes')} fieldId="nodes">
                                <TextInput id="nodes" value={state.nodes} onChange={(_event, value) => setState((current) => ({ ...current, nodes: value }))} />
                            </FormGroup>
                            <FormGroup label={_('CPUs')} fieldId="cpus">
                                <TextInput id="cpus" value={state.cpus} onChange={(_event, value) => setState((current) => ({ ...current, cpus: value }))} />
                            </FormGroup>
                            <FormGroup label={_('Memory')} fieldId="memory">
                                <TextInput id="memory" value={state.memory} onChange={(_event, value) => setState((current) => ({ ...current, memory: value }))} />
                            </FormGroup>
                        </Form>
                    </WizardStep>

                    <WizardStep id="environment" name={_('Environment')}>
                        <Form isHorizontal>
                            <FormGroup label={_('E-mail')} fieldId="email">
                                <TextInput id="email" value={state.email} onChange={(_event, value) => setState((current) => ({ ...current, email: value }))} />
                            </FormGroup>
                            <FormGroup fieldId="email-on-finish">
                                <Checkbox
                                    id="email-on-finish"
                                    label={_('Send notification on completion')}
                                    isChecked={state.emailOnFinish}
                                    onChange={(_event, checked) => setState((current) => ({ ...current, emailOnFinish: checked }))}
                                />
                            </FormGroup>
                        </Form>
                    </WizardStep>

                    <WizardStep id="review" name={_('Review')}>
                        <Card isPlain>
                            <CardTitle>{_('Review submission')}</CardTitle>
                            <CardBody>
                                <pre style={{ whiteSpace: 'pre-wrap' }}>{reviewText}</pre>
                                <Progress value={100} title={_('Ready to submit')} label={_('Ready')} aria-label={_('Ready to submit')} />
                                <Button variant="primary" onClick={() => setSubmitted(true)}>{_('Submit job')}</Button>
                            </CardBody>
                        </Card>
                    </WizardStep>
                </Wizard>
            </CardBody>
        </Card>
    );
};

import React from 'react';
import { Alert, Bullseye, Spinner } from '@patternfly/react-core';

import cockpit from 'cockpit';

const _ = cockpit.gettext;

type LoadingStateProps = {
    title: string;
    message?: string;
};

export const LoadingState = ({ title, message }: LoadingStateProps) => {
    return (
        <Bullseye>
            <Alert variant="info" title={title} isInline>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                    <Spinner size="md" />
                    <span>{message ?? _('Loading...')}</span>
                </div>
            </Alert>
        </Bullseye>
    );
};

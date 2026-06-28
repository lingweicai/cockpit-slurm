import React from 'react';
import { Alert, Bullseye, Button } from '@patternfly/react-core';

import cockpit from 'cockpit';

const _ = cockpit.gettext;

type ErrorStateProps = {
    title: string;
    message: string;
    onRetry?: () => void;
};

export const ErrorState = ({ title, message, onRetry }: ErrorStateProps) => {
    return (
        <Bullseye>
            <Alert
                variant="danger"
                title={title}
                action={
                    onRetry
                        ? (
                            <Button variant="secondary" onClick={onRetry}>
                                {_('Retry')}
                            </Button>
                        )
                        : undefined
                }
                isInline
            >
                {message}
            </Alert>
        </Bullseye>
    );
};

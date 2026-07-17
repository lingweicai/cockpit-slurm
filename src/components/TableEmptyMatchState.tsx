import React from 'react';
import { Alert } from '@patternfly/react-core';

import { EmptyState } from './EmptyState';

type TableEmptyMatchStateProps = {
    title: string;
    message?: string;
};

export const TableEmptyMatchState = ({ title, message }: TableEmptyMatchStateProps) => {
    if (message) {
        return <EmptyState title={title} message={message} />;
    }

    return <Alert variant="info" title={title} />;
};

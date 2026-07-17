import React from 'react';
import { Alert } from '@patternfly/react-core';

import type { TransientAlert } from '../hooks/useTransientAlert';

type ActionFeedbackAlertProps = {
    alert: TransientAlert | null;
};

export const ActionFeedbackAlert = ({ alert }: ActionFeedbackAlertProps) => {
    if (!alert) {
        return null;
    }

    return <Alert isInline variant={alert.variant} title={alert.title} />;
};

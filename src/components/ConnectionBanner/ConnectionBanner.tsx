import React from 'react';
import { Alert } from '@patternfly/react-core';

import cockpit from 'cockpit';

import { useChannelContext } from '../../lib/cockpit/provider';

const _ = cockpit.gettext;

function formatConnectionLabel(status: string) {
  switch (status) {
    case 'ready':
      return _('Connected to bridge');
    case 'connecting':
      return _('Connecting to bridge');
    case 'closed':
      return _('Bridge connection closed');
    case 'error':
      return _('Bridge connection error');
    default:
      return status;
  }
}

export const ConnectionBanner = () => {
  const { status, error, lastConnectedAt, lastMessageAt } = useChannelContext();

  const title = formatConnectionLabel(status);
  const details = [
    lastConnectedAt ? cockpit.format(_('Connected at $0'), new Date(lastConnectedAt).toLocaleString()) : null,
    lastMessageAt ? cockpit.format(_('Last message at $0'), new Date(lastMessageAt).toLocaleString()) : null,
    error ? cockpit.format(_('Error: $0'), error) : null,
  ].filter((value): value is string => Boolean(value));

  if (status === 'ready' && !error) {
    return (
      <Alert variant="success" isInline title={title}>
        {details.join(' · ')}
      </Alert>
    );
  }

  const variant = status === 'error' || status === 'closed' ? 'danger' : 'info';

  return (
    <Alert variant={variant} isInline title={title}>
      {details.join(' · ') || _('Waiting for bridge data...')}
    </Alert>
  );
};

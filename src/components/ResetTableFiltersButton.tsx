import React from 'react';
import { Button } from '@patternfly/react-core';

import cockpit from 'cockpit';

const _ = cockpit.gettext;

type ResetTableFiltersButtonProps = {
    onReset: () => void;
};

export const ResetTableFiltersButton = ({ onReset }: ResetTableFiltersButtonProps) => {
    return (
        <Button variant="link" onClick={onReset}>
            {_('Reset filters')}
        </Button>
    );
};

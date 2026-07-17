import React from 'react';
import { Button, Card, CardBody, CardTitle, DrawerPanelBody, DrawerPanelContent } from '@patternfly/react-core';

import cockpit from 'cockpit';

const _ = cockpit.gettext;

type EntityDrawerProps = {
    title: string;
    onClose: () => void;
    children: React.ReactNode;
};

export const EntityDrawer = ({ title, onClose, children }: EntityDrawerProps) => {
    return (
        <DrawerPanelContent minSize="320px" defaultSize="40%">
            <DrawerPanelBody>
                <Card isPlain>
                    <CardTitle>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '0.75rem' }}>
                            <span>{title}</span>
                            <Button variant="link" onClick={onClose}>
                                {_('Close')}
                            </Button>
                        </div>
                    </CardTitle>
                    <CardBody>{children}</CardBody>
                </Card>
            </DrawerPanelBody>
        </DrawerPanelContent>
    );
};

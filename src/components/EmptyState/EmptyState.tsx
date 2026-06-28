import React from 'react';
import { Bullseye, Card, CardBody } from '@patternfly/react-core';

type EmptyStateProps = {
    title: string;
    message: string;
};

export const EmptyState = ({ title, message }: EmptyStateProps) => {
    return (
        <Bullseye>
            <Card isPlain>
                <CardBody>
                    <strong>{title}</strong>
                    <div>{message}</div>
                </CardBody>
            </Card>
        </Bullseye>
    );
};

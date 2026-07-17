import React from 'react';
import { Card, CardBody, CardTitle, Gallery, GalleryItem } from '@patternfly/react-core';

export type SummaryMetric = {
    title: React.ReactNode;
    value: React.ReactNode;
    description?: React.ReactNode;
};

type SummaryMetricsGalleryProps = {
    metrics: SummaryMetric[];
};

export const SummaryMetricsGallery = ({ metrics }: SummaryMetricsGalleryProps) => {
    return (
        <Gallery hasGutter>
            {metrics.map((metric) => (
                <GalleryItem key={String(metric.title)}>
                    <Card>
                        <CardTitle>{metric.title}</CardTitle>
                        <CardBody>
                            <strong>{metric.value}</strong>
                            {metric.description && <div>{metric.description}</div>}
                        </CardBody>
                    </Card>
                </GalleryItem>
            ))}
        </Gallery>
    );
};

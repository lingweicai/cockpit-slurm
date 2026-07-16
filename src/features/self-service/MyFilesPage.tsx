import React, { useState } from 'react';
import { Alert, Card, CardBody, CardTitle, FileUpload } from '@patternfly/react-core';

import cockpit from 'cockpit';

import { EntityTable, type EntityTableColumn } from '../../components/EntityTable';
import { FILES_FIXTURES, type FileRecord } from './selfServiceData';

const _ = cockpit.gettext;

export const MyFilesPage = () => {
    const [selectedFile, setSelectedFile] = useState('');
    const [uploadText, setUploadText] = useState('#!/bin/bash\n#SBATCH --job-name=example\n');

    const columns: EntityTableColumn<FileRecord>[] = [
        {
            header: _('Path'),
            dataLabel: _('Path'),
            cell: (file) => file.path,
        },
        {
            header: _('Type'),
            dataLabel: _('Type'),
            cell: (file) => file.kind,
        },
        {
            header: _('Size'),
            dataLabel: _('Size'),
            cell: (file) => file.size,
        },
        {
            header: _('Modified'),
            dataLabel: _('Modified'),
            cell: (file) => new Date(file.modifiedAt).toLocaleString(),
        },
    ];

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <Card>
                <CardTitle>{_('Upload a file')}</CardTitle>
                <CardBody>
                    <FileUpload
                        id="self-service-file-upload"
                        type="text"
                        allowEditingUploadedText
                        browseButtonText={_('Browse')}
                        clearButtonText={_('Clear')}
                        filename={selectedFile || undefined}
                        value={uploadText}
                        aria-label={_('Upload a file')}
                        onTextChange={(_event, text) => setUploadText(text)}
                        onFileInputChange={(_event, file) => setSelectedFile(file.name)}
                        onClearClick={() => {
                            setSelectedFile('');
                            setUploadText('');
                        }}
                    />
                </CardBody>
            </Card>

            <Card>
                <CardTitle>{_('My files')}</CardTitle>
                <CardBody>
                    <EntityTable
                        ariaLabel={_('My files table')}
                        columns={columns}
                        rows={FILES_FIXTURES}
                        rowKey={(file) => file.path}
                        emptyState={<Alert variant="info" title={_('No files are currently tracked.')} />}
                    />
                </CardBody>
            </Card>
        </div>
    );
};

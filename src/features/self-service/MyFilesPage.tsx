import React, { useState } from 'react';
import { Alert, Card, CardBody, CardTitle, FileUpload } from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';

import cockpit from 'cockpit';

import { FILES_FIXTURES } from './selfServiceData';

const _ = cockpit.gettext;

export const MyFilesPage = () => {
    const [selectedFile, setSelectedFile] = useState('');
    const [uploadText, setUploadText] = useState('#!/bin/bash\n#SBATCH --job-name=example\n');

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
                    {FILES_FIXTURES.length === 0 && (
                        <Alert variant="info" title={_('No files are currently tracked.')} />
                    )}
                    {FILES_FIXTURES.length > 0 && (
                        <Table aria-label={_('My files table')} variant="compact">
                            <Thead>
                                <Tr>
                                    <Th>{_('Path')}</Th>
                                    <Th>{_('Type')}</Th>
                                    <Th>{_('Size')}</Th>
                                    <Th>{_('Modified')}</Th>
                                </Tr>
                            </Thead>
                            <Tbody>
                                {FILES_FIXTURES.map((file) => (
                                    <Tr key={file.path}>
                                        <Td dataLabel={_('Path')}>{file.path}</Td>
                                        <Td dataLabel={_('Type')}>{file.kind}</Td>
                                        <Td dataLabel={_('Size')}>{file.size}</Td>
                                        <Td dataLabel={_('Modified')}>{new Date(file.modifiedAt).toLocaleString()}</Td>
                                    </Tr>
                                ))}
                            </Tbody>
                        </Table>
                    )}
                </CardBody>
            </Card>
        </div>
    );
};

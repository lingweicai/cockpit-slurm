import React from 'react';
import { Alert } from '@patternfly/react-core/dist/esm/components/Alert/index.js';
import { Spinner } from '@patternfly/react-core/dist/esm/components/Spinner/index.js';
import {
    InnerScrollContainer,
    OuterScrollContainer,
    Table,
    Tbody,
    Td,
    Th,
    Thead,
    Tr,
} from '@patternfly/react-table';

export interface GenericTableColumn<T> {
    key: string;
    header: React.ReactNode;
    render: (item: T) => React.ReactNode;
}

export interface GenericTableProps<T> {
    items: T[];
    columns: GenericTableColumn<T>[];
    rowKey: (item: T) => React.Key;
    ariaLabel: string;
    loading?: boolean;
    error?: Error | null;
    emptyMessage?: React.ReactNode;
}

export function GenericTable<T>({
    items,
    columns,
    rowKey,
    ariaLabel,
    loading = false,
    error = null,
    emptyMessage = 'No data found',
}: GenericTableProps<T>) {
    if (loading && items.length === 0) {
        return <Alert variant='info' title='Loading'><Spinner size='md' aria-label='Loading' /></Alert>;
    }

    if (error && items.length === 0) {
        return <Alert variant='danger' title='Unable to load data'>{ error.message }</Alert>;
    }

    if (items.length === 0) {
        return <Alert variant='info' title='No data'>{ emptyMessage }</Alert>;
    }

    return (
        <OuterScrollContainer>
            <InnerScrollContainer>
                <Table aria-label={ ariaLabel } variant='compact'>
                    <Thead>
                        <Tr>
                            { columns.map(column => <Th key={ column.key }>{ column.header }</Th>) }
                        </Tr>
                    </Thead>
                    <Tbody>
                        { items.map(item => (
                            <Tr key={ rowKey(item) }>
                                { columns.map(column => {
                                    const dataLabel = typeof column.header === 'string' ? column.header : undefined;
                                    const dataLabelProps = dataLabel === undefined ? {} : { dataLabel };
                                    return <Td key={ column.key } { ...dataLabelProps }>{ column.render(item) }</Td>;
                                }) }
                            </Tr>
                        )) }
                    </Tbody>
                </Table>
            </InnerScrollContainer>
        </OuterScrollContainer>
    );
}
import type { operations } from './slurm-openapi.gen';

type Json200Response<TOperation extends { responses: { 200: { content: { 'application/json': unknown } } } }> =
    TOperation['responses'][200]['content']['application/json'];

type QueryParams<TOperation extends { parameters: { query?: unknown } }> =
    TOperation['parameters']['query'];

export type SlurmGetJobsOperation = operations['slurm_v0043_get_jobs'];
export type SlurmJobsQuery = QueryParams<SlurmGetJobsOperation>;
export type SlurmJobsResponse = Json200Response<SlurmGetJobsOperation>;
export type SlurmJob = SlurmJobsResponse['jobs'][number];

export type SlurmGetNodesOperation = operations['slurm_v0043_get_nodes'];
export type SlurmNodesQuery = QueryParams<SlurmGetNodesOperation>;
export type SlurmNodesResponse = Json200Response<SlurmGetNodesOperation>;
export type SlurmNode = SlurmNodesResponse['nodes'][number];

export type SlurmGetPartitionsOperation = operations['slurm_v0043_get_partitions'];
export type SlurmPartitionsQuery = QueryParams<SlurmGetPartitionsOperation>;
export type SlurmPartitionsResponse = Json200Response<SlurmGetPartitionsOperation>;
export type SlurmPartition = SlurmPartitionsResponse['partitions'][number];

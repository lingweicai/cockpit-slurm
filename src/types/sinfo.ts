export interface SinfoPartitionRow {
  partitionName: string;
  availability: string;
  timeLimit: string;
  nodesTotal: number;
  nodesAllocated: number;
  nodesIdle: number;
  nodesOther: number;
  nodeState: string;
  nodeList: string;
  cpusTotal: number;
  cpusAllocated: number;
  cpusIdle: number;
  cpusOther: number;
  memoryFreeMin: number;
  memoryFreeMax: number;
  memoryAllocated: number;
  reasonDescription: string;
  reasonUser: string;
  reasonTime: number;
  comment: string;
  reservation: string;
  partitionState: string[];
  partitionTRES: string;
  featuresActive: string;
  gresUsed: string;
  raw?: unknown;
}

export interface SinfoResponse {
  rows: SinfoPartitionRow[];
  updatedAt: string;
}

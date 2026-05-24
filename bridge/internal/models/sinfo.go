package models

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type SinfoResponse struct {
    Sinfo    []SinfoRecord `json:"sinfo"`
    Meta     SinfoMeta     `json:"meta"`
    Errors   []string      `json:"errors"`
    Warnings []string      `json:"warnings"`
}

type SinfoMeta struct {
    Plugin  SinfoMetaPlugin `json:"plugin"`
    Client  SinfoMetaClient `json:"client"`
    Command []string        `json:"command"`
    Slurm   SinfoMetaSlurm  `json:"slurm"`
}

type SinfoMetaPlugin struct {
    Type              string `json:"type"`
    Name              string `json:"name"`
    DataParser        string `json:"data_parser"`
    AccountingStorage string `json:"accounting_storage"`
}

type SinfoMetaClient struct {
    Source string `json:"source"`
    User   string `json:"user"`
    Group  string `json:"group"`
}

type SinfoMetaSlurm struct {
    Version SinfoSlurmVersion `json:"version"`
    Release string            `json:"release"`
    Cluster string            `json:"cluster"`
}

type SinfoSlurmVersion struct {
    Major string `json:"major"`
    Minor string `json:"minor"`
    Micro string `json:"micro"`
}

type SinfoRecord struct {
    Port        int                    `json:"port"`
    Node        SinfoNode              `json:"node"`
    Nodes       SinfoCountSummary      `json:"nodes"`
    CPUs        SinfoCountSummary      `json:"cpus"`
    Sockets     SinfoValueRange        `json:"sockets"`
    Cores       SinfoValueRange        `json:"cores"`
    Threads     SinfoValueRange        `json:"threads"`
    Disk        SinfoValueRange        `json:"disk"`
    Memory      SinfoMemory            `json:"memory"`
    Weight      SinfoValueRange        `json:"weight"`
    Features    SinfoStringPair        `json:"features"`
    Gres        SinfoStringPair        `json:"gres"`
    Cluster     string                 `json:"cluster"`
    Comment     string                 `json:"comment"`
    Extra       string                 `json:"extra"`
    Reason      SinfoReason            `json:"reason"`
    Reservation string                 `json:"reservation"`
    Partition   SinfoPartitionDetail   `json:"partition"`
}

type SinfoNode struct {
    State []string `json:"state"`
}

type SinfoCountSummary struct {
    Allocated int      `json:"allocated"`
    Idle      int      `json:"idle"`
    Other     int      `json:"other"`
    Total     int      `json:"total"`
    Hostnames []string `json:"hostnames,omitempty"`
    Addresses []string `json:"addresses,omitempty"`
    Nodes     []string `json:"nodes,omitempty"`
    Minimum   int      `json:"minimum,omitempty"`
    Maximum   int      `json:"maximum,omitempty"`
    Load      SinfoLoad `json:"load,omitempty"`
    PerNode   SinfoMax  `json:"per_node,omitempty"`
    TaskBinding int    `json:"task_binding,omitempty"`
}

type SinfoLoad struct {
    Minimum int `json:"minimum"`
    Maximum int `json:"maximum"`
}

type SinfoMax struct {
    Set      bool `json:"set"`
    Infinite bool `json:"infinite"`
    Number   int  `json:"number"`
}

type SinfoValueRange struct {
    Minimum int `json:"minimum"`
    Maximum int `json:"maximum"`
}

type SinfoValue struct {
    Set      bool  `json:"set"`
    Infinite bool  `json:"infinite"`
    Number   int64 `json:"number"`
}

type SinfoMemory struct {
    Minimum   int         `json:"minimum"`
    Maximum   int         `json:"maximum"`
    Free      SinfoFree   `json:"free"`
    Allocated int         `json:"allocated"`
}

type SinfoFree struct {
    Minimum SinfoValue `json:"minimum"`
    Maximum SinfoValue `json:"maximum"`
}

type SinfoStringPair struct {
    Total  string `json:"total"`
    Active string `json:"active"`
    Used   string `json:"used,omitempty"`
}

type SinfoReason struct {
    Description string `json:"description"`
    Time        int64  `json:"time"`
    User        string `json:"user"`
}

type SinfoPartitionDetail struct {
    Nodes        SinfoPartitionNodes      `json:"nodes"`
    Accounts     SinfoPartitionActors     `json:"accounts"`
    Groups       SinfoPartitionActors     `json:"groups"`
    QOS          SinfoPartitionQOS        `json:"qos"`
    Alternate    string                   `json:"alternate"`
    Tres         SinfoPartitionTRES       `json:"tres"`
    Cluster      string                   `json:"cluster"`
    CPUs         SinfoPartitionCPUs       `json:"cpus"`
    Defaults     SinfoPartitionDefaults   `json:"defaults"`
    GraceTime    int                      `json:"grace_time"`
    Maximums     SinfoPartitionMaximums   `json:"maximums"`
    Minimums     SinfoPartitionMinimums   `json:"minimums"`
    Name         string                   `json:"name"`
    NodeSets     string                   `json:"node_sets"`
    Priority     SinfoPartitionPriority   `json:"priority"`
    Timeouts     SinfoPartitionTimeouts   `json:"timeouts"`
    Partition    SinfoPartitionState      `json:"partition"`
    SuspendTime  SinfoValue               `json:"suspend_time"`
}

type SinfoPartitionNodes struct {
    AllowedAllocation string `json:"allowed_allocation"`
    Configured        string `json:"configured"`
    Total             int    `json:"total"`
}

type SinfoPartitionActors struct {
    Allowed string `json:"allowed"`
    Deny    string `json:"deny"`
}

type SinfoPartitionQOS struct {
    Allowed  string `json:"allowed"`
    Deny     string `json:"deny"`
    Assigned string `json:"assigned"`
}

type SinfoPartitionTRES struct {
    BillingWeights string `json:"billing_weights"`
    Configured     string `json:"configured"`
}

type SinfoPartitionCPUs struct {
    TaskBinding int `json:"task_binding"`
    Total       int `json:"total"`
}

type SinfoPartitionDefaults struct {
    MemoryPerCPU              int        `json:"memory_per_cpu"`
    PartitionMemoryPerCPU     SinfoValue `json:"partition_memory_per_cpu"`
    PartitionMemoryPerNode    SinfoValue `json:"partition_memory_per_node"`
    Time                      SinfoValue `json:"time"`
    Job                       string     `json:"job"`
}

type SinfoPartitionMaximums struct {
    CPUsPerNode          SinfoValue         `json:"cpus_per_node"`
    CPUsPerSocket        SinfoValue         `json:"cpus_per_socket"`
    MemoryPerCPU         int                `json:"memory_per_cpu"`
    PartitionMemoryPerCPU SinfoValue        `json:"partition_memory_per_cpu"`
    PartitionMemoryPerNode SinfoValue       `json:"partition_memory_per_node"`
    Nodes                SinfoValue         `json:"nodes"`
    Shares               int                `json:"shares"`
    Oversubscribe        SinfoOversubscribe `json:"oversubscribe"`
    Time                 SinfoValue         `json:"time"`
    OverTimeLimit        SinfoValue         `json:"over_time_limit"`
}

type SinfoOversubscribe struct {
    Jobs  int      `json:"jobs"`
    Flags []string `json:"flags"`
}

type SinfoPartitionMinimums struct {
    Nodes int `json:"nodes"`
}

type SinfoPartitionPriority struct {
    JobFactor int `json:"job_factor"`
    Tier      int `json:"tier"`
}

type SinfoPartitionTimeouts struct {
    Resume  SinfoValue `json:"resume"`
    Suspend SinfoValue `json:"suspend"`
}

type SinfoPartitionState struct {
    State []string `json:"state"`
}

type SinfoPartitionRow struct {
    PartitionName      string         `json:"partition_name"`
    Availability       string         `json:"availability"`
    TimeLimit          string         `json:"time_limit"`
    NodesTotal         int            `json:"nodes_total"`
    NodesAllocated     int            `json:"nodes_allocated"`
    NodesIdle          int            `json:"nodes_idle"`
    NodesOther         int            `json:"nodes_other"`
    NodeState          string         `json:"node_state"`
    NodeList           string         `json:"node_list"`
    CPUsTotal          int            `json:"cpus_total"`
    CPUsAllocated      int            `json:"cpus_allocated"`
    CPUsIdle           int            `json:"cpus_idle"`
    CPUsOther          int            `json:"cpus_other"`
    MemoryFreeMin      int64          `json:"memory_free_min"`
    MemoryFreeMax      int64          `json:"memory_free_max"`
    MemoryAllocated    int            `json:"memory_allocated"`
    ReasonDescription  string         `json:"reason_description"`
    ReasonUser         string         `json:"reason_user"`
    ReasonTime         int64          `json:"reason_time"`
    Comment            string         `json:"comment"`
    Reservation        string         `json:"reservation"`
    PartitionState     []string       `json:"partition_state"`
    PartitionTRES      string         `json:"partition_tres"`
    FeaturesActive     string         `json:"features_active"`
    GresUsed           string         `json:"gres_used"`
    Raw                *SinfoRecord   `json:"raw,omitempty"`
}

type SinfoCache struct {
    mu        sync.RWMutex        `json:"-"`
    UpdatedAt time.Time          `json:"updated_at"`
    Rows      []SinfoPartitionRow `json:"rows"`
}

func NewSinfoPartitionRow(record *SinfoRecord) SinfoPartitionRow {
    return SinfoPartitionRow{
        PartitionName:     record.Partition.Name,
        Availability:      formatAvailability(record.Partition.Partition.State),
        TimeLimit:         formatTimeLimit(record.Partition.Maximums.Time),
        NodesTotal:        record.Nodes.Total,
        NodesAllocated:    record.Nodes.Allocated,
        NodesIdle:         record.Nodes.Idle,
        NodesOther:        record.Nodes.Other,
        NodeState:         strings.Join(record.Node.State, ","),
        NodeList:          record.Partition.Nodes.Configured,
        CPUsTotal:         record.CPUs.Total,
        CPUsAllocated:     record.CPUs.Allocated,
        CPUsIdle:          record.CPUs.Idle,
        CPUsOther:         record.CPUs.Other,
        MemoryFreeMin:     record.Memory.Free.Minimum.Number,
        MemoryFreeMax:     record.Memory.Free.Maximum.Number,
        MemoryAllocated:   record.Memory.Allocated,
        ReasonDescription: record.Reason.Description,
        ReasonUser:        record.Reason.User,
        ReasonTime:        record.Reason.Time,
        Comment:           record.Comment,
        Reservation:       record.Reservation,
        PartitionState:    record.Partition.Partition.State,
        PartitionTRES:     record.Partition.Tres.Configured,
        FeaturesActive:    record.Features.Active,
        GresUsed:          record.Gres.Used,
        Raw:               record,
    }
}

func formatAvailability(states []string) string {
    if len(states) == 0 {
        return "UNKNOWN"
    }
    return strings.Join(states, ",")
}

func formatTimeLimit(value SinfoValue) string {
    if value.Infinite {
        return "UNLIMITED"
    }
    if !value.Set {
        return ""
    }
    return formatMinutesAsSlurmTime(int(value.Number))
}

func formatMinutesAsSlurmTime(minutes int) string {
    if minutes < 0 {
        return ""
    }
    days := minutes / (24 * 60)
    hours := (minutes % (24 * 60)) / 60
    mins := minutes % 60
    if days > 0 {
        return fmt.Sprintf("%d-%02d:%02d:00", days, hours, mins)
    }
    return fmt.Sprintf("%02d:%02d:00", hours, mins)
}

func (c *SinfoCache) Set(rows []SinfoPartitionRow) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.Rows = rows
    c.UpdatedAt = time.Now().UTC()
}

func (c *SinfoCache) Get() ([]SinfoPartitionRow, time.Time) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return append([]SinfoPartitionRow(nil), c.Rows...), c.UpdatedAt
}

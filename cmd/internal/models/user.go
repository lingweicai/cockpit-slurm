package models

import "encoding/json"

// User represents v0.0.45_user from the Slurm REST schema.
type User struct {
	AdministratorLevel []string          `json:"administrator_level,omitempty"`
	Associations       []UserAssocShort  `json:"associations,omitempty"`
	Coordinators       []UserCoordinator `json:"coordinators,omitempty"`
	Default            *UserDefault      `json:"default,omitempty"`
	Flags              []string          `json:"flags,omitempty"`
	Name               string            `json:"name"`
	OldName            string            `json:"old_name,omitempty"`
	Wckeys             []UserWckey       `json:"wckeys,omitempty"`
}

// UserShort represents v0.0.45_user_short.
type UserShort struct {
	AdminLevel     []string `json:"adminlevel,omitempty"`
	DefaultQOS     *int32   `json:"defaultqos,omitempty"`
	DefaultAccount string   `json:"defaultaccount,omitempty"`
	DefaultWckey   string   `json:"defaultwckey,omitempty"`
}

// UsersAddCond represents v0.0.45_users_add_cond.
type UsersAddCond struct {
	Accounts    []string         `json:"accounts,omitempty"`
	Association *UserAssocRecSet `json:"association,omitempty"`
	Clusters    []string         `json:"clusters,omitempty"`
	Partitions  []string         `json:"partitions,omitempty"`
	Users       []string         `json:"users"`
	Wckeys      []string         `json:"wckeys,omitempty"`
}

// UserDefault represents v0_0_45_user_default.
type UserDefault struct {
	QOS     *int32 `json:"qos,omitempty"`
	Account string `json:"account,omitempty"`
	Wckey   string `json:"wckey,omitempty"`
}

// UserAssocShort represents v0.0.45_assoc_short.
type UserAssocShort struct {
	Account   string `json:"account,omitempty"`
	Cluster   string `json:"cluster,omitempty"`
	Partition string `json:"partition,omitempty"`
	User      string `json:"user"`
	ID        *int32 `json:"id,omitempty"`
}

// UserCoordinator represents v0.0.45_coord.
type UserCoordinator struct {
	Name   string `json:"name"`
	Direct *bool  `json:"direct,omitempty"`
}

// UserWckey represents v0.0.45_wckey.
type UserWckey struct {
	Accounting []json.RawMessage `json:"accounting,omitempty"`
	Cluster    string            `json:"cluster"`
	ID         *int32            `json:"id,omitempty"`
	Name       string            `json:"name"`
	User       string            `json:"user"`
	Flags      []string          `json:"flags,omitempty"`
}

// UserAssocRecSet represents v0.0.45_assoc_rec_set.
type UserAssocRecSet struct {
	Comment               string            `json:"comment,omitempty"`
	DefaultQOS            string            `json:"defaultqos,omitempty"`
	GrpJobs               *UserNoValUint32  `json:"grpjobs,omitempty"`
	GrpJobsAccrue         *UserNoValUint32  `json:"grpjobsaccrue,omitempty"`
	GrpSubmitJobs         *UserNoValUint32  `json:"grpsubmitjobs,omitempty"`
	GrpTRES               []json.RawMessage `json:"grptres,omitempty"`
	GrpTRESMins           []json.RawMessage `json:"grptresmins,omitempty"`
	GrpTRESRunMins        []json.RawMessage `json:"grptresrunmins,omitempty"`
	GrpWall               *UserNoValUint32  `json:"grpwall,omitempty"`
	MaxJobs               *UserNoValUint32  `json:"maxjobs,omitempty"`
	MaxJobsAccrue         *UserNoValUint32  `json:"maxjobsaccrue,omitempty"`
	MaxSubmitJobs         *UserNoValUint32  `json:"maxsubmitjobs,omitempty"`
	MaxTRESMinsPerJob     []json.RawMessage `json:"maxtresminsperjob,omitempty"`
	MaxTRESRunMins        []json.RawMessage `json:"maxtresrunmins,omitempty"`
	MaxTRESPerJob         []json.RawMessage `json:"maxtresperjob,omitempty"`
	MaxTRESPerNode        []json.RawMessage `json:"maxtrespernode,omitempty"`
	MaxWallDurationPerJob *UserNoValUint32  `json:"maxwalldurationperjob,omitempty"`
	MinPrioThresh         *UserNoValUint32  `json:"minpriothresh,omitempty"`
	Parent                string            `json:"parent,omitempty"`
	Priority              *UserNoValUint32  `json:"priority,omitempty"`
	QOSLevel              []string          `json:"qoslevel,omitempty"`
	Fairshare             *int32            `json:"fairshare,omitempty"`
}

// UserNoValUint32 represents v0.0.45_uint32_no_val_struct.
type UserNoValUint32 struct {
	Set      bool   `json:"set"`
	Infinite bool   `json:"infinite"`
	Number   uint32 `json:"number"`
}

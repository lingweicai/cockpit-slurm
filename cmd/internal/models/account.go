package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Account represents v0.0.43_account from Slurm REST/OpenAPI and sacctmgr --json.
type Account struct {
	Associations []AccountAssocShort  `json:"associations,omitempty"`
	Coordinators []AccountCoordinator `json:"coordinators,omitempty"`
	Description  string               `json:"description"`
	Flags        []string             `json:"flags,omitempty"`
	Name         string               `json:"name"`
	Organization string               `json:"organization"`
}

// AccountShort represents v0.0.43_account_short.
type AccountShort struct {
	Description  string `json:"description,omitempty"`
	Organization string `json:"organization,omitempty"`
}

// AccountsAddCond represents v0.0.43_accounts_add_cond.
type AccountsAddCond struct {
	Accounts    []string            `json:"accounts"`
	Association *AccountAssocRecSet `json:"association,omitempty"`
	Clusters    []string            `json:"clusters,omitempty"`
}

// AccountAssocShort represents v0.0.43_assoc_short.
type AccountAssocShort struct {
	Account   string `json:"account,omitempty"`
	Cluster   string `json:"cluster,omitempty"`
	Partition string `json:"partition,omitempty"`
	User      string `json:"user"`
	ID        *int32 `json:"id,omitempty"`
}

// AccountCoordinator represents v0.0.43_coord.
type AccountCoordinator struct {
	Name   string `json:"name"`
	Direct *bool  `json:"direct,omitempty"`
}

// AccountAssocRecSet represents v0.0.43_assoc_rec_set.
type AccountAssocRecSet = UserAssocRecSet

// AccountsResponse represents the JSON payload of `sacctmgr list account --json`.
type AccountsResponse struct {
	Accounts []Account             `json:"accounts"`
	Meta     *V0043OpenapiMeta     `json:"meta,omitempty"`
	Errors   []V0043OpenapiError   `json:"errors,omitempty"`
	Warnings []V0043OpenapiWarning `json:"warnings,omitempty"`
}

// AccountEntityMethods maps account operations to Slurm REST method names.
type AccountEntityMethods struct {
	List      string `json:"list"`
	Get       string `json:"get"`
	Delete    string `json:"delete"`
	AddModify string `json:"add_modify"`
}

// AccountEntity provides helpers for account CLI and REST operations.
type AccountEntity struct{}

func NewAccountEntity() AccountEntity {
	return AccountEntity{}
}

func (AccountEntity) RESTMethods() AccountEntityMethods {
	return AccountEntityMethods{
		List:      "slurmdb_v0043_get_accounts",
		Get:       "slurmdb_v0043_get_account",
		Delete:    "slurmdb_v0043_delete_account",
		AddModify: "slurmdb_v0043_post_accounts",
	}
}

func (AccountEntity) ParseListJSON(raw []byte) (*AccountsResponse, error) {
	var resp AccountsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse sacctmgr list account JSON: %w", err)
	}
	return &resp, nil
}

func (AccountEntity) ListCommand() []string {
	return []string{"sacctmgr", "list", "account", "--json"}
}

func (AccountEntity) GetCommand(accountName string) ([]string, error) {
	name := strings.TrimSpace(accountName)
	if name == "" {
		return nil, fmt.Errorf("account name is required")
	}
	return []string{"sacctmgr", "list", "account", fmt.Sprintf("Name=%s", name), "--json"}, nil
}

func (AccountEntity) DeleteCommand(accountName string) ([]string, error) {
	name := strings.TrimSpace(accountName)
	if name == "" {
		return nil, fmt.Errorf("account name is required")
	}
	return []string{"sacctmgr", "--immediate", "delete", "account", "where", fmt.Sprintf("Name=%s", name)}, nil
}

type AccountCreateSpec struct {
	Name         string
	Description  string
	Organization string
}

type AccountUpdateSpec struct {
	Description  *string
	Organization *string
}

func (AccountEntity) AddCommand(spec AccountCreateSpec) ([]string, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("account name is required")
	}

	args := []string{"sacctmgr", "--immediate", "add", "account", fmt.Sprintf("Name=%s", name)}
	if description := strings.TrimSpace(spec.Description); description != "" {
		args = append(args, fmt.Sprintf("Description=%s", description))
	}
	if organization := strings.TrimSpace(spec.Organization); organization != "" {
		args = append(args, fmt.Sprintf("Organization=%s", organization))
	}
	return args, nil
}

func (AccountEntity) ModifyCommand(accountName string, spec AccountUpdateSpec) ([]string, error) {
	name := strings.TrimSpace(accountName)
	if name == "" {
		return nil, fmt.Errorf("account name is required")
	}

	setArgs := make([]string, 0, 2)
	if spec.Description != nil {
		description := strings.TrimSpace(*spec.Description)
		setArgs = append(setArgs, fmt.Sprintf("Description=%s", description))
	}
	if spec.Organization != nil {
		organization := strings.TrimSpace(*spec.Organization)
		setArgs = append(setArgs, fmt.Sprintf("Organization=%s", organization))
	}
	if len(setArgs) == 0 {
		return nil, fmt.Errorf("at least one field must be set for account modify")
	}

	args := []string{"sacctmgr", "--immediate", "modify", "account", "where", fmt.Sprintf("Name=%s", name), "set"}
	args = append(args, setArgs...)
	return args, nil
}

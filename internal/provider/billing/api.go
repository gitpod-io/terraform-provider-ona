// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package billing

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	onaclient "github.com/gitpod-io/terraform-provider-ona/internal/client"
	"google.golang.org/protobuf/encoding/protojson"
)

type enterpriseAIUserBudgetPolicyRequest struct {
	OrganizationID             string  `json:"organizationId,omitempty"`
	Mode                       string  `json:"mode,omitempty"`
	UserID                     *string `json:"userId,omitempty"`
	MonthlyCreditLimit         *int64  `json:"monthlyCreditLimit,omitempty"`
	MonthlyCostLimitMicrounits *int64  `json:"monthlyCostLimitMicrounits,omitempty"`
	Currency                   string  `json:"currency,omitempty"`
	NoCap                      bool    `json:"noCap,omitempty"`
}

type teamCreditAllocationRequest struct {
	OrganizationID       string `json:"organizationId,omitempty"`
	TeamID               string `json:"teamId,omitempty"`
	CreditBudget         int64  `json:"creditBudget,omitempty"`
	CostBudgetMicrounits *int64 `json:"costBudgetMicrounits,omitempty"`
	CostBudgetCurrency   string `json:"costBudgetCurrency,omitempty"`
	PreserveCreditBudget bool   `json:"preserveCreditBudget,omitempty"`
	ClearCostBudget      bool   `json:"clearCostBudget,omitempty"`
}

type policyResponse struct {
	Policy *v1.EnterpriseAIUserBudgetPolicy
}

func (r *policyResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		Policy json.RawMessage `json:"policy"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Policy) == 0 || string(raw.Policy) == "null" {
		return nil
	}
	r.Policy = &v1.EnterpriseAIUserBudgetPolicy{}
	if err := protojson.Unmarshal(raw.Policy, r.Policy); err != nil {
		return fmt.Errorf("decode policy: %w", err)
	}
	return nil
}

type allocationResponse struct {
	Allocation *v1.TeamCreditAllocationInfo
}

func (r *allocationResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		Allocation json.RawMessage `json:"allocation"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Allocation) == 0 || string(raw.Allocation) == "null" {
		return nil
	}
	r.Allocation = &v1.TeamCreditAllocationInfo{}
	if err := protojson.Unmarshal(raw.Allocation, r.Allocation); err != nil {
		return fmt.Errorf("decode allocation: %w", err)
	}
	return nil
}

func setEnterpriseAIUserBudgetPolicy(ctx context.Context, client *onaclient.Client, req *enterpriseAIUserBudgetPolicyRequest) (*v1.EnterpriseAIUserBudgetPolicy, error) {
	var resp policyResponse
	if err := client.Post(ctx, "/gitpod.v1.BillingService/SetEnterpriseAIUserBudgetPolicy", req, &resp); err != nil {
		return nil, err
	}
	return resp.Policy, nil
}

func getEnterpriseAIUserBudgetPolicy(ctx context.Context, client *onaclient.Client, req *enterpriseAIUserBudgetPolicyRequest) (*v1.EnterpriseAIUserBudgetPolicy, error) {
	var resp policyResponse
	if err := client.Post(ctx, "/gitpod.v1.BillingService/GetEnterpriseAIUserBudgetPolicy", req, &resp); err != nil {
		return nil, err
	}
	return resp.Policy, nil
}

func deleteEnterpriseAIUserBudgetPolicy(ctx context.Context, client *onaclient.Client, req *enterpriseAIUserBudgetPolicyRequest) error {
	return client.Post(ctx, "/gitpod.v1.BillingService/DeleteEnterpriseAIUserBudgetPolicy", req, nil)
}

func createTeamCreditAllocation(ctx context.Context, client *onaclient.Client, req *teamCreditAllocationRequest) (*v1.TeamCreditAllocationInfo, error) {
	var resp allocationResponse
	if err := client.Post(ctx, "/gitpod.v1.BillingService/CreateTeamCreditAllocation", req, &resp); err != nil {
		return nil, err
	}
	return resp.Allocation, nil
}

func getTeamCreditAllocation(ctx context.Context, client *onaclient.Client, req *teamCreditAllocationRequest) (*v1.TeamCreditAllocationInfo, error) {
	var resp allocationResponse
	if err := client.Post(ctx, "/gitpod.v1.BillingService/GetTeamCreditAllocation", req, &resp); err != nil {
		return nil, err
	}
	return resp.Allocation, nil
}

func updateTeamCreditAllocation(ctx context.Context, client *onaclient.Client, req *teamCreditAllocationRequest) (*v1.TeamCreditAllocationInfo, error) {
	var resp allocationResponse
	if err := client.Post(ctx, "/gitpod.v1.BillingService/UpdateTeamCreditAllocation", req, &resp); err != nil {
		return nil, err
	}
	return resp.Allocation, nil
}

func deleteTeamCreditAllocation(ctx context.Context, client *onaclient.Client, req *teamCreditAllocationRequest) error {
	return client.Post(ctx, "/gitpod.v1.BillingService/DeleteTeamCreditAllocation", req, nil)
}

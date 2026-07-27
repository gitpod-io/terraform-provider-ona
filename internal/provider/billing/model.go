// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package billing

import "github.com/hashicorp/terraform-plugin-framework/types"

type OrganizationAIBudgetModel struct {
	ID                         types.String `tfsdk:"id"`
	OrganizationID             types.String `tfsdk:"organization_id"`
	Mode                       types.String `tfsdk:"mode"`
	MonthlyCreditLimit         types.Int64  `tfsdk:"monthly_credit_limit"`
	MonthlyCostLimitMicrounits types.Int64  `tfsdk:"monthly_cost_limit_microunits"`
	Currency                   types.String `tfsdk:"currency"`
	CreatedAt                  types.String `tfsdk:"created_at"`
}

type UserAIBudgetModel struct {
	ID                         types.String `tfsdk:"id"`
	OrganizationID             types.String `tfsdk:"organization_id"`
	UserID                     types.String `tfsdk:"user_id"`
	Mode                       types.String `tfsdk:"mode"`
	MonthlyCreditLimit         types.Int64  `tfsdk:"monthly_credit_limit"`
	MonthlyCostLimitMicrounits types.Int64  `tfsdk:"monthly_cost_limit_microunits"`
	Currency                   types.String `tfsdk:"currency"`
	NoCap                      types.Bool   `tfsdk:"no_cap"`
	CreatedAt                  types.String `tfsdk:"created_at"`
}

type TeamAIBudgetModel struct {
	ID                   types.String `tfsdk:"id"`
	OrganizationID       types.String `tfsdk:"organization_id"`
	TeamID               types.String `tfsdk:"team_id"`
	Mode                 types.String `tfsdk:"mode"`
	CreditBudget         types.Int64  `tfsdk:"credit_budget"`
	CostBudgetMicrounits types.Int64  `tfsdk:"cost_budget_microunits"`
	CostBudgetCurrency   types.String `tfsdk:"cost_budget_currency"`
	CreatedAt            types.String `tfsdk:"created_at"`
}

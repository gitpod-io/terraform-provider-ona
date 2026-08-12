// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package billing

import (
	"fmt"
	"time"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func modeToProto(mode string) (v1.EnterpriseAIUserBudgetMode, error) {
	switch mode {
	case modeCredits:
		return v1.EnterpriseAIUserBudgetMode_ENTERPRISE_AI_USER_BUDGET_MODE_CREDITS, nil
	case modeBYOK:
		return v1.EnterpriseAIUserBudgetMode_ENTERPRISE_AI_USER_BUDGET_MODE_BYOK, nil
	default:
		return v1.EnterpriseAIUserBudgetMode_ENTERPRISE_AI_USER_BUDGET_MODE_UNSPECIFIED, fmt.Errorf("unsupported budget mode %q", mode)
	}
}

func modeFromProto(mode v1.EnterpriseAIUserBudgetMode) (string, error) {
	switch mode {
	case v1.EnterpriseAIUserBudgetMode_ENTERPRISE_AI_USER_BUDGET_MODE_CREDITS:
		return modeCredits, nil
	case v1.EnterpriseAIUserBudgetMode_ENTERPRISE_AI_USER_BUDGET_MODE_BYOK:
		return modeBYOK, nil
	default:
		return "", fmt.Errorf("unsupported API budget mode %q", mode.String())
	}
}

func currencyToProto(currency string) (v1.BillingCurrency, error) {
	switch currency {
	case "usd":
		return v1.BillingCurrency_BILLING_CURRENCY_USD, nil
	case "eur":
		return v1.BillingCurrency_BILLING_CURRENCY_EUR, nil
	case "gbp":
		return v1.BillingCurrency_BILLING_CURRENCY_GBP, nil
	default:
		return v1.BillingCurrency_BILLING_CURRENCY_UNSPECIFIED, fmt.Errorf("unsupported budget currency %q", currency)
	}
}

func currencyFromProto(currency v1.BillingCurrency) (string, error) {
	switch currency {
	case v1.BillingCurrency_BILLING_CURRENCY_USD:
		return "usd", nil
	case v1.BillingCurrency_BILLING_CURRENCY_EUR:
		return "eur", nil
	case v1.BillingCurrency_BILLING_CURRENCY_GBP:
		return "gbp", nil
	default:
		return "", fmt.Errorf("unsupported API budget currency %q", currency.String())
	}
}

func organizationPolicySetRequest(organizationID string, data OrganizationAIBudgetModel) (*v1.SetEnterpriseAIUserBudgetPolicyRequest, error) {
	return policySetRequest(organizationID, nil, data.Mode.ValueString(), data.MonthlyCreditLimit, data.MonthlyCostLimitMicrounits, data.Currency, false)
}

func userPolicySetRequest(organizationID string, data UserAIBudgetModel) (*v1.SetEnterpriseAIUserBudgetPolicyRequest, error) {
	userID := data.UserID.ValueString()
	return policySetRequest(organizationID, &userID, data.Mode.ValueString(), data.MonthlyCreditLimit, data.MonthlyCostLimitMicrounits, data.Currency, data.NoCap.ValueBool())
}

func policySetRequest(organizationID string, userID *string, mode string, credit, cost types.Int64, currency types.String, noCap bool) (*v1.SetEnterpriseAIUserBudgetPolicyRequest, error) {
	apiMode, err := modeToProto(mode)
	if err != nil {
		return nil, err
	}
	req := &v1.SetEnterpriseAIUserBudgetPolicyRequest{
		OrganizationId: organizationID,
		Mode:           apiMode,
		UserId:         userID,
		NoCap:          noCap,
	}
	if !credit.IsNull() {
		value := credit.ValueInt64()
		req.MonthlyCreditLimit = &value
	}
	if !cost.IsNull() {
		value := cost.ValueInt64()
		req.MonthlyCostLimitMicrounits = &value
	}
	if !currency.IsNull() {
		req.Currency, err = currencyToProto(currency.ValueString())
		if err != nil {
			return nil, err
		}
	}
	return req, nil
}

func populateOrganizationBudget(data *OrganizationAIBudgetModel, policy *v1.EnterpriseAIUserBudgetPolicy, organizationID string) error {
	if policy == nil {
		return fmt.Errorf("API returned an empty organization AI budget policy")
	}
	expectedMode := data.Mode.ValueString()
	mode, err := validatePolicyIdentity(policy, organizationID, "")
	if err != nil {
		return err
	}
	if expectedMode != "" && mode != expectedMode {
		return fmt.Errorf("API returned budget policy mode %q instead of %q", mode, expectedMode)
	}
	data.ID = types.StringValue(policy.GetId())
	data.OrganizationID = types.StringValue(organizationID)
	data.Mode = types.StringValue(mode)
	data.MonthlyCreditLimit = optionalInt64(policy.MonthlyCreditLimit)
	data.MonthlyCostLimitMicrounits = optionalInt64(policy.MonthlyCostLimitMicrounits)
	data.Currency, err = optionalCurrency(policy)
	if err != nil {
		return err
	}
	data.CreatedAt, err = timestampValue(policy.GetCreatedAt())
	return err
}

func populateUserBudget(data *UserAIBudgetModel, policy *v1.EnterpriseAIUserBudgetPolicy, organizationID, userID string) error {
	if policy == nil {
		return fmt.Errorf("API returned an empty user AI budget policy")
	}
	expectedMode := data.Mode.ValueString()
	mode, err := validatePolicyIdentity(policy, organizationID, userID)
	if err != nil {
		return err
	}
	if expectedMode != "" && mode != expectedMode {
		return fmt.Errorf("API returned budget policy mode %q instead of %q", mode, expectedMode)
	}
	data.ID = types.StringValue(policy.GetId())
	data.OrganizationID = types.StringValue(organizationID)
	data.UserID = types.StringValue(userID)
	data.Mode = types.StringValue(mode)
	data.MonthlyCreditLimit = optionalInt64(policy.MonthlyCreditLimit)
	data.MonthlyCostLimitMicrounits = optionalInt64(policy.MonthlyCostLimitMicrounits)
	data.Currency, err = optionalCurrency(policy)
	if err != nil {
		return err
	}
	data.NoCap = types.BoolValue(policy.GetNoCap())
	data.CreatedAt, err = timestampValue(policy.GetCreatedAt())
	return err
}

func validatePolicyIdentity(policy *v1.EnterpriseAIUserBudgetPolicy, organizationID, userID string) (string, error) {
	if policy.GetId() == "" {
		return "", fmt.Errorf("API returned a budget policy without an ID")
	}
	if policy.GetOrganizationId() != organizationID {
		return "", fmt.Errorf("API returned budget policy for organization %q instead of %q", policy.GetOrganizationId(), organizationID)
	}
	if policy.GetUserId() != userID {
		return "", fmt.Errorf("API returned budget policy for user %q instead of %q", policy.GetUserId(), userID)
	}
	return modeFromProto(policy.GetMode())
}

func optionalCurrency(policy *v1.EnterpriseAIUserBudgetPolicy) (types.String, error) {
	if policy.GetCurrency() == v1.BillingCurrency_BILLING_CURRENCY_UNSPECIFIED {
		return types.StringNull(), nil
	}
	value, err := currencyFromProto(policy.GetCurrency())
	if err != nil {
		return types.StringNull(), err
	}
	return types.StringValue(value), nil
}

func populateTeamBudget(data *TeamAIBudgetModel, allocation *v1.TeamCreditAllocationInfo, organizationID, teamID, mode string) (bool, error) {
	if allocation == nil {
		return false, nil
	}
	if allocation.GetId() == "" {
		return false, fmt.Errorf("API returned a team allocation without an ID")
	}
	if allocation.GetOrganizationId() != organizationID || allocation.GetTeamId() != teamID {
		return false, fmt.Errorf("API returned team allocation for organization/team %q/%q instead of %q/%q", allocation.GetOrganizationId(), allocation.GetTeamId(), organizationID, teamID)
	}
	data.ID = types.StringValue(allocation.GetId())
	data.OrganizationID = types.StringValue(organizationID)
	data.TeamID = types.StringValue(teamID)
	data.Mode = types.StringValue(mode)
	data.CreditBudget = types.Int64Null()
	data.CostBudgetMicrounits = types.Int64Null()
	data.CostBudgetCurrency = types.StringNull()

	switch mode {
	case modeCredits:
		if allocation.GetCreditBudget() == 0 {
			return false, nil
		}
		data.CreditBudget = types.Int64Value(allocation.GetCreditBudget())
	case modeBYOK:
		if allocation.CostBudgetMicrounits == nil {
			return false, nil
		}
		data.CostBudgetMicrounits = types.Int64Value(*allocation.CostBudgetMicrounits)
		currency, err := currencyFromProto(allocation.GetCostBudgetCurrency())
		if err != nil {
			return false, err
		}
		data.CostBudgetCurrency = types.StringValue(currency)
	default:
		return false, fmt.Errorf("unsupported team budget mode %q", mode)
	}
	createdAt, err := timestampValue(allocation.GetCreatedAt())
	if err != nil {
		return false, err
	}
	data.CreatedAt = createdAt
	return true, nil
}

func createTeamBudgetRequest(organizationID string, data TeamAIBudgetModel) (*v1.CreateTeamCreditAllocationRequest, error) {
	req := &v1.CreateTeamCreditAllocationRequest{OrganizationId: organizationID, TeamId: data.TeamID.ValueString()}
	switch data.Mode.ValueString() {
	case modeCredits:
		req.CreditBudget = data.CreditBudget.ValueInt64()
	case modeBYOK:
		value := data.CostBudgetMicrounits.ValueInt64()
		req.CostBudgetMicrounits = &value
		currency, err := currencyToProto(data.CostBudgetCurrency.ValueString())
		if err != nil {
			return nil, err
		}
		req.CostBudgetCurrency = currency
	default:
		return nil, fmt.Errorf("unsupported team budget mode %q", data.Mode.ValueString())
	}
	return req, nil
}

func updateTeamBudgetRequest(organizationID string, data TeamAIBudgetModel) (*v1.UpdateTeamCreditAllocationRequest, error) {
	req := &v1.UpdateTeamCreditAllocationRequest{OrganizationId: organizationID, TeamId: data.TeamID.ValueString()}
	switch data.Mode.ValueString() {
	case modeCredits:
		req.CreditBudget = data.CreditBudget.ValueInt64()
	case modeBYOK:
		value := data.CostBudgetMicrounits.ValueInt64()
		req.CostBudgetMicrounits = &value
		req.PreserveCreditBudget = true
		currency, err := currencyToProto(data.CostBudgetCurrency.ValueString())
		if err != nil {
			return nil, err
		}
		req.CostBudgetCurrency = currency
	default:
		return nil, fmt.Errorf("unsupported team budget mode %q", data.Mode.ValueString())
	}
	return req, nil
}

func clearTeamBYOKRequest(organizationID, teamID string) *v1.UpdateTeamCreditAllocationRequest {
	return &v1.UpdateTeamCreditAllocationRequest{
		OrganizationId:       organizationID,
		TeamId:               teamID,
		ClearCostBudget:      true,
		PreserveCreditBudget: true,
	}
}

func optionalInt64(value *int64) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*value)
}

func timestampValue(value *timestamppb.Timestamp) (types.String, error) {
	if value == nil {
		return types.StringNull(), nil
	}
	if err := value.CheckValid(); err != nil {
		return types.StringNull(), fmt.Errorf("invalid API timestamp: %w", err)
	}
	return types.StringValue(value.AsTime().UTC().Format(time.RFC3339)), nil
}

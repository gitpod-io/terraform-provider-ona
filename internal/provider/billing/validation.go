// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package billing

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func validateOrganizationBudget(data OrganizationAIBudgetModel, requireKnown bool, diags *diag.Diagnostics) {
	validatePolicyFields(data.Mode, data.MonthlyCreditLimit, data.MonthlyCostLimitMicrounits, data.Currency, types.BoolValue(false), false, requireKnown, diags)
}

func validateUserBudget(data UserAIBudgetModel, requireKnown bool, diags *diag.Diagnostics) {
	validateUUID(data.UserID, path.Root("user_id"), requireKnown, diags)
	validatePolicyFields(data.Mode, data.MonthlyCreditLimit, data.MonthlyCostLimitMicrounits, data.Currency, data.NoCap, true, requireKnown, diags)
}

func validatePolicyFields(mode types.String, credit, cost types.Int64, currency types.String, noCap types.Bool, allowNoCap, requireKnown bool, diags *diag.Diagnostics) {
	selected, ok := knownMode(mode, requireKnown, diags)
	if !ok {
		return
	}
	if noCap.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(path.Root("no_cap"), "Unknown No-Cap Setting", "no_cap must be known before apply.")
		}
		return
	}
	if allowNoCap && !noCap.IsNull() && noCap.ValueBool() {
		if !credit.IsNull() && !credit.IsUnknown() {
			diags.AddAttributeError(path.Root("monthly_credit_limit"), "Invalid No-Cap Budget", "Do not configure monthly_credit_limit when no_cap is true.")
		}
		if !cost.IsNull() && !cost.IsUnknown() {
			diags.AddAttributeError(path.Root("monthly_cost_limit_microunits"), "Invalid No-Cap Budget", "Do not configure monthly_cost_limit_microunits when no_cap is true.")
		}
		if !currency.IsNull() && !currency.IsUnknown() {
			diags.AddAttributeError(path.Root("currency"), "Invalid No-Cap Budget", "Do not configure currency when no_cap is true.")
		}
		return
	}

	switch selected {
	case modeCredits:
		validateInt64(credit, path.Root("monthly_credit_limit"), 0, maxWholeCreditBudget, requireKnown, diags)
		rejectConfiguredInt64(cost, path.Root("monthly_cost_limit_microunits"), "credits mode uses monthly_credit_limit", diags)
		rejectConfiguredString(currency, path.Root("currency"), "credits mode does not use currency", diags)
	case modeBYOK:
		rejectConfiguredInt64(credit, path.Root("monthly_credit_limit"), "byok mode uses monthly_cost_limit_microunits", diags)
		validateInt64(cost, path.Root("monthly_cost_limit_microunits"), 0, -1, requireKnown, diags)
		validateCurrency(currency, path.Root("currency"), requireKnown, diags)
	}
}

func validateTeamBudget(data TeamAIBudgetModel, requireKnown bool, diags *diag.Diagnostics) {
	validateUUID(data.TeamID, path.Root("team_id"), requireKnown, diags)
	selected, ok := knownMode(data.Mode, requireKnown, diags)
	if !ok {
		return
	}
	switch selected {
	case modeCredits:
		validateInt64(data.CreditBudget, path.Root("credit_budget"), 1, maxWholeCreditBudget, requireKnown, diags)
		rejectConfiguredInt64(data.CostBudgetMicrounits, path.Root("cost_budget_microunits"), "credits mode uses credit_budget", diags)
		rejectConfiguredString(data.CostBudgetCurrency, path.Root("cost_budget_currency"), "credits mode does not use cost_budget_currency", diags)
	case modeBYOK:
		rejectConfiguredInt64(data.CreditBudget, path.Root("credit_budget"), "byok mode uses cost_budget_microunits", diags)
		validateInt64(data.CostBudgetMicrounits, path.Root("cost_budget_microunits"), 1, -1, requireKnown, diags)
		validateCurrency(data.CostBudgetCurrency, path.Root("cost_budget_currency"), requireKnown, diags)
	}
}

func knownMode(value types.String, requireKnown bool, diags *diag.Diagnostics) (string, bool) {
	if value.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(path.Root("mode"), "Unknown Budget Mode", "mode must be known before apply.")
		}
		return "", false
	}
	if value.IsNull() || value.ValueString() == "" {
		diags.AddAttributeError(path.Root("mode"), "Missing Budget Mode", "Set mode to credits or byok.")
		return "", false
	}
	switch value.ValueString() {
	case modeCredits, modeBYOK:
		return value.ValueString(), true
	default:
		diags.AddAttributeError(path.Root("mode"), "Unsupported Budget Mode", fmt.Sprintf("Unsupported mode %q. Supported values are credits and byok.", value.ValueString()))
		return "", false
	}
}

func validateCurrency(value types.String, p path.Path, requireKnown bool, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(p, "Unknown Budget Currency", "Currency must be known before apply.")
		}
		return
	}
	if value.IsNull() || value.ValueString() == "" {
		diags.AddAttributeError(p, "Missing Budget Currency", "Set currency to usd, eur, or gbp.")
		return
	}
	switch value.ValueString() {
	case "usd", "eur", "gbp":
	default:
		diags.AddAttributeError(p, "Unsupported Budget Currency", fmt.Sprintf("Unsupported currency %q. Supported values are usd, eur, and gbp.", value.ValueString()))
	}
}

func validateUUID(value types.String, p path.Path, requireKnown bool, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(p, "Unknown UUID", "The UUID must be known before apply.")
		}
		return
	}
	if value.IsNull() || value.ValueString() == "" {
		diags.AddAttributeError(p, "Missing UUID", "Set a non-empty UUID.")
		return
	}
	parsed, err := uuid.Parse(value.ValueString())
	if err != nil || parsed == uuid.Nil {
		diags.AddAttributeError(p, "Invalid UUID", fmt.Sprintf("%q is not a valid non-zero UUID.", value.ValueString()))
	}
}

func validateInt64(value types.Int64, p path.Path, minimum, maximum int64, requireKnown bool, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(p, "Unknown Budget Value", "The budget value must be known before apply.")
		}
		return
	}
	if value.IsNull() {
		diags.AddAttributeError(p, "Missing Budget Value", "Set the budget value required by the selected mode.")
		return
	}
	if value.ValueInt64() < minimum || maximum >= 0 && value.ValueInt64() > maximum {
		detail := fmt.Sprintf("Value must be at least %d.", minimum)
		if maximum >= 0 {
			detail = fmt.Sprintf("Value must be between %d and %d.", minimum, maximum)
		}
		diags.AddAttributeError(p, "Budget Value Out of Range", detail)
	}
}

func rejectConfiguredInt64(value types.Int64, p path.Path, detail string, diags *diag.Diagnostics) {
	if !value.IsNull() && !value.IsUnknown() {
		diags.AddAttributeError(p, "Invalid Mode-Specific Budget Field", detail+".")
	}
}

func rejectConfiguredString(value types.String, p path.Path, detail string, diags *diag.Diagnostics) {
	if !value.IsNull() && !value.IsUnknown() {
		diags.AddAttributeError(p, "Invalid Mode-Specific Budget Field", detail+".")
	}
}

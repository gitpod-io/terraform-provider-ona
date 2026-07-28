// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package billing

import (
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/tfvalue"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func identityString(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{
		Required:            true,
		MarkdownDescription: description,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
}

func organizationAIBudgetSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Mode-specific monthly Ona AI budget default for the organization associated with the configured provider token. This feature requires an enterprise organization and suitable Billing permissions.",
		Attributes: map[string]resourceschema.Attribute{
			"id":              tfvalue.StableComputedString("Billing policy ID."),
			"organization_id": tfvalue.StableComputedString("Organization ID resolved from the configured provider token."),
			"mode":            identityString("Budget mode: `credits` or `byok`. Changing this value replaces the resource."),
			"monthly_credit_limit": resourceschema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Whole-credit monthly limit for `credits` mode. Configure a value from 0 through 9,223,372,036,854.",
			},
			"monthly_cost_limit_microunits": resourceschema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Monthly BYOK cost limit in currency microunits for `byok` mode. Must be non-negative.",
			},
			"currency": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Currency for a BYOK cost limit: `usd`, `eur`, or `gbp`.",
			},
			"created_at": tfvalue.StableComputedString("Policy creation time in RFC 3339 format."),
		},
	}
}

func userAIBudgetSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Explicit mode-specific monthly Ona AI budget override for an organization user or service account. Inherited organization defaults are not treated as managed overrides. This feature requires an enterprise organization and suitable Billing permissions.",
		Attributes: map[string]resourceschema.Attribute{
			"id":              tfvalue.StableComputedString("Billing policy ID."),
			"organization_id": tfvalue.StableComputedString("Organization ID resolved from the configured provider token."),
			"user_id":         identityString("Organization user or service-account UUID receiving the override. Changing this value replaces the resource."),
			"mode":            identityString("Budget mode: `credits` or `byok`. Changing this value replaces the resource."),
			"monthly_credit_limit": resourceschema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Whole-credit monthly limit for a capped `credits` override. Configure a value from 0 through 9,223,372,036,854.",
			},
			"monthly_cost_limit_microunits": resourceschema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Monthly BYOK cost limit in currency microunits for a capped `byok` override. Must be non-negative.",
			},
			"currency": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Currency for a BYOK cost limit: `usd`, `eur`, or `gbp`.",
			},
			"no_cap": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether this identity is exempt from the selected organization default. Do not configure limit or currency fields when true.",
			},
			"created_at": tfvalue.StableComputedString("Policy creation time in RFC 3339 format."),
		},
	}
}

func teamAIBudgetSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Mode-specific soft Ona AI budget for a team. Team budgets support reporting and alerts but are not enforced at usage time, and total team budgets may exceed the organization grant. Separate `credits` and `byok` instances for one team preserve each other. This feature requires an enterprise organization and suitable Billing permissions.",
		Attributes: map[string]resourceschema.Attribute{
			"id":              tfvalue.StableComputedString("Shared team allocation ID."),
			"organization_id": tfvalue.StableComputedString("Organization ID resolved from the configured provider token."),
			"team_id":         identityString("Team UUID receiving the budget. Changing this value replaces the resource."),
			"mode":            identityString("Budget mode: `credits` or `byok`. Changing this value replaces the resource."),
			"credit_budget": resourceschema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Positive whole-credit soft budget for `credits` mode. Configure a value from 1 through 9,223,372,036,854.",
			},
			"cost_budget_microunits": resourceschema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Positive BYOK soft budget in currency microunits for `byok` mode.",
			},
			"cost_budget_currency": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Currency for a BYOK team budget: `usd`, `eur`, or `gbp`.",
			},
			"created_at": tfvalue.StableComputedString("Shared allocation creation time in RFC 3339 format."),
		},
	}
}

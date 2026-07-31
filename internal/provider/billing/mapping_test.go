// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package billing

import (
	"testing"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPolicySetRequestPreservesOptionalZero(t *testing.T) {
	t.Parallel()

	data := OrganizationAIBudgetModel{
		Mode:               types.StringValue(modeCredits),
		MonthlyCreditLimit: types.Int64Value(0),
	}
	got, err := organizationPolicySetRequest("org", data)
	if err != nil {
		t.Fatalf("organizationPolicySetRequest() error = %v", err)
	}
	if got.MonthlyCreditLimit == nil || *got.MonthlyCreditLimit != 0 {
		t.Fatalf("monthly credit limit = %v, want pointer to zero", got.MonthlyCreditLimit)
	}
	if got.MonthlyCostLimitMicrounits != nil {
		t.Fatalf("monthly cost limit = %v, want nil", got.MonthlyCostLimitMicrounits)
	}
}

func TestUserPolicySetRequestNoCap(t *testing.T) {
	t.Parallel()

	got, err := userPolicySetRequest("org", UserAIBudgetModel{
		UserID: types.StringValue("user"),
		Mode:   types.StringValue(modeBYOK),
		NoCap:  types.BoolValue(true),
	})
	if err != nil {
		t.Fatalf("userPolicySetRequest() error = %v", err)
	}
	if got.UserID == nil || *got.UserID != "user" || !got.NoCap {
		t.Fatalf("request identity/no_cap = %#v", got)
	}
	if got.MonthlyCreditLimit != nil || got.MonthlyCostLimitMicrounits != nil {
		t.Fatalf("no-cap request contains limits: %#v", got)
	}
}

func TestPopulatePolicyBudget(t *testing.T) {
	t.Parallel()

	zero := int64(0)
	createdAt := timestamppb.New(time.Date(2026, time.July, 27, 10, 11, 12, 0, time.UTC))
	policy := &v1.EnterpriseAIUserBudgetPolicy{
		Id:                 "policy",
		OrganizationId:     "org",
		Mode:               v1.EnterpriseAIUserBudgetMode_ENTERPRISE_AI_USER_BUDGET_MODE_CREDITS,
		MonthlyCreditLimit: &zero,
		CreatedAt:          createdAt,
	}
	data := OrganizationAIBudgetModel{Mode: types.StringValue(modeCredits)}
	if err := populateOrganizationBudget(&data, policy, "org"); err != nil {
		t.Fatalf("populateOrganizationBudget() error = %v", err)
	}
	if data.MonthlyCreditLimit.IsNull() || data.MonthlyCreditLimit.ValueInt64() != 0 {
		t.Fatalf("monthly_credit_limit = %#v, want explicit zero", data.MonthlyCreditLimit)
	}
	if got := data.CreatedAt.ValueString(); got != "2026-07-27T10:11:12Z" {
		t.Fatalf("created_at = %q", got)
	}

	wrongMode := OrganizationAIBudgetModel{Mode: types.StringValue(modeBYOK)}
	if err := populateOrganizationBudget(&wrongMode, policy, "org"); err == nil {
		t.Fatal("populateOrganizationBudget() accepted a response for the wrong mode")
	}
}

func TestPopulateTeamBudgetOwnsSelectedDimension(t *testing.T) {
	t.Parallel()

	cost := int64(2_500_000)
	allocation := &v1.TeamCreditAllocationInfo{
		Id:                   "allocation",
		OrganizationId:       "org",
		TeamId:               "team",
		CreditBudget:         50,
		CostBudgetMicrounits: &cost,
		CostBudgetCurrency:   v1.BillingCurrency_BILLING_CURRENCY_USD,
		CreatedAt:            timestamppb.New(time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)),
	}

	tests := []struct {
		name string
		mode string
		want TeamAIBudgetModel
	}{
		{
			name: "credits ignores cost",
			mode: modeCredits,
			want: TeamAIBudgetModel{
				ID: types.StringValue("allocation"), OrganizationID: types.StringValue("org"), TeamID: types.StringValue("team"), Mode: types.StringValue(modeCredits),
				CreditBudget: types.Int64Value(50), CostBudgetMicrounits: types.Int64Null(), CostBudgetCurrency: types.StringNull(), CreatedAt: types.StringValue("2026-07-27T00:00:00Z"),
			},
		},
		{
			name: "byok ignores credits",
			mode: modeBYOK,
			want: TeamAIBudgetModel{
				ID: types.StringValue("allocation"), OrganizationID: types.StringValue("org"), TeamID: types.StringValue("team"), Mode: types.StringValue(modeBYOK),
				CreditBudget: types.Int64Null(), CostBudgetMicrounits: types.Int64Value(cost), CostBudgetCurrency: types.StringValue("usd"), CreatedAt: types.StringValue("2026-07-27T00:00:00Z"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got TeamAIBudgetModel
			exists, err := populateTeamBudget(&got, allocation, "org", "team", tt.mode)
			if err != nil {
				t.Fatalf("populateTeamBudget() error = %v", err)
			}
			if !exists {
				t.Fatal("populateTeamBudget() exists = false")
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("team model mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPopulateTeamBudgetDetectsAbsentDimension(t *testing.T) {
	t.Parallel()

	allocation := &v1.TeamCreditAllocationInfo{Id: "allocation", OrganizationId: "org", TeamId: "team"}
	for _, mode := range []string{modeCredits, modeBYOK} {
		var data TeamAIBudgetModel
		exists, err := populateTeamBudget(&data, allocation, "org", "team", mode)
		if err != nil {
			t.Fatalf("populateTeamBudget(%q) error = %v", mode, err)
		}
		if exists {
			t.Fatalf("populateTeamBudget(%q) exists = true", mode)
		}
	}
}

func TestTeamWriteRequestsPreserveComplement(t *testing.T) {
	t.Parallel()

	credits, err := updateTeamBudgetRequest("org", TeamAIBudgetModel{TeamID: types.StringValue("team"), Mode: types.StringValue(modeCredits), CreditBudget: types.Int64Value(25)})
	if err != nil {
		t.Fatalf("credits update request: %v", err)
	}
	if credits.CreditBudget != 25 || credits.CostBudgetMicrounits != nil || credits.PreserveCreditBudget || credits.ClearCostBudget {
		t.Fatalf("credits update request = %#v", credits)
	}

	byok, err := updateTeamBudgetRequest("org", TeamAIBudgetModel{TeamID: types.StringValue("team"), Mode: types.StringValue(modeBYOK), CostBudgetMicrounits: types.Int64Value(99), CostBudgetCurrency: types.StringValue("eur")})
	if err != nil {
		t.Fatalf("byok update request: %v", err)
	}
	if byok.CreditBudget != 0 || byok.CostBudgetMicrounits == nil || *byok.CostBudgetMicrounits != 99 || !byok.PreserveCreditBudget || byok.ClearCostBudget {
		t.Fatalf("byok update request = %#v", byok)
	}

	clearRequest := clearTeamBYOKRequest("org", "team")
	if !clearRequest.ClearCostBudget || !clearRequest.PreserveCreditBudget || clearRequest.CreditBudget != 0 {
		t.Fatalf("clear request = %#v", clearRequest)
	}
}

func TestBudgetValidation(t *testing.T) {
	t.Parallel()

	validUser := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	validTeam := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	tests := []struct {
		name     string
		validate func(*diag.Diagnostics)
		wantErr  bool
	}{
		{name: "organization explicit zero credits", validate: func(diags *diag.Diagnostics) {
			validateOrganizationBudget(OrganizationAIBudgetModel{Mode: types.StringValue(modeCredits), MonthlyCreditLimit: types.Int64Value(0)}, true, diags)
		}},
		{name: "organization credits overflow", wantErr: true, validate: func(diags *diag.Diagnostics) {
			validateOrganizationBudget(OrganizationAIBudgetModel{Mode: types.StringValue(modeCredits), MonthlyCreditLimit: types.Int64Value(maxWholeCreditBudget + 1)}, true, diags)
		}},
		{name: "organization byok zero", validate: func(diags *diag.Diagnostics) {
			validateOrganizationBudget(OrganizationAIBudgetModel{Mode: types.StringValue(modeBYOK), MonthlyCostLimitMicrounits: types.Int64Value(0), Currency: types.StringValue("gbp")}, true, diags)
		}},
		{name: "user no cap", validate: func(diags *diag.Diagnostics) {
			validateUserBudget(UserAIBudgetModel{UserID: types.StringValue(validUser), Mode: types.StringValue(modeCredits), NoCap: types.BoolValue(true)}, true, diags)
		}},
		{name: "user no cap with limit", wantErr: true, validate: func(diags *diag.Diagnostics) {
			validateUserBudget(UserAIBudgetModel{UserID: types.StringValue(validUser), Mode: types.StringValue(modeCredits), NoCap: types.BoolValue(true), MonthlyCreditLimit: types.Int64Value(1)}, true, diags)
		}},
		{name: "invalid user uuid", wantErr: true, validate: func(diags *diag.Diagnostics) {
			validateUserBudget(UserAIBudgetModel{UserID: types.StringValue("nope"), Mode: types.StringValue(modeCredits), NoCap: types.BoolValue(true)}, true, diags)
		}},
		{name: "team credits positive", validate: func(diags *diag.Diagnostics) {
			validateTeamBudget(TeamAIBudgetModel{TeamID: types.StringValue(validTeam), Mode: types.StringValue(modeCredits), CreditBudget: types.Int64Value(1)}, true, diags)
		}},
		{name: "team credits zero", wantErr: true, validate: func(diags *diag.Diagnostics) {
			validateTeamBudget(TeamAIBudgetModel{TeamID: types.StringValue(validTeam), Mode: types.StringValue(modeCredits), CreditBudget: types.Int64Value(0)}, true, diags)
		}},
		{name: "team byok positive", validate: func(diags *diag.Diagnostics) {
			validateTeamBudget(TeamAIBudgetModel{TeamID: types.StringValue(validTeam), Mode: types.StringValue(modeBYOK), CostBudgetMicrounits: types.Int64Value(1), CostBudgetCurrency: types.StringValue("usd")}, true, diags)
		}},
		{name: "team byok needs currency", wantErr: true, validate: func(diags *diag.Diagnostics) {
			validateTeamBudget(TeamAIBudgetModel{TeamID: types.StringValue(validTeam), Mode: types.StringValue(modeBYOK), CostBudgetMicrounits: types.Int64Value(1)}, true, diags)
		}},
		{name: "unknown deferred during config validation", validate: func(diags *diag.Diagnostics) {
			validateOrganizationBudget(OrganizationAIBudgetModel{Mode: types.StringUnknown(), MonthlyCreditLimit: types.Int64Unknown()}, false, diags)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got diag.Diagnostics
			tt.validate(&got)
			if got.HasError() != tt.wantErr {
				t.Fatalf("HasError() = %v, want %v; diagnostics = %v", got.HasError(), tt.wantErr, got)
			}
		})
	}
}

// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
)

type ssoOptionalFieldStateResult struct {
	AdditionalScopesNull bool
	AdditionalScopes     []string
	ClaimsExpressionNull bool
	ClaimsExpression     string
}

func TestSSOConfigurationMappings(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		StateString       string
		State             v1.SSOConfigurationState
		StateOK           bool
		ProviderType      string
		SecretNeedsUpdate bool
	}

	tests := []struct {
		Name     string
		Input    string
		Plan     SSOConfigurationModel
		Prior    SSOConfigurationModel
		Expected Expectation
	}{
		{
			Name:  "active_custom_without_secret_rotation",
			Input: ssoStateActive,
			Plan: SSOConfigurationModel{
				ClientID:            types.StringValue("client-1"),
				ClientSecretVersion: types.StringValue("v1"),
			},
			Prior: SSOConfigurationModel{
				ClientID:            types.StringValue("client-1"),
				ClientSecretVersion: types.StringValue("v1"),
			},
			Expected: Expectation{
				StateString:       ssoStateActive,
				State:             v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_ACTIVE,
				StateOK:           true,
				ProviderType:      ssoProviderTypeCustom,
				SecretNeedsUpdate: false,
			},
		},
		{
			Name:  "inactive_builtin_with_secret_rotation",
			Input: ssoStateInactive,
			Plan: SSOConfigurationModel{
				ClientID:            types.StringValue("client-2"),
				ClientSecretVersion: types.StringValue("v2"),
			},
			Prior: SSOConfigurationModel{
				ClientID:            types.StringValue("client-1"),
				ClientSecretVersion: types.StringValue("v1"),
			},
			Expected: Expectation{
				StateString:       ssoStateInactive,
				State:             v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_INACTIVE,
				StateOK:           true,
				ProviderType:      ssoProviderTypeBuiltin,
				SecretNeedsUpdate: true,
			},
		},
		{
			Name:  "invalid_state",
			Input: "paused",
			Expected: Expectation{
				StateString:       ssoStateInactive,
				State:             v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_UNSPECIFIED,
				ProviderType:      ssoProviderTypeCustom,
				SecretNeedsUpdate: false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			state, ok := ssoStateFromString(tc.Input)
			got := Expectation{
				StateString:       ssoStateToString(state),
				State:             state,
				StateOK:           ok,
				SecretNeedsUpdate: ssoSecretRequiredForUpdate(tc.Plan, tc.Prior),
			}
			if tc.Name == "inactive_builtin_with_secret_rotation" {
				got.ProviderType = ssoProviderTypeToString(v1.SSOConfiguration_PROVIDER_TYPE_BUILTIN)
			} else {
				got.ProviderType = ssoProviderTypeToString(v1.SSOConfiguration_PROVIDER_TYPE_CUSTOM)
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("SSO mapping mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateSSOConfigurationRequest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result *v1.CreateSSOConfigurationRequest
		Err    string
	}

	emailDomains := stringSet(t, []string{"z.example.com", "a.example.com"})
	additionalScopes := stringSet(t, []string{"groups", "profile"})
	claimsExpression := "claims.email_verified"

	tests := []struct {
		Name     string
		Input    SSOConfigurationModel
		Secret   types.String
		Expected Expectation
	}{
		{
			Name: "builds_sorted_request",
			Input: SSOConfigurationModel{
				ClientID:         types.StringValue("client-1"),
				IssuerURL:        types.StringValue("https://idp.example.com"),
				DisplayName:      types.StringValue("Example IdP"),
				EmailDomains:     emailDomains,
				AdditionalScopes: additionalScopes,
				ClaimsExpression: types.StringValue(claimsExpression),
				State:            types.StringValue(ssoStateActive),
			},
			Secret: types.StringValue("secret"),
			Expected: Expectation{
				Result: &v1.CreateSSOConfigurationRequest{
					OrganizationId:   "org-1",
					ClientId:         "client-1",
					ClientSecret:     "secret",
					IssuerUrl:        "https://idp.example.com",
					DisplayName:      "Example IdP",
					EmailDomains:     []string{"a.example.com", "z.example.com"},
					AdditionalScopes: []string{"groups", "profile"},
					ClaimsExpression: &claimsExpression,
				},
			},
		},
		{
			Name: "requires_secret",
			Input: SSOConfigurationModel{
				ClientID:     types.StringValue("client-1"),
				IssuerURL:    types.StringValue("https://idp.example.com"),
				EmailDomains: emailDomains,
				State:        types.StringValue(ssoStateActive),
			},
			Secret: types.StringNull(),
			Expected: Expectation{
				Err: "Missing SSO Client Secret",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, diags := createSSOConfigurationRequest(t.Context(), "org-1", tc.Input, tc.Secret)
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("createSSOConfigurationRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPopulateSSOConfigurationOptionalOnlyFields(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result ssoOptionalFieldStateResult
		Err    string
	}

	tests := []struct {
		Name     string
		Planned  SSOConfigurationModel
		Expected Expectation
	}{
		{
			Name: "omitted_fields_remain_null",
			Planned: SSOConfigurationModel{
				AdditionalScopes: types.SetNull(types.StringType),
				ClaimsExpression: types.StringNull(),
			},
			Expected: Expectation{
				Result: ssoOptionalFieldStateResult{
					AdditionalScopesNull: true,
					ClaimsExpressionNull: true,
				},
			},
		},
		{
			Name: "explicit_empty_fields_are_preserved",
			Planned: SSOConfigurationModel{
				AdditionalScopes: stringSet(t, []string{}),
				ClaimsExpression: types.StringValue(""),
			},
			Expected: Expectation{
				Result: ssoOptionalFieldStateResult{
					AdditionalScopes: []string{},
					ClaimsExpression: "",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var data SSOConfigurationModel
			diags := diag.Diagnostics{}
			populateSSOConfigurationModel(t.Context(), &data, &v1.SSOConfiguration{
				Id:               "sso-1",
				ClientId:         "client-1",
				IssuerUrl:        "https://idp.example.com",
				DisplayName:      "Example IdP",
				EmailDomains:     []string{"example.com"},
				AdditionalScopes: []string{"groups"},
				ClaimsExpression: "claims.email_verified",
				State:            v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_ACTIVE,
				ProviderType:     v1.SSOConfiguration_PROVIDER_TYPE_CUSTOM,
			}, tc.Planned, &diags)
			preserveSSOConfigurationPlannedInputs(&data, tc.Planned)

			var got Expectation
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				got.Result = ssoOptionalFieldState(t, data)
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("populateSSOConfigurationModel() optional fields mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func ssoOptionalFieldState(t *testing.T, data SSOConfigurationModel) ssoOptionalFieldStateResult {
	t.Helper()

	result := ssoOptionalFieldStateResult{
		AdditionalScopesNull: data.AdditionalScopes.IsNull(),
		ClaimsExpressionNull: data.ClaimsExpression.IsNull(),
	}
	if !data.AdditionalScopes.IsNull() && !data.AdditionalScopes.IsUnknown() {
		diags := data.AdditionalScopes.ElementsAs(t.Context(), &result.AdditionalScopes, false)
		if diags.HasError() {
			t.Fatalf("AdditionalScopes.ElementsAs() returned diagnostics: %s", diags[0].Summary())
		}
	}
	if !data.ClaimsExpression.IsNull() && !data.ClaimsExpression.IsUnknown() {
		result.ClaimsExpression = data.ClaimsExpression.ValueString()
	}
	return result
}

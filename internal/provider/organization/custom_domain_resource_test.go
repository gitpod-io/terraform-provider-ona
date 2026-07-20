// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"encoding/json"
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCustomDomainProviderMapping(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Provider v1.CustomDomainProvider
		Value    string
		Errors   []string
	}

	tests := []struct {
		Name     string
		Value    types.String
		Provider v1.CustomDomainProvider
		Expected Expectation
	}{
		{
			Name:     "aws_round_trip",
			Value:    types.StringValue("aws"),
			Provider: v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_AWS,
			Expected: Expectation{
				Provider: v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_AWS,
				Value:    "aws",
			},
		},
		{
			Name:     "gcp_round_trip",
			Value:    types.StringValue("gcp"),
			Provider: v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_GCP,
			Expected: Expectation{
				Provider: v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_GCP,
				Value:    "gcp",
			},
		},
		{
			Name:     "unsupported_terraform_value",
			Value:    types.StringValue("azure"),
			Provider: v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_AWS,
			Expected: Expectation{
				Value:  "aws",
				Errors: []string{"Invalid Cloud Provider"},
			},
		},
		{
			Name:     "unsupported_api_value",
			Value:    types.StringValue("aws"),
			Provider: v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_UNSPECIFIED,
			Expected: Expectation{
				Provider: v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_AWS,
				Errors:   []string{"Unsupported Cloud Provider"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			provider, providerDiags := customDomainProviderFromTerraform(tc.Value, path.Root("cloud_provider"))
			value, valueDiags := customDomainProviderToTerraform(tc.Provider, path.Root("cloud_provider"))
			got.Provider = provider
			if !value.IsNull() && !value.IsUnknown() {
				got.Value = value.ValueString()
			}
			got.Errors = append(diagSummaries(providerDiags), diagSummaries(valueDiags)...)

			if diff := cmp.Diff(tc.Expected, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("custom domain provider mapping mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateCustomDomainImportID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Errors []string
	}

	tests := []struct {
		Name     string
		ImportID string
		Expected Expectation
	}{
		{
			Name:     "current",
			ImportID: "current",
		},
		{
			Name:     "authenticated_organization_id",
			ImportID: "org-1",
		},
		{
			Name:     "empty",
			ImportID: "",
			Expected: Expectation{
				Errors: []string{"Invalid Import ID"},
			},
		},
		{
			Name:     "composite",
			ImportID: "org-1/domain-1",
			Expected: Expectation{
				Errors: []string{"Invalid Import ID"},
			},
		},
		{
			Name:     "other_organization",
			ImportID: "org-2",
			Expected: Expectation{
				Errors: []string{"Invalid Import ID"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{
				Errors: diagSummaries(validateCustomDomainImportID(tc.ImportID, "org-1")),
			}

			if diff := cmp.Diff(tc.Expected, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("validateCustomDomainImportID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateCustomDomainValues(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Errors []string
	}

	tests := []struct {
		Name           string
		DomainName     types.String
		CloudProvider  types.String
		CloudAccountID types.String
		Expected       Expectation
	}{
		{
			Name:           "valid_aws",
			DomainName:     types.StringValue("ona.example.com"),
			CloudProvider:  types.StringValue("aws"),
			CloudAccountID: types.StringValue("123456789012"),
		},
		{
			Name:           "valid_gcp",
			DomainName:     types.StringValue("ona.example.com"),
			CloudProvider:  types.StringValue("gcp"),
			CloudAccountID: types.StringValue("my-gcp-project"),
		},
		{
			Name:           "reject_url_domain",
			DomainName:     types.StringValue("https://ona.example.com"),
			CloudProvider:  types.StringValue("aws"),
			CloudAccountID: types.StringValue("123456789012"),
			Expected: Expectation{
				Errors: []string{"Invalid Domain Name"},
			},
		},
		{
			Name:           "reject_path_domain",
			DomainName:     types.StringValue("ona.example.com/path"),
			CloudProvider:  types.StringValue("aws"),
			CloudAccountID: types.StringValue("123456789012"),
			Expected: Expectation{
				Errors: []string{"Invalid Domain Name"},
			},
		},
		{
			Name:           "reject_wildcard_domain",
			DomainName:     types.StringValue("*.example.com"),
			CloudProvider:  types.StringValue("aws"),
			CloudAccountID: types.StringValue("123456789012"),
			Expected: Expectation{
				Errors: []string{"Invalid Domain Name"},
			},
		},
		{
			Name:           "reject_unknown_provider",
			DomainName:     types.StringValue("ona.example.com"),
			CloudProvider:  types.StringValue("azure"),
			CloudAccountID: types.StringValue("account"),
			Expected: Expectation{
				Errors: []string{"Invalid Cloud Provider"},
			},
		},
		{
			Name:           "reject_aws_letters",
			DomainName:     types.StringValue("ona.example.com"),
			CloudProvider:  types.StringValue("aws"),
			CloudAccountID: types.StringValue("12345678901a"),
			Expected: Expectation{
				Errors: []string{"Invalid AWS Account ID"},
			},
		},
		{
			Name:           "reject_gcp_uppercase",
			DomainName:     types.StringValue("ona.example.com"),
			CloudProvider:  types.StringValue("gcp"),
			CloudAccountID: types.StringValue("My-GCP-Project"),
			Expected: Expectation{
				Errors: []string{"Invalid GCP Project ID"},
			},
		},
		{
			Name:           "reject_gcp_starts_with_number",
			DomainName:     types.StringValue("ona.example.com"),
			CloudProvider:  types.StringValue("gcp"),
			CloudAccountID: types.StringValue("1-gcp-project"),
			Expected: Expectation{
				Errors: []string{"Invalid GCP Project ID"},
			},
		},
		{
			Name:           "reject_gcp_trailing_hyphen",
			DomainName:     types.StringValue("ona.example.com"),
			CloudProvider:  types.StringValue("gcp"),
			CloudAccountID: types.StringValue("my-gcp-project-"),
			Expected: Expectation{
				Errors: []string{"Invalid GCP Project ID"},
			},
		},
		{
			Name:           "unknown_provider_skips_account_validation",
			DomainName:     types.StringValue("ona.example.com"),
			CloudProvider:  types.StringUnknown(),
			CloudAccountID: types.StringValue("not-yet-known"),
		},
		{
			Name:           "unknown_account_skips_account_validation",
			DomainName:     types.StringValue("ona.example.com"),
			CloudProvider:  types.StringValue("aws"),
			CloudAccountID: types.StringUnknown(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateCustomDomainName(path.Root("domain_name"), tc.DomainName, &diags)
			validateCustomDomainCloudProvider(path.Root("cloud_provider"), tc.CloudProvider, &diags)
			validateCustomDomainCloudAccountID(path.Root("cloud_account_id"), tc.CloudProvider, tc.CloudAccountID, &diags)
			got := Expectation{
				Errors: diagSummaries(diags),
			}

			if diff := cmp.Diff(tc.Expected, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("custom domain validation mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestVerifyPrivateOrganizationID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Stored string
		Errors []string
	}

	tests := []struct {
		Name            string
		StoredID        string
		AuthenticatedID string
		SetBeforeVerify bool
		SetDuringVerify bool
		Expected        Expectation
	}{
		{
			Name:            "missing_scope_allows_read",
			AuthenticatedID: "org-1",
		},
		{
			Name:            "matching_scope",
			StoredID:        "org-1",
			AuthenticatedID: "org-1",
			SetBeforeVerify: true,
			Expected: Expectation{
				Stored: "org-1",
			},
		},
		{
			Name:            "mismatched_scope",
			StoredID:        "org-2",
			AuthenticatedID: "org-1",
			SetBeforeVerify: true,
			Expected: Expectation{
				Stored: "org-2",
				Errors: []string{"Custom Domain Organization Scope Changed"},
			},
		},
		{
			Name:            "store_scope",
			AuthenticatedID: "org-1",
			SetDuringVerify: true,
			Expected: Expectation{
				Stored: "org-1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			state := &fakePrivateState{data: map[string][]byte{}}
			var diags diag.Diagnostics
			if tc.SetBeforeVerify {
				diags.Append(setPrivateOrganizationID(t.Context(), state, tc.StoredID)...)
			}
			diags.Append(verifyPrivateOrganizationID(t.Context(), state, tc.AuthenticatedID)...)
			if tc.SetDuringVerify {
				diags.Append(setPrivateOrganizationID(t.Context(), state, tc.AuthenticatedID)...)
			}

			stored, storedDiags := privateOrganizationID(t.Context(), state)
			diags.Append(storedDiags...)
			got := Expectation{
				Stored: stored,
				Errors: diagSummaries(diags),
			}

			if diff := cmp.Diff(tc.Expected, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("verifyPrivateOrganizationID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type fakePrivateState struct {
	data map[string][]byte
}

func (s *fakePrivateState) GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics) {
	return s.data[key], nil
}

func (s *fakePrivateState) SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics {
	if len(value) == 0 {
		delete(s.data, key)
		return nil
	}
	s.data[key] = append([]byte(nil), value...)
	return nil
}

func privateOrganizationID(ctx context.Context, state privateState) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	data, stateDiags := state.GetKey(ctx, customDomainPrivateOrganizationIDKey)
	diags.Append(stateDiags...)
	if diags.HasError() || len(data) == 0 {
		return "", diags
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		diags.AddError("decode failed", err.Error())
	}
	return value, diags
}

func diagSummaries(diags diag.Diagnostics) []string {
	result := make([]string, 0, len(diags))
	for _, diagnostic := range diags {
		result = append(result, diagnostic.Summary())
	}
	return result
}

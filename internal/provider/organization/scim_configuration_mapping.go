// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	defaultSCIMName = "SCIM Configuration"
)

func createSCIMConfigurationRequest(ctx context.Context, organizationID string, data SCIMConfigurationModel, cfg tfsdk.Config) (*v1.CreateSCIMConfigurationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateSCIMConfigurationConfig(ctx, cfg, &diags)
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.CreateSCIMConfigurationRequest{
		OrganizationId:                     organizationID,
		SsoConfigurationId:                 data.SSOConfigurationID.ValueString(),
		TokenExpiresIn:                     scimTokenExpiresIn(ctx, cfg, &diags),
		Name:                               stringPtrFromConfig(ctx, cfg, path.Root("name"), data.Name, &diags),
		AllowUnverifiedEmailAccountLinking: boolPtrFromConfig(ctx, cfg, path.Root("allow_unverified_email_account_linking"), data.AllowUnverifiedEmailAccountLinking, &diags),
	}
	if diags.HasError() {
		return nil, diags
	}
	return req, diags
}

func updateSCIMConfigurationRequestFromConfig(ctx context.Context, plan SCIMConfigurationModel, cfg tfsdk.Config) (*v1.UpdateSCIMConfigurationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateSCIMConfigurationConfig(ctx, cfg, &diags)
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.UpdateSCIMConfigurationRequest{
		ScimConfigurationId: plan.ID.ValueString(),
	}
	req.Name = stringPtrFromConfig(ctx, cfg, path.Root("name"), plan.Name, &diags)
	req.Enabled = boolPtrFromConfig(ctx, cfg, path.Root("enabled"), plan.Enabled, &diags)
	req.SsoConfigurationId = stringPtrFromConfig(ctx, cfg, path.Root("sso_configuration_id"), plan.SSOConfigurationID, &diags)
	req.AllowUnverifiedEmailAccountLinking = boolPtrFromConfig(ctx, cfg, path.Root("allow_unverified_email_account_linking"), plan.AllowUnverifiedEmailAccountLinking, &diags)
	if diags.HasError() {
		return nil, diags
	}
	return req, diags
}

func populateSCIMConfigurationModel(data *SCIMConfigurationModel, scim *v1.SCIMConfiguration, prior SCIMConfigurationModel) {
	data.ID = types.StringValue(scim.GetId())
	data.SSOConfigurationID = types.StringValue(scim.GetSsoConfigurationId())
	data.Name = types.StringValue(scim.GetName())
	data.Enabled = types.BoolValue(scim.GetEnabled())
	data.AllowUnverifiedEmailAccountLinking = types.BoolValue(scim.GetAllowUnverifiedEmailAccountLinking())
	data.TokenExpiresIn = prior.TokenExpiresIn
	data.TokenExpiresAt = timestampString(scim.GetTokenExpiresAt())
	data.CreatedAt = timestampString(scim.GetCreatedAt())
}

func preserveSCIMConfigurationPlannedInputs(data *SCIMConfigurationModel, planned SCIMConfigurationModel) {
	data.SSOConfigurationID = preserveString(data.SSOConfigurationID, planned.SSOConfigurationID)
	data.Name = preserveString(data.Name, planned.Name)
	data.Enabled = preserveBool(data.Enabled, planned.Enabled)
	data.AllowUnverifiedEmailAccountLinking = preserveBool(data.AllowUnverifiedEmailAccountLinking, planned.AllowUnverifiedEmailAccountLinking)
	data.TokenExpiresIn = preserveString(data.TokenExpiresIn, planned.TokenExpiresIn)
}

func validateSCIMConfigurationConfig(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) {
	validateSCIMDuration(ctx, cfg, path.Root("token_expires_in"), diags)
}

func validateSCIMDuration(ctx context.Context, cfg tfsdk.Config, attrPath path.Path, diags *diag.Diagnostics) {
	var value types.String
	diags.Append(cfg.GetAttribute(ctx, attrPath, &value)...)
	if diags.HasError() {
		return
	}
	validateSCIMDurationValue(value, attrPath, diags)
}

func validateSCIMDurationValue(value types.String, attrPath path.Path, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	duration, err := time.ParseDuration(value.ValueString())
	if err != nil {
		diags.AddAttributeError(attrPath, "Invalid SCIM Token Duration", "Use a number followed by a time unit, such as \"24h\" or \"8760h\".")
		return
	}
	if duration < 24*time.Hour || duration > 2*365*24*time.Hour {
		diags.AddAttributeError(attrPath, "Invalid SCIM Token Duration", "token_expires_in must be at least 24h and at most 17520h.")
	}
}

func scimTokenExpiresIn(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) *durationpb.Duration {
	var value types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("token_expires_in"), &value)...)
	if diags.HasError() || value.IsNull() || value.IsUnknown() {
		return nil
	}
	duration, err := time.ParseDuration(value.ValueString())
	if err != nil {
		diags.AddAttributeError(path.Root("token_expires_in"), "Invalid SCIM Token Duration", "Use a number followed by a time unit, such as \"24h\" or \"8760h\".")
		return nil
	}
	return durationpb.New(duration)
}

func stringPtrFromConfig(ctx context.Context, cfg tfsdk.Config, attrPath path.Path, planValue types.String, diags *diag.Diagnostics) *string {
	value, ok := stringFromConfig(ctx, cfg, attrPath, planValue, diags)
	if !ok {
		return nil
	}
	return value
}

func boolPtrFromConfig(ctx context.Context, cfg tfsdk.Config, attrPath path.Path, planValue types.Bool, diags *diag.Diagnostics) *bool {
	value, ok := boolFromConfig(ctx, cfg, attrPath, planValue, diags)
	if !ok {
		return nil
	}
	return value
}

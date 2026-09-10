// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package githubapp

import (
	"context"
	"strings"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func validateCredentials(ctx context.Context, config Model, required, allowUnknown bool, diags *diag.Diagnostics) credentialsModel {
	var credentials credentialsModel
	if config.Credentials.IsUnknown() && allowUnknown {
		return credentials
	}
	if config.Credentials.IsNull() || config.Credentials.IsUnknown() {
		if required {
			diags.AddAttributeError(path.Root("credentials"), "Missing GitHub App Credentials", "Provide the complete private_key, client_secret, and webhook_secret bundle on creation and credential version changes.")
		}
		return credentials
	}
	diags.Append(config.Credentials.As(ctx, &credentials, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return credentials
	}
	fields := []struct {
		name  string
		value basetypes.StringValue
	}{
		{"private_key", credentials.PrivateKey}, {"client_secret", credentials.ClientSecret}, {"webhook_secret", credentials.WebhookSecret},
	}
	var hasKnownValue, hasUnknown bool
	for _, field := range fields {
		hasUnknown = hasUnknown || field.value.IsUnknown()
		hasKnownValue = hasKnownValue || (!field.value.IsNull() && !field.value.IsUnknown())
	}
	// Terraform Query can generate an empty sensitive object for write-only credentials.
	if !hasKnownValue && !hasUnknown {
		if required {
			diags.AddAttributeError(path.Root("credentials"), "Missing GitHub App Credentials", "Provide the complete private_key, client_secret, and webhook_secret bundle on creation and credential version changes.")
		}
		return credentials
	}
	if !required && allowUnknown && !hasKnownValue {
		return credentials
	}
	if config.CredentialsVersion.IsNull() || (!allowUnknown && config.CredentialsVersion.IsUnknown()) {
		diags.AddAttributeError(path.Root("credentials_version"), "Missing GitHub App Credentials Version", "Set a positive credentials_version together with the credentials bundle.")
		return credentials
	}
	for _, field := range fields {
		if field.value.IsUnknown() && allowUnknown {
			continue
		}
		if field.value.IsNull() || field.value.IsUnknown() || strings.TrimSpace(field.value.ValueString()) == "" {
			diags.AddAttributeError(path.Root("credentials").AtName(field.name), "Incomplete GitHub App Credentials", "Each credential in the bundle must be a known, non-empty value when creating or rotating the GitHub App integration.")
		}
	}
	return credentials
}

func mutationAuth(ctx context.Context, plan, config Model, diags *diag.Diagnostics) *v1.IntegrationAuthentication {
	if plan.AppID.IsUnknown() || plan.AppID.IsNull() || !validAppID(plan.AppID.ValueString()) {
		diags.AddAttributeError(path.Root("app_id"), "Invalid GitHub App ID", "A known canonical positive GitHub App ID is required before applying.")
	}
	if plan.CredentialsVersion.IsNull() || plan.CredentialsVersion.IsUnknown() || plan.CredentialsVersion.ValueInt64() < 1 {
		diags.AddAttributeError(path.Root("credentials_version"), "Missing GitHub App Credentials Version", "Set a positive credentials_version together with the complete credentials bundle.")
	}
	credentials := validateCredentials(ctx, config, true, false, diags)
	if diags.HasError() {
		return nil
	}
	return &v1.IntegrationAuthentication{
		RequiresAuth: true,
		ProprietaryApp: &v1.IntegrationProprietaryAppConfig{
			AppId: plan.AppID.ValueString(), PrivateKey: credentials.PrivateKey.ValueString(),
			ClientSecret: credentials.ClientSecret.ValueString(), WebhookSecret: credentials.WebhookSecret.ValueString(),
		},
	}
}

// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package webhook

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

const maxRepositoryScopes = 100

func validateModel(ctx context.Context, data Model, requireKnown bool, diags *diag.Diagnostics) {
	validateString(data.Name, path.Root("name"), "Webhook Name", 1, 80, requireKnown, diags)
	validateString(data.Description, path.Root("description"), "Webhook Description", 0, 500, requireKnown, diags)
	if requireKnown && data.SecretVersion.IsUnknown() {
		diags.AddAttributeError(path.Root("secret_version"), "Unknown Webhook Secret Version", "secret_version must be known before apply.")
	}

	if data.Type.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(path.Root("type"), "Unknown Webhook Type", "type must be known before apply.")
		}
		return
	}
	if data.Type.IsNull() {
		diags.AddAttributeError(path.Root("type"), "Missing Webhook Type", "Set type to \"repository\" or \"organization\".")
		return
	}
	if _, ok := webhookTypeFromString(data.Type.ValueString()); !ok {
		diags.AddAttributeError(path.Root("type"), "Invalid Webhook Type", "Supported values are \"repository\" and \"organization\".")
		return
	}

	if data.Provider.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(path.Root("scm_provider"), "Unknown Webhook Provider", "scm_provider must be known before apply.")
		}
	} else if data.Provider.IsNull() {
		diags.AddAttributeError(path.Root("scm_provider"), "Missing Webhook Provider", "Set scm_provider to \"github\", \"gitlab\", or \"bitbucket\".")
	} else if _, ok := webhookProviderFromString(data.Provider.ValueString()); !ok {
		diags.AddAttributeError(path.Root("scm_provider"), "Invalid Webhook Provider", "Supported values are \"github\", \"gitlab\", and \"bitbucket\".")
	}

	switch data.Type.ValueString() {
	case webhookTypeRepository:
		validateRepositoryScopes(ctx, data.RepositoryScopes, requireKnown, diags)
		if !data.OrganizationScope.IsNull() && !data.OrganizationScope.IsUnknown() {
			diags.AddAttributeError(path.Root("organization_scope"), "Invalid Webhook Scope", "Do not set organization_scope when type is \"repository\".")
		}
	case webhookTypeOrganization:
		if !data.RepositoryScopes.IsNull() && !data.RepositoryScopes.IsUnknown() && len(data.RepositoryScopes.Elements()) > 0 {
			diags.AddAttributeError(path.Root("repository_scopes"), "Invalid Webhook Scope", "Do not set repository_scopes when type is \"organization\".")
		}
		validateOrganizationScope(ctx, data.OrganizationScope, requireKnown, diags)
	}
}

func validateRepositoryScopes(ctx context.Context, value types.Set, requireKnown bool, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(path.Root("repository_scopes"), "Unknown Repository Scopes", "repository_scopes must be known before apply.")
		}
		return
	}
	if value.IsNull() || len(value.Elements()) == 0 {
		diags.AddAttributeError(path.Root("repository_scopes"), "Missing Repository Scopes", "Set at least one repository_scopes entry when type is \"repository\".")
		return
	}
	if len(value.Elements()) > maxRepositoryScopes {
		diags.AddAttributeError(path.Root("repository_scopes"), "Too Many Repository Scopes", fmt.Sprintf("repository_scopes supports at most %d entries.", maxRepositoryScopes))
	}

	var scopes []RepositoryScopeModel
	conversionDiags := value.ElementsAs(ctx, &scopes, !requireKnown)
	if conversionDiags.HasError() && !requireKnown {
		return
	}
	diags.Append(conversionDiags...)
	for _, scope := range scopes {
		validateString(scope.Host, path.Root("repository_scopes"), "Repository Host", 1, 0, requireKnown, diags)
		validateString(scope.Owner, path.Root("repository_scopes"), "Repository Owner", 1, 0, requireKnown, diags)
		validateString(scope.Name, path.Root("repository_scopes"), "Repository Name", 1, 0, requireKnown, diags)
	}
}

func validateOrganizationScope(ctx context.Context, value types.Object, requireKnown bool, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(path.Root("organization_scope"), "Unknown Organization Scope", "organization_scope must be known before apply.")
		}
		return
	}
	if value.IsNull() {
		diags.AddAttributeError(path.Root("organization_scope"), "Missing Organization Scope", "Set organization_scope when type is \"organization\".")
		return
	}
	var scope OrganizationScopeModel
	conversionDiags := value.As(ctx, &scope, basetypes.ObjectAsOptions{
		UnhandledUnknownAsEmpty: !requireKnown,
	})
	diags.Append(conversionDiags...)
	if diags.HasError() {
		return
	}
	validateString(scope.Host, path.Root("organization_scope"), "Organization Host", 1, 0, requireKnown, diags)
	validateString(scope.Name, path.Root("organization_scope"), "Organization Name", 1, 0, requireKnown, diags)
}

func validateString(value types.String, attributePath path.Path, name string, minLength int, maxLength int, requireKnown bool, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(attributePath, "Unknown "+name, name+" must be known before apply.")
		}
		return
	}
	if value.IsNull() {
		if minLength > 0 {
			diags.AddAttributeError(attributePath, "Missing "+name, name+" must not be empty.")
		}
		return
	}
	rawValue := value.ValueString()
	if len(strings.TrimSpace(rawValue)) < minLength {
		diags.AddAttributeError(attributePath, "Invalid "+name, fmt.Sprintf("%s must contain at least %d character(s).", name, minLength))
	}
	if maxLength > 0 && len(rawValue) > maxLength {
		diags.AddAttributeError(attributePath, "Invalid "+name, fmt.Sprintf("%s must not exceed %d characters.", name, maxLength))
	}
}

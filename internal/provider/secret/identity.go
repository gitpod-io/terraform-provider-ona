// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package secret

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type IdentityModel struct {
	ID               types.String `tfsdk:"id"`
	Scope            types.String `tfsdk:"scope"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	ProjectID        types.String `tfsdk:"project_id"`
	UserID           types.String `tfsdk:"user_id"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
}

func (r *Resource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"id":                 identityschema.StringAttribute{RequiredForImport: true, Description: "Secret ID."},
		"scope":              identityschema.StringAttribute{RequiredForImport: true, Description: "Secret scope."},
		"organization_id":    identityschema.StringAttribute{OptionalForImport: true, Description: "Organization owner ID for organization-scoped secrets."},
		"project_id":         identityschema.StringAttribute{OptionalForImport: true, Description: "Project owner ID for project-scoped secrets."},
		"user_id":            identityschema.StringAttribute{OptionalForImport: true, Description: "User owner ID for user-scoped secrets."},
		"service_account_id": identityschema.StringAttribute{OptionalForImport: true, Description: "Service-account owner ID for service-account-scoped secrets."},
	}}
}

func identityFromModel(data Model) IdentityModel {
	identity := IdentityModel{
		ID:               data.ID,
		Scope:            data.Scope,
		OrganizationID:   types.StringNull(),
		ProjectID:        types.StringNull(),
		UserID:           types.StringNull(),
		ServiceAccountID: types.StringNull(),
	}
	switch data.Scope.ValueString() {
	case scopeOrganization:
		identity.OrganizationID = data.OrganizationID
	case scopeProject:
		identity.ProjectID = data.ProjectID
	case scopeUser:
		identity.UserID = data.UserID
	case scopeServiceAccount:
		identity.ServiceAccountID = data.ServiceAccountID
	}
	return identity
}

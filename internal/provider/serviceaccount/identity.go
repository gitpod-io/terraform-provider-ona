// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package serviceaccount

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type IdentityModel struct {
	ServiceAccountID types.String `tfsdk:"service_account_id"`
}

func (r *Resource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"service_account_id": identityschema.StringAttribute{RequiredForImport: true, Description: "Service account ID."},
	}}
}

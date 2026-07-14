// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package warmpool

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type IdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func (r *WarmPoolResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Warm pool ID.",
			},
		},
	}
}

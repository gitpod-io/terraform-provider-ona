// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type TeamMembershipIdentityModel struct {
	TeamID types.String `tfsdk:"team_id"`
	UserID types.String `tfsdk:"user_id"`
}

func (r *TeamMembershipResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"team_id": identityschema.StringAttribute{RequiredForImport: true, Description: "Team ID."},
		"user_id": identityschema.StringAttribute{RequiredForImport: true, Description: "User ID."},
	}}
}

func teamMembershipIdentity(data TeamMembershipModel) (TeamMembershipIdentityModel, error) {
	if data.TeamID.IsNull() || data.TeamID.IsUnknown() || data.TeamID.ValueString() == "" {
		return TeamMembershipIdentityModel{}, fmt.Errorf("team_id must be known and configured")
	}
	if data.UserID.IsNull() || data.UserID.IsUnknown() || data.UserID.ValueString() == "" {
		return TeamMembershipIdentityModel{}, fmt.Errorf("user_id must be known and configured")
	}
	return TeamMembershipIdentityModel{TeamID: data.TeamID, UserID: data.UserID}, nil
}

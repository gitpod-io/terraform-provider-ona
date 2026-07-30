// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

const privateOrganizationIDKey = "ona_skill_authenticated_organization_id"

type privateState interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func setPrivateOrganizationID(ctx context.Context, state privateState, organizationID string) diag.Diagnostics {
	var diags diag.Diagnostics
	if state == nil {
		diags.AddError("Unable to Store Ona Skill Private State", "Terraform did not provide private state storage for ona_skill.")
		return diags
	}
	if organizationID == "" {
		diags.Append(state.SetKey(ctx, privateOrganizationIDKey, nil)...)
		return diags
	}
	data, err := json.Marshal(organizationID)
	if err != nil {
		diags.AddError("Unable to Store Ona Skill Private State", fmt.Sprintf("Could not encode the authenticated organization ID: %s.", err))
		return diags
	}
	diags.Append(state.SetKey(ctx, privateOrganizationIDKey, data)...)
	return diags
}

func verifyPrivateOrganizationID(ctx context.Context, state privateState, authenticatedOrganizationID string) diag.Diagnostics {
	var diags diag.Diagnostics
	if state == nil {
		return diags
	}
	data, stateDiags := state.GetKey(ctx, privateOrganizationIDKey)
	diags.Append(stateDiags...)
	if diags.HasError() || len(data) == 0 {
		return diags
	}
	var storedOrganizationID string
	if err := json.Unmarshal(data, &storedOrganizationID); err != nil {
		diags.AddError("Unable to Read Ona Skill Private State", fmt.Sprintf("Could not decode the stored organization ID: %s.", err))
		return diags
	}
	if storedOrganizationID != authenticatedOrganizationID {
		diags.AddError(
			"Ona Skill Organization Changed",
			fmt.Sprintf("This ona_skill belongs to organization %q, but the configured provider token is scoped to organization %q. Restore a token for the original organization before refreshing, updating, or destroying it.", storedOrganizationID, authenticatedOrganizationID),
		)
	}
	return diags
}

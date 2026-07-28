// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package billing

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	modeCredits = "credits"
	modeBYOK    = "byok"

	maxWholeCreditBudget int64 = 9_223_372_036_854
)

func guardOrganizationScope(diags *diag.Diagnostics, stateID types.String, authenticatedID, resourceType string) bool {
	if stateID.IsNull() || stateID.IsUnknown() || stateID.ValueString() == "" || stateID.ValueString() == authenticatedID {
		return true
	}
	diags.AddError(
		"Authenticated Organization Changed",
		fmt.Sprintf("%s state belongs to organization %q, but the configured Ona token is authenticated for organization %q.", resourceType, stateID.ValueString(), authenticatedID),
	)
	return false
}

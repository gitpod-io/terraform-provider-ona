// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package secret

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

type importState struct {
	Scope            string
	ID               string
	ProjectID        string
	UserID           string
	ServiceAccountID string
}

func parseImportID(id string) (importState, diag.Diagnostics) {
	var diags diag.Diagnostics
	parts := strings.Split(id, "/")
	fail := func() (importState, diag.Diagnostics) {
		diags.AddError("Invalid Import ID", "Expected one of: organization/<secret_id>, project/<project_id>/<secret_id>, user/<user_id>/<secret_id>, or service_account/<service_account_id>/<secret_id>.")
		return importState{}, diags
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return fail()
		}
	}

	switch {
	case len(parts) == 2 && parts[0] == scopeOrganization:
		if !uuidPattern.MatchString(parts[1]) {
			return fail()
		}
		return importState{Scope: scopeOrganization, ID: parts[1]}, diags
	case len(parts) == 3 && parts[0] == scopeProject:
		if !uuidPattern.MatchString(parts[1]) || !uuidPattern.MatchString(parts[2]) {
			return fail()
		}
		return importState{Scope: scopeProject, ProjectID: parts[1], ID: parts[2]}, diags
	case len(parts) == 3 && parts[0] == scopeUser:
		if !uuidPattern.MatchString(parts[1]) || !uuidPattern.MatchString(parts[2]) {
			return fail()
		}
		return importState{Scope: scopeUser, UserID: parts[1], ID: parts[2]}, diags
	case len(parts) == 3 && parts[0] == scopeServiceAccount:
		if !uuidPattern.MatchString(parts[1]) || !uuidPattern.MatchString(parts[2]) {
			return fail()
		}
		return importState{Scope: scopeServiceAccount, ServiceAccountID: parts[1], ID: parts[2]}, diags
	default:
		return fail()
	}
}

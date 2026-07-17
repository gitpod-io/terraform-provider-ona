// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
)

func TestOrganizationRoleMapping(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		RoleToAPI map[string]v1.ResourceRole
		APIToRole map[v1.ResourceRole]string
	}

	expected := Expectation{
		RoleToAPI: map[string]v1.ResourceRole{
			"organization_admin":  v1.ResourceRole_RESOURCE_ROLE_ORG_ADMIN,
			"runners_admin":       v1.ResourceRole_RESOURCE_ROLE_ORG_RUNNERS_ADMIN,
			"projects_admin":      v1.ResourceRole_RESOURCE_ROLE_ORG_PROJECTS_ADMIN,
			"automations_admin":   v1.ResourceRole_RESOURCE_ROLE_ORG_AUTOMATIONS_ADMIN,
			"groups_admin":        v1.ResourceRole_RESOURCE_ROLE_ORG_GROUPS_ADMIN,
			"environments_reader": v1.ResourceRole_RESOURCE_ROLE_ORG_ENVIRONMENTS_READER,
			"insights_viewer":     v1.ResourceRole_RESOURCE_ROLE_ORG_INSIGHTS_VIEWER,
			"audit_log_reader":    v1.ResourceRole_RESOURCE_ROLE_ORG_AUDIT_LOG_READER,
			"billing_viewer":      v1.ResourceRole_RESOURCE_ROLE_ORG_BILLING_VIEWER,
		},
		APIToRole: map[v1.ResourceRole]string{
			v1.ResourceRole_RESOURCE_ROLE_ORG_ADMIN:               "organization_admin",
			v1.ResourceRole_RESOURCE_ROLE_ORG_RUNNERS_ADMIN:       "runners_admin",
			v1.ResourceRole_RESOURCE_ROLE_ORG_PROJECTS_ADMIN:      "projects_admin",
			v1.ResourceRole_RESOURCE_ROLE_ORG_AUTOMATIONS_ADMIN:   "automations_admin",
			v1.ResourceRole_RESOURCE_ROLE_ORG_GROUPS_ADMIN:        "groups_admin",
			v1.ResourceRole_RESOURCE_ROLE_ORG_ENVIRONMENTS_READER: "environments_reader",
			v1.ResourceRole_RESOURCE_ROLE_ORG_INSIGHTS_VIEWER:     "insights_viewer",
			v1.ResourceRole_RESOURCE_ROLE_ORG_AUDIT_LOG_READER:    "audit_log_reader",
			v1.ResourceRole_RESOURCE_ROLE_ORG_BILLING_VIEWER:      "billing_viewer",
		},
	}

	got := Expectation{
		RoleToAPI: roleToAPI,
		APIToRole: apiToRole,
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("organization role mapping mismatch (-want +got):\n%s", diff)
	}
}

// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccOrganizationRoleAssignmentQuery(t *testing.T) {
	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.assignments["org"] = &v1.RoleAssignment{Id: accessControlAssignmentID, GroupId: accessControlGroupID, ResourceId: accessControlOrgID, OrganizationId: accessControlOrgID, ResourceType: v1.ResourceType_RESOURCE_TYPE_ORGANIZATION, ResourceRole: v1.ResourceRole_RESOURCE_ROLE_ORG_ADMIN}
	server.service.assignments["project"] = &v1.RoleAssignment{Id: "project-role", GroupId: accessControlGroupID, ResourceId: "project-1", ResourceType: v1.ResourceType_RESOURCE_TYPE_PROJECT, ResourceRole: v1.ResourceRole_RESOURCE_ROLE_PROJECT_EDITOR}
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: organizationRoleAssignmentQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_organization_role_assignment.all", 1),
		querycheck.ExpectIdentity("ona_organization_role_assignment.all", map[string]knownvalue.Check{"group_id": knownvalue.StringExact(accessControlGroupID), "organization_id": knownvalue.StringExact(accessControlOrgID), "role": knownvalue.StringExact("organization_admin")}),
		querycheck.ExpectResourceKnownValues("ona_organization_role_assignment.all", queryfilter.ByDisplayName(knownvalue.StringExact(accessControlGroupID+"/organization_admin")), []querycheck.KnownValueCheck{
			{Path: tfjsonpath.New("group_id"), KnownValue: knownvalue.StringExact(accessControlGroupID)}, {Path: tfjsonpath.New("role"), KnownValue: knownvalue.StringExact("organization_admin")},
		}),
	}}))
}

func organizationRoleAssignmentQueryConfig() string {
	return `
list "ona_organization_role_assignment" "all" {
  provider = ona
  include_resource = true
  config { organization_id = "11111111-1111-4111-8111-111111111111" }
}
`
}

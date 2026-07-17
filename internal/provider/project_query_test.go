// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func (s *fakeProjectService) ListProjects(ctx context.Context, req *connect.Request[v1.ListProjectsRequest]) (*connect.Response[v1.ListProjectsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var projects []*v1.Project
	for _, project := range s.projects {
		projects = append(projects, cloneProject(project))
	}
	return connect.NewResponse(&v1.ListProjectsResponse{Projects: projects}), nil
}

func TestAccProjectQuery(t *testing.T) {
	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.projects["project-1"] = &v1.Project{
		Id: "project-1",
		Metadata: &v1.ProjectMetadata{OrganizationId: "org-1", Name: "Example", Creator: &v1.Subject{
			Id: "user-1", Principal: v1.Principal_PRINCIPAL_USER,
		}},
		Initializer: &v1.EnvironmentInitializer{Specs: []*v1.EnvironmentInitializer_Spec{{
			Spec: &v1.EnvironmentInitializer_Spec_Git{Git: &v1.GitInitializer{RemoteUri: "https://github.com/ona/example.git", CloneTarget: "main"}},
		}}},
		EnvironmentClasses: []*v1.ProjectEnvironmentClass{{EnvironmentClass: &v1.ProjectEnvironmentClass_EnvironmentClassId{EnvironmentClassId: "class-1"}, Order: 0}},
	}
	server.service.projects["project-context-url"] = &v1.Project{
		Id:          "project-context-url",
		Metadata:    &v1.ProjectMetadata{Name: "Context URL"},
		Initializer: &v1.EnvironmentInitializer{Specs: []*v1.EnvironmentInitializer_Spec{{Spec: &v1.EnvironmentInitializer_Spec_ContextUrl{ContextUrl: &v1.ContextURLInitializer{Url: "https://github.com/ona/context"}}}}},
	}
	server.service.projects["project-missing-clone-url"] = &v1.Project{
		Id:       "project-missing-clone-url",
		Metadata: &v1.ProjectMetadata{Name: "Missing clone URL"},
		Initializer: &v1.EnvironmentInitializer{Specs: []*v1.EnvironmentInitializer_Spec{{
			Spec: &v1.EnvironmentInitializer_Spec_Git{Git: &v1.GitInitializer{CloneTarget: "main"}},
		}}},
	}
	server.service.projects["project-missing-branch"] = &v1.Project{
		Id:       "project-missing-branch",
		Metadata: &v1.ProjectMetadata{Name: "Missing branch"},
		Initializer: &v1.EnvironmentInitializer{Specs: []*v1.EnvironmentInitializer_Spec{{
			Spec: &v1.EnvironmentInitializer_Spec_Git{Git: &v1.GitInitializer{RemoteUri: "https://github.com/ona/no-branch.git"}},
		}}},
	}
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: projectQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_project.all", 1), querycheck.ExpectIdentity("ona_project.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("project-1")}),
		querycheck.ExpectResourceKnownValues("ona_project.all", queryfilter.ByDisplayName(knownvalue.StringExact("Example")), []querycheck.KnownValueCheck{{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("project-1")}, {Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("Example")}, {Path: tfjsonpath.New("repository_clone_url"), KnownValue: knownvalue.StringExact("https://github.com/ona/example.git")}, {Path: tfjsonpath.New("branch"), KnownValue: knownvalue.StringExact("main")}}),
	}}))
}
func projectQueryConfig() string {
	return `
list "ona_project" "all" {
  provider         = ona
  include_resource = true
}
`
}

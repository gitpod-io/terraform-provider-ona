// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"google.golang.org/protobuf/testing/protocmp"
)

func (s *fakeProjectService) ListProjects(ctx context.Context, req *connect.Request[v1.ListProjectsRequest]) (*connect.Response[v1.ListProjectsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filter := &v1.ListProjectsRequest_Filter{}
	if req.Msg.GetFilter() != nil {
		filter = req.Msg.GetFilter()
	}
	s.listFilters = append(s.listFilters, filter)
	var projects []*v1.Project
	for _, project := range s.projects {
		projects = append(projects, cloneProject(project))
	}
	return connect.NewResponse(&v1.ListProjectsResponse{Projects: projects}), nil
}

func (s *fakeProjectService) lastListFilter() *v1.ListProjectsRequest_Filter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.listFilters) == 0 {
		return nil
	}
	return s.listFilters[len(s.listFilters)-1]
}

func TestAccProjectQueryReturnsRemoteInsightsStatus(t *testing.T) {
	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.projects["project-1"] = exampleQueryableProject()
	server.insights.enabled["project-1"] = true
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
		querycheck.ExpectResourceKnownValues("ona_project.all", queryfilter.ByDisplayName(knownvalue.StringExact("example")), []querycheck.KnownValueCheck{{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("project-1")}, {Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("Example")}, {Path: tfjsonpath.New("repository_clone_url"), KnownValue: knownvalue.StringExact("https://github.com/ona/example.git")}, {Path: tfjsonpath.New("branch"), KnownValue: knownvalue.StringExact("main")}, {Path: tfjsonpath.New("insights_enabled"), KnownValue: knownvalue.Bool(true)}}),
	}}))
	if diff := cmp.Diff(&v1.ListProjectsRequest_Filter{Search: "example"}, server.service.lastListFilter(), protocmp.Transform()); diff != "" {
		t.Errorf("project list filter mismatch (-want +got):\n%s", diff)
	}
}

func TestAccProjectQueryExcludesMissingEnvironmentClasses(t *testing.T) {
	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.projects["project-1"] = exampleQueryableProject()
	server.insights.enabled["project-1"] = false
	server.service.projects["project-missing-environment-class"] = &v1.Project{
		Id:       "project-missing-environment-class",
		Metadata: &v1.ProjectMetadata{Name: "Missing environment class"},
		Initializer: &v1.EnvironmentInitializer{Specs: []*v1.EnvironmentInitializer_Spec{{
			Spec: &v1.EnvironmentInitializer_Spec_Git{Git: &v1.GitInitializer{RemoteUri: "https://github.com/ona/no-environment-class.git", CloneTarget: "main"}},
		}}},
	}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: projectQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_project.all", 1),
			querycheck.ExpectIdentity("ona_project.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("project-1")}),
		},
	}))
}

func TestAccProjectQueryReportsInsightsError(t *testing.T) {
	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.projects["project-1"] = exampleQueryableProject()
	server.insights.getErr = errors.New("insights unavailable")

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      projectQueryConfig(),
		ExpectError: regexp.MustCompile("Unable to Read Ona Project Insights"),
	}))
}

func TestAccProjectQueryIdentityOnlyDoesNotReadInsights(t *testing.T) {
	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.projects["project-1"] = exampleQueryableProject()
	server.insights.getErr = errors.New("insights must not be read")

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: projectIdentityQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_project.all", 1),
			querycheck.ExpectIdentity("ona_project.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("project-1")}),
		},
	}))
}

func exampleQueryableProject() *v1.Project {
	return &v1.Project{
		Id: "project-1",
		Metadata: &v1.ProjectMetadata{OrganizationId: "org-1", Name: "Example", Creator: &v1.Subject{
			Id: "user-1", Principal: v1.Principal_PRINCIPAL_USER,
		}},
		Initializer: &v1.EnvironmentInitializer{Specs: []*v1.EnvironmentInitializer_Spec{{
			Spec: &v1.EnvironmentInitializer_Spec_Git{Git: &v1.GitInitializer{RemoteUri: "https://github.com/ona/example.git", CloneTarget: "main"}},
		}}},
		EnvironmentClasses: []*v1.ProjectEnvironmentClass{{EnvironmentClass: &v1.ProjectEnvironmentClass_EnvironmentClassId{EnvironmentClassId: "class-1"}, Order: 0}},
	}
}

func projectQueryConfig() string {
	return `
list "ona_project" "all" {
  provider         = ona
  include_resource = true

  config {
    search = "example"
  }
}
`
}

func projectIdentityQueryConfig() string {
	return `
list "ona_project" "all" {
  provider = ona

  config {
    search = "example"
  }
}
`
}

// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func (s *fakeProjectService) ListProjects(ctx context.Context, req *connect.Request[v1.ListProjectsRequest]) (*connect.Response[v1.ListProjectsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listRequests = append(s.listRequests, cloneListProjectsRequest(req.Msg))
	if s.listErr != nil {
		return nil, s.listErr
	}

	var projects []*v1.Project
	for _, project := range s.projects {
		if projectMatchesListFilter(project, req.Msg.GetFilter()) {
			projects = append(projects, project)
		}
	}
	sort.Slice(projects, func(i, j int) bool {
		if req.Msg.GetSort().GetOrder() == v1.SortOrder_SORT_ORDER_DESC {
			return projects[i].GetId() > projects[j].GetId()
		}
		return projects[i].GetId() < projects[j].GetId()
	})
	start, end, nextToken, err := fakePage(req.Msg.GetPagination(), len(projects), 0)
	if err != nil {
		return nil, err
	}
	page := make([]*v1.Project, 0, end-start)
	for _, project := range projects[start:end] {
		summary := cloneProject(project)
		summary.EnvironmentClasses = nil
		page = append(page, summary)
	}
	return connect.NewResponse(&v1.ListProjectsResponse{
		Pagination: &v1.PaginationResponse{NextToken: nextToken},
		Projects:   page,
	}), nil
}

func (s *fakeProjectService) ListProjectEnvironmentClasses(ctx context.Context, req *connect.Request[v1.ListProjectEnvironmentClassesRequest]) (*connect.Response[v1.ListProjectEnvironmentClassesResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.environmentClassListRequests = append(s.environmentClassListRequests, cloneListProjectEnvironmentClassesRequest(req.Msg))
	if s.environmentClassListErr != nil {
		return nil, s.environmentClassListErr
	}
	project := s.projects[req.Msg.GetProjectId()]
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	classes := project.GetEnvironmentClasses()
	start, end, nextToken, err := fakePage(req.Msg.GetPagination(), len(classes), s.environmentClassPageSizeLimit)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ListProjectEnvironmentClassesResponse{
		Pagination:                &v1.PaginationResponse{NextToken: nextToken},
		ProjectEnvironmentClasses: cloneProjectEnvironmentClasses(classes[start:end]),
	}), nil
}

func (s *fakeProjectService) listProjectRequestsSnapshot() []*v1.ListProjectsRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*v1.ListProjectsRequest, 0, len(s.listRequests))
	for _, request := range s.listRequests {
		result = append(result, cloneListProjectsRequest(request))
	}
	return result
}

func (s *fakeProjectService) environmentClassListRequestsSnapshot() []*v1.ListProjectEnvironmentClassesRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*v1.ListProjectEnvironmentClassesRequest, 0, len(s.environmentClassListRequests))
	for _, request := range s.environmentClassListRequests {
		result = append(result, cloneListProjectEnvironmentClassesRequest(request))
	}
	return result
}

func cloneListProjectsRequest(request *v1.ListProjectsRequest) *v1.ListProjectsRequest {
	result := &v1.ListProjectsRequest{}
	proto.Merge(result, request)
	return result
}

func cloneListProjectEnvironmentClassesRequest(request *v1.ListProjectEnvironmentClassesRequest) *v1.ListProjectEnvironmentClassesRequest {
	result := &v1.ListProjectEnvironmentClassesRequest{}
	proto.Merge(result, request)
	return result
}

func projectMatchesListFilter(project *v1.Project, filter *v1.ListProjectsRequest_Filter) bool {
	if filter == nil {
		return true
	}
	remoteURI := projectRepositoryRemoteURI(project)
	if filter.GetSearch() != "" {
		haystack := strings.ToLower(strings.Join([]string{project.GetId(), project.GetMetadata().GetName(), remoteURI}, " "))
		if !strings.Contains(haystack, strings.ToLower(filter.GetSearch())) {
			return false
		}
	}
	if len(filter.GetSpecRemoteUris()) == 0 {
		return true
	}
	for _, candidate := range filter.GetSpecRemoteUris() {
		if remoteURI == candidate {
			return true
		}
	}
	return false
}

func projectRepositoryRemoteURI(project *v1.Project) string {
	for _, spec := range project.GetInitializer().GetSpecs() {
		if spec.GetGit() != nil {
			return spec.GetGit().GetRemoteUri()
		}
	}
	return ""
}

func fakePage(pagination *v1.PaginationRequest, total int, pageSizeLimit int32) (int, int, string, error) {
	start := 0
	if pagination.GetToken() != "" {
		parsed, err := strconv.Atoi(pagination.GetToken())
		if err != nil || parsed < 0 || parsed > total {
			return 0, 0, "", connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pagination token"))
		}
		start = parsed
	}
	pageSize := pagination.GetPageSize()
	if pageSize <= 0 {
		pageSize = 25
	}
	if pageSizeLimit > 0 && pageSize > pageSizeLimit {
		pageSize = pageSizeLimit
	}
	end := min(start+int(pageSize), total)
	if end == total {
		return start, end, "", nil
	}
	return start, end, strconv.Itoa(end), nil
}

func (s *fakeProjectService) lastListRequest() *v1.ListProjectsRequest {
	requests := s.listProjectRequestsSnapshot()
	if len(requests) == 0 {
		return nil
	}
	return requests[len(requests)-1]
}

func TestAccProjectQueryReturnsRemoteInsightsStatus(t *testing.T) {
	t.Parallel()

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
	wantRequest := &v1.ListProjectsRequest{
		Pagination: &v1.PaginationRequest{PageSize: 100},
		Filter:     &v1.ListProjectsRequest_Filter{Search: "example"},
		Sort:       &v1.Sort{Field: "id", Order: v1.SortOrder_SORT_ORDER_ASC},
	}
	if diff := cmp.Diff(wantRequest, server.service.lastListRequest(), protocmp.Transform()); diff != "" {
		t.Errorf("project list request mismatch (-want +got):\n%s", diff)
	}
}

func TestAccProjectQueryReturnsDisabledInsightsStatus(t *testing.T) {
	t.Parallel()

	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.projects["project-1"] = exampleQueryableProject()
	server.insights.enabled["project-1"] = false

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: projectQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectResourceKnownValues(
				"ona_project.all",
				queryfilter.ByDisplayName(knownvalue.StringExact("example")),
				[]querycheck.KnownValueCheck{{Path: tfjsonpath.New("insights_enabled"), KnownValue: knownvalue.Bool(false)}},
			),
		},
	}))

	if got := server.insights.getCallCount("project-1"); got != 1 {
		t.Fatalf("expected one project Insights status request, got %d", got)
	}
}

func TestAccProjectQueryExcludesMissingEnvironmentClasses(t *testing.T) {
	t.Parallel()

	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.projects["project-1"] = exampleQueryableProject()
	server.insights.enabled["project-1"] = false
	server.service.projects["project-missing-environment-class"] = &v1.Project{
		Id:       "project-missing-environment-class",
		Metadata: &v1.ProjectMetadata{Name: "Example missing environment class"},
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

func TestAccProjectQueryPaginatesAndBackfillsLimit(t *testing.T) {
	t.Parallel()

	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.projects["project-1"] = &v1.Project{
		Id:       "project-1",
		Metadata: &v1.ProjectMetadata{Name: "Unsupported context project"},
		Initializer: &v1.EnvironmentInitializer{Specs: []*v1.EnvironmentInitializer_Spec{{
			Spec: &v1.EnvironmentInitializer_Spec_ContextUrl{ContextUrl: &v1.ContextURLInitializer{Url: "https://github.com/ona/context"}},
		}}},
	}
	server.service.projects["project-2"] = queryableProject("project-2", "Second", "https://github.com/ona/second.git")
	server.service.projects["project-3"] = queryableProject("project-3", "Third", "https://github.com/ona/third.git")

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: projectLimitedIdentityQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_project.all", 2),
			querycheck.ExpectIdentity("ona_project.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("project-2")}),
			querycheck.ExpectIdentity("ona_project.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("project-3")}),
		},
	}))

	wantRequests := []*v1.ListProjectsRequest{
		{Pagination: &v1.PaginationRequest{PageSize: 2}, Filter: &v1.ListProjectsRequest_Filter{}, Sort: &v1.Sort{Field: "id", Order: v1.SortOrder_SORT_ORDER_ASC}},
		{Pagination: &v1.PaginationRequest{PageSize: 1, Token: "2"}, Filter: &v1.ListProjectsRequest_Filter{}, Sort: &v1.Sort{Field: "id", Order: v1.SortOrder_SORT_ORDER_ASC}},
	}
	if diff := cmp.Diff(wantRequests, server.service.listProjectRequestsSnapshot(), protocmp.Transform()); diff != "" {
		t.Errorf("project list requests mismatch (-want +got):\n%s", diff)
	}
}

func TestAccProjectQueryFiltersRepositoryCloneURLs(t *testing.T) {
	t.Parallel()

	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.projects["project-1"] = queryableProject("project-1", "Selected", "https://github.com/ona/selected.git")
	server.service.projects["project-2"] = queryableProject("project-2", "Other", "https://github.com/ona/other.git")

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: projectRepositoryQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_project.all", 1),
			querycheck.ExpectIdentity("ona_project.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("project-1")}),
		},
	}))

	wantFilter := &v1.ListProjectsRequest_Filter{SpecRemoteUris: []string{"https://github.com/ona/selected.git"}}
	if diff := cmp.Diff(wantFilter, server.service.lastListRequest().GetFilter(), protocmp.Transform()); diff != "" {
		t.Errorf("project repository filter mismatch (-want +got):\n%s", diff)
	}
}

func TestAccProjectQueryPaginatesEnvironmentClasses(t *testing.T) {
	t.Parallel()

	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	project := exampleQueryableProject()
	project.EnvironmentClasses = append(project.EnvironmentClasses, &v1.ProjectEnvironmentClass{
		EnvironmentClass: &v1.ProjectEnvironmentClass_LocalRunner{LocalRunner: true},
		Order:            1,
	})
	server.service.projects["project-1"] = project
	server.service.environmentClassPageSizeLimit = 1

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: projectIdentityQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_project.all", 1),
			querycheck.ExpectIdentity("ona_project.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("project-1")}),
		},
	}))

	wantRequests := []*v1.ListProjectEnvironmentClassesRequest{
		{Pagination: &v1.PaginationRequest{PageSize: 100}, ProjectId: "project-1"},
		{Pagination: &v1.PaginationRequest{PageSize: 100, Token: "1"}, ProjectId: "project-1"},
	}
	if diff := cmp.Diff(wantRequests, server.service.environmentClassListRequestsSnapshot(), protocmp.Transform()); diff != "" {
		t.Errorf("project environment class list requests mismatch (-want +got):\n%s", diff)
	}
}

func TestAccProjectQueryReportsListProjectsError(t *testing.T) {
	t.Parallel()

	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.listErr = errors.New("projects unavailable")

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      projectIdentityQueryConfig(),
		ExpectError: regexp.MustCompile("Unable to List Ona Projects"),
	}))
}

func TestAccProjectQueryReportsEnvironmentClassListError(t *testing.T) {
	t.Parallel()

	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)
	server.service.projects["project-1"] = exampleQueryableProject()
	server.service.environmentClassListErr = errors.New("environment classes unavailable")

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      projectIdentityQueryConfig(),
		ExpectError: regexp.MustCompile("Unable to List Ona Project Environment Classes"),
	}))
}

func TestAccProjectQueryReportsInsightsError(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	return queryableProject("project-1", "Example", "https://github.com/ona/example.git")
}

func queryableProject(id, name, repositoryCloneURL string) *v1.Project {
	return &v1.Project{
		Id: id,
		Metadata: &v1.ProjectMetadata{OrganizationId: "org-1", Name: name, Creator: &v1.Subject{
			Id: "user-1", Principal: v1.Principal_PRINCIPAL_USER,
		}},
		Initializer: &v1.EnvironmentInitializer{Specs: []*v1.EnvironmentInitializer_Spec{{
			Spec: &v1.EnvironmentInitializer_Spec_Git{Git: &v1.GitInitializer{RemoteUri: repositoryCloneURL, CloneTarget: "main"}},
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

func projectLimitedIdentityQueryConfig() string {
	return `
list "ona_project" "all" {
  provider = ona
  limit    = 2

  config {}
}
`
}

func projectRepositoryQueryConfig() string {
	return `
list "ona_project" "all" {
  provider = ona

  config {
    repository_clone_urls = ["https://github.com/ona/selected.git"]
  }
}
`
}

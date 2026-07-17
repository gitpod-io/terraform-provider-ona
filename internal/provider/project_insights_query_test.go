// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"connectrpc.com/connect"
	"context"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"net/http"
	"net/http/httptest"
	"testing"
)

const projectInsightsTestProjectID = "11111111-1111-4111-8111-111111111111"

type fakeInsightsProjectService struct {
	v1connect.UnimplementedProjectServiceHandler
}

func (s *fakeInsightsProjectService) ListProjects(ctx context.Context, req *connect.Request[v1.ListProjectsRequest]) (*connect.Response[v1.ListProjectsResponse], error) {
	return connect.NewResponse(&v1.ListProjectsResponse{Projects: []*v1.Project{{Id: projectInsightsTestProjectID, Metadata: &v1.ProjectMetadata{Name: "API"}}}}), nil
}
func TestAccProjectInsightsQuery(t *testing.T) {
	insights := &fakeInsightsService{enabled: map[string]bool{projectInsightsTestProjectID: true}, enableCalls: map[string]int{}, disableCalls: map[string]int{}}
	mux := http.NewServeMux()
	projectPath, projectHandler := v1connect.NewProjectServiceHandler(&fakeInsightsProjectService{})
	insightsPath, insightsHandler := v1connect.NewInsightsServiceHandler(insights)
	mux.Handle(projectPath, projectHandler)
	mux.Handle(insightsPath, insightsHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	t.Cleanup(server.Close)
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: projectInsightsQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_project_insights.all", 1), querycheck.ExpectIdentity("ona_project_insights.all", map[string]knownvalue.Check{"project_id": knownvalue.StringExact(projectInsightsTestProjectID)}), querycheck.ExpectResourceKnownValues("ona_project_insights.all", queryfilter.ByDisplayName(knownvalue.StringExact("API")), []querycheck.KnownValueCheck{{Path: tfjsonpath.New("project_id"), KnownValue: knownvalue.StringExact(projectInsightsTestProjectID)}, {Path: tfjsonpath.New("enabled"), KnownValue: knownvalue.Bool(true)}}),
	}}))
}
func projectInsightsQueryConfig() string {
	return `
list "ona_project_insights" "all" {
  provider         = ona
  include_resource = true
}
`
}

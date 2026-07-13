// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccProjectInsightsResourceLifecycle(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newProjectInsightsAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if got := server.service.disableCallsFor(projectInsightsTestProjectID); got != 2 {
				return fmt.Errorf("disable calls = %d, want 2", got)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccProjectInsightsResourceConfig(server.URL, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_project_insights.api", "id", projectInsightsTestProjectID),
					resource.TestCheckResourceAttr("ona_project_insights.api", "project_id", projectInsightsTestProjectID),
					resource.TestCheckResourceAttr("ona_project_insights.api", "enabled", "true"),
					resource.TestCheckResourceAttr("ona_project_insights.api", "last_ran_at", "2026-07-09T12:00:00Z"),
					resource.TestCheckResourceAttr("ona_project_insights.api", "data_collected_through", "2026-07-09T11:00:00Z"),
				),
			},
			{
				Config: testAccProjectInsightsResourceConfig(server.URL, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "ona_project_insights.api",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccProjectInsightsResourceConfig(server.URL, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_project_insights.api", "enabled", "false"),
					resource.TestCheckNoResourceAttr("ona_project_insights.api", "last_ran_at"),
					resource.TestCheckNoResourceAttr("ona_project_insights.api", "data_collected_through"),
				),
			},
			{
				Config: testAccProjectInsightsResourceConfig(server.URL, true),
				Check:  resource.TestCheckResourceAttr("ona_project_insights.api", "enabled", "true"),
			},
			{
				PreConfig: func() {
					server.service.forget(projectInsightsTestProjectID)
				},
				Config: testAccProjectInsightsResourceConfig(server.URL, true),
				Check:  resource.TestCheckResourceAttr("ona_project_insights.api", "enabled", "true"),
			},
		},
	})
}

const projectInsightsTestProjectID = "00000000-0000-0000-0000-000000000001"

func testAccProjectInsightsResourceConfig(host string, enabled bool) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_project_insights" "api" {
  project_id = %[2]q
  enabled    = %[3]t
}
`, host, projectInsightsTestProjectID, enabled)
}

type projectInsightsAPIServer struct {
	*httptest.Server
	service *fakeInsightsService
}

func newProjectInsightsAPIServer(t *testing.T) *projectInsightsAPIServer {
	t.Helper()

	service := &fakeInsightsService{
		enabled:      make(map[string]bool),
		enableCalls:  make(map[string]int),
		disableCalls: make(map[string]int),
	}
	path, handler := v1connect.NewInsightsServiceHandler(service)
	server := httptest.NewServer(http.StripPrefix("/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path || len(r.URL.Path) > len(path) && r.URL.Path[:len(path)] == path {
			handler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	return &projectInsightsAPIServer{Server: server, service: service}
}

type fakeInsightsService struct {
	v1connect.UnimplementedInsightsServiceHandler

	mu           sync.Mutex
	enabled      map[string]bool
	enableCalls  map[string]int
	disableCalls map[string]int
}

func (s *fakeInsightsService) EnableProjectInsights(ctx context.Context, req *connect.Request[v1.EnableProjectInsightsRequest]) (*connect.Response[v1.EnableProjectInsightsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	projectID := req.Msg.GetProjectId()
	s.enabled[projectID] = true
	s.enableCalls[projectID]++
	return connect.NewResponse(&v1.EnableProjectInsightsResponse{}), nil
}

func (s *fakeInsightsService) DisableProjectInsights(ctx context.Context, req *connect.Request[v1.DisableProjectInsightsRequest]) (*connect.Response[v1.DisableProjectInsightsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	projectID := req.Msg.GetProjectId()
	if _, ok := s.enabled[projectID]; !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	s.enabled[projectID] = false
	s.disableCalls[projectID]++
	return connect.NewResponse(&v1.DisableProjectInsightsResponse{}), nil
}

func (s *fakeInsightsService) GetProjectInsightsStatus(ctx context.Context, req *connect.Request[v1.GetProjectInsightsStatusRequest]) (*connect.Response[v1.GetProjectInsightsStatusResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	enabled, ok := s.enabled[req.Msg.GetProjectId()]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	result := &v1.GetProjectInsightsStatusResponse{Enabled: enabled}
	if enabled {
		result.LastRanAt = timestamppb.New(time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC))
		result.DataCollectedThrough = timestamppb.New(time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC))
	}
	return connect.NewResponse(result), nil
}

func (s *fakeInsightsService) disableCallsFor(projectID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disableCalls[projectID]
}

func (s *fakeInsightsService) forget(projectID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.enabled, projectID)
}

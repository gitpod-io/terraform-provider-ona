// Copyright IBM Corp. 2021, 2026
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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccProjectResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.deleted("project-1") {
				return errors.New("project-1 was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccProjectResourceConfig(server.URL, "acme-api", "class-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_project.api", "id", "project-1"),
					resource.TestCheckNoResourceAttr("ona_project.api", "organization_id"),
					resource.TestCheckResourceAttr("ona_project.api", "name", "acme-api"),
					resource.TestCheckResourceAttr("ona_project.api", "repository_clone_url", "https://github.com/acme/api.git"),
					resource.TestCheckResourceAttr("ona_project.api", "branch", "main"),
					resource.TestCheckResourceAttr("ona_project.api", "insights_enabled", "false"),
					resource.TestCheckNoResourceAttr("ona_project.api", "updated_at"),
					resource.TestCheckResourceAttr("ona_project.api", "devcontainer_file_path", ".devcontainer/devcontainer.json"),
					resource.TestCheckResourceAttr("ona_project.api", "automations_file_path", ".ona/automations.yaml"),
					resource.TestCheckResourceAttr("ona_project.api", "environment_class.0.environment_class_id", "class-1"),
					resource.TestCheckResourceAttr("ona_project.api", "environment_class.0.order", "0"),
					resource.TestCheckResourceAttr("ona_project.api", "creator.principal", "user"),
				),
			},
			{
				Config: testAccProjectResourceConfig(server.URL, "acme-api", "class-1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_project.api",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccProjectResourceConfigWithPrebuild(server.URL, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_project.api", "name", "acme-api-updated"),
					resource.TestCheckResourceAttr("ona_project.api", "repository_clone_url", "https://github.com/acme/api.git"),
					resource.TestCheckResourceAttr("ona_project.api", "branch", "stable"),
					resource.TestCheckResourceAttr("ona_project.api", "environment_class.0.local_runner", "true"),
					resource.TestCheckResourceAttr("ona_project.api", "prebuild_configuration.0.enabled", "true"),
					resource.TestCheckResourceAttr("ona_project.api", "prebuild_configuration.0.environment_class_ids.#", "1"),
					resource.TestCheckResourceAttr("ona_project.api", "prebuild_configuration.0.timeout", "30m0s"),
					resource.TestCheckResourceAttr("ona_project.api", "prebuild_configuration.0.daily_schedule.0.hour_utc", "3"),
					resource.TestCheckResourceAttr("ona_project.api", "prebuild_configuration.0.executor.0.id", "service-account-1"),
					resource.TestCheckResourceAttr("ona_project.api", "prebuild_configuration.0.executor.0.principal", "service_account"),
					resource.TestCheckResourceAttr("ona_project.api", "prebuild_configuration.0.enable_jetbrains_warmup", "true"),
					resource.TestCheckResourceAttr("ona_project.api", "insights_enabled", "true"),
				),
			},
			{
				Config: testAccProjectResourceConfigWithPrebuild(server.URL, false),
				Check:  resource.TestCheckResourceAttr("ona_project.api", "insights_enabled", "false"),
			},
		},
	})
}

func testAccProjectResourceConfig(host string, name string, environmentClassID string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_project" "api" {
  name                 = %[2]q
  repository_clone_url = "https://github.com/acme/api.git"
  branch               = "main"

  devcontainer_file_path = ".devcontainer/devcontainer.json"
  automations_file_path  = ".ona/automations.yaml"

  environment_class {
    environment_class_id = %[3]q
    order                = 0
  }
}
`, host, name, environmentClassID)
}

func testAccProjectResourceConfigWithPrebuild(host string, insightsEnabled bool) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_project" "api" {
  name                 = "acme-api-updated"
  repository_clone_url = "https://github.com/acme/api.git"
  branch               = "stable"
  insights_enabled     = %[2]t

  devcontainer_file_path = ".devcontainer/devcontainer.json"
  automations_file_path  = ".ona/automations.yaml"

  environment_class {
    local_runner = true
    order        = 0
  }

  prebuild_configuration {
    enabled               = true
    environment_class_ids = ["class-2"]
    timeout               = "30m0s"

    daily_schedule {
      hour_utc = 3
    }

    executor {
      id        = "service-account-1"
      principal = "service_account"
    }

    enable_jetbrains_warmup = true
  }
}
`, host, insightsEnabled)
}

type projectAPIServer struct {
	*httptest.Server
	service  *fakeProjectService
	insights *fakeInsightsService
}

func newProjectAPIServer(t *testing.T) *projectAPIServer {
	t.Helper()

	service := &fakeProjectService{
		projects: map[string]*v1.Project{},
		now:      time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC),
	}
	insights := &fakeInsightsService{
		enabled:      make(map[string]bool),
		enableCalls:  make(map[string]int),
		disableCalls: make(map[string]int),
	}
	projectPath, projectHandler := v1connect.NewProjectServiceHandler(service)
	insightsPath, insightsHandler := v1connect.NewInsightsServiceHandler(insights)
	server := httptest.NewServer(http.StripPrefix("/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == projectPath || len(r.URL.Path) > len(projectPath) && r.URL.Path[:len(projectPath)] == projectPath {
			projectHandler.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == insightsPath || len(r.URL.Path) > len(insightsPath) && r.URL.Path[:len(insightsPath)] == insightsPath {
			insightsHandler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	return &projectAPIServer{
		Server:   server,
		service:  service,
		insights: insights,
	}
}

type fakeProjectService struct {
	v1connect.UnimplementedProjectServiceHandler

	mu       sync.Mutex
	projects map[string]*v1.Project
	deletes  []string
	now      time.Time
}

func (s *fakeProjectService) CreateProject(ctx context.Context, req *connect.Request[v1.CreateProjectRequest]) (*connect.Response[v1.CreateProjectResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("project-%d", len(s.projects)+1)
	project := &v1.Project{
		Id: id,
		Metadata: &v1.ProjectMetadata{
			OrganizationId: "org-1",
			Name:           req.Msg.GetName(),
			Creator: &v1.Subject{
				Id:        "user-1",
				Principal: v1.Principal_PRINCIPAL_USER,
			},
			CreatedAt: timestamppb.New(s.now),
			UpdatedAt: timestamppb.New(s.now),
		},
		Initializer:           cloneEnvironmentInitializer(req.Msg.GetInitializer()),
		DevcontainerFilePath:  req.Msg.GetDevcontainerFilePath(),
		AutomationsFilePath:   req.Msg.GetAutomationsFilePath(),
		PrebuildConfiguration: clonePrebuildConfiguration(req.Msg.GetPrebuildConfiguration()),
	}
	s.projects[id] = project
	return connect.NewResponse(&v1.CreateProjectResponse{Project: cloneProject(project)}), nil
}

func (s *fakeProjectService) GetProject(ctx context.Context, req *connect.Request[v1.GetProjectRequest]) (*connect.Response[v1.GetProjectResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	project := s.projects[req.Msg.GetProjectId()]
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	return connect.NewResponse(&v1.GetProjectResponse{Project: cloneProject(project)}), nil
}

func (s *fakeProjectService) UpdateProject(ctx context.Context, req *connect.Request[v1.UpdateProjectRequest]) (*connect.Response[v1.UpdateProjectResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	project := s.projects[req.Msg.GetProjectId()]
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	if req.Msg.Name != nil {
		project.Metadata.Name = req.Msg.GetName()
	}
	if req.Msg.GetInitializer() != nil {
		project.Initializer = cloneEnvironmentInitializer(req.Msg.GetInitializer())
	}
	if req.Msg.DevcontainerFilePath != nil {
		project.DevcontainerFilePath = req.Msg.GetDevcontainerFilePath()
	}
	if req.Msg.AutomationsFilePath != nil {
		project.AutomationsFilePath = req.Msg.GetAutomationsFilePath()
	}
	if req.Msg.GetPrebuildConfiguration() != nil {
		if isDisabledPrebuildUpdate(req.Msg.GetPrebuildConfiguration()) {
			project.PrebuildConfiguration = nil
		} else {
			project.PrebuildConfiguration = clonePrebuildConfiguration(req.Msg.GetPrebuildConfiguration())
		}
	}
	project.Metadata.UpdatedAt = timestamppb.New(s.now.Add(time.Hour))
	return connect.NewResponse(&v1.UpdateProjectResponse{Project: cloneProject(project)}), nil
}

func (s *fakeProjectService) DeleteProject(ctx context.Context, req *connect.Request[v1.DeleteProjectRequest]) (*connect.Response[v1.DeleteProjectResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.projects[req.Msg.GetProjectId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	delete(s.projects, req.Msg.GetProjectId())
	s.deletes = append(s.deletes, req.Msg.GetProjectId())
	return connect.NewResponse(&v1.DeleteProjectResponse{}), nil
}

func (s *fakeProjectService) UpdateProjectEnvironmentClasses(ctx context.Context, req *connect.Request[v1.UpdateProjectEnvironmentClassesRequest]) (*connect.Response[v1.UpdateProjectEnvironmentClassesResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	project := s.projects[req.Msg.GetProjectId()]
	if project == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("project not found"))
	}
	project.EnvironmentClasses = cloneProjectEnvironmentClasses(req.Msg.GetProjectEnvironmentClasses())
	project.Metadata.UpdatedAt = timestamppb.New(s.now.Add(time.Hour))
	return connect.NewResponse(&v1.UpdateProjectEnvironmentClassesResponse{}), nil
}

func (s *fakeProjectService) deleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, deleted := range s.deletes {
		if deleted == id {
			return true
		}
	}
	return false
}

func cloneProject(project *v1.Project) *v1.Project {
	if project == nil {
		return nil
	}
	result := &v1.Project{}
	proto.Merge(result, project)
	return result
}

func cloneEnvironmentInitializer(initializer *v1.EnvironmentInitializer) *v1.EnvironmentInitializer {
	if initializer == nil {
		return nil
	}
	result := &v1.EnvironmentInitializer{}
	proto.Merge(result, initializer)
	return result
}

func clonePrebuildConfiguration(cfg *v1.ProjectPrebuildConfiguration) *v1.ProjectPrebuildConfiguration {
	if cfg == nil {
		return nil
	}
	result := &v1.ProjectPrebuildConfiguration{}
	proto.Merge(result, cfg)
	return result
}

func cloneProjectEnvironmentClasses(classes []*v1.ProjectEnvironmentClass) []*v1.ProjectEnvironmentClass {
	result := make([]*v1.ProjectEnvironmentClass, 0, len(classes))
	for _, class := range classes {
		clone := &v1.ProjectEnvironmentClass{}
		proto.Merge(clone, class)
		result = append(result, clone)
	}
	return result
}

func isDisabledPrebuildUpdate(cfg *v1.ProjectPrebuildConfiguration) bool {
	return !cfg.GetEnabled() &&
		len(cfg.GetEnvironmentClassIds()) == 0 &&
		cfg.GetTimeout() == nil &&
		cfg.GetTrigger() == nil &&
		cfg.GetExecutor() == nil &&
		!cfg.GetEnableJetbrainsWarmup()
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
	return connect.NewResponse(&v1.GetProjectInsightsStatusResponse{Enabled: enabled}), nil
}

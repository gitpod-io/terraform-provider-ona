// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"sync"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1/v1connect"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccRunnerResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t, nil)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.deleted("runner-1") {
				return errors.New("runner-1 was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerResourceConfig(server.URL, "Frankfurt Runner", "info"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner.test", "id", "runner-1"),
					resource.TestCheckResourceAttr("ona_runner.test", "runner_id", "runner-1"),
					resource.TestCheckResourceAttr("ona_runner.test", "name", "Frankfurt Runner"),
					resource.TestCheckResourceAttr("ona_runner.test", "runner_provider", "aws_ec2"),
					resource.TestCheckResourceAttr("ona_runner.test", "kind", "remote"),
					resource.TestCheckResourceAttr("ona_runner.test", "cloudformation_template_url", "https://gitpod-flex-releases.s3.amazonaws.com/ec2/stable/gitpod-ec2-runner.json"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.region", "eu-central-1"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.release_channel", "stable"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.log_level", "info"),
					resource.TestCheckNoResourceAttr("ona_runner.test", "status"),
					resource.TestCheckNoResourceAttr("ona_runner.test", "runner_manager_id"),
					resource.TestCheckNoResourceAttr("ona_runner.test", "updated_at"),
					resource.TestCheckResourceAttr("ona_runner.test", "creator.principal", "user"),
				),
			},
			{
				ResourceName:      "ona_runner.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:    "ona_runner.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected one imported runner state, got %d", len(states))
					}
					if states[0].ID != "runner-1" || states[0].Attributes["runner_id"] != "runner-1" {
						return fmt.Errorf("structured identity imported unexpected runner state: %#v", states[0].Attributes)
					}
					return nil
				},
			},
			{
				Config: testAccRunnerResourceConfig(server.URL, "Frankfurt Runner Updated", "debug"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner.test", "id", "runner-1"),
					resource.TestCheckResourceAttr("ona_runner.test", "name", "Frankfurt Runner Updated"),
					resource.TestCheckResourceAttr("ona_runner.test", "cloudformation_template_url", "https://gitpod-flex-releases.s3.amazonaws.com/ec2/stable/gitpod-ec2-runner.json"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.log_level", "debug"),
				),
			},
			{
				Config: testAccRunnerResourceConfigWithoutUpdateWindow(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner.test", "id", "runner-1"),
					resource.TestCheckResourceAttr("ona_runner.test", "name", "Frankfurt Runner Updated"),
					resource.TestCheckResourceAttr("ona_runner.test", "cloudformation_template_url", "https://gitpod-flex-releases.s3.amazonaws.com/ec2/stable/gitpod-ec2-runner.json"),
					resource.TestCheckNoResourceAttr("ona_runner.test", "configuration.update_window.start"),
					resource.TestCheckNoResourceAttr("ona_runner.test", "configuration.update_window.end"),
				),
			},
		},
	})
}

func TestAccRunnerResourceConfigurationDefaults(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t, nil)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerResourceConfigWithDefaults(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.release_channel", "stable"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.auto_update", "true"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.devcontainer_image_cache_enabled", "true"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.log_level", "info"),
					resource.TestCheckResourceAttr("ona_runner.test", "cloudformation_template_url", "https://gitpod-flex-releases.s3.amazonaws.com/ec2/stable/gitpod-ec2-runner.json"),
				),
			},
		},
	})
}

func TestAccRunnerResourceMetrics(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t, nil)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerResourceConfigWithManagedMetrics(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.metrics.managed.enabled", "true"),
					resource.TestCheckNoResourceAttr("ona_runner.test", "configuration.metrics.custom"),
					checkRunnerMetrics(server.service, &v1.MetricsConfiguration{ManagedMetricsEnabled: true}),
				),
			},
			{
				Config: testAccRunnerResourceConfigWithCustomMetrics(server.URL, "metrics-token-1", "1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ona_runner.test", "configuration.metrics.managed"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.metrics.custom.enabled", "true"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.metrics.custom.url", "https://metrics.example.com/api/v1/write"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.metrics.custom.username", "runner"),
					resource.TestCheckNoResourceAttr("ona_runner.test", "configuration.metrics.custom.password"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.metrics.custom.password_version", "1"),
					checkRunnerMetrics(server.service, &v1.MetricsConfiguration{
						Enabled:  true,
						Url:      "https://metrics.example.com/api/v1/write",
						Username: "runner",
						Password: "metrics-token-1",
					}),
				),
			},
			{
				Config: testAccRunnerResourceConfigWithCustomMetrics(server.URL, "metrics-token-2", "2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ona_runner.test", "configuration.metrics.custom.password"),
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.metrics.custom.password_version", "2"),
					checkRunnerMetrics(server.service, &v1.MetricsConfiguration{
						Enabled:  true,
						Url:      "https://metrics.example.com/api/v1/write",
						Username: "runner",
						Password: "metrics-token-2",
					}),
				),
			},
			{
				Config: testAccRunnerResourceConfigWithManagedMetrics(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.metrics.managed.enabled", "true"),
					resource.TestCheckNoResourceAttr("ona_runner.test", "configuration.metrics.custom"),
					checkRunnerMetrics(server.service, &v1.MetricsConfiguration{ManagedMetricsEnabled: true}),
				),
			},
			{
				Config: testAccRunnerResourceConfigWithDefaults(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ona_runner.test", "configuration.metrics"),
					checkRunnerMetrics(server.service, nil),
				),
			},
		},
	})
}

func TestAccRunnerResourceLatestCloudFormationTemplateURL(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t, nil)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerResourceConfigWithReleaseChannel(server.URL, "stable"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.release_channel", "stable"),
					resource.TestCheckResourceAttr("ona_runner.test", "cloudformation_template_url", "https://gitpod-flex-releases.s3.amazonaws.com/ec2/stable/gitpod-ec2-runner.json"),
				),
			},
			{
				Config: testAccRunnerResourceConfigWithReleaseChannel(server.URL, "latest"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner.test", "configuration.release_channel", "latest"),
					resource.TestCheckResourceAttr("ona_runner.test", "cloudformation_template_url", "https://gitpod-flex-releases.s3.amazonaws.com/ec2/latest/gitpod-ec2-runner.json"),
				),
			},
		},
	})
}

func TestAccRunnerResourceRequiresRegionForAWSEC2(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t, nil)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccRunnerResourceConfigWithoutRegion(server.URL, "aws_ec2"),
				ExpectError: regexp.MustCompile("Missing Runner Region"),
			},
		},
	})
}

func TestAccRunnerResourceUnauthenticatedDiagnostic(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t, nil)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccRunnerResourceConfigWithToken(server.URL, "wrong-token"),
				ExpectError: regexp.MustCompile("Ona rejected the API token[\\s\\S]*`ONA_TOKEN`[\\s\\S]*authorization header"),
			},
		},
	})
}

func TestAccRunnerResourceAllowsGCPWithoutRegion(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t, nil)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerResourceConfigWithoutRegion(server.URL, "gcp"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner.test", "runner_provider", "gcp"),
					resource.TestCheckNoResourceAttr("ona_runner.test", "cloudformation_template_url"),
					resource.TestCheckNoResourceAttr("ona_runner.test", "configuration.region"),
				),
			},
		},
	})
}

func testAccRunnerResourceConfigWithReleaseChannel(host string, releaseChannel string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner" "test" {
  name            = "Release Channel Runner"
  runner_provider = "aws_ec2"

  configuration {
    region          = "eu-central-1"
    release_channel = %[2]q
  }
}
`, host, releaseChannel)
}

func testAccRunnerResourceConfig(host string, name string, logLevel string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner" "test" {
  name            = %[2]q
  runner_provider = "aws_ec2"

  configuration {
    region                           = "eu-central-1"
    release_channel                  = "stable"
    auto_update                      = true
    devcontainer_image_cache_enabled = true
    log_level                        = %[3]q

    update_window {
      start = "02:00"
      end   = "04:00"
    }
  }
}
	`, host, name, logLevel)
}

func testAccRunnerResourceConfigWithDefaults(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner" "test" {
  name            = "Defaulted Runner"
  runner_provider = "aws_ec2"

  configuration {
    region = "eu-central-1"

    update_window {
      start = "02:00"
      end   = "04:00"
    }
  }
}
`, host)
}

func testAccRunnerResourceConfigWithManagedMetrics(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner" "test" {
  name            = "Metrics Runner"
  runner_provider = "aws_ec2"

  configuration {
    region = "eu-central-1"

    metrics {
      managed {
        enabled = true
      }
    }
  }
}
`, host)
}

func testAccRunnerResourceConfigWithCustomMetrics(host string, password string, passwordVersion string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner" "test" {
  name            = "Metrics Runner"
  runner_provider = "aws_ec2"

  configuration {
    region = "eu-central-1"

    metrics {
      custom {
        enabled  = true
        url      = "https://metrics.example.com/api/v1/write"
		username = "runner"
		password = %[2]q
		password_version = %[3]q
      }
    }
  }
}
	`, host, password, passwordVersion)
}

func testAccRunnerResourceConfigWithToken(host string, token string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = %[2]q
}

resource "ona_runner" "test" {
  name            = "Unauthenticated Runner"
  runner_provider = "aws_ec2"

  configuration {
    region = "eu-central-1"
  }
}
`, host, token)
}

func testAccRunnerResourceConfigWithoutUpdateWindow(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner" "test" {
  name            = "Frankfurt Runner Updated"
  runner_provider = "aws_ec2"

  configuration {
    region                           = "eu-central-1"
    release_channel                  = "stable"
    auto_update                      = true
    devcontainer_image_cache_enabled = true
    log_level                        = "debug"
  }
}
`, host)
}

func testAccRunnerResourceConfigWithoutRegion(host string, provider string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner" "test" {
  name            = "Runner Without Region"
  runner_provider = %[2]q

  configuration {
    update_window {
      start = "02:00"
      end   = "04:00"
    }
  }
}
`, host, provider)
}

type runnerAPIServer struct {
	*httptest.Server
	service *fakeRunnerService
}

func newRunnerAPIServer(t *testing.T, runners map[string]*v1.Runner) *runnerAPIServer {
	t.Helper()

	service := &fakeRunnerService{
		runners: runners,
	}
	if service.runners == nil {
		service.runners = map[string]*v1.Runner{}
	}

	_, handler := v1connect.NewRunnerServiceHandler(service)
	server := httptest.NewServer(http.StripPrefix("/api", handler))
	return &runnerAPIServer{
		Server:  server,
		service: service,
	}
}

type fakeRunnerService struct {
	v1connect.UnimplementedRunnerServiceHandler

	mu            sync.Mutex
	runners       map[string]*v1.Runner
	policies      map[string]*v1.RunnerPolicy
	deletes       []string
	policyDeletes []string
	tokens        []string
}

func (s *fakeRunnerService) CreateRunner(ctx context.Context, req *connect.Request[v1.CreateRunnerRequest]) (*connect.Response[v1.CreateRunnerResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if got := req.Header().Get("Authorization"); got != "Bearer test-token" {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authorization header = %q, want Bearer test-token", got))
	}

	id := fmt.Sprintf("runner-%d", len(s.runners)+1)
	runner := newTestRunner(id, req.Msg.GetName())
	runner.Provider = req.Msg.GetProvider()
	runner.RunnerManagerId = req.Msg.GetRunnerManagerId()
	runner.Spec = req.Msg.GetSpec()
	if runner.Spec == nil {
		runner.Spec = &v1.RunnerSpec{}
	}
	if runner.Spec.Configuration == nil {
		runner.Spec.Configuration = &v1.RunnerConfiguration{}
	}
	runner.Status.Region = runner.Spec.Configuration.GetRegion()

	s.runners[id] = runner
	return connect.NewResponse(&v1.CreateRunnerResponse{Runner: runner}), nil
}

func (s *fakeRunnerService) GetRunner(ctx context.Context, req *connect.Request[v1.GetRunnerRequest]) (*connect.Response[v1.GetRunnerResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runner := s.runners[req.Msg.GetRunnerId()]
	if runner == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("runner not found"))
	}
	result := cloneRunner(runner)
	if result.GetSpec().GetConfiguration().GetMetrics() != nil {
		result.Spec.Configuration.Metrics.Password = ""
	}
	return connect.NewResponse(&v1.GetRunnerResponse{Runner: result}), nil
}

func (s *fakeRunnerService) ListRunners(ctx context.Context, req *connect.Request[v1.ListRunnersRequest]) (*connect.Response[v1.ListRunnersResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runners := make([]*v1.Runner, 0, len(s.runners))
	for _, runner := range s.runners {
		filter := req.Msg.GetFilter()
		if len(filter.GetCreatorIds()) > 0 && !slices.Contains(filter.GetCreatorIds(), runner.GetCreator().GetId()) {
			continue
		}
		if len(filter.GetKinds()) > 0 && !slices.Contains(filter.GetKinds(), runner.GetKind()) {
			continue
		}
		if len(filter.GetProviders()) > 0 && !slices.Contains(filter.GetProviders(), runner.GetProvider()) {
			continue
		}
		runners = append(runners, cloneRunner(runner))
	}
	return connect.NewResponse(&v1.ListRunnersResponse{Runners: runners}), nil
}

func (s *fakeRunnerService) UpdateRunner(ctx context.Context, req *connect.Request[v1.UpdateRunnerRequest]) (*connect.Response[v1.UpdateRunnerResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runner := s.runners[req.Msg.GetRunnerId()]
	if runner == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("runner not found"))
	}
	if req.Msg.Name != nil {
		runner.Name = req.Msg.GetName()
	}
	if config := req.Msg.GetSpec().GetConfiguration(); config != nil {
		applyUpdate(runner.Spec.GetConfiguration(), config)
	}
	runner.UpdatedAt = timestamppb.Now()
	return connect.NewResponse(&v1.UpdateRunnerResponse{}), nil
}

func (s *fakeRunnerService) DeleteRunner(ctx context.Context, req *connect.Request[v1.DeleteRunnerRequest]) (*connect.Response[v1.DeleteRunnerResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runners[req.Msg.GetRunnerId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("runner not found"))
	}
	delete(s.runners, req.Msg.GetRunnerId())
	s.deletes = append(s.deletes, req.Msg.GetRunnerId())
	return connect.NewResponse(&v1.DeleteRunnerResponse{}), nil
}

func (s *fakeRunnerService) CreateRunnerToken(ctx context.Context, req *connect.Request[v1.CreateRunnerTokenRequest]) (*connect.Response[v1.CreateRunnerTokenResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runners[req.Msg.GetRunnerId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("runner not found"))
	}

	token := "exchange-token-" + req.Msg.GetRunnerId()
	s.tokens = append(s.tokens, token)
	return connect.NewResponse(&v1.CreateRunnerTokenResponse{ExchangeToken: token}), nil
}

func (s *fakeRunnerService) deleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, deleted := range s.deletes {
		if deleted == id {
			return true
		}
	}
	return false
}

func (s *fakeRunnerService) tokenCreated(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, created := range s.tokens {
		if created == token {
			return true
		}
	}
	return false
}

func (s *fakeRunnerService) tokenCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.tokens)
}

func (s *fakeRunnerService) metrics(id string) *v1.MetricsConfiguration {
	s.mu.Lock()
	defer s.mu.Unlock()

	runner := s.runners[id]
	if runner == nil || runner.GetSpec().GetConfiguration().GetMetrics() == nil {
		return nil
	}
	return proto.CloneOf(runner.GetSpec().GetConfiguration().GetMetrics())
}

func checkRunnerMetrics(service *fakeRunnerService, expected *v1.MetricsConfiguration) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		got := service.metrics("runner-1")
		if diff := cmp.Diff(expected, got, protocmp.Transform()); diff != "" {
			return fmt.Errorf("runner metrics mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
}

func newTestRunner(id string, name string) *v1.Runner {
	return &v1.Runner{
		RunnerId:  id,
		Name:      name,
		Kind:      v1.RunnerKind_RUNNER_KIND_REMOTE,
		Provider:  v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2,
		CreatedAt: timestamppb.Now(),
		UpdatedAt: timestamppb.Now(),
		Creator: &v1.Subject{
			Id:        "creator-1",
			Principal: v1.Principal_PRINCIPAL_USER,
		},
		Status: &v1.RunnerStatus{
			Phase:   v1.RunnerPhase_RUNNER_PHASE_ACTIVE,
			Version: "1.2.3",
		},
		Spec: &v1.RunnerSpec{
			Configuration: &v1.RunnerConfiguration{},
		},
	}
}

func cloneRunner(runner *v1.Runner) *v1.Runner {
	cloned, ok := proto.Clone(runner).(*v1.Runner)
	if !ok {
		return nil
	}
	return cloned
}

func applyUpdate(target *v1.RunnerConfiguration, update *v1.UpdateRunnerRequest_RunnerConfiguration) {
	if update.ReleaseChannel != nil {
		target.ReleaseChannel = update.GetReleaseChannel()
	}
	if update.AutoUpdate != nil {
		target.AutoUpdate = update.GetAutoUpdate()
	}
	if update.Metrics != nil {
		if target.Metrics == nil {
			target.Metrics = &v1.MetricsConfiguration{}
		}
		if update.Metrics.Enabled != nil {
			target.Metrics.Enabled = update.Metrics.GetEnabled()
		}
		if update.Metrics.Url != nil {
			target.Metrics.Url = update.Metrics.GetUrl()
		}
		if update.Metrics.Username != nil {
			target.Metrics.Username = update.Metrics.GetUsername()
		}
		if update.Metrics.Password != nil {
			target.Metrics.Password = update.Metrics.GetPassword()
		}
		if update.Metrics.ManagedMetricsEnabled != nil {
			target.Metrics.ManagedMetricsEnabled = update.Metrics.GetManagedMetricsEnabled()
		}
		if !target.Metrics.GetEnabled() &&
			target.Metrics.GetUrl() == "" &&
			target.Metrics.GetUsername() == "" &&
			target.Metrics.GetPassword() == "" &&
			!target.Metrics.GetManagedMetricsEnabled() {
			target.Metrics = nil
		}
	}
	if update.LogLevel != nil {
		target.LogLevel = update.GetLogLevel()
	}
	if update.DevcontainerImageCacheEnabled != nil {
		target.DevcontainerImageCacheEnabled = update.GetDevcontainerImageCacheEnabled()
	}
	if update.UpdateWindow != nil {
		target.UpdateWindow = update.GetUpdateWindow()
	}
}

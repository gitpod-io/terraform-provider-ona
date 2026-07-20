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
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
)

func TestAccIntegrationResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newIntegrationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedDefinition(testIntegrationDefinition("definition-b", "Example", "example.com"))
	server.service.seedDefinition(testIntegrationDefinition("definition-a", "Alpha", "alpha.example.com"))
	server.service.seedDefinition(testIntegrationDefinition("definition-c", "Charlie", "charlie.example.com"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if remaining := server.service.remaining(); len(remaining) > 0 {
				return fmt.Errorf("integrations were not deleted: %v", remaining)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccDefinitionBackedIntegrationConfig(server.URL, false, "https://mcp.example.com/v1", "client-v1", "secret-v1", "v1", "mcp"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_integration.test", "id", "integration-1"),
					resource.TestCheckResourceAttr("ona_integration.test", "organization_id", "organization-1"),
					resource.TestCheckResourceAttr("ona_integration.test", "integration_definition_id", "definition-b"),
					resource.TestCheckResourceAttr("ona_integration.test", "enabled", "false"),
					resource.TestCheckResourceAttr("ona_integration.test", "name", "Example"),
					resource.TestCheckResourceAttr("ona_integration.test", "capabilities.mcp.url", "https://mcp.example.com/v1"),
					resource.TestCheckResourceAttr("ona_integration.test", "auth.oauth.client_id", "client-v1"),
					resource.TestCheckResourceAttr("ona_integration.test", "auth.oauth.client_secret_version", "v1"),
					resource.TestCheckNoResourceAttr("ona_integration.test", "credentials"),
					resource.TestCheckResourceAttr("data.ona_integration_definitions.all", "definitions.#", "3"),
					resource.TestCheckResourceAttr("data.ona_integration_definitions.all", "definitions.0.id", "definition-a"),
					resource.TestCheckResourceAttr("data.ona_integration_definitions.all", "definitions.1.id", "definition-b"),
					resource.TestCheckResourceAttr("data.ona_integration_definitions.all", "definitions.2.id", "definition-c"),
					func(*terraform.State) error {
						if got := server.service.lastOAuthSecret("integration-1"); got != "secret-v1" {
							return fmt.Errorf("created OAuth secret = %q, want secret-v1", got)
						}
						return nil
					},
				),
			},
			{
				Config: testAccDefinitionBackedIntegrationConfig(server.URL, false, "https://mcp.example.com/v1", "client-v1", "secret-v1", "v1", "mcp"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccDefinitionBackedIntegrationConfig(server.URL, true, "https://mcp.example.com/v2", "client-v2", "secret-v2", "v2", "ai"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_integration.test", "id", "integration-1"),
					resource.TestCheckResourceAttr("ona_integration.test", "enabled", "true"),
					resource.TestCheckResourceAttr("ona_integration.test", "capabilities.mcp.url", "https://mcp.example.com/v2"),
					resource.TestCheckResourceAttr("ona_integration.test", "auth.oauth.client_id", "client-v2"),
					resource.TestCheckResourceAttr("ona_integration.test", "auth.oauth.client_secret_version", "v2"),
					func(*terraform.State) error {
						if got := server.service.lastOAuthSecret("integration-1"); got != "secret-v2" {
							return fmt.Errorf("updated OAuth secret = %q, want secret-v2", got)
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "ona_integration.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"auth.oauth.client_secret_version"},
			},
			{
				PreConfig: func() { server.service.setEnabled("integration-1", false) },
				Config:    testAccDefinitionBackedIntegrationConfig(server.URL, true, "https://mcp.example.com/v2", "client-v2", "secret-v2", "v2", "ai"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("ona_integration.test", "enabled", "true"),
			},
			{
				PreConfig: func() { server.service.remove("integration-1") },
				Config:    testAccDefinitionBackedIntegrationConfig(server.URL, true, "https://mcp.example.com/v2", "client-v2", "secret-v2", "v2", "ai"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("ona_integration.test", "id", "integration-2"),
			},
		},
	})
}

func TestAccCustomIntegrationReplacement(t *testing.T) {
	t.Parallel()

	server := newIntegrationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCustomIntegrationConfig(server.URL, "https://mcp.custom.example.com/v1", "secret-v1", "v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_integration.custom", "id", "integration-1"),
					resource.TestCheckResourceAttr("ona_integration.custom", "host", "custom.example.com"),
					resource.TestCheckNoResourceAttr("ona_integration.custom", "credentials"),
				),
			},
			{
				Config: testAccCustomIntegrationConfig(server.URL, "https://mcp.custom.example.com/v1", "secret-v2", "v2"),
				Check:  resource.TestCheckResourceAttr("ona_integration.custom", "id", "integration-2"),
			},
			{
				Config: testAccCustomIntegrationConfig(server.URL, "https://mcp.custom.example.com/v2", "secret-v2", "v2"),
				Check:  resource.TestCheckResourceAttr("ona_integration.custom", "id", "integration-3"),
			},
		},
	})
}

func TestAccIntegrationPermissionDiagnostic(t *testing.T) {
	t.Parallel()

	server := newIntegrationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedDefinition(testIntegrationDefinition("definition-b", "Example", "example.com"))
	server.service.denyCreate = true

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccDefinitionBackedIntegrationConfig(server.URL, false, "https://mcp.example.com/v1", "client-v1", "secret-v1", "v1", "mcp"),
			ExpectError: regexp.MustCompile(`Unable to Create Ona Integration[\s\S]*permission denied`),
		}},
	})
}

func TestAccCustomIntegrationDynamicRegistration(t *testing.T) {
	t.Parallel()

	server := newIntegrationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccCustomDynamicIntegrationConfig(server.URL),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("ona_integration.custom", "id", "integration-1"),
				resource.TestCheckResourceAttr("ona_integration.custom", "auth.oauth.dynamic_registration", "true"),
				resource.TestCheckNoResourceAttr("ona_integration.custom", "credentials"),
			),
		}},
	})
}

func TestAccDefinitionBackedCategoryClearingDiagnostic(t *testing.T) {
	t.Parallel()

	server := newIntegrationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedDefinition(testIntegrationDefinition("definition-b", "Example", "example.com"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDefinitionBackedIntegrationConfig(server.URL, false, "https://mcp.example.com/v1", "client-v1", "secret-v1", "v1", "mcp"),
			},
			{
				Config:      testAccDefinitionBackedIntegrationWithoutCategoriesConfig(server.URL),
				ExpectError: regexp.MustCompile("Unable to Clear Definition-Backed Integration Categories"),
			},
		},
	})
}

func testAccDefinitionBackedIntegrationConfig(host string, enabled bool, mcpURL, clientID, clientSecret, version, category string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

data "ona_integration_definitions" "all" {}

resource "ona_integration" "test" {
  integration_definition_id = "definition-b"
  enabled                   = %[2]t
  categories                = [%[7]q]

  capabilities = {
    mcp = {
      url = %[3]q
    }
  }

  auth = {
    oauth = {
      client_id             = %[4]q
      client_secret_version = %[6]q
    }
  }

  credentials = {
    oauth_client_secret = %[5]q
  }
}
`, host, enabled, mcpURL, clientID, clientSecret, version, category)
}

func testAccCustomIntegrationConfig(host, mcpURL, clientSecret, version string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_integration" "custom" {
  name = "Custom MCP"

  capabilities = {
    mcp = {
      url = %[2]q
    }
  }

  auth = {
    oauth = {
      client_id             = "custom-client"
      client_secret_version = %[4]q
    }
  }

  credentials = {
    oauth_client_secret = %[3]q
  }
}
`, host, mcpURL, clientSecret, version)
}

func testAccCustomDynamicIntegrationConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_integration" "custom" {
  name = "Dynamic MCP"

  capabilities = {
    mcp = {
      url = "https://mcp.dynamic.example.com/mcp"
    }
  }

  auth = {
    oauth = {
      dynamic_registration = true
    }
  }
}
`, host)
}

func testAccDefinitionBackedIntegrationWithoutCategoriesConfig(host string) string {
	config := testAccDefinitionBackedIntegrationConfig(host, false, "https://mcp.example.com/v1", "client-v1", "secret-v1", "v1", "mcp")
	return strings.Replace(config, `categories                = ["mcp"]`, `categories                = []`, 1)
}

type integrationAPIServer struct {
	*httptest.Server
	service *fakeIntegrationService
}

func newIntegrationAPIServer(t *testing.T) *integrationAPIServer {
	t.Helper()
	service := &fakeIntegrationService{
		definitions:  map[string]*v1.IntegrationDefinition{},
		integrations: map[string]*v1.Integration{},
	}
	servicePath, handler := v1connect.NewIntegrationServiceHandler(service)
	server := httptest.NewServer(http.StripPrefix("/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == servicePath || strings.HasPrefix(r.URL.Path, servicePath) {
			handler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	return &integrationAPIServer{Server: server, service: service}
}

type fakeIntegrationService struct {
	v1connect.UnimplementedIntegrationServiceHandler

	mu           sync.Mutex
	definitions  map[string]*v1.IntegrationDefinition
	integrations map[string]*v1.Integration
	nextID       int
	denyCreate   bool
}

func (s *fakeIntegrationService) ListIntegrationDefinitions(ctx context.Context, req *connect.Request[v1.ListIntegrationDefinitionsRequest]) (*connect.Response[v1.ListIntegrationDefinitionsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	definitions := make([]*v1.IntegrationDefinition, 0, len(s.definitions))
	for _, definition := range s.definitions {
		definitions = append(definitions, censorDefinition(cloneDefinition(definition)))
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].GetId() > definitions[j].GetId() })
	offset := 0
	if req.Msg.GetPagination().GetToken() != "" {
		var err error
		offset, err = strconv.Atoi(req.Msg.GetPagination().GetToken())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	pageSize := int(req.Msg.GetPagination().GetPageSize())
	if pageSize <= 0 || pageSize > 2 {
		pageSize = 2
	}
	end := offset + pageSize
	if end > len(definitions) {
		end = len(definitions)
	}
	nextToken := ""
	if end < len(definitions) {
		nextToken = strconv.Itoa(end)
	}
	return connect.NewResponse(&v1.ListIntegrationDefinitionsResponse{
		Definitions: definitions[offset:end],
		Pagination:  &v1.PaginationResponse{NextToken: nextToken},
	}), nil
}

func (s *fakeIntegrationService) CreateIntegration(ctx context.Context, req *connect.Request[v1.CreateIntegrationRequest]) (*connect.Response[v1.CreateIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.denyCreate {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("permission denied: integration:create is required"))
	}
	if req.Msg.GetIntegrationDefinitionId() != "" && s.definitions[req.Msg.GetIntegrationDefinitionId()] == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("integration definition not found"))
	}
	s.nextID++
	id := fmt.Sprintf("integration-%d", s.nextID)
	integration := &v1.Integration{
		Id:                      id,
		OrganizationId:          "organization-1",
		IntegrationDefinitionId: req.Msg.GetIntegrationDefinitionId(),
		RunnerId:                req.Msg.GetRunnerId(),
		Enabled:                 req.Msg.GetEnabled(),
		Capabilities:            proto.CloneOf(req.Msg.GetCapabilities()),
		Auth:                    proto.CloneOf(req.Msg.GetAuth()),
		Host:                    req.Msg.GetHost(),
		Name:                    req.Msg.GetName(),
		Description:             req.Msg.GetDescription(),
		Categories:              append([]v1.IntegrationCategory(nil), req.Msg.GetCategories()...),
	}
	if integration.GetIntegrationDefinitionId() == "" && integration.GetHost() == "" {
		integration.Host = "custom.example.com"
	}
	s.integrations[id] = integration
	return connect.NewResponse(&v1.CreateIntegrationResponse{Integration: s.resolvedLocked(integration)}), nil
}

func (s *fakeIntegrationService) GetIntegration(ctx context.Context, req *connect.Request[v1.GetIntegrationRequest]) (*connect.Response[v1.GetIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	integration := s.integrations[req.Msg.GetId()]
	if integration == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("integration not found"))
	}
	return connect.NewResponse(&v1.GetIntegrationResponse{Integration: s.resolvedLocked(integration)}), nil
}

func (s *fakeIntegrationService) UpdateIntegration(ctx context.Context, req *connect.Request[v1.UpdateIntegrationRequest]) (*connect.Response[v1.UpdateIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	integration := s.integrations[req.Msg.GetId()]
	if integration == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("integration not found"))
	}
	if req.Msg.Enabled != nil {
		integration.Enabled = req.Msg.GetEnabled()
	}
	if req.Msg.Capabilities != nil {
		integration.Capabilities = proto.CloneOf(req.Msg.GetCapabilities())
	}
	if req.Msg.Auth != nil {
		integration.Auth = proto.CloneOf(req.Msg.GetAuth())
	}
	if len(req.Msg.GetCategories()) > 0 {
		integration.Categories = append([]v1.IntegrationCategory(nil), req.Msg.GetCategories()...)
	}
	return connect.NewResponse(&v1.UpdateIntegrationResponse{Integration: s.resolvedLocked(integration)}), nil
}

func (s *fakeIntegrationService) DeleteIntegration(ctx context.Context, req *connect.Request[v1.DeleteIntegrationRequest]) (*connect.Response[v1.DeleteIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.integrations[req.Msg.GetId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("integration not found"))
	}
	delete(s.integrations, req.Msg.GetId())
	return connect.NewResponse(&v1.DeleteIntegrationResponse{}), nil
}

func (s *fakeIntegrationService) seedDefinition(definition *v1.IntegrationDefinition) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.definitions[definition.GetId()] = cloneDefinition(definition)
}

func (s *fakeIntegrationService) resolvedLocked(integration *v1.Integration) *v1.Integration {
	resolved := proto.CloneOf(integration)
	if definition := s.definitions[integration.GetIntegrationDefinitionId()]; definition != nil {
		if resolved.Name == "" {
			resolved.Name = definition.GetName()
		}
		if resolved.Description == "" {
			resolved.Description = definition.GetDescription()
		}
		if resolved.IconUrl == "" {
			resolved.IconUrl = definition.GetIconUrl()
		}
		if resolved.Host == "" {
			resolved.Host = definition.GetHost()
		}
		if resolved.Capabilities == nil {
			resolved.Capabilities = cloneCapabilities(definition.GetCapabilities())
		}
		if resolved.Auth == nil {
			resolved.Auth = cloneAuth(definition.GetAuth())
		}
		if len(resolved.Categories) == 0 {
			resolved.Categories = append([]v1.IntegrationCategory(nil), definition.GetCategories()...)
		}
	}
	return censorIntegration(resolved)
}

func (s *fakeIntegrationService) lastOAuthSecret(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.integrations[id].GetAuth().GetOauth().GetClientSecret()
}

func (s *fakeIntegrationService) setEnabled(id string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if integration := s.integrations[id]; integration != nil {
		integration.Enabled = enabled
	}
}

func (s *fakeIntegrationService) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.integrations, id)
}

func (s *fakeIntegrationService) remaining() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.integrations))
	for id := range s.integrations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func testIntegrationDefinition(id, name, host string) *v1.IntegrationDefinition {
	return &v1.IntegrationDefinition{
		Id:          id,
		Name:        name,
		Description: name + " integration",
		IconUrl:     "https://icons.example.com/" + id + ".png",
		Host:        host,
		Capabilities: &v1.IntegrationCapabilities{
			Mcp: &v1.IntegrationMCPCapability{Url: "https://mcp." + host + "/mcp"},
		},
		Auth: &v1.IntegrationAuthentication{
			RequiresAuth: true,
			Oauth:        &v1.IntegrationOAuthConfig{ClientId: "definition-client", ClientSecret: "definition-secret"},
		},
		Categories: []v1.IntegrationCategory{v1.IntegrationCategory_INTEGRATION_CATEGORY_MCP},
	}
}

func cloneDefinition(value *v1.IntegrationDefinition) *v1.IntegrationDefinition {
	if value == nil {
		return nil
	}
	return proto.CloneOf(value)
}

func cloneCapabilities(value *v1.IntegrationCapabilities) *v1.IntegrationCapabilities {
	if value == nil {
		return nil
	}
	return proto.CloneOf(value)
}

func cloneAuth(value *v1.IntegrationAuthentication) *v1.IntegrationAuthentication {
	if value == nil {
		return nil
	}
	return proto.CloneOf(value)
}

func censorDefinition(value *v1.IntegrationDefinition) *v1.IntegrationDefinition {
	if value.GetAuth().GetOauth() != nil {
		value.Auth.Oauth.ClientSecret = ""
	}
	return value
}

func censorIntegration(value *v1.Integration) *v1.Integration {
	if value.GetAuth().GetOauth() != nil {
		value.Auth.Oauth.ClientSecret = ""
	}
	return value
}

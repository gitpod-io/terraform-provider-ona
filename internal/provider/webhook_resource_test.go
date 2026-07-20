// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccWebhookResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newWebhookAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if remaining := server.service.remaining(); len(remaining) > 0 {
				return fmt.Errorf("webhooks were not deleted: %v", remaining)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookRepositoryConfig(server.URL, "Deployments", "Deployment events", "terraform-provider-ona", "v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_webhook.deployments", "id", webhookID1),
					resource.TestCheckResourceAttr("ona_webhook.deployments", "name", "Deployments"),
					resource.TestCheckResourceAttr("ona_webhook.deployments", "description", "Deployment events"),
					resource.TestCheckResourceAttr("ona_webhook.deployments", "type", "repository"),
					resource.TestCheckResourceAttr("ona_webhook.deployments", "scm_provider", "github"),
					resource.TestCheckResourceAttr("ona_webhook.deployments", "repository_scopes.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("ona_webhook.deployments", "repository_scopes.*", map[string]string{
						"host":  "github.com",
						"owner": "gitpod-io",
						"name":  "terraform-provider-ona",
					}),
					resource.TestCheckNoResourceAttr("ona_webhook.deployments", "organization_id"),
					resource.TestCheckNoResourceAttr("ona_webhook.deployments", "bound_workflow_count"),
					resource.TestCheckNoResourceAttr("ona_webhook.deployments", "last_triggered_at"),
					resource.TestCheckNoResourceAttr("ona_webhook.deployments", "updated_at"),
					resource.TestCheckResourceAttr("ona_webhook.deployments", "secret_version", "v1"),
				),
			},
			{
				Config: testAccWebhookRepositoryConfig(server.URL, "Deployments", "Deployment events", "terraform-provider-ona", "v1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:            "ona_webhook.deployments",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret_version"},
			},
			{
				Config: testAccWebhookRepositoryConfig(server.URL, "Deployments Updated", "Updated events", "terraform-provider", "v2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_webhook.deployments", "name", "Deployments Updated"),
					resource.TestCheckResourceAttr("ona_webhook.deployments", "description", "Updated events"),
					resource.TestCheckTypeSetElemNestedAttrs("ona_webhook.deployments", "repository_scopes.*", map[string]string{
						"name": "terraform-provider",
					}),
					resource.TestCheckResourceAttr("ona_webhook.deployments", "secret_version", "v2"),
					func(*terraform.State) error {
						if got := server.service.rotationCount(webhookID1); got != 1 {
							return fmt.Errorf("webhook secret rotation count = %d, want 1", got)
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					server.service.updateName(webhookID1, "Out-of-band name")
					server.service.updateDescription(webhookID1, "")
				},
				Config: testAccWebhookRepositoryConfig(server.URL, "Deployments Updated", "Updated events", "terraform-provider", "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_webhook.deployments", "name", "Deployments Updated"),
					resource.TestCheckResourceAttr("ona_webhook.deployments", "description", "Updated events"),
				),
			},
			{
				PreConfig: func() {
					server.service.remove(webhookID1)
				},
				Config: testAccWebhookRepositoryConfig(server.URL, "Deployments Updated", "Updated events", "terraform-provider", "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("ona_webhook.deployments", "id", webhookID2),
			},
		},
	})
}

func TestAccWebhookOrganizationResource(t *testing.T) {
	t.Parallel()

	server := newWebhookAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookOrganizationConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_webhook.organization", "type", "organization"),
					resource.TestCheckResourceAttr("ona_webhook.organization", "scm_provider", "gitlab"),
					resource.TestCheckResourceAttr("ona_webhook.organization", "organization_scope.host", "gitlab.com"),
					resource.TestCheckResourceAttr("ona_webhook.organization", "organization_scope.name", "gitpod-io"),
					resource.TestCheckNoResourceAttr("ona_webhook.organization", "repository_scopes.#"),
				),
			},
		},
	})
}

func TestAccWebhookSecretEphemeralResource(t *testing.T) {
	t.Parallel()

	server := newWebhookAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed(newTestWebhook(webhookID1, "Webhook", v1.WebhookType_WEBHOOK_TYPE_SCM_REPOSITORY, v1.WebhookProvider_WEBHOOK_PROVIDER_GITHUB), "webhook-secret")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookSecretConfig(server.URL),
				Check: func(state *terraform.State) error {
					if got := server.service.secretReadCount(webhookID1); got == 0 {
						return errors.New("webhook secret was not retrieved")
					}
					if state.RootModule().Resources["ona_webhook_secret.test"] != nil {
						return errors.New("ephemeral webhook secret was stored as managed resource state")
					}
					return nil
				},
			},
		},
	})
}

func TestAccWebhookSecretRotationFeedsEphemeralValue(t *testing.T) {
	t.Parallel()

	server := newWebhookAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWebhookRotationConfig(server.URL, "v1"),
				Check: func(*terraform.State) error {
					if got := server.service.lastSecretRead(webhookID1); got != "secret-1" {
						return fmt.Errorf("ephemeral secret after create = %q, want %q", got, "secret-1")
					}
					return nil
				},
			},
			{
				Config: testAccWebhookRotationConfig(server.URL, "v2"),
				Check: func(*terraform.State) error {
					if got := server.service.rotationCount(webhookID1); got != 1 {
						return fmt.Errorf("webhook secret rotation count = %d, want 1", got)
					}
					if got := server.service.lastSecretRead(webhookID1); got != "secret-2" {
						return fmt.Errorf("ephemeral secret after rotation = %q, want %q", got, "secret-2")
					}
					return nil
				},
			},
		},
	})
}

func testAccWebhookRepositoryConfig(host, name, description, repository, secretVersion string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_webhook" "deployments" {
  name           = %[2]q
  description    = %[3]q
  type           = "repository"
  scm_provider   = "github"
  secret_version = %[5]q

  repository_scopes = [
    {
      host  = "github.com"
      owner = "gitpod-io"
      name  = %[4]q
    }
  ]
}
`, host, name, description, repository, secretVersion)
}

func testAccWebhookOrganizationConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_webhook" "organization" {
  name     = "GitLab organization"
  type     = "organization"
  scm_provider = "gitlab"

  organization_scope = {
    host = "gitlab.com"
    name = "gitpod-io"
  }
}
`, host)
}

func testAccWebhookSecretConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

ephemeral "ona_webhook_secret" "test" {
  webhook_id = %[2]q
}

provider "echo" {
  data = ephemeral.ona_webhook_secret.test
}

resource "echo" "test" {}
`, host, webhookID1)
}

func testAccWebhookRotationConfig(host, secretVersion string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_webhook" "rotation" {
  name           = "Rotation test"
  type           = "repository"
  scm_provider   = "github"
  secret_version = %[2]q

  repository_scopes = [
    {
      host  = "github.com"
      owner = "gitpod-io"
      name  = "terraform-provider-ona"
    }
  ]
}

ephemeral "ona_webhook_secret" "rotation" {
  webhook_id = ona_webhook.rotation.id
}

provider "echo" {
  data = ephemeral.ona_webhook_secret.rotation
}

resource "echo" "rotation" {}
`, host, secretVersion)
}

const (
	webhookID1 = "00000000-0000-0000-0000-000000000001"
	webhookID2 = "00000000-0000-0000-0000-000000000002"
)

type webhookAPIServer struct {
	*httptest.Server
	service *fakeWebhookService
}

func newWebhookAPIServer(t *testing.T) *webhookAPIServer {
	t.Helper()
	service := &fakeWebhookService{
		webhooks:        make(map[string]*v1.Webhook),
		secrets:         make(map[string]string),
		rotations:       make(map[string]int),
		secretReads:     make(map[string]int),
		lastSecretsRead: make(map[string]string),
		now:             time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC),
		organizationID:  "00000000-0000-0000-0000-000000000010",
	}
	path, handler := v1connect.NewWebhookServiceHandler(service)
	server := httptest.NewServer(http.StripPrefix("/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path || len(r.URL.Path) > len(path) && r.URL.Path[:len(path)] == path {
			handler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	return &webhookAPIServer{Server: server, service: service}
}

type fakeWebhookService struct {
	v1connect.UnimplementedWebhookServiceHandler

	mu              sync.Mutex
	webhooks        map[string]*v1.Webhook
	secrets         map[string]string
	rotations       map[string]int
	secretReads     map[string]int
	lastSecretsRead map[string]string
	nextID          int
	now             time.Time
	organizationID  string
}

func (s *fakeWebhookService) CreateWebhook(ctx context.Context, req *connect.Request[v1.CreateWebhookRequest]) (*connect.Response[v1.CreateWebhookResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	id := fmt.Sprintf("00000000-0000-0000-0000-%012d", s.nextID)
	webhook := &v1.Webhook{
		Id: id,
		Metadata: &v1.Webhook_Metadata{
			OrganizationId: s.organizationID,
			Name:           req.Msg.GetName(),
			Description:    req.Msg.GetDescription(),
			Creator:        &v1.Subject{Id: "00000000-0000-0000-0000-000000000020", Principal: v1.Principal_PRINCIPAL_USER},
			CreatedAt:      timestamppb.New(s.now),
			UpdatedAt:      timestamppb.New(s.now),
		},
		Spec: &v1.Webhook_Spec{
			Type:              req.Msg.GetType(),
			Provider:          req.Msg.GetProvider(),
			Scopes:            cloneRepositoryScopes(req.Msg.GetScopes()),
			OrganizationScope: cloneOrganizationScope(req.Msg.GetOrganizationScope()),
		},
		Url:                "https://example.com/webhooks/" + id,
		BoundWorkflowCount: 1,
	}
	s.webhooks[id] = webhook
	s.secrets[id] = "secret-1"
	return connect.NewResponse(&v1.CreateWebhookResponse{Webhook: cloneWebhook(webhook)}), nil
}

func (s *fakeWebhookService) GetWebhook(ctx context.Context, req *connect.Request[v1.GetWebhookRequest]) (*connect.Response[v1.GetWebhookResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	webhook := s.webhooks[req.Msg.GetWebhookId()]
	if webhook == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("webhook not found"))
	}
	return connect.NewResponse(&v1.GetWebhookResponse{Webhook: cloneWebhook(webhook)}), nil
}

func (s *fakeWebhookService) UpdateWebhook(ctx context.Context, req *connect.Request[v1.UpdateWebhookRequest]) (*connect.Response[v1.UpdateWebhookResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	webhook := s.webhooks[req.Msg.GetWebhookId()]
	if webhook == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("webhook not found"))
	}
	if req.Msg.Name != nil {
		webhook.Metadata.Name = req.Msg.GetName()
	}
	if req.Msg.Description != nil {
		webhook.Metadata.Description = req.Msg.GetDescription()
	}
	if len(req.Msg.GetScopes()) > 0 {
		webhook.Spec.Scopes = cloneRepositoryScopes(req.Msg.GetScopes())
		webhook.Spec.OrganizationScope = nil
	}
	if req.Msg.GetOrganizationScope() != nil {
		webhook.Spec.OrganizationScope = cloneOrganizationScope(req.Msg.GetOrganizationScope())
		webhook.Spec.Scopes = nil
	}
	webhook.Metadata.UpdatedAt = timestamppb.New(s.now.Add(time.Minute))
	return connect.NewResponse(&v1.UpdateWebhookResponse{Webhook: cloneWebhook(webhook)}), nil
}

func (s *fakeWebhookService) DeleteWebhook(ctx context.Context, req *connect.Request[v1.DeleteWebhookRequest]) (*connect.Response[v1.DeleteWebhookResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := req.Msg.GetWebhookId()
	if s.webhooks[id] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("webhook not found"))
	}
	delete(s.webhooks, id)
	delete(s.secrets, id)
	return connect.NewResponse(&v1.DeleteWebhookResponse{AffectedWorkflowIds: []string{"workflow-1"}}), nil
}

func (s *fakeWebhookService) GetWebhookSecret(ctx context.Context, req *connect.Request[v1.GetWebhookSecretRequest]) (*connect.Response[v1.GetWebhookSecretResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := req.Msg.GetWebhookId()
	if s.webhooks[id] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("webhook not found"))
	}
	s.secretReads[id]++
	s.lastSecretsRead[id] = s.secrets[id]
	return connect.NewResponse(&v1.GetWebhookSecretResponse{Secret: s.secrets[id]}), nil
}

func (s *fakeWebhookService) RotateWebhookSecret(ctx context.Context, req *connect.Request[v1.RotateWebhookSecretRequest]) (*connect.Response[v1.RotateWebhookSecretResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := req.Msg.GetWebhookId()
	if s.webhooks[id] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("webhook not found"))
	}
	s.rotations[id]++
	secret := fmt.Sprintf("secret-%d", s.rotations[id]+1)
	s.secrets[id] = secret
	return connect.NewResponse(&v1.RotateWebhookSecretResponse{Secret: secret}), nil
}

func (s *fakeWebhookService) seed(webhook *v1.Webhook, secret string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webhooks[webhook.GetId()] = cloneWebhook(webhook)
	s.secrets[webhook.GetId()] = secret
}

func (s *fakeWebhookService) updateName(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if webhook := s.webhooks[id]; webhook != nil {
		webhook.Metadata.Name = name
	}
}

func (s *fakeWebhookService) updateDescription(id, description string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if webhook := s.webhooks[id]; webhook != nil {
		webhook.Metadata.Description = description
	}
}

func (s *fakeWebhookService) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.webhooks, id)
	delete(s.secrets, id)
}

func (s *fakeWebhookService) remaining() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, 0, len(s.webhooks))
	for id := range s.webhooks {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func (s *fakeWebhookService) rotationCount(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rotations[id]
}

func (s *fakeWebhookService) secretReadCount(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.secretReads[id]
}

func (s *fakeWebhookService) lastSecretRead(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSecretsRead[id]
}

func newTestWebhook(id, name string, webhookType v1.WebhookType, provider v1.WebhookProvider) *v1.Webhook {
	return &v1.Webhook{
		Id: id,
		Metadata: &v1.Webhook_Metadata{
			OrganizationId: "00000000-0000-0000-0000-000000000010",
			Name:           name,
			Creator:        &v1.Subject{Id: "00000000-0000-0000-0000-000000000020", Principal: v1.Principal_PRINCIPAL_USER},
			CreatedAt:      timestamppb.New(time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)),
			UpdatedAt:      timestamppb.New(time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)),
		},
		Spec: &v1.Webhook_Spec{
			Type:     webhookType,
			Provider: provider,
			Scopes: []*v1.WebhookRepositoryScope{{
				Host: "github.com", Owner: "gitpod-io", Name: "terraform-provider-ona",
			}},
		},
		Url: "https://example.com/webhooks/" + id,
	}
}

func cloneWebhook(webhook *v1.Webhook) *v1.Webhook {
	if webhook == nil {
		return nil
	}
	cloned, ok := proto.Clone(webhook).(*v1.Webhook)
	if !ok {
		return nil
	}
	return cloned
}

func cloneRepositoryScopes(scopes []*v1.WebhookRepositoryScope) []*v1.WebhookRepositoryScope {
	result := make([]*v1.WebhookRepositoryScope, 0, len(scopes))
	for _, scope := range scopes {
		cloned, ok := proto.Clone(scope).(*v1.WebhookRepositoryScope)
		if !ok {
			continue
		}
		result = append(result, cloned)
	}
	return result
}

func cloneOrganizationScope(scope *v1.WebhookOrganizationScope) *v1.WebhookOrganizationScope {
	if scope == nil {
		return nil
	}
	cloned, ok := proto.Clone(scope).(*v1.WebhookOrganizationScope)
	if !ok {
		return nil
	}
	return cloned
}

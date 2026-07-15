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

func TestAccServiceAccountResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newServiceAccountAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.suspended(serviceAccountID1) {
				return errors.New("service account was not suspended")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAccountResourceConfig(server.URL, "CI Pipeline", "CI/CD Pipeline"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_service_account.test", "id", serviceAccountID1),
					resource.TestCheckNoResourceAttr("ona_service_account.test", "service_account_id"),
					resource.TestCheckNoResourceAttr("ona_service_account.test", "organization_id"),
					resource.TestCheckResourceAttr("ona_service_account.test", "name", "CI Pipeline"),
					resource.TestCheckResourceAttr("ona_service_account.test", "description", "CI/CD Pipeline"),
					resource.TestCheckResourceAttr("ona_service_account.test", "valid_until", serviceAccountValidUntil),
					resource.TestCheckResourceAttr("ona_service_account.test", "created_at", serviceAccountCreatedAt),
					resource.TestCheckResourceAttr("ona_service_account.test", "creator.principal", "user"),
					resource.TestCheckNoResourceAttr("ona_service_account.test", "system_managed"),
				),
			},
			{
				ResourceName:      "ona_service_account.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccServiceAccountResourceConfig(server.URL, "CI Pipeline Updated", "Updated CI/CD Pipeline"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_service_account.test", "id", serviceAccountID1),
					resource.TestCheckResourceAttr("ona_service_account.test", "name", "CI Pipeline Updated"),
					resource.TestCheckResourceAttr("ona_service_account.test", "description", "Updated CI/CD Pipeline"),
				),
			},
			{
				Config: testAccServiceAccountResourceConfig(server.URL, "CI Pipeline Updated", "Updated CI/CD Pipeline"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccServiceAccountResourceConfigWithValidUntil(server.URL, "CI Pipeline Updated", "Updated CI/CD Pipeline", "2100-01-02T03:04:05Z"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_service_account.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}

func TestAccServiceAccountTokenEphemeralResource(t *testing.T) {
	t.Parallel()

	server := newServiceAccountAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed(newTestServiceAccount(serviceAccountID1, "Token Account", ""))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAccountTokenEphemeralConfig(server.URL),
				Check: func(state *terraform.State) error {
					if !server.service.accessTokenCreated("Bearer test-token", serviceAccountID1) {
						return errors.New("service account access token was not created with provider token")
					}
					if !server.service.serviceAccountTokenCreated("Bearer access-token-"+serviceAccountID1, "GitHub Actions", 2160) {
						return errors.New("service account token was not created with impersonated token")
					}
					if state.RootModule().Resources["ona_service_account_token.test"] != nil {
						return errors.New("ephemeral service account token was stored as managed resource state")
					}
					return nil
				},
			},
		},
	})
}

func TestAccServiceAccountResourceReadRemovesNotFound(t *testing.T) {
	t.Parallel()

	server := newServiceAccountAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.suspended(serviceAccountID1) {
				return errors.New("service account was not suspended")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAccountResourceConfig(server.URL, "CI Pipeline", "CI/CD Pipeline"),
			},
			{
				PreConfig: func() {
					server.service.remove(serviceAccountID1)
				},
				Config: testAccServiceAccountResourceConfig(server.URL, "CI Pipeline", "CI/CD Pipeline"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_service_account.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccServiceAccountResourceReadRemovesSuspended(t *testing.T) {
	t.Parallel()

	server := newServiceAccountAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.suspended(serviceAccountID1) {
				return errors.New("service account was not suspended")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAccountResourceConfig(server.URL, "CI Pipeline", "CI/CD Pipeline"),
			},
			{
				PreConfig: func() {
					server.service.suspend(serviceAccountID1)
				},
				Config: testAccServiceAccountResourceConfig(server.URL, "CI Pipeline", "CI/CD Pipeline"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_service_account.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func testAccServiceAccountResourceConfig(host string, name string, description string) string {
	return testAccServiceAccountResourceConfigWithValidUntil(host, name, description, serviceAccountValidUntil)
}

func testAccServiceAccountResourceConfigWithValidUntil(host string, name string, description string, validUntil string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_service_account" "test" {
  name        = %[2]q
  description = %[3]q
  valid_until = %[4]q
}
`, host, name, description, validUntil)
}

func testAccServiceAccountTokenEphemeralConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

ephemeral "ona_service_account_token" "test" {
  service_account_id = %[2]q
  description        = "GitHub Actions"
  valid_for          = "2160h"
}

provider "echo" {
  data = ephemeral.ona_service_account_token.test
}

resource "echo" "test" {}
`, host, serviceAccountID1)
}

type serviceAccountAPIServer struct {
	*httptest.Server
	service *fakeServiceAccountService
}

func newServiceAccountAPIServer(t *testing.T) *serviceAccountAPIServer {
	t.Helper()

	service := &fakeServiceAccountService{
		accounts: map[string]*v1.ServiceAccount{},
	}
	_, handler := v1connect.NewServiceAccountServiceHandler(service)
	server := httptest.NewServer(http.StripPrefix("/api", handler))
	return &serviceAccountAPIServer{
		Server:  server,
		service: service,
	}
}

type fakeServiceAccountService struct {
	v1connect.UnimplementedServiceAccountServiceHandler

	mu                sync.Mutex
	accounts          map[string]*v1.ServiceAccount
	accessTokenCalls  []serviceAccountAccessTokenCall
	serviceTokenCalls []serviceAccountTokenCall
}

type serviceAccountAccessTokenCall struct {
	Authorization    string
	ServiceAccountID string
}

type serviceAccountTokenCall struct {
	Authorization string
	Description   string
	ValidForHours int64
}

const (
	serviceAccountID1        = "00000000-0000-0000-0000-000000000001"
	serviceAccountTokenID1   = "00000000-0000-0000-0000-000000000101"
	organizationID1          = "10000000-0000-0000-0000-000000000001"
	serviceAccountCreatedAt  = "2026-01-02T03:04:05Z"
	serviceAccountValidUntil = "2099-01-02T03:04:05Z"
)

func (s *fakeServiceAccountService) CreateServiceAccount(ctx context.Context, req *connect.Request[v1.CreateServiceAccountRequest]) (*connect.Response[v1.CreateServiceAccountResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if got := req.Header().Get("Authorization"); got != "Bearer test-token" {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authorization header = %q, want Bearer test-token", got))
	}

	account := newTestServiceAccount(serviceAccountID1, req.Msg.GetName(), req.Msg.GetDescription())
	account.ValidUntil = req.Msg.GetValidUntil()
	s.accounts[account.GetId()] = account
	return connect.NewResponse(&v1.CreateServiceAccountResponse{ServiceAccount: cloneServiceAccount(account)}), nil
}

func (s *fakeServiceAccountService) GetServiceAccount(ctx context.Context, req *connect.Request[v1.GetServiceAccountRequest]) (*connect.Response[v1.GetServiceAccountResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	account := s.accounts[req.Msg.GetServiceAccountId()]
	if account == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("service account not found"))
	}
	return connect.NewResponse(&v1.GetServiceAccountResponse{ServiceAccount: cloneServiceAccount(account)}), nil
}

func (s *fakeServiceAccountService) UpdateServiceAccount(ctx context.Context, req *connect.Request[v1.UpdateServiceAccountRequest]) (*connect.Response[v1.UpdateServiceAccountResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	account := s.accounts[req.Msg.GetServiceAccountId()]
	if account == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("service account not found"))
	}
	if req.Msg.Name != nil {
		account.Name = req.Msg.GetName()
	}
	if req.Msg.Description != nil {
		account.Description = req.Msg.GetDescription()
	}
	return connect.NewResponse(&v1.UpdateServiceAccountResponse{ServiceAccount: cloneServiceAccount(account)}), nil
}

func (s *fakeServiceAccountService) DeleteServiceAccount(ctx context.Context, req *connect.Request[v1.DeleteServiceAccountRequest]) (*connect.Response[v1.DeleteServiceAccountResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	account := s.accounts[req.Msg.GetServiceAccountId()]
	if account == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("service account not found"))
	}
	account.Suspended = true
	return connect.NewResponse(&v1.DeleteServiceAccountResponse{}), nil
}

func (s *fakeServiceAccountService) CreateServiceAccountAccessToken(ctx context.Context, req *connect.Request[v1.CreateServiceAccountAccessTokenRequest]) (*connect.Response[v1.CreateServiceAccountAccessTokenResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	call := serviceAccountAccessTokenCall{
		Authorization:    req.Header().Get("Authorization"),
		ServiceAccountID: req.Msg.GetServiceAccountId(),
	}
	s.accessTokenCalls = append(s.accessTokenCalls, call)
	if s.accounts[req.Msg.GetServiceAccountId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("service account not found"))
	}
	return connect.NewResponse(&v1.CreateServiceAccountAccessTokenResponse{
		Token: "access-token-" + req.Msg.GetServiceAccountId(),
	}), nil
}

func (s *fakeServiceAccountService) CreateServiceAccountToken(ctx context.Context, req *connect.Request[v1.CreateServiceAccountTokenRequest]) (*connect.Response[v1.CreateServiceAccountTokenResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	call := serviceAccountTokenCall{
		Authorization: req.Header().Get("Authorization"),
		Description:   req.Msg.GetDescription(),
	}
	if req.Msg.GetValidFor() != nil {
		call.ValidForHours = int64(req.Msg.GetValidFor().AsDuration().Hours())
	}
	s.serviceTokenCalls = append(s.serviceTokenCalls, call)

	validFor := time.Hour * 24 * 30
	if req.Msg.GetValidFor() != nil {
		validFor = req.Msg.GetValidFor().AsDuration()
	}
	token := &v1.ServiceAccountToken{
		Id:               serviceAccountTokenID1,
		ServiceAccountId: serviceAccountID1,
		Description:      req.Msg.GetDescription(),
		CreatedAt:        mustTimestamp(serviceAccountCreatedAt),
		ExpiresAt:        timestamppb.New(mustTimestamp(serviceAccountCreatedAt).AsTime().Add(validFor)),
	}
	return connect.NewResponse(&v1.CreateServiceAccountTokenResponse{
		Token:               "sat-token-value",
		ServiceAccountToken: token,
	}), nil
}

func (s *fakeServiceAccountService) seed(account *v1.ServiceAccount) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[account.GetId()] = account
}

func (s *fakeServiceAccountService) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.accounts, id)
}

func (s *fakeServiceAccountService) suspend(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	account := s.accounts[id]
	if account != nil {
		account.Suspended = true
	}
}

func (s *fakeServiceAccountService) suspended(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	account := s.accounts[id]
	return account != nil && account.GetSuspended()
}

func (s *fakeServiceAccountService) accessTokenCreated(authorization string, serviceAccountID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, call := range s.accessTokenCalls {
		if call.Authorization == authorization && call.ServiceAccountID == serviceAccountID {
			return true
		}
	}
	return false
}

func (s *fakeServiceAccountService) serviceAccountTokenCreated(authorization string, description string, validForHours int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, call := range s.serviceTokenCalls {
		if call.Authorization == authorization && call.Description == description && call.ValidForHours == validForHours {
			return true
		}
	}
	return false
}

func newTestServiceAccount(id string, name string, description string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		Id:             id,
		OrganizationId: organizationID1,
		Name:           name,
		Description:    description,
		Creator: &v1.Subject{
			Id:        "20000000-0000-0000-0000-000000000001",
			Principal: v1.Principal_PRINCIPAL_USER,
		},
		CreatedAt:     mustTimestamp(serviceAccountCreatedAt),
		ValidUntil:    mustTimestamp(serviceAccountValidUntil),
		SystemManaged: false,
	}
}

func cloneServiceAccount(account *v1.ServiceAccount) *v1.ServiceAccount {
	cloned, ok := proto.Clone(account).(*v1.ServiceAccount)
	if !ok {
		return nil
	}
	return cloned
}

func mustTimestamp(value string) *timestamppb.Timestamp {
	parsed, err := timeFromRFC3339(value)
	if err != nil {
		panic(err)
	}
	return timestamppb.New(parsed)
}

func timeFromRFC3339(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}

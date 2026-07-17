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

const (
	customDomainOrgID     = "11111111-1111-4111-8111-111111111111"
	customDomainOtherOrg  = "22222222-2222-4222-8222-222222222222"
	customDomainID        = "33333333-3333-4333-8333-333333333333"
	customDomainCreatedAt = "2026-01-02T03:04:05Z"
	customDomainUpdatedAt = "2026-01-03T03:04:05Z"
)

func TestAccCustomDomainResourceLifecycle(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newCustomDomainAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.deleted() {
				return errors.New("custom domain was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccCustomDomainConfig(server.URL, "ona.example.com", "aws", "123456789012"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_custom_domain.primary", "id", customDomainID),
					resource.TestCheckResourceAttr("ona_custom_domain.primary", "domain_name", "ona.example.com"),
					resource.TestCheckResourceAttr("ona_custom_domain.primary", "cloud_provider", "aws"),
					resource.TestCheckResourceAttr("ona_custom_domain.primary", "cloud_account_id", "123456789012"),
					resource.TestCheckResourceAttr("ona_custom_domain.primary", "created_at", customDomainCreatedAt),
					resource.TestCheckResourceAttr("ona_custom_domain.primary", "updated_at", customDomainCreatedAt),
					resource.TestCheckNoResourceAttr("ona_custom_domain.primary", "organization_id"),
				),
			},
			{
				Config: testAccCustomDomainConfig(server.URL, "ona.example.com", "aws", "123456789012"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_custom_domain.primary",
				ImportState:       true,
				ImportStateId:     "current",
				ImportStateVerify: true,
			},
			{
				ResourceName:      "ona_custom_domain.primary",
				ImportState:       true,
				ImportStateId:     customDomainOrgID,
				ImportStateVerify: true,
			},
			{
				Config: testAccCustomDomainConfig(server.URL, "workspaces.example.com", "aws", "123456789012"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_custom_domain.primary", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_custom_domain.primary", "domain_name", "workspaces.example.com"),
					resource.TestCheckResourceAttr("ona_custom_domain.primary", "updated_at", customDomainUpdatedAt),
				),
			},
			{
				Config: testAccCustomDomainConfig(server.URL, "workspaces.example.com", "aws", "210987654321"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_custom_domain.primary", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("ona_custom_domain.primary", "cloud_account_id", "210987654321"),
			},
			{
				Config: testAccCustomDomainConfig(server.URL, "workspaces.example.com", "gcp", "my-gcp-project"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_custom_domain.primary", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_custom_domain.primary", "cloud_provider", "gcp"),
					resource.TestCheckResourceAttr("ona_custom_domain.primary", "cloud_account_id", "my-gcp-project"),
				),
			},
		},
	})
}

func TestAccCustomDomainResourceReadRemovesNotFound(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newCustomDomainAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCustomDomainConfig(server.URL, "ona.example.com", "aws", "123456789012"),
			},
			{
				PreConfig: func() {
					server.service.deleteRemote()
				},
				Config: testAccCustomDomainConfig(server.URL, "ona.example.com", "aws", "123456789012"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_custom_domain.primary", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccCustomDomainResourceRejectsDuplicateCreate(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newCustomDomainAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed("existing.example.com", v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_AWS, "123456789012")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCustomDomainConfig(server.URL, "ona.example.com", "aws", "123456789012"),
				ExpectError: regexp.MustCompile(`custom domain already exists`),
			},
		},
	})
}

func TestAccCustomDomainResourceRejectsOtherOrganizationImport(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newCustomDomainAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed("ona.example.com", v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_AWS, "123456789012")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName:  "ona_custom_domain.primary",
				ImportState:   true,
				ImportStateId: customDomainOtherOrg,
				Config:        testAccCustomDomainConfig(server.URL, "ona.example.com", "aws", "123456789012"),
				ExpectError:   regexp.MustCompile(`Invalid Import ID`),
			},
		},
	})
}

func testAccCustomDomainConfig(host string, domainName string, cloudProvider string, cloudAccountID string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_custom_domain" "primary" {
  domain_name      = %[2]q
  cloud_provider   = %[3]q
  cloud_account_id = %[4]q
}
`, host, domainName, cloudProvider, cloudAccountID)
}

type customDomainAPIServer struct {
	*httptest.Server
	service *fakeCustomDomainService
}

func newCustomDomainAPIServer(t *testing.T) *customDomainAPIServer {
	t.Helper()

	service := &fakeCustomDomainService{
		organizationID: customDomainOrgID,
		now:            timestampForCustomDomainTest(customDomainCreatedAt).AsTime(),
	}
	mux := http.NewServeMux()
	organizationPath, organizationHandler := v1connect.NewOrganizationServiceHandler(service)
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)
	mux.Handle(organizationPath, organizationHandler)
	mux.Handle(identityPath, identityHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	return &customDomainAPIServer{
		Server:  server,
		service: service,
	}
}

type fakeCustomDomainService struct {
	v1connect.UnimplementedOrganizationServiceHandler
	v1connect.UnimplementedIdentityServiceHandler

	mu             sync.Mutex
	organizationID string
	customDomain   *v1.CustomDomain
	deletedRemote  bool
	now            time.Time
}

func (s *fakeCustomDomainService) CreateCustomDomain(ctx context.Context, req *connect.Request[v1.CreateCustomDomainRequest]) (*connect.Response[v1.CreateCustomDomainResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.organizationID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
	}
	if s.customDomain != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("custom domain already exists"))
	}
	customDomain := &v1.CustomDomain{
		Id:             customDomainID,
		OrganizationId: req.Msg.GetOrganizationId(),
		DomainName:     req.Msg.GetDomainName(),
		Provider:       req.Msg.GetProvider(),
		CloudAccountId: req.Msg.GetCloudAccountId(),
		CreatedAt:      timestamppb.New(s.now),
		UpdatedAt:      timestamppb.New(s.now),
	}
	s.customDomain = customDomain
	s.deletedRemote = false
	return connect.NewResponse(&v1.CreateCustomDomainResponse{CustomDomain: cloneCustomDomain(customDomain)}), nil
}

func (s *fakeCustomDomainService) GetCustomDomain(ctx context.Context, req *connect.Request[v1.GetCustomDomainRequest]) (*connect.Response[v1.GetCustomDomainResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.organizationID || s.customDomain == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("custom domain not found"))
	}
	return connect.NewResponse(&v1.GetCustomDomainResponse{CustomDomain: cloneCustomDomain(s.customDomain)}), nil
}

func (s *fakeCustomDomainService) UpdateCustomDomain(ctx context.Context, req *connect.Request[v1.UpdateCustomDomainRequest]) (*connect.Response[v1.UpdateCustomDomainResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.organizationID || s.customDomain == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("custom domain not found"))
	}
	s.customDomain.DomainName = req.Msg.GetDomainName()
	if req.Msg.CloudAccountId != nil {
		s.customDomain.CloudAccountId = req.Msg.GetCloudAccountId()
	}
	if req.Msg.Provider != nil {
		s.customDomain.Provider = req.Msg.GetProvider()
	}
	s.customDomain.UpdatedAt = timestampForCustomDomainTest(customDomainUpdatedAt)
	return connect.NewResponse(&v1.UpdateCustomDomainResponse{CustomDomain: cloneCustomDomain(s.customDomain)}), nil
}

func (s *fakeCustomDomainService) DeleteCustomDomain(ctx context.Context, req *connect.Request[v1.DeleteCustomDomainRequest]) (*connect.Response[v1.DeleteCustomDomainResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.organizationID || s.customDomain == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("custom domain not found"))
	}
	s.customDomain = nil
	s.deletedRemote = true
	return connect.NewResponse(&v1.DeleteCustomDomainResponse{}), nil
}

func (s *fakeCustomDomainService) GetAuthenticatedIdentity(ctx context.Context, req *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{
		Subject: &v1.Subject{
			Id:        "service-account-1",
			Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT,
		},
		OrganizationId: s.organizationID,
	}), nil
}

func (s *fakeCustomDomainService) seed(domainName string, provider v1.CustomDomainProvider, cloudAccountID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.customDomain = &v1.CustomDomain{
		Id:             customDomainID,
		OrganizationId: s.organizationID,
		DomainName:     domainName,
		Provider:       provider,
		CloudAccountId: cloudAccountID,
		CreatedAt:      timestampForCustomDomainTest(customDomainCreatedAt),
		UpdatedAt:      timestampForCustomDomainTest(customDomainCreatedAt),
	}
}

func (s *fakeCustomDomainService) deleteRemote() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.customDomain = nil
	s.deletedRemote = true
}

func (s *fakeCustomDomainService) deleted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.deletedRemote
}

func cloneCustomDomain(customDomain *v1.CustomDomain) *v1.CustomDomain {
	return proto.CloneOf(customDomain)
}

func timestampForCustomDomainTest(value string) *timestamppb.Timestamp {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return timestamppb.New(parsed)
}

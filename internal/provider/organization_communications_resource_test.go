// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestAccOrganizationCommunicationsResourcesLifecycle(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newOrganizationCommunicationsAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if got := server.service.bannerState(); got.Enabled || got.Message != "" {
				return fmt.Errorf("announcement banner was not disabled and cleared after destroy: %+v", got)
			}
			if server.service.termsEnabled() {
				return errors.New("terms of service was not disabled after destroy")
			}
			if got := server.service.termsVersionCount(); got != 2 {
				return fmt.Errorf("terms version history count = %d, want 2", got)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationCommunicationsConfig(server.URL, true, "Initial banner", true, "Initial terms"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_announcement_banner.home", "id", organizationCommunicationsOrgID),
					resource.TestCheckResourceAttr("ona_announcement_banner.home", "enabled", "true"),
					resource.TestCheckResourceAttr("ona_announcement_banner.home", "message", "Initial banner"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "id", organizationCommunicationsOrgID),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "enabled", "true"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "markdown", "Initial terms"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "current_version_id", "terms-version-1"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "current_version", "1"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "current_version_created_at", "2026-07-10T12:01:00Z"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "current_version_created_by_user_id", organizationCommunicationsUserID),
					testCheckTermsVersionCount(server, 1),
				),
			},
			{
				Config: testAccOrganizationCommunicationsConfig(server.URL, true, "Initial banner", true, "Initial terms"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_announcement_banner.home",
				ImportState:       true,
				ImportStateId:     "current",
				ImportStateVerify: true,
			},
			{
				ResourceName:      "ona_terms_of_service.org",
				ImportState:       true,
				ImportStateId:     "current",
				ImportStateVerify: true,
			},
			{
				Config: testAccOrganizationCommunicationsConfig(server.URL, false, "Updated banner", false, "Updated terms"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_announcement_banner.home", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("ona_terms_of_service.org", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_announcement_banner.home", "enabled", "false"),
					resource.TestCheckResourceAttr("ona_announcement_banner.home", "message", "Updated banner"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "enabled", "false"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "markdown", "Updated terms"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "current_version_id", "terms-version-2"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "current_version", "2"),
					testCheckTermsVersionCount(server, 2),
				),
			},
			{
				Config: testAccOrganizationCommunicationsConfig(server.URL, false, "Updated banner", true, "Updated terms"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_terms_of_service.org", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "enabled", "true"),
					resource.TestCheckResourceAttr("ona_terms_of_service.org", "current_version_id", "terms-version-2"),
					testCheckTermsVersionCount(server, 2),
				),
			},
		},
	})
}

func testAccOrganizationCommunicationsConfig(host string, bannerEnabled bool, bannerMessage string, termsEnabled bool, termsMarkdown string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_announcement_banner" "home" {
  enabled = %[2]t
  message = %[3]q
}

resource "ona_terms_of_service" "org" {
  enabled  = %[4]t
  markdown = %[5]q
}
`, host, bannerEnabled, bannerMessage, termsEnabled, termsMarkdown)
}

func testCheckTermsVersionCount(server *organizationCommunicationsAPIServer, expected int) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		if got := server.service.termsVersionCount(); got != expected {
			return fmt.Errorf("terms version count = %d, want %d", got, expected)
		}
		return nil
	}
}

const (
	organizationCommunicationsOrgID  = "org-1"
	organizationCommunicationsUserID = "user-1"
)

type organizationCommunicationsAPIServer struct {
	*httptest.Server
	service *fakeOrganizationCommunicationsService
}

func newOrganizationCommunicationsAPIServer(t *testing.T) *organizationCommunicationsAPIServer {
	t.Helper()

	service := &fakeOrganizationCommunicationsService{
		organizationID: organizationCommunicationsOrgID,
		subjectID:      organizationCommunicationsUserID,
		principal:      v1.Principal_PRINCIPAL_USER,
		now:            time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
	}
	mux := http.NewServeMux()
	organizationPath, organizationHandler := v1connect.NewOrganizationServiceHandler(service)
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)
	mux.Handle(organizationPath, organizationHandler)
	mux.Handle(identityPath, identityHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	return &organizationCommunicationsAPIServer{
		Server:  server,
		service: service,
	}
}

type fakeOrganizationCommunicationsService struct {
	v1connect.UnimplementedOrganizationServiceHandler
	v1connect.UnimplementedIdentityServiceHandler

	mu             sync.Mutex
	organizationID string
	subjectID      string
	principal      v1.Principal
	now            time.Time
	banner         *v1.AnnouncementBanner
	terms          *v1.TermsOfService
	versions       []*v1.TermsOfServiceVersion
}

func (s *fakeOrganizationCommunicationsService) GetAuthenticatedIdentity(ctx context.Context, req *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{
		Subject: &v1.Subject{
			Id:        s.subjectID,
			Principal: s.principal,
		},
		OrganizationId: s.organizationID,
	}), nil
}

func (s *fakeOrganizationCommunicationsService) GetAnnouncementBanner(ctx context.Context, req *connect.Request[v1.GetAnnouncementBannerRequest]) (*connect.Response[v1.GetAnnouncementBannerResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.organizationID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("announcement banner not found"))
	}
	if s.banner == nil {
		return connect.NewResponse(&v1.GetAnnouncementBannerResponse{
			Banner: &v1.AnnouncementBanner{
				OrganizationId: s.organizationID,
			},
		}), nil
	}
	return connect.NewResponse(&v1.GetAnnouncementBannerResponse{Banner: cloneAnnouncementBanner(s.banner)}), nil
}

func (s *fakeOrganizationCommunicationsService) UpdateAnnouncementBanner(ctx context.Context, req *connect.Request[v1.UpdateAnnouncementBannerRequest]) (*connect.Response[v1.UpdateAnnouncementBannerResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.organizationID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("announcement banner not found"))
	}
	message := req.Msg.GetMessage()
	enabled := req.Msg.GetEnabled()
	if enabled && strings.TrimSpace(message) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("message is required when enabling announcement banner"))
	}
	s.banner = &v1.AnnouncementBanner{
		OrganizationId: s.organizationID,
		Message:        message,
		Enabled:        enabled,
	}
	return connect.NewResponse(&v1.UpdateAnnouncementBannerResponse{Banner: cloneAnnouncementBanner(s.banner)}), nil
}

func (s *fakeOrganizationCommunicationsService) GetTermsOfService(ctx context.Context, req *connect.Request[v1.GetTermsOfServiceRequest]) (*connect.Response[v1.GetTermsOfServiceResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.organizationID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("terms of service not found"))
	}
	if s.terms == nil {
		return connect.NewResponse(&v1.GetTermsOfServiceResponse{
			TermsOfService: &v1.TermsOfService{
				OrganizationId: s.organizationID,
			},
		}), nil
	}
	return connect.NewResponse(&v1.GetTermsOfServiceResponse{TermsOfService: cloneTermsOfService(s.terms)}), nil
}

func (s *fakeOrganizationCommunicationsService) UpdateTermsOfService(ctx context.Context, req *connect.Request[v1.UpdateTermsOfServiceRequest]) (*connect.Response[v1.UpdateTermsOfServiceResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.organizationID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("terms of service not found"))
	}
	if req.Msg.Markdown != nil && s.principal != v1.Principal_PRINCIPAL_USER {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("terms publishing requires user principal"))
	}

	var currentVersion *v1.TermsOfServiceVersion
	if s.terms != nil {
		currentVersion = s.terms.GetCurrentVersion()
	}
	if req.Msg.Markdown != nil && (currentVersion == nil || currentVersion.GetMarkdown() != req.Msg.GetMarkdown()) {
		versionNumber := int32(len(s.versions) + 1)
		currentVersion = &v1.TermsOfServiceVersion{
			Id:              fmt.Sprintf("terms-version-%d", versionNumber),
			Version:         versionNumber,
			Markdown:        req.Msg.GetMarkdown(),
			CreatedAt:       timestamppb.New(s.now.Add(time.Duration(versionNumber) * time.Minute)),
			CreatedByUserId: s.subjectID,
		}
		s.versions = append(s.versions, currentVersion)
	}

	enabled := false
	if s.terms != nil {
		enabled = s.terms.GetEnabled()
	}
	if req.Msg.Enabled != nil {
		enabled = req.Msg.GetEnabled()
	}
	if enabled && currentVersion == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("markdown is required when enabling terms of service"))
	}

	s.terms = &v1.TermsOfService{
		OrganizationId: s.organizationID,
		Enabled:        enabled,
		CurrentVersion: cloneTermsOfServiceVersion(currentVersion),
	}
	return connect.NewResponse(&v1.UpdateTermsOfServiceResponse{TermsOfService: cloneTermsOfService(s.terms)}), nil
}

func (s *fakeOrganizationCommunicationsService) bannerState() *v1.AnnouncementBanner {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneAnnouncementBanner(s.banner)
}

func (s *fakeOrganizationCommunicationsService) termsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.terms.GetEnabled()
}

func (s *fakeOrganizationCommunicationsService) termsVersionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.versions)
}

func cloneAnnouncementBanner(banner *v1.AnnouncementBanner) *v1.AnnouncementBanner {
	if banner == nil {
		return nil
	}
	cloned, ok := proto.Clone(banner).(*v1.AnnouncementBanner)
	if !ok {
		return nil
	}
	return cloned
}

func cloneTermsOfService(terms *v1.TermsOfService) *v1.TermsOfService {
	if terms == nil {
		return nil
	}
	cloned, ok := proto.Clone(terms).(*v1.TermsOfService)
	if !ok {
		return nil
	}
	return cloned
}

func cloneTermsOfServiceVersion(version *v1.TermsOfServiceVersion) *v1.TermsOfServiceVersion {
	if version == nil {
		return nil
	}
	cloned, ok := proto.Clone(version).(*v1.TermsOfServiceVersion)
	if !ok {
		return nil
	}
	return cloned
}

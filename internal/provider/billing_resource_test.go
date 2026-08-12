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
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/gitpod-sdk-go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	billingOrgID        = "11111111-1111-4111-8111-111111111111"
	billingTeamID       = "22222222-2222-4222-8222-222222222222"
	billingIdentityID   = "33333333-3333-4333-8333-333333333333"
	billingAllocationID = "44444444-4444-4444-8444-444444444444"
)

func TestAccTeamAIBudgetModesCoexist(t *testing.T) {
	t.Parallel()

	server := newBillingAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if server.service.allocationSnapshot() != nil {
				return errors.New("team allocation was not deleted after the final mode")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccTeamAIBudgetConfig(server.URL, 100, 2_000_000, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_team_ai_budget.credits", "id", billingAllocationID),
					resource.TestCheckResourceAttr("ona_team_ai_budget.credits", "organization_id", billingOrgID),
					resource.TestCheckResourceAttr("ona_team_ai_budget.credits", "mode", "credits"),
					resource.TestCheckResourceAttr("ona_team_ai_budget.credits", "credit_budget", "100"),
					resource.TestCheckNoResourceAttr("ona_team_ai_budget.credits", "cost_budget_microunits"),
					resource.TestCheckResourceAttr("ona_team_ai_budget.byok", "id", billingAllocationID),
					resource.TestCheckResourceAttr("ona_team_ai_budget.byok", "mode", "byok"),
					resource.TestCheckResourceAttr("ona_team_ai_budget.byok", "cost_budget_microunits", "2000000"),
					resource.TestCheckResourceAttr("ona_team_ai_budget.byok", "cost_budget_currency", "usd"),
					resource.TestCheckNoResourceAttr("ona_team_ai_budget.byok", "credit_budget"),
				),
			},
			{
				Config:           testAccTeamAIBudgetConfig(server.URL, 100, 2_000_000, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:                         "ona_team_ai_budget.credits",
				ImportState:                          true,
				ImportStateId:                        billingOrgID + "/" + billingTeamID + "/credits",
				ImportStateVerifyIdentifierAttribute: "mode",
				ImportStateVerify:                    true,
			},
			{
				ResourceName:                         "ona_team_ai_budget.byok",
				ImportState:                          true,
				ImportStateId:                        billingOrgID + "/" + billingTeamID + "/byok",
				ImportStateVerifyIdentifierAttribute: "mode",
				ImportStateVerify:                    true,
			},
			{
				Config: testAccTeamAIBudgetConfig(server.URL, 150, 3_000_000, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_team_ai_budget.credits", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("ona_team_ai_budget.byok", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_team_ai_budget.credits", "credit_budget", "150"),
					resource.TestCheckResourceAttr("ona_team_ai_budget.byok", "cost_budget_microunits", "3000000"),
				),
			},
			{
				Config: testAccTeamAIBudgetConfig(server.URL, 0, 3_000_000, false),
				Check: func(*terraform.State) error {
					allocation := server.service.allocationSnapshot()
					if allocation == nil || allocation.GetCreditBudget() != 0 || allocation.GetCostBudgetMicrounits() != 3_000_000 {
						return fmt.Errorf("deleting credits did not preserve BYOK: %#v", allocation)
					}
					return nil
				},
			},
		},
	})
}

func testAccTeamAIBudgetConfig(host string, creditBudget, costBudget int64, includeCredits bool) string {
	credits := ""
	if includeCredits {
		credits = fmt.Sprintf(`
resource "ona_team_ai_budget" "credits" {
  team_id       = %q
  mode          = "credits"
  credit_budget = %d
}
`, billingTeamID, creditBudget)
	}
	return fmt.Sprintf(`
provider "ona" {
  host  = %q
  token = "test-token"
}
%s
resource "ona_team_ai_budget" "byok" {
  team_id                = %q
  mode                   = "byok"
  cost_budget_microunits = %d
  cost_budget_currency   = "usd"
}
`, host, credits, billingTeamID, costBudget)
}

type billingAPIServer struct {
	*httptest.Server
	service *fakeBillingService
}

func newBillingAPIServer(t *testing.T) *billingAPIServer {
	t.Helper()
	service := &fakeBillingService{}
	mux := http.NewServeMux()
	billingPath, billingHandler := v1connect.NewBillingServiceHandler(service)
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)
	mux.Handle(billingPath, billingHandler)
	mux.Handle(identityPath, identityHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	return &billingAPIServer{Server: server, service: service}
}

type fakeBillingService struct {
	v1connect.UnimplementedBillingServiceHandler
	v1connect.UnimplementedIdentityServiceHandler

	mu         sync.Mutex
	allocation *v1.TeamCreditAllocationInfo
}

func (s *fakeBillingService) GetAuthenticatedIdentity(context.Context, *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{
		Subject:        &v1.Subject{Id: billingIdentityID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT},
		OrganizationId: billingOrgID,
	}), nil
}

func (s *fakeBillingService) CreateTeamCreditAllocation(_ context.Context, req *connect.Request[v1.CreateTeamCreditAllocationRequest]) (*connect.Response[v1.CreateTeamCreditAllocationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.allocation != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("allocation already exists"))
	}
	s.allocation = &v1.TeamCreditAllocationInfo{
		Id:                   billingAllocationID,
		OrganizationId:       req.Msg.GetOrganizationId(),
		TeamId:               req.Msg.GetTeamId(),
		CreditBudget:         req.Msg.GetCreditBudget(),
		CostBudgetMicrounits: copyInt64(req.Msg.CostBudgetMicrounits),
		CostBudgetCurrency:   req.Msg.GetCostBudgetCurrency(),
		CreatedAt:            timestamppb.New(time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)),
	}
	return connect.NewResponse(&v1.CreateTeamCreditAllocationResponse{Allocation: cloneTeamAllocation(s.allocation)}), nil
}

func (s *fakeBillingService) GetTeamCreditAllocation(_ context.Context, req *connect.Request[v1.GetTeamCreditAllocationRequest]) (*connect.Response[v1.GetTeamCreditAllocationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.allocation == nil || req.Msg.GetOrganizationId() != s.allocation.GetOrganizationId() || req.Msg.GetTeamId() != s.allocation.GetTeamId() {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("allocation not found"))
	}
	return connect.NewResponse(&v1.GetTeamCreditAllocationResponse{Allocation: cloneTeamAllocation(s.allocation)}), nil
}

func (s *fakeBillingService) UpdateTeamCreditAllocation(_ context.Context, req *connect.Request[v1.UpdateTeamCreditAllocationRequest]) (*connect.Response[v1.UpdateTeamCreditAllocationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.allocation == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("allocation not found"))
	}
	if req.Msg.GetClearCostBudget() {
		s.allocation.CostBudgetMicrounits = nil
		s.allocation.CostBudgetCurrency = v1.BillingCurrency_BILLING_CURRENCY_UNSPECIFIED
	} else if req.Msg.CostBudgetMicrounits != nil {
		s.allocation.CostBudgetMicrounits = copyInt64(req.Msg.CostBudgetMicrounits)
		s.allocation.CostBudgetCurrency = req.Msg.GetCostBudgetCurrency()
	}
	if req.Msg.GetCreditBudget() > 0 {
		s.allocation.CreditBudget = req.Msg.GetCreditBudget()
	} else if !req.Msg.GetPreserveCreditBudget() {
		s.allocation.CreditBudget = 0
	}
	return connect.NewResponse(&v1.UpdateTeamCreditAllocationResponse{Allocation: cloneTeamAllocation(s.allocation)}), nil
}

func (s *fakeBillingService) DeleteTeamCreditAllocation(_ context.Context, _ *connect.Request[v1.DeleteTeamCreditAllocationRequest]) (*connect.Response[v1.DeleteTeamCreditAllocationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.allocation == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("allocation not found"))
	}
	if s.allocation.CostBudgetMicrounits != nil && s.allocation.GetCreditBudget() > 0 {
		s.allocation.CreditBudget = 0
	} else {
		s.allocation = nil
	}
	return connect.NewResponse(&v1.DeleteTeamCreditAllocationResponse{}), nil
}

func (s *fakeBillingService) allocationSnapshot() *v1.TeamCreditAllocationInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneTeamAllocation(s.allocation)
}

func cloneTeamAllocation(value *v1.TeamCreditAllocationInfo) *v1.TeamCreditAllocationInfo {
	if value == nil {
		return nil
	}
	return proto.CloneOf(value)
}

func copyInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

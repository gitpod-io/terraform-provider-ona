// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/encoding/protojson"
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
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)
	mux.HandleFunc("/gitpod.v1.BillingService/CreateTeamCreditAllocation", service.createTeamCreditAllocation)
	mux.HandleFunc("/gitpod.v1.BillingService/GetTeamCreditAllocation", service.getTeamCreditAllocation)
	mux.HandleFunc("/gitpod.v1.BillingService/UpdateTeamCreditAllocation", service.updateTeamCreditAllocation)
	mux.HandleFunc("/gitpod.v1.BillingService/DeleteTeamCreditAllocation", service.deleteTeamCreditAllocation)
	mux.Handle(identityPath, identityHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	return &billingAPIServer{Server: server, service: service}
}

type fakeBillingService struct {
	v1connect.UnimplementedIdentityServiceHandler

	mu         sync.Mutex
	allocation *v1.TeamCreditAllocationInfo
}

type fakeTeamCreditAllocationRequest struct {
	OrganizationID       string `json:"organizationId"`
	TeamID               string `json:"teamId"`
	CreditBudget         int64  `json:"creditBudget"`
	CostBudgetMicrounits *int64 `json:"costBudgetMicrounits"`
	CostBudgetCurrency   string `json:"costBudgetCurrency"`
	PreserveCreditBudget bool   `json:"preserveCreditBudget"`
	ClearCostBudget      bool   `json:"clearCostBudget"`
}

func (s *fakeBillingService) GetAuthenticatedIdentity(context.Context, *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{
		Subject:        &v1.Subject{Id: billingIdentityID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT},
		OrganizationId: billingOrgID,
	}), nil
}

func (s *fakeBillingService) createTeamCreditAllocation(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeFakeTeamRequest(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.allocation != nil {
		writeFakeError(w, http.StatusConflict, "already_exists", "allocation already exists")
		return
	}
	s.allocation = &v1.TeamCreditAllocationInfo{
		Id:                   billingAllocationID,
		OrganizationId:       req.OrganizationID,
		TeamId:               req.TeamID,
		CreditBudget:         req.CreditBudget,
		CostBudgetMicrounits: copyInt64(req.CostBudgetMicrounits),
		CostBudgetCurrency:   fakeCurrency(req.CostBudgetCurrency),
		CreatedAt:            timestamppb.New(time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)),
	}
	writeFakeAllocation(w, s.allocation)
}

func (s *fakeBillingService) getTeamCreditAllocation(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeFakeTeamRequest(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.allocation == nil || req.OrganizationID != s.allocation.GetOrganizationId() || req.TeamID != s.allocation.GetTeamId() {
		writeFakeError(w, http.StatusNotFound, "not_found", "allocation not found")
		return
	}
	writeFakeAllocation(w, s.allocation)
}

func (s *fakeBillingService) updateTeamCreditAllocation(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeFakeTeamRequest(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.allocation == nil {
		writeFakeError(w, http.StatusNotFound, "not_found", "allocation not found")
		return
	}
	if req.ClearCostBudget {
		s.allocation.CostBudgetMicrounits = nil
		s.allocation.CostBudgetCurrency = v1.BillingCurrency_BILLING_CURRENCY_UNSPECIFIED
	} else if req.CostBudgetMicrounits != nil {
		s.allocation.CostBudgetMicrounits = copyInt64(req.CostBudgetMicrounits)
		s.allocation.CostBudgetCurrency = fakeCurrency(req.CostBudgetCurrency)
	}
	if req.CreditBudget > 0 {
		s.allocation.CreditBudget = req.CreditBudget
	} else if !req.PreserveCreditBudget {
		s.allocation.CreditBudget = 0
	}
	writeFakeAllocation(w, s.allocation)
}

func (s *fakeBillingService) deleteTeamCreditAllocation(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.allocation == nil {
		writeFakeError(w, http.StatusNotFound, "not_found", "allocation not found")
		return
	}
	if s.allocation.CostBudgetMicrounits != nil && s.allocation.GetCreditBudget() > 0 {
		s.allocation.CreditBudget = 0
	} else {
		s.allocation = nil
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, "{}")
}

func decodeFakeTeamRequest(w http.ResponseWriter, r *http.Request) (fakeTeamCreditAllocationRequest, bool) {
	var req fakeTeamCreditAllocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFakeError(w, http.StatusBadRequest, "invalid_argument", err.Error())
		return req, false
	}
	return req, true
}

func writeFakeAllocation(w http.ResponseWriter, allocation *v1.TeamCreditAllocationInfo) {
	raw, err := protojson.Marshal(cloneTeamAllocation(allocation))
	if err != nil {
		writeFakeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"allocation":%s}`, raw)
}

func writeFakeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}

func fakeCurrency(value string) v1.BillingCurrency {
	switch value {
	case v1.BillingCurrency_BILLING_CURRENCY_USD.String():
		return v1.BillingCurrency_BILLING_CURRENCY_USD
	default:
		return v1.BillingCurrency_BILLING_CURRENCY_UNSPECIFIED
	}
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
	cloned := *value
	cloned.CostBudgetMicrounits = copyInt64(value.CostBudgetMicrounits)
	return &cloned
}

func copyInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

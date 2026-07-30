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
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccAutomationResource(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	server.service.gracefulDelete = true
	fullCodexSettings := `
  codex_settings = {
    model            = "gpt-5.6-sol"
    reasoning_effort = "high"
    service_tier     = "fast"
  }
`
	partialCodexSettings := `
  codex_settings = {
    reasoning_effort = "medium"
  }
`
	emptyCodexSettings := `
  codex_settings = {}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			force, calls := server.service.deleteStats()
			if calls == 0 {
				return errors.New("DeleteWorkflow was not called")
			}
			if force {
				return errors.New("DeleteWorkflow used force=true")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccWorkflowConfigWithCodexSettings(server.URL, "Nightly checks", "Runs checks", "make test", true, false, fullCodexSettings),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_automation.test", "id", workflowAccID1),
					resource.TestCheckResourceAttr("ona_automation.test", "name", "Nightly checks"),
					resource.TestCheckResourceAttr("ona_automation.test", "description", "Runs checks"),
					resource.TestCheckResourceAttr("ona_automation.test", "disabled", "true"),
					resource.TestCheckResourceAttr("ona_automation.test", "executor.id", workflowCreatorID),
					resource.TestCheckResourceAttr("ona_automation.test", "executor.principal", "user"),
					resource.TestCheckResourceAttr("ona_automation.test", "triggers.#", "1"),
					resource.TestCheckResourceAttr("ona_automation.test", "action.limits.max_parallel", "2"),
					resource.TestCheckResourceAttr("ona_automation.test", "action.limits.max_time", "60m"),
					resource.TestCheckResourceAttr("ona_automation.test", "action.steps.0.task.command", "make test"),
					resource.TestCheckResourceAttr("ona_automation.test", "codex_settings.model", "gpt-5.6-sol"),
					resource.TestCheckResourceAttr("ona_automation.test", "codex_settings.reasoning_effort", "high"),
					resource.TestCheckResourceAttr("ona_automation.test", "codex_settings.service_tier", "fast"),
					func(*terraform.State) error {
						if got := server.service.disabledUpdateCount(); got != 1 {
							return fmt.Errorf("initial disabled update count = %d, want 1", got)
						}
						create, _ := server.service.requestStats()
						if create == nil || create.GetAgentId() != workflowCodexAgentID {
							return fmt.Errorf("CreateWorkflow agent_id = %q, want %q", create.GetAgentId(), workflowCodexAgentID)
						}
						return nil
					},
				),
			},
			{
				Config:           testAccWorkflowConfigWithCodexSettings(server.URL, "Nightly checks", "Runs checks", "make test", true, false, fullCodexSettings),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:            "ona_automation.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"action.limits.max_time"},
			},
			{
				Config:           testAccWorkflowConfigWithCodexSettings(server.URL, "Nightly checks", "Runs checks", "make test", true, false, fullCodexSettings),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: testAccWorkflowConfigWithCodexSettings(server.URL, "Nightly checks updated", "Updated checks", "make test-unit", false, true, partialCodexSettings),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_automation.test", "name", "Nightly checks updated"),
					resource.TestCheckResourceAttr("ona_automation.test", "disabled", "false"),
					resource.TestCheckResourceAttr("ona_automation.test", "executor.id", workflowServiceAccountID),
					resource.TestCheckResourceAttr("ona_automation.test", "executor.principal", "service_account"),
					resource.TestCheckResourceAttr("ona_automation.test", "action.steps.0.task.command", "make test-unit"),
					resource.TestCheckNoResourceAttr("ona_automation.test", "codex_settings.model"),
					resource.TestCheckResourceAttr("ona_automation.test", "codex_settings.reasoning_effort", "medium"),
					func(*terraform.State) error {
						_, update := server.service.requestStats()
						if update == nil || update.AgentId != nil {
							return fmt.Errorf("UpdateWorkflow agent_id = %#v, want absent", update.GetAgentId())
						}
						return nil
					},
				),
			},
			{
				Config: testAccWorkflowConfigWithCodexSettings(server.URL, "Nightly checks updated", "Updated checks", "make test-unit", false, true, emptyCodexSettings),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ona_automation.test", "codex_settings.model"),
					resource.TestCheckNoResourceAttr("ona_automation.test", "codex_settings.reasoning_effort"),
					resource.TestCheckNoResourceAttr("ona_automation.test", "codex_settings.service_tier"),
				),
			},
			{
				Config:           testAccWorkflowConfigWithCodexSettings(server.URL, "Nightly checks updated", "Updated checks", "make test-unit", false, true, emptyCodexSettings),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: testAccWorkflowConfig(server.URL, "Nightly checks updated", "", "make test-unit", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_automation.test", "description", ""),
					resource.TestCheckResourceAttr("ona_automation.test", "executor.id", workflowServiceAccountID),
					resource.TestCheckResourceAttr("ona_automation.test", "executor.principal", "service_account"),
				),
			},
			{
				Config:           testAccWorkflowConfig(server.URL, "Nightly checks updated", "", "make test-unit", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				PreConfig: func() {
					server.service.updateCodexSettings(workflowAccID1, &v1.CodexSettings{Model: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_5})
				},
				Config:           testAccWorkflowConfig(server.URL, "Nightly checks updated", "", "make test-unit", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ona_automation.test", "codex_settings.model"),
					func(*terraform.State) error {
						_, update := server.service.requestStats()
						if update == nil || update.GetCodexSettings() == nil || update.GetCodexSettings().GetModel() != v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_UNSPECIFIED {
							return fmt.Errorf("UpdateWorkflow codex_settings = %#v, want explicit defaults", update.GetCodexSettings())
						}
						return nil
					},
				),
			},
			{
				Config:           testAccWorkflowConfig(server.URL, "Nightly checks updated", "", "make test-unit", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				PreConfig:        func() { server.service.updateName(workflowAccID1, "Out-of-band name") },
				Config:           testAccWorkflowConfig(server.URL, "Nightly checks updated", "", "make test-unit", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()}},
				Check:            resource.TestCheckResourceAttr("ona_automation.test", "name", "Nightly checks updated"),
			},
			{
				PreConfig: func() { server.service.markDeleting(workflowAccID1) },
				Config:    testAccWorkflowConfig(server.URL, "Nightly checks updated", "", "make test-unit", false),
				Check:     resource.TestCheckResourceAttr("ona_automation.test", "id", workflowAccID2),
			},
			{
				PreConfig: func() { server.service.remove(workflowAccID2) },
				Config:    testAccWorkflowConfig(server.URL, "Nightly checks updated", "", "make test-unit", false),
				Check:     resource.TestCheckResourceAttr("ona_automation.test", "id", workflowAccID3),
			},
		},
	})
}

func TestAccAutomationsDataSource(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed(testAPIWorkflow(workflowAccID2, "Second"))
	server.service.seed(testAPIWorkflow(workflowAccID1, "First"))
	server.service.listPageSize = 1

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccAutomationsDataSourceConfig(server.URL),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.ona_automations.test", "id", "automations"),
				resource.TestCheckResourceAttr("data.ona_automations.test", "automations.#", "2"),
				resource.TestCheckResourceAttr("data.ona_automations.test", "automations.0.id", workflowAccID1),
				resource.TestCheckResourceAttr("data.ona_automations.test", "automations.1.id", workflowAccID2),
				resource.TestCheckResourceAttr("data.ona_automations.test", "automations.0.executor.principal", "user"),
				func(*terraform.State) error {
					filter, calls := server.service.listStats()
					if calls != 2 {
						return fmt.Errorf("ListWorkflows call count = %d, want 2", calls)
					}
					if filter == nil || filter.Disabled == nil || filter.GetDisabled() {
						return fmt.Errorf("ListWorkflows disabled filter = %#v, want false", filter)
					}
					return nil
				},
			),
		}},
	})
}

func TestAccWorkflowCreateAPIError(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	server.service.createErr = connect.NewError(connect.CodePermissionDenied, errors.New("service accounts cannot create automations"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccWorkflowConfig(server.URL, "Nightly checks", "", "make test", false),
			ExpectError: regexp.MustCompile("service accounts cannot create automations"),
		}},
	})
}

func TestAccWorkflowInitialDisableRetry(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	server.service.disableErrOnce = true

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccWorkflowConfig(server.URL, "Retry disable", "", "make test", true),
				ExpectError: regexp.MustCompile("disable failed"),
			},
			{
				Config: testAccWorkflowConfig(server.URL, "Retry disable", "", "make test", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_automation.test", "id", workflowAccID2),
					resource.TestCheckResourceAttr("ona_automation.test", "disabled", "true"),
					func(*terraform.State) error {
						creates, deletes, active := server.service.lifecycleCounts()
						if creates != 2 || deletes != 1 || active != 1 {
							return fmt.Errorf("workflow lifecycle counts = creates:%d deletes:%d active:%d, want 2/1/1", creates, deletes, active)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccWorkflowCreateResponseRequiresCodex(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	server.service.createResponseAgentID = workflowOnaAgentID

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			_, deletes, active := server.service.lifecycleCounts()
			if deletes != 1 || active != 0 {
				return fmt.Errorf("workflow cleanup = deletes:%d active:%d, want 1/0", deletes, active)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config:      testAccWorkflowConfig(server.URL, "Invalid create response", "", "make test", false),
			ExpectError: regexp.MustCompile("Unable to Create Codex Automation"),
		}},
	})
}

func TestAccWorkflowInitialDisableResponseRequiresCodex(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	server.service.setNextUpdateResponseAgent(workflowOnaAgentID)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccWorkflowConfig(server.URL, "Invalid disable response", "", "make test", true),
			ExpectError: regexp.MustCompile("Unable to Disable Codex Automation"),
		}},
	})
}

func TestAccWorkflowUpdateOwnershipError(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccWorkflowConfig(server.URL, "Ownership", "", "make test", false)},
			{
				PreConfig: func() {
					server.service.setUpdateError(connect.NewError(connect.CodeFailedPrecondition, errors.New("updating automation spec requires setting the executor to yourself or a service account")))
				},
				Config:      testAccWorkflowConfig(server.URL, "Ownership", "", "make test-unit", false),
				ExpectError: regexp.MustCompile("(?s)requires setting.*executor"),
			},
		},
	})
}

func TestAccWorkflowUpdatePreflightError(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccWorkflowConfig(server.URL, "Preflight", "", "make test", false)},
			{
				PreConfig: func() {
					server.service.setGetErrorAfter(1, connect.NewError(connect.CodeInternal, errors.New("preflight failed")))
				},
				Config:      testAccWorkflowConfig(server.URL, "Preflight updated", "", "make test", false),
				ExpectError: regexp.MustCompile("preflight failed"),
			},
		},
	})
}

func TestAccWorkflowUpdatePreflightNotFound(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccWorkflowConfig(server.URL, "Preflight missing", "", "make test", false)},
			{
				PreConfig:   func() { server.service.setGetNotFoundAfter(1) },
				Config:      testAccWorkflowConfig(server.URL, "Preflight missing updated", "", "make test", false),
				ExpectError: regexp.MustCompile("workflow no longer exists"),
			},
		},
	})
}

func TestAccAutomationsDataSourceAPIError(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed(testAPIWorkflow(workflowAccID1, "First"))
	server.service.seed(testAPIWorkflow(workflowAccID2, "Second"))
	server.service.listPageSize = 1
	server.service.listErr = connect.NewError(connect.CodeInternal, errors.New("list failed"))
	server.service.listErrAfter = 1

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccAutomationsDataSourceConfig(server.URL),
			ExpectError: regexp.MustCompile("list failed"),
		}},
	})
}

func TestAccWorkflowUnsupportedImport(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	workflow := testAPIWorkflow(workflowUnsupportedID, "Unsupported")
	workflow.Spec.Report = &v1.WorkflowAction{Limits: &v1.WorkflowAction_Limits{MaxParallel: 1, MaxTotal: 1}, Steps: []*v1.WorkflowStep{{Step: &v1.WorkflowStep_Report_{Report: &v1.WorkflowStep_Report{}}}}}
	server.service.seed(workflow)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:        testAccWorkflowConfig(server.URL, "Unsupported", "", "make test", false),
			ResourceName:  "ona_automation.test",
			ImportState:   true,
			ImportStateId: workflowUnsupportedID,
			ExpectError:   regexp.MustCompile("Unsupported Ona Workflow"),
		}},
	})
}

func TestAccWorkflowNonCodexTransition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name    string
		AgentID string
	}{
		{Name: "unpinned", AgentID: ""},
		{Name: "ona_agent", AgentID: workflowOnaAgentID},
		{Name: "unknown_agent", AgentID: "00000000-0000-0000-0000-000000009999"},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newWorkflowAPIServer(t)
			t.Cleanup(server.Close)
			workflow := testAPIWorkflow(workflowUnsupportedID, "Existing automation")
			workflow.Metadata.Description = "Existing description"
			workflow.Spec.AgentId = tc.AgentID
			workflow.Spec.CodexSettings = &v1.CodexSettings{}
			workflow.Spec.Action.Limits.PerExecution = &v1.WorkflowAction_Limits_PerExecution{MaxTime: durationpb.New(time.Hour)}
			server.service.seed(workflow)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy: func(*terraform.State) error {
					_, calls := server.service.deleteStats()
					if calls == 0 {
						return errors.New("DeleteWorkflow was not called for the non-Codex automation")
					}
					return nil
				},
				Steps: []resource.TestStep{
					{
						Config:             testAccImportedWorkflowConfig(server.URL, "Existing automation", "Existing description", "make test"),
						ResourceName:       "ona_automation.test",
						ImportState:        true,
						ImportStateId:      workflowUnsupportedID,
						ImportStatePersist: true,
					},
					{
						Config:           testAccImportedWorkflowConfig(server.URL, "Existing automation", "Existing description", "make test"),
						ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
					},
					{
						Config:      testAccImportedWorkflowConfig(server.URL, "Changed automation", "Existing description", "make test"),
						ExpectError: regexp.MustCompile("Unable to Update Non-Codex Automation"),
					},
				},
			})

			if got := server.service.updateCallCount(); got != 0 {
				t.Errorf("UpdateWorkflow call count = %d, want 0", got)
			}
		})
	}
}

func TestAccWorkflowUpdateResponseLosesCodexAgent(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccWorkflowConfig(server.URL, "Codex automation", "", "make test", false)},
			{
				PreConfig:   func() { server.service.setNextUpdateResponseAgent(workflowOnaAgentID) },
				Config:      testAccWorkflowConfig(server.URL, "Changed automation", "", "make test", false),
				ExpectError: regexp.MustCompile("Unable to Update Codex Automation"),
			},
		},
	})
}

func TestAccWorkflowEmptyGetResponse(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccWorkflowConfig(server.URL, "Empty response", "", "make test", false)},
			{
				PreConfig:   func() { server.service.setEmptyGetResponse(true) },
				Config:      testAccWorkflowConfig(server.URL, "Empty response", "", "make test", false),
				ExpectError: regexp.MustCompile("the Ona API returned an empty workflow"),
			},
		},
	})
}

func TestAccWorkflowDeleteNotFound(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	server.service.deleteNotFound = true

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			_, calls := server.service.deleteStats()
			if calls != 1 {
				return fmt.Errorf("DeleteWorkflow call count = %d, want 1", calls)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: testAccWorkflowConfig(server.URL, "Delete missing", "", "make test", false),
		}},
	})
}

func testAccWorkflowConfig(host, name, description, command string, disabled bool) string {
	return testAccWorkflowConfigWithCodexSettings(host, name, description, command, disabled, false, "")
}

func testAccImportedWorkflowConfig(host, name, description, command string) string {
	return strings.Replace(testAccWorkflowConfig(host, name, description, command, false), `max_time     = "60m"`, `max_time     = "1h0m0s"`, 1)
}

func testAccWorkflowConfigWithCodexSettings(host, name, description, command string, disabled, explicitExecutor bool, codexSettings string) string {
	executor := ""
	if explicitExecutor {
		executor = fmt.Sprintf(`
  executor = {
    id        = %q
    principal = "service_account"
  }
`, workflowServiceAccountID)
	}
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_automation" "test" {
  name        = %[2]q
  description = %[3]q
  disabled    = %[5]t
%[6]s
%[8]s
  triggers = [{
    manual = {}
    context = {
      projects = {
        project_ids = [%[7]q]
      }
    }
  }]

  action = {
    limits = {
      max_parallel = 2
      max_total    = 10
      max_time     = "60m"
    }
    steps = [{
      task = {
        command = %[4]q
      }
    }]
  }
}
`, host, name, description, command, disabled, executor, workflowProjectID, codexSettings)
}

func testAccAutomationsDataSourceConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

data "ona_automations" "test" {
  automation_ids = [%[2]q, %[3]q]
  disabled       = false
}
`, host, workflowAccID2, workflowAccID1)
}

type workflowAPIServer struct {
	*httptest.Server
	service *fakeWorkflowService
}

func newWorkflowAPIServer(t *testing.T) *workflowAPIServer {
	t.Helper()
	service := &fakeWorkflowService{workflows: make(map[string]*v1.Workflow), now: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)}
	servicePath, handler := v1connect.NewWorkflowServiceHandler(service)
	server := httptest.NewServer(http.StripPrefix("/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == servicePath || len(r.URL.Path) > len(servicePath) && r.URL.Path[:len(servicePath)] == servicePath {
			handler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	return &workflowAPIServer{Server: server, service: service}
}

type fakeWorkflowService struct {
	v1connect.UnimplementedWorkflowServiceHandler

	mu                    sync.Mutex
	workflows             map[string]*v1.Workflow
	nextID                int
	now                   time.Time
	createErr             error
	updateErr             error
	listErr               error
	listErrAfter          int
	emptyGet              bool
	getErr                error
	getErrAfter           int
	getNotFoundAfter      int
	getCalls              int
	deleteNotFound        bool
	disableErrOnce        bool
	gracefulDelete        bool
	createCalls           int
	updateCalls           int
	listPageSize          int
	listCalls             int
	lastFilter            *v1.ListWorkflowsRequest_Filter
	disableUpdates        int
	deleteCalls           int
	lastDeleteForce       bool
	lastCreate            *v1.CreateWorkflowRequest
	lastUpdate            *v1.UpdateWorkflowRequest
	nextResponseAgentID   string
	createResponseAgentID string
}

func (s *fakeWorkflowService) CreateWorkflow(ctx context.Context, req *connect.Request[v1.CreateWorkflowRequest]) (*connect.Response[v1.CreateWorkflowResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.createCalls++
	s.lastCreate = cloneCreateWorkflowRequest(req.Msg)
	s.nextID++
	id := fmt.Sprintf("00000000-0000-0000-0000-%012d", s.nextID)
	executor := req.Msg.GetExecutor()
	if executor == nil {
		executor = &v1.Subject{Id: workflowCreatorID, Principal: v1.Principal_PRINCIPAL_USER}
	}
	workflow := &v1.Workflow{
		Id: id,
		Metadata: &v1.Workflow_Metadata{
			Name: req.Msg.GetName(), Description: req.Msg.GetDescription(), Creator: &v1.Subject{Id: workflowCreatorID, Principal: v1.Principal_PRINCIPAL_USER},
			Executor: cloneSubject(executor), CreatedAt: timestamppb.New(s.now), UpdatedAt: timestamppb.New(s.now),
		},
		Spec: &v1.Workflow_Spec{
			Triggers: cloneTriggers(req.Msg.GetTriggers()), Action: cloneAction(req.Msg.GetAction()), AgentId: req.Msg.GetAgentId(), CodexSettings: cloneCodexSettings(req.Msg.GetCodexSettings()),
		},
		WebhookUrl: "https://example.com/workflows/" + id + "/webhooks",
	}
	if s.createResponseAgentID != "" {
		workflow.Spec.AgentId = s.createResponseAgentID
	}
	s.workflows[id] = workflow
	return connect.NewResponse(&v1.CreateWorkflowResponse{Workflow: cloneWorkflow(workflow)}), nil
}

func (s *fakeWorkflowService) GetWorkflow(ctx context.Context, req *connect.Request[v1.GetWorkflowRequest]) (*connect.Response[v1.GetWorkflowResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getCalls++
	if s.getErr != nil && s.getErrAfter > 0 && s.getCalls > s.getErrAfter {
		return nil, s.getErr
	}
	if s.getNotFoundAfter > 0 && s.getCalls > s.getNotFoundAfter {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("workflow not found"))
	}
	if s.emptyGet {
		return connect.NewResponse(&v1.GetWorkflowResponse{}), nil
	}
	workflow := s.workflows[req.Msg.GetWorkflowId()]
	if workflow == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("workflow not found"))
	}
	return connect.NewResponse(&v1.GetWorkflowResponse{Workflow: cloneWorkflow(workflow)}), nil
}

func (s *fakeWorkflowService) setEmptyGetResponse(value bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emptyGet = value
}

func (s *fakeWorkflowService) setGetErrorAfter(successfulCalls int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getErrAfter = s.getCalls + successfulCalls
	s.getErr = err
}

func (s *fakeWorkflowService) setGetNotFoundAfter(successfulCalls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getNotFoundAfter = s.getCalls + successfulCalls
}

func (s *fakeWorkflowService) UpdateWorkflow(ctx context.Context, req *connect.Request[v1.UpdateWorkflowRequest]) (*connect.Response[v1.UpdateWorkflowResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	s.updateCalls++
	s.lastUpdate = cloneUpdateWorkflowRequest(req.Msg)
	workflow := s.workflows[req.Msg.GetWorkflowId()]
	if workflow == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("workflow not found"))
	}
	if req.Msg.Name != nil {
		workflow.Metadata.Name = req.Msg.GetName()
	}
	if req.Msg.Description != nil {
		workflow.Metadata.Description = req.Msg.GetDescription()
	}
	if len(req.Msg.GetTriggers()) > 0 {
		workflow.Spec.Triggers = cloneTriggers(req.Msg.GetTriggers())
	}
	if req.Msg.GetAction() != nil {
		workflow.Spec.Action = cloneAction(req.Msg.GetAction())
	}
	if req.Msg.GetExecutor() != nil {
		workflow.Metadata.Executor = cloneSubject(req.Msg.GetExecutor())
	}
	if req.Msg.Disabled != nil {
		if s.disableErrOnce && req.Msg.Name == nil && req.Msg.Action == nil && len(req.Msg.Triggers) == 0 {
			s.disableErrOnce = false
			return nil, connect.NewError(connect.CodeInternal, errors.New("disable failed"))
		}
		workflow.Spec.Disabled = req.Msg.GetDisabled()
		if req.Msg.Name == nil && req.Msg.Action == nil && len(req.Msg.Triggers) == 0 {
			s.disableUpdates++
		}
	}
	if req.Msg.AgentId != nil {
		workflow.Spec.AgentId = req.Msg.GetAgentId()
	}
	if req.Msg.CodexSettings != nil {
		workflow.Spec.CodexSettings = cloneCodexSettings(req.Msg.GetCodexSettings())
	}
	if s.nextResponseAgentID != "" {
		workflow.Spec.AgentId = s.nextResponseAgentID
		s.nextResponseAgentID = ""
	}
	workflow.Metadata.UpdatedAt = timestamppb.New(s.now.Add(time.Minute))
	return connect.NewResponse(&v1.UpdateWorkflowResponse{Workflow: cloneWorkflow(workflow)}), nil
}

func (s *fakeWorkflowService) DeleteWorkflow(ctx context.Context, req *connect.Request[v1.DeleteWorkflowRequest]) (*connect.Response[v1.DeleteWorkflowResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteCalls++
	s.lastDeleteForce = req.Msg.GetForce()
	if s.deleteNotFound {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("workflow not found"))
	}
	if s.workflows[req.Msg.GetWorkflowId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("workflow not found"))
	}
	if s.gracefulDelete {
		s.workflows[req.Msg.GetWorkflowId()].Spec.Deleting = true
	} else {
		delete(s.workflows, req.Msg.GetWorkflowId())
	}
	return connect.NewResponse(&v1.DeleteWorkflowResponse{}), nil
}

func (s *fakeWorkflowService) ListWorkflows(ctx context.Context, req *connect.Request[v1.ListWorkflowsRequest]) (*connect.Response[v1.ListWorkflowsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listCalls++
	if s.listErr != nil && (s.listErrAfter == 0 || s.listCalls > s.listErrAfter) {
		return nil, s.listErr
	}
	if req.Msg.GetFilter() != nil {
		s.lastFilter = cloneWorkflowFilter(req.Msg.GetFilter())
	}
	allowedIDs := make(map[string]struct{})
	for _, id := range req.Msg.GetFilter().GetWorkflowIds() {
		allowedIDs[id] = struct{}{}
	}
	var workflows []*v1.Workflow
	for _, workflow := range s.workflows {
		if len(allowedIDs) > 0 {
			if _, ok := allowedIDs[workflow.GetId()]; !ok {
				continue
			}
		}
		if req.Msg.GetFilter().Disabled != nil && workflow.GetSpec().GetDisabled() != req.Msg.GetFilter().GetDisabled() {
			continue
		}
		workflows = append(workflows, cloneWorkflow(workflow))
	}
	// Deliberately return reverse-ID order so the acceptance test proves that
	// the data source performs its own deterministic sort after pagination.
	sort.Slice(workflows, func(i, j int) bool { return workflows[i].GetId() > workflows[j].GetId() })
	start := 0
	if req.Msg.GetPagination().GetToken() != "" {
		start, _ = strconv.Atoi(req.Msg.GetPagination().GetToken())
	}
	pageSize := len(workflows)
	if s.listPageSize > 0 {
		pageSize = s.listPageSize
	}
	end := start + pageSize
	if end > len(workflows) {
		end = len(workflows)
	}
	next := ""
	if end < len(workflows) {
		next = strconv.Itoa(end)
	}
	return connect.NewResponse(&v1.ListWorkflowsResponse{Workflows: workflows[start:end], Pagination: &v1.PaginationResponse{NextToken: next}}), nil
}

func (s *fakeWorkflowService) seed(workflow *v1.Workflow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workflows[workflow.GetId()] = cloneWorkflow(workflow)
}

func (s *fakeWorkflowService) updateName(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workflow := s.workflows[id]; workflow != nil {
		workflow.Metadata.Name = name
	}
}

func (s *fakeWorkflowService) updateCodexSettings(id string, settings *v1.CodexSettings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workflow := s.workflows[id]; workflow != nil {
		workflow.Spec.CodexSettings = cloneCodexSettings(settings)
	}
}

func (s *fakeWorkflowService) markDeleting(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workflow := s.workflows[id]; workflow != nil {
		workflow.Spec.Deleting = true
	}
}

func (s *fakeWorkflowService) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workflows, id)
}

func (s *fakeWorkflowService) setUpdateError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateErr = err
}

func (s *fakeWorkflowService) disabledUpdateCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disableUpdates
}

func (s *fakeWorkflowService) lifecycleCounts() (creates, deletes, active int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createCalls, s.deleteCalls, len(s.workflows)
}

func (s *fakeWorkflowService) deleteStats() (bool, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastDeleteForce, s.deleteCalls
}

func (s *fakeWorkflowService) listStats() (*v1.ListWorkflowsRequest_Filter, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneWorkflowFilter(s.lastFilter), s.listCalls
}

func (s *fakeWorkflowService) requestStats() (*v1.CreateWorkflowRequest, *v1.UpdateWorkflowRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var create *v1.CreateWorkflowRequest
	if s.lastCreate != nil {
		create = cloneCreateWorkflowRequest(s.lastCreate)
	}
	var update *v1.UpdateWorkflowRequest
	if s.lastUpdate != nil {
		update = cloneUpdateWorkflowRequest(s.lastUpdate)
	}
	return create, update
}

func (s *fakeWorkflowService) updateCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateCalls
}

func (s *fakeWorkflowService) setNextUpdateResponseAgent(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextResponseAgentID = agentID
}

func testAPIWorkflow(id, name string) *v1.Workflow {
	return &v1.Workflow{
		Id: id,
		Metadata: &v1.Workflow_Metadata{
			Name: name, Creator: &v1.Subject{Id: workflowCreatorID, Principal: v1.Principal_PRINCIPAL_USER}, Executor: &v1.Subject{Id: workflowCreatorID, Principal: v1.Principal_PRINCIPAL_USER},
			CreatedAt: timestamppb.New(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)), UpdatedAt: timestamppb.New(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)),
		},
		Spec: &v1.Workflow_Spec{
			Triggers: []*v1.WorkflowTrigger{{Trigger: &v1.WorkflowTrigger_Manual_{Manual: &v1.WorkflowTrigger_Manual{}}, Context: &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_Projects_{Projects: &v1.WorkflowTriggerContext_Projects{ProjectIds: []string{workflowProjectID}}}}}},
			Action:   &v1.WorkflowAction{Limits: &v1.WorkflowAction_Limits{MaxParallel: 2, MaxTotal: 10}, Steps: []*v1.WorkflowStep{{Step: &v1.WorkflowStep_Task_{Task: &v1.WorkflowStep_Task{Command: "make test"}}}}},
		},
		WebhookUrl: "https://example.com/workflows/" + id + "/webhooks",
	}
}

func cloneWorkflow(value *v1.Workflow) *v1.Workflow {
	if value == nil {
		return nil
	}
	result := &v1.Workflow{}
	proto.Merge(result, value)
	return result
}

func cloneAction(value *v1.WorkflowAction) *v1.WorkflowAction {
	if value == nil {
		return nil
	}
	result := &v1.WorkflowAction{}
	proto.Merge(result, value)
	return result
}

func cloneTriggers(value []*v1.WorkflowTrigger) []*v1.WorkflowTrigger {
	result := make([]*v1.WorkflowTrigger, 0, len(value))
	for _, trigger := range value {
		cloned := &v1.WorkflowTrigger{}
		proto.Merge(cloned, trigger)
		result = append(result, cloned)
	}
	return result
}

func cloneSubject(value *v1.Subject) *v1.Subject {
	if value == nil {
		return nil
	}
	result := &v1.Subject{}
	proto.Merge(result, value)
	return result
}

func cloneCodexSettings(value *v1.CodexSettings) *v1.CodexSettings {
	if value == nil {
		return nil
	}
	result := &v1.CodexSettings{}
	proto.Merge(result, value)
	return result
}

func cloneCreateWorkflowRequest(value *v1.CreateWorkflowRequest) *v1.CreateWorkflowRequest {
	if value == nil {
		return nil
	}
	result := &v1.CreateWorkflowRequest{}
	proto.Merge(result, value)
	return result
}

func cloneUpdateWorkflowRequest(value *v1.UpdateWorkflowRequest) *v1.UpdateWorkflowRequest {
	if value == nil {
		return nil
	}
	result := &v1.UpdateWorkflowRequest{}
	proto.Merge(result, value)
	return result
}

func cloneWorkflowFilter(value *v1.ListWorkflowsRequest_Filter) *v1.ListWorkflowsRequest_Filter {
	if value == nil {
		return nil
	}
	result := &v1.ListWorkflowsRequest_Filter{}
	proto.Merge(result, value)
	return result
}

const (
	workflowAccID1           = "00000000-0000-0000-0000-000000000001"
	workflowAccID2           = "00000000-0000-0000-0000-000000000002"
	workflowAccID3           = "00000000-0000-0000-0000-000000000003"
	workflowUnsupportedID    = "00000000-0000-0000-0000-000000000099"
	workflowCreatorID        = "00000000-0000-0000-0000-000000000010"
	workflowServiceAccountID = "00000000-0000-0000-0000-000000000020"
	workflowProjectID        = "00000000-0000-0000-0000-000000000030"
	workflowCodexAgentID     = "00000000-0000-0000-0000-000000007800"
	workflowOnaAgentID       = "00000000-0000-0000-0000-000000007100"
)

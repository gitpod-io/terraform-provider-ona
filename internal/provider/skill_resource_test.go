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

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
)

const (
	skillTestOrgID      = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	skillTestOtherOrgID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	skillTestID1        = "11111111-1111-4111-8111-111111111111"
	skillTestID2        = "22222222-2222-4222-8222-222222222222"
	skillTestID3        = "33333333-3333-4333-8333-333333333333"
)

func TestAccSkillResourceLifecycle(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newSkillAPIServer(t)
	t.Cleanup(server.Close)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if server.service.count() != 0 {
				return errors.New("skill was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccSkillResourceConfig(server.URL, "Security review", "Review security-sensitive changes.", "Initial prompt.", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_skill.test", "id", skillTestID1),
					resource.TestCheckResourceAttr("ona_skill.test", "name", "Security review"),
					resource.TestCheckResourceAttr("ona_skill.test", "description", "Review security-sensitive changes."),
					resource.TestCheckResourceAttr("ona_skill.test", "prompt", "Initial prompt."),
					resource.TestCheckNoResourceAttr("ona_skill.test", "command"),
				),
			},
			{
				Config: testAccSkillResourceConfig(server.URL, "Security review", "Review security-sensitive changes.", "Initial prompt.", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
			{
				ResourceName:      "ona_skill.test",
				ImportState:       true,
				ImportStateId:     skillTestID1,
				ImportStateVerify: true,
			},
			{
				ResourceName:    "ona_skill.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 || states[0].Attributes["id"] != skillTestID1 {
						return fmt.Errorf("structured identity imported unexpected skill state: %#v", states)
					}
					return nil
				},
			},
			{
				PreConfig: func() { server.service.setName(skillTestID1, "Remote edit") },
				Config:    testAccSkillResourceConfig(server.URL, "Security review", "Review security-sensitive changes.", "Initial prompt.", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_skill.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckResourceAttr("ona_skill.test", "name", "Security review"),
			},
			{
				Config: testAccSkillResourceConfig(server.URL, "Security review v2", "Review all changes.", "Updated prompt.", "security-review"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_skill.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_skill.test", "id", skillTestID1),
					resource.TestCheckResourceAttr("ona_skill.test", "name", "Security review v2"),
					resource.TestCheckResourceAttr("ona_skill.test", "description", "Review all changes."),
					resource.TestCheckResourceAttr("ona_skill.test", "prompt", "Updated prompt."),
					resource.TestCheckResourceAttr("ona_skill.test", "command", "security-review"),
				),
			},
			{
				Config: testAccSkillResourceConfig(server.URL, "Security review v2", "Review all changes.", "Updated prompt.", "secure-review"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_skill.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckResourceAttr("ona_skill.test", "command", "secure-review"),
			},
			{
				Config: testAccSkillResourceConfig(server.URL, "Security review v2", "Review all changes.", "Updated prompt.", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_skill.test", plancheck.ResourceActionReplace),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_skill.test", "id", skillTestID2),
					resource.TestCheckNoResourceAttr("ona_skill.test", "command"),
				),
			},
		},
	})
}

func TestAccSkillResourceRejectsDuplicateCommand(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newSkillAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed(&v1.Prompt{
		Id:       skillTestID1,
		Metadata: &v1.PromptMetadata{OrganizationId: skillTestOrgID, Name: "Existing", Description: "Existing skill."},
		Spec:     &v1.PromptSpec{Prompt: "Existing prompt.", IsSkill: true, IsCommand: true, Command: "security-review"},
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccSkillResourceConfig(server.URL, "Security review", "Review changes.", "Prompt.", "security-review"),
			ExpectError: regexp.MustCompile("command already exists"),
		}},
	})
}

func TestAccSkillResourceRejectsNonSkillImport(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newSkillAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed(&v1.Prompt{
		Id:       skillTestID1,
		Metadata: &v1.PromptMetadata{OrganizationId: skillTestOrgID, Name: "Template", Description: "Not a skill."},
		Spec:     &v1.PromptSpec{Prompt: "Template prompt.", IsTemplate: true},
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:          testAccSkillResourceConfig(server.URL, "Template", "Not a skill.", "Template prompt.", ""),
			ResourceName:    "ona_skill.test",
			ImportState:     true,
			ImportStateId:   skillTestID1,
			ImportStateKind: resource.ImportCommandWithID,
			ExpectError:     regexp.MustCompile("Prompt Is Not an Ona Skill"),
		}},
	})
}

func TestAccSkillResourceReadNotFoundPlansCreate(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newSkillAPIServer(t)
	t.Cleanup(server.Close)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccSkillResourceConfig(server.URL, "Security review", "Review changes.", "Prompt.", "")},
			{
				PreConfig: server.service.deleteAll,
				Config:    testAccSkillResourceConfig(server.URL, "Security review", "Review changes.", "Prompt.", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_skill.test", plancheck.ResourceActionCreate),
				}},
			},
		},
	})
}

func TestAccSkillResourceRejectsOrganizationChange(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newSkillAPIServer(t)
	t.Cleanup(server.Close)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccSkillResourceConfig(server.URL, "Security review", "Review changes.", "Prompt.", "")},
			{
				PreConfig:   func() { server.service.setNextOrganization(skillTestOtherOrgID) },
				Config:      testAccSkillResourceConfig(server.URL, "Security review", "Review changes.", "Prompt.", ""),
				ExpectError: regexp.MustCompile("Ona Skill Organization Changed"),
			},
		},
	})
}

func testAccSkillResourceConfig(host, name, description, prompt, command string) string {
	commandLine := ""
	if command != "" {
		commandLine = fmt.Sprintf("  command = %q\n", command)
	}
	return fmt.Sprintf(`
provider "ona" {
  host  = %q
  token = "test-token"
}

resource "ona_skill" "test" {
  name        = %q
  description = %q
  prompt      = %q
%s}
`, host, name, description, prompt, commandLine)
}

type skillAPIServer struct {
	*httptest.Server
	service *fakeSkillService
}

func newSkillAPIServer(t *testing.T) *skillAPIServer {
	t.Helper()

	service := &fakeSkillService{organizationID: skillTestOrgID, prompts: map[string]*v1.Prompt{}}
	mux := http.NewServeMux()
	agentPath, agentHandler := v1connect.NewAgentServiceHandler(service)
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)
	mux.Handle(agentPath, agentHandler)
	mux.Handle(identityPath, identityHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	return &skillAPIServer{Server: server, service: service}
}

type fakeSkillService struct {
	v1connect.UnimplementedAgentServiceHandler
	v1connect.UnimplementedIdentityServiceHandler

	mu             sync.Mutex
	organizationID string
	nextOrgID      string
	prompts        map[string]*v1.Prompt
	nextID         int
	listRequests   []*v1.ListPromptsRequest
	listErr        error
	listPageSize   int32
	getCalls       int
}

func (s *fakeSkillService) GetAuthenticatedIdentity(context.Context, *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	organizationID := s.organizationID
	if s.nextOrgID != "" {
		organizationID = s.nextOrgID
		s.nextOrgID = ""
	}
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{OrganizationId: organizationID}), nil
}

func (s *fakeSkillService) CreatePrompt(_ context.Context, req *connect.Request[v1.CreatePromptRequest]) (*connect.Response[v1.CreatePromptResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Msg.GetIsCommand() {
		for _, existing := range s.prompts {
			if existing.GetSpec().GetCommand() == req.Msg.GetCommand() {
				return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("command already exists"))
			}
		}
	}
	ids := []string{skillTestID1, skillTestID2, "33333333-3333-4333-8333-333333333333"}
	id := ids[min(s.nextID, len(ids)-1)]
	s.nextID++
	prompt := &v1.Prompt{
		Id:       id,
		Metadata: &v1.PromptMetadata{OrganizationId: s.organizationID, Name: req.Msg.GetName(), Description: req.Msg.GetDescription()},
		Spec:     &v1.PromptSpec{Prompt: req.Msg.GetPrompt(), IsTemplate: req.Msg.GetIsTemplate(), IsCommand: req.Msg.GetIsCommand(), Command: req.Msg.GetCommand(), IsSkill: req.Msg.GetIsSkill()},
	}
	s.prompts[id] = prompt
	return connect.NewResponse(&v1.CreatePromptResponse{Prompt: proto.CloneOf(prompt)}), nil
}

func (s *fakeSkillService) GetPrompt(_ context.Context, req *connect.Request[v1.GetPromptRequest]) (*connect.Response[v1.GetPromptResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getCalls++
	prompt := s.prompts[req.Msg.GetPromptId()]
	if prompt == nil || prompt.GetMetadata().GetOrganizationId() != s.organizationID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("prompt not found"))
	}
	return connect.NewResponse(&v1.GetPromptResponse{Prompt: proto.CloneOf(prompt)}), nil
}

func (s *fakeSkillService) UpdatePrompt(_ context.Context, req *connect.Request[v1.UpdatePromptRequest]) (*connect.Response[v1.UpdatePromptResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prompt := s.prompts[req.Msg.GetPromptId()]
	if prompt == nil || prompt.GetMetadata().GetOrganizationId() != s.organizationID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("prompt not found"))
	}
	if metadata := req.Msg.GetMetadata(); metadata != nil {
		if metadata.Name != nil {
			prompt.Metadata.Name = metadata.GetName()
		}
		if metadata.Description != nil {
			prompt.Metadata.Description = metadata.GetDescription()
		}
	}
	if spec := req.Msg.GetSpec(); spec != nil {
		if spec.Prompt != nil {
			prompt.Spec.Prompt = spec.GetPrompt()
		}
		if spec.IsTemplate != nil {
			prompt.Spec.IsTemplate = spec.GetIsTemplate()
		}
		if spec.IsCommand != nil {
			prompt.Spec.IsCommand = spec.GetIsCommand()
		}
		if spec.Command != nil {
			prompt.Spec.Command = spec.GetCommand()
		}
		if spec.IsSkill != nil {
			prompt.Spec.IsSkill = spec.GetIsSkill()
		}
	}
	return connect.NewResponse(&v1.UpdatePromptResponse{Prompt: proto.CloneOf(prompt)}), nil
}

func (s *fakeSkillService) DeletePrompt(_ context.Context, req *connect.Request[v1.DeletePromptRequest]) (*connect.Response[v1.DeletePromptResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prompts[req.Msg.GetPromptId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("prompt not found"))
	}
	delete(s.prompts, req.Msg.GetPromptId())
	return connect.NewResponse(&v1.DeletePromptResponse{}), nil
}

func (s *fakeSkillService) deleteAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts = map[string]*v1.Prompt{}
}

func (s *fakeSkillService) seed(prompt *v1.Prompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts[prompt.GetId()] = proto.CloneOf(prompt)
	s.nextID++
}

func (s *fakeSkillService) setName(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prompt := s.prompts[id]; prompt != nil {
		prompt.Metadata.Name = name
	}
}

func (s *fakeSkillService) setNextOrganization(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextOrgID = id
}

func (s *fakeSkillService) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.prompts)
}

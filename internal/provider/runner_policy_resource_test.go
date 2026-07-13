// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
)

func TestAccRunnerPolicyResourceLifecycle(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newRunnerAPIServer(t, map[string]*v1.Runner{
		"runner-1": newTestRunner("runner-1", "Shared Runner"),
	})
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.runnerPolicyDeleted("runner-1/group-1") {
				return errors.New("runner policy was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerPolicyResourceConfig(server.URL, "group-1", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner_policy.test", "id", "runner-1/group-1"),
					resource.TestCheckResourceAttr("ona_runner_policy.test", "runner_id", "runner-1"),
					resource.TestCheckResourceAttr("ona_runner_policy.test", "group_id", "group-1"),
					resource.TestCheckResourceAttr("ona_runner_policy.test", "role", "user"),
				),
			},
			{
				Config: testAccRunnerPolicyResourceConfig(server.URL, "group-1", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_runner_policy.test",
				ImportState:       true,
				ImportStateId:     "runner-1/group-1",
				ImportStateVerify: true,
			},
			{
				Config: testAccRunnerPolicyResourceConfig(server.URL, "group-2", "user"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_runner_policy.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}

func TestAccRunnerPolicyResourceRejectsUnsupportedRole(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newRunnerAPIServer(t, map[string]*v1.Runner{
		"runner-1": newTestRunner("runner-1", "Shared Runner"),
	})
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccRunnerPolicyResourceConfig(server.URL, "group-1", "admin"),
				ExpectError: regexp.MustCompile("Unsupported Runner Policy Role"),
			},
		},
	})
}

func (s *fakeRunnerService) ListRunnerPolicies(ctx context.Context, req *connect.Request[v1.ListRunnerPoliciesRequest]) (*connect.Response[v1.ListRunnerPoliciesResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runners[req.Msg.GetRunnerId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("runner not found"))
	}

	keys := make([]string, 0, len(s.policies))
	for key := range s.policies {
		if strings.HasPrefix(key, req.Msg.GetRunnerId()+"/") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	start := 0
	if token := req.Msg.GetPagination().GetToken(); token != "" {
		_, err := fmt.Sscanf(token, "%d", &start)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid pagination token %q", token))
		}
	}

	pageSize := int(req.Msg.GetPagination().GetPageSize())
	if pageSize <= 0 {
		pageSize = 100
	}
	end := start + pageSize
	if end > len(keys) {
		end = len(keys)
	}

	policies := make([]*v1.RunnerPolicy, 0, end-start)
	for _, key := range keys[start:end] {
		policies = append(policies, cloneRunnerPolicy(s.policies[key]))
	}

	nextToken := ""
	if end < len(keys) {
		nextToken = fmt.Sprintf("%d", end)
	}
	return connect.NewResponse(&v1.ListRunnerPoliciesResponse{
		Pagination: &v1.PaginationResponse{NextToken: nextToken},
		Policies:   policies,
	}), nil
}

func (s *fakeRunnerService) CreateRunnerPolicy(ctx context.Context, req *connect.Request[v1.CreateRunnerPolicyRequest]) (*connect.Response[v1.CreateRunnerPolicyResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runners[req.Msg.GetRunnerId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("runner not found"))
	}
	if req.Msg.GetRole() != v1.RunnerRole_RUNNER_ROLE_USER {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("you can only share runners with User role"))
	}
	if s.policies == nil {
		s.policies = map[string]*v1.RunnerPolicy{}
	}
	key := runnerPolicyKey(req.Msg.GetRunnerId(), req.Msg.GetGroupId())
	policy := &v1.RunnerPolicy{
		GroupId: req.Msg.GetGroupId(),
		Role:    req.Msg.GetRole(),
	}
	s.policies[key] = policy
	return connect.NewResponse(&v1.CreateRunnerPolicyResponse{Policy: cloneRunnerPolicy(policy)}), nil
}

func (s *fakeRunnerService) DeleteRunnerPolicy(ctx context.Context, req *connect.Request[v1.DeleteRunnerPolicyRequest]) (*connect.Response[v1.DeleteRunnerPolicyResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := runnerPolicyKey(req.Msg.GetRunnerId(), req.Msg.GetGroupId())
	if s.policies[key] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("runner policy not found"))
	}
	delete(s.policies, key)
	s.policyDeletes = append(s.policyDeletes, key)
	return connect.NewResponse(&v1.DeleteRunnerPolicyResponse{}), nil
}

func (s *fakeRunnerService) runnerPolicyDeleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, deleted := range s.policyDeletes {
		if deleted == id {
			return true
		}
	}
	return false
}

func testAccRunnerPolicyResourceConfig(host string, groupID string, role string) string {
	roleLine := ""
	if role != "" {
		roleLine = fmt.Sprintf("  role      = %q\n", role)
	}
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner_policy" "test" {
  runner_id = "runner-1"
  group_id  = %[2]q
%[3]s}
`, host, groupID, roleLine)
}

func runnerPolicyKey(runnerID string, groupID string) string {
	return runnerID + "/" + groupID
}

func cloneRunnerPolicy(policy *v1.RunnerPolicy) *v1.RunnerPolicy {
	cloned, ok := proto.Clone(policy).(*v1.RunnerPolicy)
	if !ok {
		return nil
	}
	return cloned
}

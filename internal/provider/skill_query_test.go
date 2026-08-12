// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func (s *fakeSkillService) ListPrompts(_ context.Context, req *connect.Request[v1.ListPromptsRequest]) (*connect.Response[v1.ListPromptsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listRequests = append(s.listRequests, proto.CloneOf(req.Msg))
	if s.listErr != nil {
		return nil, s.listErr
	}

	var prompts []*v1.Prompt
	for _, prompt := range s.prompts {
		if skillPromptMatchesFilter(prompt, req.Msg.GetFilter()) {
			prompts = append(prompts, proto.CloneOf(prompt))
		}
	}
	sort.Slice(prompts, func(i, j int) bool { return prompts[i].GetId() < prompts[j].GetId() })
	start, end, nextToken, err := fakePage(req.Msg.GetPagination(), len(prompts), s.listPageSize)
	if err != nil {
		return nil, err
	}
	page := prompts[start:end]
	if req.Msg.GetFilter().GetExcludePromptContent() {
		for _, prompt := range page {
			if prompt.GetSpec() != nil {
				prompt.Spec.Prompt = ""
			}
		}
	}
	return connect.NewResponse(&v1.ListPromptsResponse{
		Pagination: &v1.PaginationResponse{NextToken: nextToken},
		Prompts:    page,
	}), nil
}

func skillPromptMatchesFilter(prompt *v1.Prompt, filter *v1.ListPromptsRequest_Filter) bool {
	if filter == nil {
		return true
	}
	spec := prompt.GetSpec()
	if filter.GetIsSkill() && !spec.GetIsSkill() || filter.GetIsCommand() && !spec.GetIsCommand() {
		return false
	}
	if filter.GetCommand() != "" && spec.GetCommand() != filter.GetCommand() {
		return false
	}
	if filter.GetCommandPrefix() != "" && !strings.HasPrefix(spec.GetCommand(), filter.GetCommandPrefix()) {
		return false
	}
	if filter.GetSearch() != "" {
		haystack := strings.ToLower(strings.Join([]string{prompt.GetMetadata().GetName(), prompt.GetMetadata().GetDescription(), spec.GetCommand()}, " "))
		if !strings.Contains(haystack, strings.ToLower(filter.GetSearch())) {
			return false
		}
	}
	return true
}

func TestAccSkillQueryIncludesManagedResource(t *testing.T) {
	t.Parallel()

	server := newSkillAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed(testSkillPrompt(skillTestID1, "security-review"))
	server.service.seed(&v1.Prompt{
		Id:       skillTestID2,
		Metadata: &v1.PromptMetadata{OrganizationId: skillTestOrgID, Name: "Documentation", Description: "Write docs."},
		Spec:     &v1.PromptSpec{Prompt: "Write clear docs.", IsSkill: true},
	})

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: skillFullQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_skill.security", 1),
			querycheck.ExpectIdentity("ona_skill.security", map[string]knownvalue.Check{"skill_id": knownvalue.StringExact(skillTestID1)}),
			querycheck.ExpectResourceDisplayName("ona_skill.security", queryfilter.ByResourceIdentity(map[string]knownvalue.Check{"skill_id": knownvalue.StringExact(skillTestID1)}), knownvalue.StringExact("security_review_11111111_1111_4111_8111_111111111111")),
			querycheck.ExpectResourceKnownValues("ona_skill.security", queryfilter.ByResourceIdentity(map[string]knownvalue.Check{"skill_id": knownvalue.StringExact(skillTestID1)}), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(skillTestID1)},
				{Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("Security review")},
				{Path: tfjsonpath.New("description"), KnownValue: knownvalue.StringExact("Review security-sensitive changes.")},
				{Path: tfjsonpath.New("prompt"), KnownValue: knownvalue.StringExact("Follow the security checklist.")},
				{Path: tfjsonpath.New("command"), KnownValue: knownvalue.StringExact("security-review")},
			}),
		},
	}))

	requests, getCalls := server.service.listSnapshot()
	want := []*v1.ListPromptsRequest{{
		Pagination: &v1.PaginationRequest{PageSize: 100},
		Filter:     &v1.ListPromptsRequest_Filter{IsSkill: true, Search: "security", ExcludePromptContent: false},
	}}
	if diff := cmp.Diff(want, requests, protocmp.Transform()); diff != "" {
		t.Errorf("ListPrompts requests mismatch (-want +got):\n%s", diff)
	}
	if getCalls != 0 {
		t.Errorf("Query made %d GetPrompt calls, want 0", getCalls)
	}
}

func TestAccSkillQueryIdentityOnlyPaginatesPastHybrid(t *testing.T) {
	t.Parallel()

	server := newSkillAPIServer(t)
	t.Cleanup(server.Close)
	server.service.listPageSize = 1
	hybrid := testSkillPrompt(skillTestID1, "")
	hybrid.Spec.IsTemplate = true
	server.service.seed(hybrid)
	server.service.seed(testSkillPrompt(skillTestID2, ""))
	server.service.seed(testSkillPrompt(skillTestID3, ""))

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: skillIdentityQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_skill.all", 1),
			querycheck.ExpectIdentity("ona_skill.all", map[string]knownvalue.Check{"skill_id": knownvalue.StringExact(skillTestID2)}),
		},
	}))

	requests, getCalls := server.service.listSnapshot()
	if len(requests) != 2 {
		t.Fatalf("ListPrompts call count = %d, want 2", len(requests))
	}
	for _, request := range requests {
		if !request.GetFilter().GetExcludePromptContent() {
			t.Errorf("identity-only request did not exclude prompt content: %v", request)
		}
	}
	if getCalls != 0 {
		t.Errorf("identity-only Query made %d GetPrompt calls, want 0", getCalls)
	}
}

func TestAccSkillQueryCommandFilter(t *testing.T) {
	t.Parallel()

	server := newSkillAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed(testSkillPrompt(skillTestID1, "security-review"))
	server.service.seed(testSkillPrompt(skillTestID2, "docs-review"))

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: skillCommandQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_skill.command", 1),
			querycheck.ExpectIdentity("ona_skill.command", map[string]knownvalue.Check{"skill_id": knownvalue.StringExact(skillTestID1)}),
		},
	}))

	requests, _ := server.service.listSnapshot()
	if len(requests) != 1 || !requests[0].GetFilter().GetIsCommand() || requests[0].GetFilter().GetCommandPrefix() != "security" {
		t.Errorf("command prefix filter was not mapped correctly: %v", requests)
	}
}

func TestAccSkillQueryReportsListError(t *testing.T) {
	t.Parallel()

	server := newSkillAPIServer(t)
	t.Cleanup(server.Close)
	server.service.listErr = connect.NewError(connect.CodePermissionDenied, errors.New("prompt read denied"))

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      skillIdentityQueryConfig(),
		ExpectError: regexp.MustCompile("Unable to List Ona Skills"),
	}))
}

func (s *fakeSkillService) listSnapshot() ([]*v1.ListPromptsRequest, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	requests := make([]*v1.ListPromptsRequest, 0, len(s.listRequests))
	for _, request := range s.listRequests {
		requests = append(requests, proto.CloneOf(request))
	}
	return requests, s.getCalls
}

func skillFullQueryConfig() string {
	return `
list "ona_skill" "security" {
  provider         = ona
  include_resource = true

  config {
    search = "security"
  }
}
`
}

func skillIdentityQueryConfig() string {
	return `
list "ona_skill" "all" {
  provider = ona
  limit    = 1

  config {}
}
`
}

func skillCommandQueryConfig() string {
	return `
list "ona_skill" "command" {
  provider = ona

  config {
    command_prefix = "security"
  }
}
`
}

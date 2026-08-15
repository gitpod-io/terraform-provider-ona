// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"regexp"
	"sort"
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

func (s *fakeWebhookService) ListWebhooks(ctx context.Context, req *connect.Request[v1.ListWebhooksRequest]) (*connect.Response[v1.ListWebhooksResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listRequests = append(s.listRequests, proto.CloneOf(req.Msg))
	if s.listErr != nil {
		return nil, s.listErr
	}

	webhooks := make([]*v1.Webhook, 0, len(s.webhooks))
	for _, webhook := range s.webhooks {
		webhooks = append(webhooks, proto.CloneOf(webhook))
	}
	sort.Slice(webhooks, func(i, j int) bool {
		return webhooks[i].GetId() < webhooks[j].GetId()
	})
	start, end, nextToken, err := fakePage(req.Msg.GetPagination(), len(webhooks), s.listPageLimit)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ListWebhooksResponse{
		Pagination: &v1.PaginationResponse{NextToken: nextToken},
		Webhooks:   webhooks[start:end],
	}), nil
}

func (s *fakeWebhookService) listRequestsSnapshot() []*v1.ListWebhooksRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	requests := make([]*v1.ListWebhooksRequest, 0, len(s.listRequests))
	for _, request := range s.listRequests {
		requests = append(requests, proto.CloneOf(request))
	}
	return requests
}

func TestAccWebhookQuery(t *testing.T) {
	t.Parallel()

	server := newWebhookAPIServer(t)
	t.Cleanup(server.Close)
	server.service.listPageLimit = 1
	server.service.seed(newTestWebhook(webhookID1, "Deployments", v1.WebhookType_WEBHOOK_TYPE_SCM_REPOSITORY, v1.WebhookProvider_WEBHOOK_PROVIDER_GITHUB), "repository-secret")
	organizationWebhook := newTestWebhook(webhookID2, "Deployments", v1.WebhookType_WEBHOOK_TYPE_SCM_ORGANIZATION, v1.WebhookProvider_WEBHOOK_PROVIDER_GITLAB)
	organizationWebhook.Spec.Scopes = nil
	organizationWebhook.Spec.OrganizationScope = &v1.WebhookOrganizationScope{Host: "gitlab.com", Name: "gitpod-io"}
	server.service.seed(organizationWebhook, "organization-secret")

	repositoryIdentity := webhookQueryIdentity(webhookID1)
	organizationIdentity := webhookQueryIdentity(webhookID2)
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: webhookQueryConfig(true, 0),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_webhook.all", 2),
			querycheck.ExpectIdentity("ona_webhook.all", repositoryIdentity),
			querycheck.ExpectIdentity("ona_webhook.all", organizationIdentity),
			querycheck.ExpectResourceDisplayName("ona_webhook.all", queryfilter.ByResourceIdentity(repositoryIdentity), knownvalue.StringExact("deployments")),
			querycheck.ExpectResourceDisplayName("ona_webhook.all", queryfilter.ByResourceIdentity(organizationIdentity), knownvalue.StringExact("deployments_2")),
			querycheck.ExpectResourceKnownValues("ona_webhook.all", queryfilter.ByResourceIdentity(repositoryIdentity), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(webhookID1)},
				{Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("Deployments")},
				{Path: tfjsonpath.New("type"), KnownValue: knownvalue.StringExact("repository")},
				{Path: tfjsonpath.New("scm_provider"), KnownValue: knownvalue.StringExact("github")},
				{Path: tfjsonpath.New("repository_scopes"), KnownValue: knownvalue.SetExact([]knownvalue.Check{knownvalue.ObjectExact(map[string]knownvalue.Check{
					"host": knownvalue.StringExact("github.com"), "owner": knownvalue.StringExact("gitpod-io"), "name": knownvalue.StringExact("terraform-provider-ona"),
				})})},
				{Path: tfjsonpath.New("organization_scope"), KnownValue: knownvalue.Null()},
				{Path: tfjsonpath.New("secret_version"), KnownValue: knownvalue.Null()},
			}),
			querycheck.ExpectResourceKnownValues("ona_webhook.all", queryfilter.ByResourceIdentity(organizationIdentity), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("type"), KnownValue: knownvalue.StringExact("organization")},
				{Path: tfjsonpath.New("scm_provider"), KnownValue: knownvalue.StringExact("gitlab")},
				{Path: tfjsonpath.New("repository_scopes"), KnownValue: knownvalue.Null()},
				{Path: tfjsonpath.New("organization_scope"), KnownValue: knownvalue.ObjectExact(map[string]knownvalue.Check{
					"host": knownvalue.StringExact("gitlab.com"), "name": knownvalue.StringExact("gitpod-io"),
				})},
				{Path: tfjsonpath.New("secret_version"), KnownValue: knownvalue.Null()},
			}),
		},
	}))

	type Expectation struct {
		Requests    []*v1.ListWebhooksRequest
		SecretReads map[string]int
	}
	expected := Expectation{
		Requests: []*v1.ListWebhooksRequest{
			{Pagination: &v1.PaginationRequest{PageSize: 100}},
			{Pagination: &v1.PaginationRequest{PageSize: 99, Token: "1"}},
		},
		SecretReads: map[string]int{webhookID1: 0, webhookID2: 0},
	}
	got := Expectation{
		Requests: server.service.listRequestsSnapshot(),
		SecretReads: map[string]int{
			webhookID1: server.service.secretReadCount(webhookID1),
			webhookID2: server.service.secretReadCount(webhookID2),
		},
	}
	if diff := cmp.Diff(expected, got, protocmp.Transform()); diff != "" {
		t.Errorf("webhook query mismatch (-want +got):\n%s", diff)
	}
}

func TestAccWebhookQueryLimit(t *testing.T) {
	t.Parallel()

	server := newWebhookAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed(newTestWebhook(webhookID1, "First", v1.WebhookType_WEBHOOK_TYPE_SCM_REPOSITORY, v1.WebhookProvider_WEBHOOK_PROVIDER_GITHUB), "first-secret")
	server.service.seed(newTestWebhook(webhookID2, "Second", v1.WebhookType_WEBHOOK_TYPE_SCM_REPOSITORY, v1.WebhookProvider_WEBHOOK_PROVIDER_GITHUB), "second-secret")

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: webhookQueryConfig(false, 1),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_webhook.all", 1),
			querycheck.ExpectIdentity("ona_webhook.all", webhookQueryIdentity(webhookID1)),
		},
	}))

	expected := []*v1.ListWebhooksRequest{{Pagination: &v1.PaginationRequest{PageSize: 1}}}
	if diff := cmp.Diff(expected, server.service.listRequestsSnapshot(), protocmp.Transform()); diff != "" {
		t.Errorf("ListWebhooks requests mismatch (-want +got):\n%s", diff)
	}
}

func TestAccWebhookQueryReportsListError(t *testing.T) {
	t.Parallel()

	server := newWebhookAPIServer(t)
	t.Cleanup(server.Close)
	server.service.listErr = connect.NewError(connect.CodePermissionDenied, errors.New("webhook read denied"))

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      webhookQueryConfig(false, 0),
		ExpectError: regexp.MustCompile("Unable to List Ona Webhooks"),
	}))
}

func webhookQueryIdentity(id string) map[string]knownvalue.Check {
	return map[string]knownvalue.Check{"id": knownvalue.StringExact(id)}
}

func webhookQueryConfig(includeResource bool, limit int) string {
	includeResourceLine := ""
	if includeResource {
		includeResourceLine = "  include_resource = true\n"
	}
	limitLine := ""
	if limit > 0 {
		limitLine = "  limit            = 1\n"
	}
	return `
list "ona_webhook" "all" {
  provider = ona
` + includeResourceLine + limitLine + `}
`
}

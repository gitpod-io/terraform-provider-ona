// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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
)

func TestAccRunnerLLMIntegrationQuery(t *testing.T) {
	t.Parallel()

	server := newRunnerLLMIntegrationQueryAPIServer(t)
	t.Cleanup(server.Close)
	server.service.llmListPageSize = 1

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: runnerLLMIntegrationQueryConfig(true, 0, nil),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_runner_llm_integration.all", 2),
			querycheck.ExpectIdentity("ona_runner_llm_integration.all", runnerLLMIntegrationQueryIdentity("llm-1")),
			querycheck.ExpectIdentity("ona_runner_llm_integration.all", runnerLLMIntegrationQueryIdentity("llm-2")),
			querycheck.ExpectResourceDisplayName("ona_runner_llm_integration.all", queryfilter.ByResourceIdentity(runnerLLMIntegrationQueryIdentity("llm-1")), knownvalue.StringExact("anthropic")),
			querycheck.ExpectResourceDisplayName("ona_runner_llm_integration.all", queryfilter.ByResourceIdentity(runnerLLMIntegrationQueryIdentity("llm-2")), knownvalue.StringExact("openai")),
			querycheck.ExpectResourceKnownValues("ona_runner_llm_integration.all", queryfilter.ByResourceIdentity(runnerLLMIntegrationQueryIdentity("llm-1")), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("llm-1")},
				{Path: tfjsonpath.New("runner_id"), KnownValue: knownvalue.StringExact("runner-1")},
				{Path: tfjsonpath.New("models"), KnownValue: knownvalue.SetExact([]knownvalue.Check{knownvalue.StringExact("sonnet_3_7")})},
				{Path: tfjsonpath.New("endpoint"), KnownValue: knownvalue.StringExact("https://api.anthropic.com/v1")},
				{Path: tfjsonpath.New("api_key"), KnownValue: knownvalue.Null()},
				{Path: tfjsonpath.New("api_key_version"), KnownValue: knownvalue.Null()},
				{Path: tfjsonpath.New("max_tokens"), KnownValue: knownvalue.Int64Exact(4000)},
				{Path: tfjsonpath.New("enabled"), KnownValue: knownvalue.Bool(true)},
				{Path: tfjsonpath.New("llm_provider"), KnownValue: knownvalue.StringExact("anthropic")},
			}),
			runnerLLMIntegrationQueryOmitsSecrets{},
		},
	}))

	calls, pageSizes := server.service.llmListStats()
	got := runnerLLMIntegrationListExpectation{Calls: calls, PageSizes: pageSizes}
	expected := runnerLLMIntegrationListExpectation{Calls: 2, PageSizes: []int32{100, 99}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ListLLMIntegrations requests mismatch (-want +got):\n%s", diff)
	}
}

func TestAccRunnerLLMIntegrationQueryRunnerFilter(t *testing.T) {
	t.Parallel()

	server := newRunnerLLMIntegrationQueryAPIServer(t)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: runnerLLMIntegrationQueryConfig(false, 0, []string{"runner-2"}),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_runner_llm_integration.all", 1),
			querycheck.ExpectIdentity("ona_runner_llm_integration.all", runnerLLMIntegrationQueryIdentity("llm-2")),
		},
	}))
}

func TestAccRunnerLLMIntegrationQueryLimit(t *testing.T) {
	t.Parallel()

	server := newRunnerLLMIntegrationQueryAPIServer(t)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: runnerLLMIntegrationQueryConfig(false, 1, nil),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_runner_llm_integration.all", 1),
			querycheck.ExpectIdentity("ona_runner_llm_integration.all", runnerLLMIntegrationQueryIdentity("llm-1")),
		},
	}))

	calls, pageSizes := server.service.llmListStats()
	got := runnerLLMIntegrationListExpectation{Calls: calls, PageSizes: pageSizes}
	expected := runnerLLMIntegrationListExpectation{Calls: 1, PageSizes: []int32{1}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ListLLMIntegrations requests mismatch (-want +got):\n%s", diff)
	}
}

func TestAccRunnerLLMIntegrationQueryReportsListError(t *testing.T) {
	t.Parallel()

	server := newRunnerLLMIntegrationQueryAPIServer(t)
	t.Cleanup(server.Close)
	server.service.llmListErr = connect.NewError(connect.CodePermissionDenied, errors.New("LLM integration read denied"))

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      runnerLLMIntegrationQueryConfig(false, 0, nil),
		ExpectError: regexp.MustCompile("Unable to List Ona Runner LLM Integrations"),
	}))
}

func TestAccRunnerLLMIntegrationQueryRejectsMissingID(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.llmIntegrations[""] = &v1.LLMIntegration{RunnerId: "runner-1", Provider: v1.LLMProvider_LLM_PROVIDER_ANTHROPIC}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      runnerLLMIntegrationQueryConfig(false, 0, nil),
		ExpectError: regexp.MustCompile("runner LLM integration without an ID"),
	}))
}

func newRunnerLLMIntegrationQueryAPIServer(t *testing.T) *runnerConfigurationAPIServer {
	t.Helper()

	server := newRunnerConfigurationAPIServer(t)
	server.service.llmIntegrations["llm-1"] = &v1.LLMIntegration{
		Id:              "llm-1",
		RunnerId:        "runner-1",
		Models:          []v1.SupportedModel{v1.SupportedModel_SUPPORTED_MODEL_SONNET_3_7},
		Endpoint:        "https://api.anthropic.com/v1",
		EncryptedApiKey: []byte("must-not-appear"),
		MaxTokens:       4000,
		Phase:           v1.LLMIntegrationPhase_LLM_INTEGRATION_PHASE_AVAILABLE,
		Provider:        v1.LLMProvider_LLM_PROVIDER_ANTHROPIC,
	}
	server.service.llmIntegrations["llm-2"] = &v1.LLMIntegration{
		Id:              "llm-2",
		RunnerId:        "runner-2",
		Models:          []v1.SupportedModel{v1.SupportedModel_SUPPORTED_MODEL_OPENAI_4O},
		Endpoint:        "https://api.openai.com/v1",
		EncryptedApiKey: []byte("must-not-appear-either"),
		Phase:           v1.LLMIntegrationPhase_LLM_INTEGRATION_PHASE_DISABLED,
		Provider:        v1.LLMProvider_LLM_PROVIDER_OPENAI,
	}
	return server
}

func runnerLLMIntegrationQueryIdentity(id string) map[string]knownvalue.Check {
	return map[string]knownvalue.Check{"id": knownvalue.StringExact(id)}
}

func runnerLLMIntegrationQueryConfig(includeResource bool, limit int, runnerIDs []string) string {
	includeResourceLine := ""
	if includeResource {
		includeResourceLine = "  include_resource = true\n"
	}
	limitLine := ""
	if limit > 0 {
		limitLine = fmt.Sprintf("  limit            = %d\n", limit)
	}
	configBlock := ""
	if len(runnerIDs) > 0 {
		configBlock = fmt.Sprintf("\n  config {\n    runner_ids = %s\n  }\n", hclStringList(runnerIDs))
	}
	return `
list "ona_runner_llm_integration" "all" {
  provider = ona
` + includeResourceLine + limitLine + configBlock + `}
`
}

type runnerLLMIntegrationListExpectation struct {
	Calls     int
	PageSizes []int32
}

type runnerLLMIntegrationQueryOmitsSecrets struct{}

func (runnerLLMIntegrationQueryOmitsSecrets) CheckQuery(_ context.Context, req querycheck.CheckQueryRequest, resp *querycheck.CheckQueryResponse) {
	for _, result := range req.Query {
		if result.ResourceObject["api_key"] != nil || result.ResourceObject["api_key_version"] != nil {
			resp.Error = fmt.Errorf("runner LLM integration %q returned API key data in the resource object", result.DisplayName)
			return
		}
		for _, field := range []string{"api_key", "api_key_version"} {
			if strings.Contains(result.Config, field) {
				resp.Error = fmt.Errorf("runner LLM integration %q generated configuration containing %q", result.DisplayName, field)
				return
			}
		}
	}
}

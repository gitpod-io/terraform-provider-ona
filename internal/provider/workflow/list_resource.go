// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &Resource{}

func NewListResource() list.ListResource { return &Resource{} }

func (r *Resource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists importable persistent Ona automations. System-managed, deleting, report-based, and unsupported legacy automations are excluded.",
	}
}

func (r *Resource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_automation resources")))
			return
		}

		var token string
		seenTokens := make(map[string]struct{})
		var emitted int64
		displayNames := listutil.NewDisplayNames()
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.WorkflowService().ListWorkflows(ctx, connect.NewRequest(&v1.ListWorkflowsRequest{
				Pagination: &v1.PaginationRequest{
					PageSize: listutil.PageSize(req.Limit, emitted),
					Token:    token,
				},
				Filter: &v1.ListWorkflowsRequest_Filter{},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Automations", fmt.Errorf("list automations: %w", err)))
				return
			}
			if result == nil || result.Msg == nil {
				push(listutil.Error("Unable to List Ona Automations", fmt.Errorf("the Ona API returned an empty response")))
				return
			}

			for _, workflow := range result.Msg.GetWorkflows() {
				if !importableWorkflow(workflow) {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				if workflow.GetId() == "" {
					push(listutil.Error("Unable to List Ona Automations", fmt.Errorf("the Ona API returned an automation without an ID")))
					return
				}

				item := req.NewListResult(ctx)
				item.DisplayName = displayNames.Unique(workflow.GetMetadata().GetName(), workflow.GetId(), "automation")
				item.Diagnostics.Append(item.Identity.Set(ctx, IdentityModel{AutomationID: types.StringValue(workflow.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model Model
					populateModel(ctx, &model, workflow, &item.Diagnostics)
					if !item.Diagnostics.HasError() {
						item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
					}
				}
				if !push(item) || item.Diagnostics.HasError() {
					return
				}
				emitted++
			}

			nextToken := result.Msg.GetPagination().GetNextToken()
			if err := listutil.NextPageToken(seenTokens, nextToken); err != nil {
				push(listutil.Error("Unable to List Ona Automations", err))
				return
			}
			if nextToken == "" {
				return
			}
			token = nextToken
		}
	}
}

func importableWorkflow(workflow *v1.Workflow) bool {
	return workflow != nil && !workflow.GetSpec().GetDeleting() && unsupportedWorkflowReason(workflow) == ""
}

// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"context"
	"fmt"
	"sort"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ datasource.DataSource = &CollectionDataSource{}
var _ datasource.DataSourceWithConfigure = &CollectionDataSource{}

func NewCollectionDataSource() datasource.DataSource { return &CollectionDataSource{} }

type CollectionDataSource struct {
	client *managementclient.ManagementPlane
}

func (d *CollectionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_automations"
}

func (d *CollectionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = collectionDataSourceSchema()
}

func (d *CollectionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providerdata.Data)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *providerdata.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	d.client = data.Client
}

func (d *CollectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CollectionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if d.client == nil {
		resp.Diagnostics.AddError("Ona API Client Is Not Configured", "Set the provider token argument or ONA_TOKEN before reading ona_automations data sources.")
		return
	}
	filter := collectionFilter(data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	workflows, err := d.listWorkflows(ctx, filter)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to List Ona Automations", "listing Ona automations", err)
		return
	}
	sort.SliceStable(workflows, func(i, j int) bool { return workflows[i].GetId() < workflows[j].GetId() })
	data.ID = types.StringValue("automations")
	data.Automations = make([]SummaryModel, 0, len(workflows))
	for _, workflow := range workflows {
		data.Automations = append(data.Automations, summaryFromWorkflow(workflow, &resp.Diagnostics))
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *CollectionDataSource) listWorkflows(ctx context.Context, filter *v1.ListWorkflowsRequest_Filter) ([]*v1.Workflow, error) {
	var workflows []*v1.Workflow
	var token string
	for {
		result, err := d.client.WorkflowService().ListWorkflows(ctx, connect.NewRequest(&v1.ListWorkflowsRequest{
			Pagination: &v1.PaginationRequest{PageSize: 100, Token: token},
			Filter:     filter,
		}))
		if err != nil {
			return nil, fmt.Errorf("list workflows: %w", err)
		}
		workflows = append(workflows, result.Msg.GetWorkflows()...)
		token = result.Msg.GetPagination().GetNextToken()
		if token == "" {
			return workflows, nil
		}
	}
}

func collectionFilter(data CollectionModel, diags *diag.Diagnostics) *v1.ListWorkflowsRequest_Filter {
	automationIDs := collectionStringSet(data.AutomationIDs, path.Root("automation_ids"), 25, true, diags)
	creatorIDs := collectionStringSet(data.CreatorIDs, path.Root("creator_ids"), 25, true, diags)
	statusNames := collectionStringSet(data.StatusPhases, path.Root("status_phases"), 10, false, diags)
	filter := &v1.ListWorkflowsRequest_Filter{WorkflowIds: automationIDs, Search: data.Search.ValueString(), CreatorIds: creatorIDs}
	if !data.Search.IsNull() && !data.Search.IsUnknown() && len([]rune(data.Search.ValueString())) > 256 {
		diags.AddAttributeError(path.Root("search"), "Automation Search Is Too Long", "search must not exceed 256 characters.")
	}
	if data.Search.IsUnknown() {
		diags.AddAttributeError(path.Root("search"), "Unknown Automation Search", "search must be known before reading the data source.")
	}
	for _, name := range statusNames {
		phase, ok := workflowExecutionPhaseFromString(name)
		if !ok {
			diags.AddAttributeError(path.Root("status_phases"), "Invalid Automation Execution Phase", fmt.Sprintf("Unsupported phase %q.", name))
			continue
		}
		filter.StatusPhases = append(filter.StatusPhases, phase)
	}
	if !data.HasFailedExecutionSince.IsNull() && !data.HasFailedExecutionSince.IsUnknown() {
		parsed, err := time.Parse(time.RFC3339, data.HasFailedExecutionSince.ValueString())
		if err != nil {
			diags.AddAttributeError(path.Root("has_failed_execution_since"), "Invalid Failed-Execution Timestamp", err.Error())
		} else {
			timestamp := timestamppb.New(parsed)
			if err := timestamp.CheckValid(); err != nil {
				diags.AddAttributeError(path.Root("has_failed_execution_since"), "Invalid Failed-Execution Timestamp", err.Error())
			} else {
				filter.HasFailedExecutionSince = timestamp
			}
		}
	} else if data.HasFailedExecutionSince.IsUnknown() {
		diags.AddAttributeError(path.Root("has_failed_execution_since"), "Unknown Failed-Execution Timestamp", "has_failed_execution_since must be known before reading the data source.")
	}
	if len(filter.StatusPhases) > 0 && filter.HasFailedExecutionSince != nil {
		diags.AddAttributeError(path.Root("status_phases"), "Incompatible Automation Filters", "status_phases and has_failed_execution_since are mutually exclusive.")
	}
	if !data.Disabled.IsNull() && !data.Disabled.IsUnknown() {
		value := data.Disabled.ValueBool()
		filter.Disabled = &value
	} else if data.Disabled.IsUnknown() {
		diags.AddAttributeError(path.Root("disabled"), "Unknown Disabled Filter", "disabled must be known before reading the data source.")
	}
	return filter
}

func collectionStringSet(value types.Set, p path.Path, maxItems int, validateUUID bool, diags *diag.Diagnostics) []string {
	if value.IsNull() {
		return nil
	}
	if value.IsUnknown() {
		diags.AddAttributeError(p, "Unknown Collection Filter", "This filter must be known before reading the data source.")
		return nil
	}
	values := make([]string, 0, len(value.Elements()))
	for _, element := range value.Elements() {
		stringValue, ok := element.(types.String)
		if !ok {
			diags.AddAttributeError(p, "Invalid Collection Filter", "This filter must contain only string values.")
			continue
		}
		if stringValue.IsNull() || stringValue.IsUnknown() {
			diags.AddAttributeError(p, "Unknown Collection Filter", "Every filter value must be known before reading the data source.")
			continue
		}
		values = append(values, stringValue.ValueString())
	}
	sort.Strings(values)
	if len(values) > maxItems {
		diags.AddAttributeError(p, "Too Many Filter Values", fmt.Sprintf("The Ona API accepts at most %d values.", maxItems))
	}
	if validateUUID {
		for _, value := range values {
			if _, err := uuid.Parse(value); err != nil {
				diags.AddAttributeError(p, "Invalid Filter UUID", fmt.Sprintf("Value %q is not a valid UUID: %v", value, err))
			}
		}
	}
	return values
}

func workflowExecutionPhaseFromString(value string) (v1.WorkflowExecutionPhase, bool) {
	switch value {
	case "pending":
		return v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_PENDING, true
	case "running":
		return v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_RUNNING, true
	case "stopping":
		return v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_STOPPING, true
	case "stopped":
		return v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_STOPPED, true
	case "deleting":
		return v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_DELETING, true
	case "deleted":
		return v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_DELETED, true
	case "completed":
		return v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_COMPLETED, true
	default:
		return v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_UNSPECIFIED, false
	}
}

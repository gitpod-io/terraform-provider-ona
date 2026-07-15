// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package warmpool

import (
	"context"
	"fmt"
	"sort"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultWarmPoolMinSize int32 = 1
	defaultWarmPoolMaxSize int32 = 2
	maxWarmPoolSize        int32 = 20
)

type WarmPoolModel struct {
	ID                 types.String `tfsdk:"id"`
	ProjectID          types.String `tfsdk:"project_id"`
	EnvironmentClassID types.String `tfsdk:"environment_class_id"`
	MinSize            types.Int32  `tfsdk:"min_size"`
	MaxSize            types.Int32  `tfsdk:"max_size"`
	CreatedAt          types.String `tfsdk:"created_at"`
}

type WarmPoolDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	WarmPoolID         types.String `tfsdk:"warm_pool_id"`
	ProjectID          types.String `tfsdk:"project_id"`
	EnvironmentClassID types.String `tfsdk:"environment_class_id"`
	MinSize            types.Int32  `tfsdk:"min_size"`
	MaxSize            types.Int32  `tfsdk:"max_size"`
	CreatedAt          types.String `tfsdk:"created_at"`
}

type WarmPoolCollectionModel struct {
	ID                  types.String              `tfsdk:"id"`
	ProjectIDs          types.Set                 `tfsdk:"project_ids"`
	EnvironmentClassIDs types.Set                 `tfsdk:"environment_class_ids"`
	PageSize            types.Int32               `tfsdk:"page_size"`
	WarmPools           []WarmPoolDataSourceModel `tfsdk:"warm_pools"`
}

func createWarmPoolRequest(data WarmPoolModel) (*v1.CreateWarmPoolRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateWarmPoolConfig(data, &diags)
	validateKnownWarmPoolSizes(data.MinSize, data.MaxSize, &diags)
	if diags.HasError() {
		return nil, diags
	}
	minSize := data.MinSize.ValueInt32()
	maxSize := data.MaxSize.ValueInt32()
	return &v1.CreateWarmPoolRequest{
		ProjectId:          data.ProjectID.ValueString(),
		EnvironmentClassId: data.EnvironmentClassID.ValueString(),
		MinSize:            &minSize,
		MaxSize:            &maxSize,
	}, diags
}

func updateWarmPoolRequest(data WarmPoolModel) (*v1.UpdateWarmPoolRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateWarmPoolConfig(data, &diags)
	validateKnownWarmPoolSizes(data.MinSize, data.MaxSize, &diags)
	if diags.HasError() {
		return nil, diags
	}
	minSize := data.MinSize.ValueInt32()
	maxSize := data.MaxSize.ValueInt32()
	return &v1.UpdateWarmPoolRequest{
		WarmPoolId: data.ID.ValueString(),
		MinSize:    &minSize,
		MaxSize:    &maxSize,
	}, diags
}

func validateWarmPoolConfig(data WarmPoolModel, diags *diag.Diagnostics) {
	validateRequiredString(data.ProjectID, path.Root("project_id"), "Project ID", diags)
	validateRequiredString(data.EnvironmentClassID, path.Root("environment_class_id"), "Environment Class ID", diags)
	validateWarmPoolSizes(data.MinSize, data.MaxSize, diags)
}

func validateWarmPoolSizes(minSize types.Int32, maxSize types.Int32, diags *diag.Diagnostics) {
	if minSize.IsUnknown() || minSize.IsNull() || maxSize.IsUnknown() || maxSize.IsNull() {
		return
	}
	minValue := minSize.ValueInt32()
	maxValue := maxSize.ValueInt32()
	if minValue < 0 || minValue > maxWarmPoolSize {
		diags.AddAttributeError(path.Root("min_size"), "Invalid Warm Pool Minimum Size", "min_size must be between 0 and 20.")
	}
	if maxValue < 1 || maxValue > maxWarmPoolSize {
		diags.AddAttributeError(path.Root("max_size"), "Invalid Warm Pool Maximum Size", "max_size must be between 1 and 20.")
	}
	if minValue > maxValue {
		diags.AddAttributeError(path.Root("min_size"), "Invalid Warm Pool Size Range", "min_size must be less than or equal to max_size.")
	}
}

func validateKnownWarmPoolSizes(minSize types.Int32, maxSize types.Int32, diags *diag.Diagnostics) {
	if minSize.IsUnknown() || minSize.IsNull() {
		diags.AddAttributeError(path.Root("min_size"), "Missing Warm Pool Minimum Size", "min_size must be known before apply.")
	}
	if maxSize.IsUnknown() || maxSize.IsNull() {
		diags.AddAttributeError(path.Root("max_size"), "Missing Warm Pool Maximum Size", "max_size must be known before apply.")
	}
}

func populateWarmPoolModel(data *WarmPoolModel, warmPool *v1.WarmPool) {
	metadata := warmPool.GetMetadata()
	spec := warmPool.GetSpec()

	data.ID = stringOptionalValue(warmPool.GetId())
	data.ProjectID = stringOptionalValue(metadata.GetProjectId())
	data.EnvironmentClassID = stringOptionalValue(metadata.GetEnvironmentClassId())
	data.MinSize = types.Int32Value(spec.GetMinSize())
	data.MaxSize = types.Int32Value(spec.GetMaxSize())
	data.CreatedAt = timestampValue(metadata.GetCreatedAt())
}

func populateWarmPoolDataSourceModel(data *WarmPoolDataSourceModel, warmPool *v1.WarmPool) {
	var model WarmPoolModel
	populateWarmPoolModel(&model, warmPool)

	data.ID = model.ID
	data.WarmPoolID = model.ID
	data.ProjectID = model.ProjectID
	data.EnvironmentClassID = model.EnvironmentClassID
	data.MinSize = model.MinSize
	data.MaxSize = model.MaxSize
	data.CreatedAt = model.CreatedAt
}

func preserveWarmPoolPlannedInputs(data *WarmPoolModel, planned WarmPoolModel) {
	data.ProjectID = preserveString(data.ProjectID, planned.ProjectID)
	data.EnvironmentClassID = preserveString(data.EnvironmentClassID, planned.EnvironmentClassID)
	data.MinSize = preserveInt32(data.MinSize, planned.MinSize)
	data.MaxSize = preserveInt32(data.MaxSize, planned.MaxSize)
}

func warmPoolIsDeleted(warmPool *v1.WarmPool) bool {
	if warmPool == nil {
		return true
	}
	spec := warmPool.GetSpec()
	status := warmPool.GetStatus()
	return spec.GetDesiredPhase() == v1.WarmPoolPhase_WARM_POOL_PHASE_DELETED ||
		status.GetPhase() == v1.WarmPoolPhase_WARM_POOL_PHASE_DELETED
}

func warmPoolCollectionFilters(ctx context.Context, data WarmPoolCollectionModel) (*v1.ListWarmPoolsRequest_Filter, diag.Diagnostics) {
	var diags diag.Diagnostics
	filter := &v1.ListWarmPoolsRequest_Filter{}
	if !data.ProjectIDs.IsNull() && !data.ProjectIDs.IsUnknown() {
		diags.Append(data.ProjectIDs.ElementsAs(ctx, &filter.ProjectIds, false)...)
	}
	if !data.EnvironmentClassIDs.IsNull() && !data.EnvironmentClassIDs.IsUnknown() {
		diags.Append(data.EnvironmentClassIDs.ElementsAs(ctx, &filter.EnvironmentClassIds, false)...)
	}
	if diags.HasError() {
		return nil, diags
	}
	sort.Strings(filter.ProjectIds)
	sort.Strings(filter.EnvironmentClassIds)
	if len(filter.ProjectIds) == 0 && len(filter.EnvironmentClassIds) == 0 {
		return nil, diags
	}
	return filter, diags
}

func pageSize(data WarmPoolCollectionModel) int32 {
	if data.PageSize.IsNull() || data.PageSize.IsUnknown() || data.PageSize.ValueInt32() <= 0 {
		return 100
	}
	return data.PageSize.ValueInt32()
}

func validateRequiredString(value types.String, p path.Path, name string, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		return
	}
	if value.IsNull() || value.ValueString() == "" {
		diags.AddAttributeError(p, "Missing Warm Pool "+name, name+" must not be empty.")
	}
}

func timestampValue(ts *timestamppb.Timestamp) types.String {
	if ts == nil || !ts.IsValid() {
		return types.StringNull()
	}
	return types.StringValue(ts.AsTime().Format("2006-01-02T15:04:05Z07:00"))
}

func stringOptionalValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func preserveString(current types.String, planned types.String) types.String {
	if !planned.IsNull() && !planned.IsUnknown() {
		return planned
	}
	return current
}

func preserveInt32(current types.Int32, planned types.Int32) types.Int32 {
	if !planned.IsNull() && !planned.IsUnknown() {
		return planned
	}
	return current
}

func invalidMappingError(name string) error {
	return fmt.Errorf("invalid warm pool mapping: %s", name)
}

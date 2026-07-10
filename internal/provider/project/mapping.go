// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"context"
	"fmt"
	"sort"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	principalUser           = "user"
	principalServiceAccount = "service_account"
)

type ProjectModel struct {
	ID                   types.String                 `tfsdk:"id"`
	OrganizationID       types.String                 `tfsdk:"organization_id"`
	Name                 types.String                 `tfsdk:"name"`
	RepositoryCloneURL   types.String                 `tfsdk:"repository_clone_url"`
	Branch               types.String                 `tfsdk:"branch"`
	DevcontainerFilePath types.String                 `tfsdk:"devcontainer_file_path"`
	AutomationsFilePath  types.String                 `tfsdk:"automations_file_path"`
	EnvironmentClasses   []EnvironmentClassModel      `tfsdk:"environment_class"`
	Prebuild             []PrebuildConfigurationModel `tfsdk:"prebuild_configuration"`
	CreatedAt            types.String                 `tfsdk:"created_at"`
	UpdatedAt            types.String                 `tfsdk:"updated_at"`
	Creator              types.Object                 `tfsdk:"creator"`
}

type EnvironmentClassModel struct {
	EnvironmentClassID types.String `tfsdk:"environment_class_id"`
	LocalRunner        types.Bool   `tfsdk:"local_runner"`
	Order              types.Int64  `tfsdk:"order"`
}

type PrebuildConfigurationModel struct {
	Enabled               types.Bool           `tfsdk:"enabled"`
	EnvironmentClassIDs   types.Set            `tfsdk:"environment_class_ids"`
	Timeout               types.String         `tfsdk:"timeout"`
	DailySchedule         []DailyScheduleModel `tfsdk:"daily_schedule"`
	Executor              []SubjectModel       `tfsdk:"executor"`
	EnableJetbrainsWarmup types.Bool           `tfsdk:"enable_jetbrains_warmup"`
}

type DailyScheduleModel struct {
	HourUTC types.Int64 `tfsdk:"hour_utc"`
}

type SubjectModel struct {
	ID        types.String `tfsdk:"id"`
	Principal types.String `tfsdk:"principal"`
}

var subjectObjectAttributeTypes = map[string]attr.Type{
	"id":        types.StringType,
	"principal": types.StringType,
}

func projectCreateRequest(ctx context.Context, data ProjectModel) (*v1.CreateProjectRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	prebuild, prebuildDiags := prebuildConfigurationFromModel(ctx, data.Prebuild, path.Root("prebuild_configuration"))
	diags.Append(prebuildDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return &v1.CreateProjectRequest{
		Name:                  data.Name.ValueString(),
		Initializer:           environmentInitializerFromRepository(data),
		DevcontainerFilePath:  optionalString(data.DevcontainerFilePath),
		AutomationsFilePath:   optionalString(data.AutomationsFilePath),
		PrebuildConfiguration: prebuild,
	}, diags
}

func projectUpdateRequest(ctx context.Context, plan ProjectModel, prior ProjectModel) (*v1.UpdateProjectRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	req := &v1.UpdateProjectRequest{
		ProjectId: plan.ID.ValueString(),
	}
	if isKnownString(plan.Name) {
		req.Name = ptr(plan.Name.ValueString())
	}
	req.Initializer = environmentInitializerFromRepository(plan)
	if !plan.DevcontainerFilePath.IsUnknown() {
		req.DevcontainerFilePath = ptr(optionalString(plan.DevcontainerFilePath))
	} else if !prior.DevcontainerFilePath.IsNull() {
		req.DevcontainerFilePath = ptr("")
	}
	if !plan.AutomationsFilePath.IsUnknown() {
		req.AutomationsFilePath = ptr(optionalString(plan.AutomationsFilePath))
	} else if !prior.AutomationsFilePath.IsNull() {
		req.AutomationsFilePath = ptr("")
	}

	prebuild, prebuildDiags := prebuildConfigurationFromModel(ctx, plan.Prebuild, path.Root("prebuild_configuration"))
	diags.Append(prebuildDiags...)
	if len(plan.Prebuild) > 0 {
		req.PrebuildConfiguration = prebuild
	} else if len(prior.Prebuild) > 0 {
		req.PrebuildConfiguration = &v1.ProjectPrebuildConfiguration{Enabled: false}
	}
	if diags.HasError() {
		return nil, diags
	}
	return req, diags
}

func environmentInitializerFromRepository(data ProjectModel) *v1.EnvironmentInitializer {
	return &v1.EnvironmentInitializer{
		Specs: []*v1.EnvironmentInitializer_Spec{
			{
				Spec: &v1.EnvironmentInitializer_Spec_Git{
					Git: &v1.GitInitializer{
						RemoteUri:   data.RepositoryCloneURL.ValueString(),
						TargetMode:  v1.GitInitializer_CLONE_TARGET_MODE_REMOTE_BRANCH,
						CloneTarget: data.Branch.ValueString(),
					},
				},
			},
		},
	}
}

func projectEnvironmentClassesFromModel(values []EnvironmentClassModel, root path.Path, allowUnknown bool) ([]*v1.ProjectEnvironmentClass, diag.Diagnostics) {
	var diags diag.Diagnostics
	if len(values) == 0 {
		diags.AddAttributeError(root, "Missing Project Environment Class", "Set at least one environment_class block.")
		return nil, diags
	}
	if len(values) > 30 {
		diags.AddAttributeError(root, "Too Many Project Environment Classes", "Set no more than 30 environment_class blocks.")
		return nil, diags
	}
	classes := make([]*v1.ProjectEnvironmentClass, 0, len(values))
	for i, value := range values {
		p := root.AtListIndex(i)
		idUnknown := value.EnvironmentClassID.IsUnknown()
		hasID := isKnownString(value.EnvironmentClassID) || (allowUnknown && idUnknown)
		hasLocalRunner := isKnownBool(value.LocalRunner) && value.LocalRunner.ValueBool()
		if hasID == hasLocalRunner {
			diags.AddAttributeError(p, "Invalid Project Environment Class", "Set exactly one of environment_class_id or local_runner = true.")
			continue
		}
		if idUnknown {
			if !allowUnknown {
				diags.AddAttributeError(p.AtName("environment_class_id"), "Unknown Project Environment Class ID", "environment_class_id must be known before apply.")
			}
			continue
		}
		class := &v1.ProjectEnvironmentClass{
			Order: int32(value.Order.ValueInt64()),
		}
		if hasID {
			class.EnvironmentClass = &v1.ProjectEnvironmentClass_EnvironmentClassId{
				EnvironmentClassId: value.EnvironmentClassID.ValueString(),
			}
		} else {
			class.EnvironmentClass = &v1.ProjectEnvironmentClass_LocalRunner{LocalRunner: true}
		}
		classes = append(classes, class)
	}
	if diags.HasError() {
		return nil, diags
	}
	sort.SliceStable(classes, func(i, j int) bool {
		return classes[i].GetOrder() < classes[j].GetOrder()
	})
	return classes, diags
}

func prebuildConfigurationFromModel(ctx context.Context, values []PrebuildConfigurationModel, root path.Path) (*v1.ProjectPrebuildConfiguration, diag.Diagnostics) {
	var diags diag.Diagnostics
	if len(values) == 0 {
		return nil, diags
	}
	if len(values) > 1 {
		diags.AddAttributeError(root, "Too Many Prebuild Configurations", "Set no more than one prebuild_configuration block.")
		return nil, diags
	}
	value := values[0]

	timeout := time.Hour
	if !value.Timeout.IsNull() && !value.Timeout.IsUnknown() {
		parsed, err := time.ParseDuration(value.Timeout.ValueString())
		if err != nil {
			diags.AddAttributeError(root.AtName("timeout"), "Invalid Prebuild Timeout", err.Error())
			return nil, diags
		}
		timeout = parsed
	}
	if timeout < 5*time.Minute || timeout > 2*time.Hour {
		diags.AddAttributeError(root.AtName("timeout"), "Invalid Prebuild Timeout", "Timeout must be between 5m and 2h.")
		return nil, diags
	}

	var environmentClassIDs []string
	if !value.EnvironmentClassIDs.IsNull() && !value.EnvironmentClassIDs.IsUnknown() {
		diags.Append(value.EnvironmentClassIDs.ElementsAs(ctx, &environmentClassIDs, false)...)
		if diags.HasError() {
			return nil, diags
		}
		sort.Strings(environmentClassIDs)
	}

	cfg := &v1.ProjectPrebuildConfiguration{
		Enabled:               value.Enabled.ValueBool(),
		EnvironmentClassIds:   environmentClassIDs,
		Timeout:               durationpb.New(timeout),
		EnableJetbrainsWarmup: value.EnableJetbrainsWarmup.ValueBool(),
	}
	if len(value.DailySchedule) > 1 {
		diags.AddAttributeError(root.AtName("daily_schedule"), "Too Many Daily Schedules", "Set no more than one daily_schedule block.")
		return nil, diags
	}
	if len(value.DailySchedule) == 1 {
		if value.DailySchedule[0].HourUTC.IsNull() || value.DailySchedule[0].HourUTC.IsUnknown() {
			diags.AddAttributeError(root.AtName("daily_schedule").AtListIndex(0).AtName("hour_utc"), "Missing Daily Schedule Hour", "hour_utc must be known and must be between 0 and 23.")
			return nil, diags
		}
		hour := value.DailySchedule[0].HourUTC.ValueInt64()
		if hour < 0 || hour > 23 {
			diags.AddAttributeError(root.AtName("daily_schedule").AtListIndex(0).AtName("hour_utc"), "Invalid Daily Schedule Hour", "hour_utc must be between 0 and 23.")
			return nil, diags
		}
		cfg.Trigger = &v1.PrebuildTrigger{
			Trigger: &v1.PrebuildTrigger_DailySchedule_{
				DailySchedule: &v1.PrebuildTrigger_DailySchedule{HourUtc: int32(hour)},
			},
		}
	}
	if len(value.Executor) > 1 {
		diags.AddAttributeError(root.AtName("executor"), "Too Many Prebuild Executors", "Set no more than one executor block.")
		return nil, diags
	}
	if len(value.Executor) == 1 {
		if value.Executor[0].ID.IsNull() || value.Executor[0].ID.IsUnknown() || value.Executor[0].ID.ValueString() == "" {
			diags.AddAttributeError(root.AtName("executor").AtListIndex(0).AtName("id"), "Missing Prebuild Executor ID", "Executor id must not be empty.")
			return nil, diags
		}
		if value.Executor[0].Principal.IsNull() || value.Executor[0].Principal.IsUnknown() {
			diags.AddAttributeError(root.AtName("executor").AtListIndex(0).AtName("principal"), "Missing Prebuild Executor Principal", "Supported values are user and service_account.")
			return nil, diags
		}
		principal, ok := principalFromString(value.Executor[0].Principal.ValueString())
		if !ok {
			diags.AddAttributeError(root.AtName("executor").AtListIndex(0).AtName("principal"), "Invalid Prebuild Executor Principal", "Supported values are user and service_account.")
			return nil, diags
		}
		cfg.Executor = &v1.Subject{
			Id:        value.Executor[0].ID.ValueString(),
			Principal: principal,
		}
	}
	return cfg, diags
}

func projectModelFromProto(ctx context.Context, project *v1.Project) (ProjectModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	metadata := project.GetMetadata()
	repository, repoDiags := repositoryFromInitializer(project.GetInitializer())
	diags.Append(repoDiags...)
	data := ProjectModel{
		ID:                   types.StringValue(project.GetId()),
		OrganizationID:       stringOptionalValue(metadata.GetOrganizationId()),
		Name:                 types.StringValue(metadata.GetName()),
		RepositoryCloneURL:   repository.CloneURL,
		Branch:               repository.Branch,
		DevcontainerFilePath: stringOptionalValue(project.GetDevcontainerFilePath()),
		AutomationsFilePath:  stringOptionalValue(project.GetAutomationsFilePath()),
		EnvironmentClasses:   environmentClassesFromProto(project.GetEnvironmentClasses()),
		Prebuild:             prebuildConfigurationFromProto(ctx, project.GetPrebuildConfiguration(), &diags),
		CreatedAt:            timestampValue(metadata.GetCreatedAt()),
		UpdatedAt:            timestampValue(metadata.GetUpdatedAt()),
		Creator:              subjectObjectFromProto(metadata.GetCreator(), &diags),
	}
	return data, diags
}

type repositoryFields struct {
	CloneURL types.String
	Branch   types.String
}

func repositoryFromInitializer(initializer *v1.EnvironmentInitializer) (repositoryFields, diag.Diagnostics) {
	var diags diag.Diagnostics
	for _, spec := range initializer.GetSpecs() {
		git := spec.GetGit()
		if git == nil {
			continue
		}
		if git.GetRemoteUri() == "" || git.GetCloneTarget() == "" {
			diags.AddError(
				"Unsupported Ona Project Repository",
				"Project must have a Git repository clone URL and branch to be managed by Terraform.",
			)
			return repositoryFields{}, diags
		}
		return repositoryFields{
			CloneURL: types.StringValue(git.GetRemoteUri()),
			Branch:   types.StringValue(git.GetCloneTarget()),
		}, diags
	}
	diags.AddError(
		"Unsupported Ona Project Repository",
		"Project must have a Git repository clone URL and branch to be managed by Terraform.",
	)
	return repositoryFields{}, diags
}

func environmentClassesFromProto(classes []*v1.ProjectEnvironmentClass) []EnvironmentClassModel {
	result := make([]EnvironmentClassModel, 0, len(classes))
	for _, class := range classes {
		item := EnvironmentClassModel{
			Order: types.Int64Value(int64(class.GetOrder())),
		}
		switch class.GetEnvironmentClass().(type) {
		case *v1.ProjectEnvironmentClass_EnvironmentClassId:
			item.EnvironmentClassID = types.StringValue(class.GetEnvironmentClassId())
			item.LocalRunner = types.BoolNull()
		case *v1.ProjectEnvironmentClass_LocalRunner:
			item.EnvironmentClassID = types.StringNull()
			item.LocalRunner = types.BoolValue(true)
		default:
			item.EnvironmentClassID = types.StringNull()
			item.LocalRunner = types.BoolNull()
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Order.ValueInt64() < result[j].Order.ValueInt64()
	})
	return result
}

func prebuildConfigurationFromProto(ctx context.Context, cfg *v1.ProjectPrebuildConfiguration, diags *diag.Diagnostics) []PrebuildConfigurationModel {
	if cfg == nil || isEmptyDisabledPrebuildConfiguration(cfg) {
		return nil
	}
	timeout := time.Hour
	if cfg.GetTimeout() != nil {
		timeout = cfg.GetTimeout().AsDuration()
	}
	ids, setDiags := types.SetValueFrom(ctx, types.StringType, cfg.GetEnvironmentClassIds())
	diags.Append(setDiags...)
	result := PrebuildConfigurationModel{
		Enabled:               types.BoolValue(cfg.GetEnabled()),
		EnvironmentClassIDs:   ids,
		Timeout:               types.StringValue(timeout.String()),
		EnableJetbrainsWarmup: types.BoolValue(cfg.GetEnableJetbrainsWarmup()),
	}
	if executor := subjectFromProto(cfg.GetExecutor()); executor != nil {
		result.Executor = []SubjectModel{*executor}
	}
	if schedule := cfg.GetTrigger().GetDailySchedule(); schedule != nil {
		result.DailySchedule = []DailyScheduleModel{{HourUTC: types.Int64Value(int64(schedule.GetHourUtc()))}}
	}
	return []PrebuildConfigurationModel{result}
}

func isEmptyDisabledPrebuildConfiguration(cfg *v1.ProjectPrebuildConfiguration) bool {
	return !cfg.GetEnabled() &&
		len(cfg.GetEnvironmentClassIds()) == 0 &&
		cfg.GetTimeout() == nil &&
		cfg.GetTrigger() == nil &&
		cfg.GetExecutor() == nil &&
		!cfg.GetEnableJetbrainsWarmup()
}

func subjectFromProto(subject *v1.Subject) *SubjectModel {
	if subject == nil {
		return nil
	}
	return &SubjectModel{
		ID:        stringOptionalValue(subject.GetId()),
		Principal: stringOptionalValue(principalToString(subject.GetPrincipal())),
	}
}

func subjectObjectFromProto(subject *v1.Subject, diags *diag.Diagnostics) types.Object {
	if subject == nil {
		return types.ObjectNull(subjectObjectAttributeTypes)
	}
	result, objectDiags := types.ObjectValue(
		subjectObjectAttributeTypes,
		map[string]attr.Value{
			"id":        stringOptionalValue(subject.GetId()),
			"principal": stringOptionalValue(principalToString(subject.GetPrincipal())),
		},
	)
	diags.Append(objectDiags...)
	return result
}

func principalFromString(value string) (v1.Principal, bool) {
	switch value {
	case principalUser:
		return v1.Principal_PRINCIPAL_USER, true
	case principalServiceAccount:
		return v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, true
	default:
		return v1.Principal_PRINCIPAL_UNSPECIFIED, false
	}
}

func principalToString(principal v1.Principal) string {
	switch principal {
	case v1.Principal_PRINCIPAL_USER:
		return principalUser
	case v1.Principal_PRINCIPAL_SERVICE_ACCOUNT:
		return principalServiceAccount
	case v1.Principal_PRINCIPAL_ACCOUNT:
		return "account"
	case v1.Principal_PRINCIPAL_RUNNER:
		return "runner"
	case v1.Principal_PRINCIPAL_ENVIRONMENT:
		return "environment"
	case v1.Principal_PRINCIPAL_RUNNER_MANAGER:
		return "runner_manager"
	default:
		return ""
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

func optionalString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

func isKnownString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueString() != ""
}

func isKnownBool(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func ptr[T any](value T) *T {
	return &value
}

func preserveProjectPlannedInputs(data *ProjectModel, planned ProjectModel) {
	data.Name = preserveString(data.Name, planned.Name)
	data.RepositoryCloneURL = preserveString(data.RepositoryCloneURL, planned.RepositoryCloneURL)
	data.Branch = preserveString(data.Branch, planned.Branch)
	data.DevcontainerFilePath = preserveString(data.DevcontainerFilePath, planned.DevcontainerFilePath)
	data.AutomationsFilePath = preserveString(data.AutomationsFilePath, planned.AutomationsFilePath)
	if len(planned.EnvironmentClasses) > 0 {
		data.EnvironmentClasses = planned.EnvironmentClasses
	}
	data.Prebuild = preservePrebuildConfiguration(data.Prebuild, planned.Prebuild)
}

func preservePrebuildConfiguration(current []PrebuildConfigurationModel, planned []PrebuildConfigurationModel) []PrebuildConfigurationModel {
	if len(planned) == 0 {
		return current
	}
	if len(current) == 0 {
		return planned
	}
	current[0].Enabled = preserveBool(current[0].Enabled, planned[0].Enabled)
	current[0].EnvironmentClassIDs = planned[0].EnvironmentClassIDs
	current[0].Timeout = preserveString(current[0].Timeout, planned[0].Timeout)
	current[0].DailySchedule = planned[0].DailySchedule
	current[0].Executor = planned[0].Executor
	current[0].EnableJetbrainsWarmup = preserveBool(current[0].EnableJetbrainsWarmup, planned[0].EnableJetbrainsWarmup)
	return current
}

func preserveString(current types.String, planned types.String) types.String {
	if !planned.IsNull() && !planned.IsUnknown() {
		return planned
	}
	return current
}

func preserveBool(current types.Bool, planned types.Bool) types.Bool {
	if !planned.IsNull() && !planned.IsUnknown() {
		return planned
	}
	return current
}

func invalidMappingError(name string) error {
	return fmt.Errorf("invalid project mapping: %s", name)
}

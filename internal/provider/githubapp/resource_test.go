// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package githubapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/gitpod-sdk-go/v1/v1connect"
	"github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
)

type fakeService struct {
	v1connect.IntegrationServiceClient
	create                                                         *connect.Response[v1.CreateIntegrationResponse]
	createErr                                                      error
	get                                                            *connect.Response[v1.GetIntegrationResponse]
	getErr                                                         error
	update                                                         *connect.Response[v1.UpdateIntegrationResponse]
	updateErr                                                      error
	deleteErr                                                      error
	definitionPages                                                []*connect.Response[v1.ListIntegrationDefinitionsResponse]
	definitionErr                                                  error
	pages                                                          []*connect.Response[v1.ListIntegrationsResponse]
	listErr                                                        error
	definitionCalls, listCalls, createCalls, getCalls, deleteCalls int
	createRequest                                                  *v1.CreateIntegrationRequest
	updateRequest                                                  *v1.UpdateIntegrationRequest
}

func (s *fakeService) CreateIntegration(_ context.Context, req *connect.Request[v1.CreateIntegrationRequest]) (*connect.Response[v1.CreateIntegrationResponse], error) {
	s.createCalls++
	s.createRequest = req.Msg
	return s.create, s.createErr
}
func (s *fakeService) GetIntegration(_ context.Context, _ *connect.Request[v1.GetIntegrationRequest]) (*connect.Response[v1.GetIntegrationResponse], error) {
	s.getCalls++
	return s.get, s.getErr
}
func (s *fakeService) UpdateIntegration(_ context.Context, req *connect.Request[v1.UpdateIntegrationRequest]) (*connect.Response[v1.UpdateIntegrationResponse], error) {
	s.updateRequest = req.Msg
	return s.update, s.updateErr
}
func (s *fakeService) DeleteIntegration(_ context.Context, _ *connect.Request[v1.DeleteIntegrationRequest]) (*connect.Response[v1.DeleteIntegrationResponse], error) {
	s.deleteCalls++
	return connect.NewResponse(&v1.DeleteIntegrationResponse{}), s.deleteErr
}
func (s *fakeService) ListIntegrationDefinitions(_ context.Context, _ *connect.Request[v1.ListIntegrationDefinitionsRequest]) (*connect.Response[v1.ListIntegrationDefinitionsResponse], error) {
	defer func() { s.definitionCalls++ }()
	if s.definitionErr != nil {
		return nil, s.definitionErr
	}
	if s.definitionCalls < len(s.definitionPages) {
		return s.definitionPages[s.definitionCalls], nil
	}
	return connect.NewResponse(&v1.ListIntegrationDefinitionsResponse{Definitions: []*v1.IntegrationDefinition{testDefinition()}}), nil
}
func (s *fakeService) ListIntegrations(_ context.Context, _ *connect.Request[v1.ListIntegrationsRequest]) (*connect.Response[v1.ListIntegrationsResponse], error) {
	defer func() { s.listCalls++ }()
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listCalls < len(s.pages) {
		return s.pages[s.listCalls], nil
	}
	return connect.NewResponse(&v1.ListIntegrationsResponse{}), nil
}

func testResource(s *fakeService) *Resource {
	return &Resource{client: managementclient.NewWithServices(managementclient.Services{IntegrationService: s}), deployment: "https://ona.example.com/api"}
}
func testIdentity(t *testing.T) *tfsdk.ResourceIdentity {
	t.Helper()
	var resp resource.IdentitySchemaResponse
	(&Resource{}).IdentitySchema(t.Context(), resource.IdentitySchemaRequest{}, &resp)
	return &tfsdk.ResourceIdentity{Schema: resp.IdentitySchema}
}

func TestCreateIdentityAndPayload(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		ID, Identity, Slug string
		Calls, Reads       int
		SecretPresent      bool
		Errors             []string
		Request            *v1.CreateIntegrationRequest
	}
	request := &v1.CreateIntegrationRequest{IntegrationDefinitionId: "github-definition", Enabled: true, Auth: &v1.IntegrationAuthentication{RequiresAuth: true, ProprietaryApp: &v1.IntegrationProprietaryAppConfig{AppId: "123", PrivateKey: "private-must-not-appear", ClientSecret: "client-must-not-appear", WebhookSecret: "webhook-must-not-appear"}}}
	tests := []struct {
		Name     string
		Setup    func(*fakeService)
		Expected Expectation
	}{
		{Name: "create_uses_minimal_bundle_and_returned_metadata", Expected: Expectation{ID: testID, Identity: testID, Slug: "owned-app", Calls: 1, Request: request}},
		{Name: "malformed_metadata_retains_created_identity", Setup: func(s *fakeService) { s.create.Msg.Integration.Auth.ProprietaryApp.AppSlug = "" }, Expected: Expectation{ID: testID, Identity: testID, Calls: 1, Errors: []string{"Incomplete GitHub App Metadata"}, Request: request}},
		{Name: "nonmatching_definition_retains_created_identity", Setup: func(s *fakeService) { s.create.Msg.Integration.IntegrationDefinitionId = "other" }, Expected: Expectation{ID: testID, Identity: testID, Calls: 1, Errors: []string{"Not an Organization-Owned GitHub App"}, Request: request}},
		{Name: "shared_metadata_retains_created_identity", Setup: func(s *fakeService) { s.create.Msg.Integration = testSharedIntegration() }, Expected: Expectation{ID: testID, Identity: testID, Calls: 1, Errors: []string{"Not an Organization-Owned GitHub App"}, Request: request}},
		{Name: "nil_response", Setup: func(s *fakeService) { s.create = nil }, Expected: Expectation{Calls: 1, Errors: []string{"Unable to Create GitHub App Integration"}, Request: request}},
		{Name: "nil_message", Setup: func(s *fakeService) { s.create.Msg = nil }, Expected: Expectation{Calls: 1, Errors: []string{"Unable to Create GitHub App Integration"}, Request: request}},
		{Name: "missing_id", Setup: func(s *fakeService) { s.create.Msg.Integration.Id = "" }, Expected: Expectation{Calls: 1, Errors: []string{"Unable to Create GitHub App Integration"}, Request: request}},
		{Name: "error_sanitized", Setup: func(s *fakeService) {
			s.createErr = fmt.Errorf("request with outer-must-not-appear: %w", connect.NewError(connect.CodeInvalidArgument, errors.New("private-must-not-appear client-must-not-appear webhook-must-not-appear")))
		}, Expected: Expectation{Calls: 1, Errors: []string{"Unable to Create GitHub App Integration"}, Request: request}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s := &fakeService{create: connect.NewResponse(&v1.CreateIntegrationResponse{Integration: testIntegration()})}
			if tc.Setup != nil {
				tc.Setup(s)
			}
			config := testConfig()
			plan := config
			plan.Credentials = types.ObjectNull(credentialsTypes)
			req := resource.CreateRequest{Config: modelConfig(t, config), Plan: modelPlan(t, plan)}
			resp := resource.CreateResponse{State: modelState(t, emptyModel()), Identity: testIdentity(t)}
			testResource(s).Create(t.Context(), req, &resp)
			got := Expectation{Calls: s.createCalls, Reads: s.getCalls, Errors: errorSummaries(resp.Diagnostics), Request: s.createRequest}
			var model Model
			diags := resp.State.Get(t.Context(), &model)
			got.Errors = append(got.Errors, errorSummaries(diags)...)
			got.ID = model.ID.ValueString()
			got.Slug = model.AppSlug.ValueString()
			if got.ID != "" {
				var identity IdentityModel
				got.Errors = append(got.Errors, errorSummaries(resp.Identity.Get(t.Context(), &identity))...)
				got.Identity = identity.IntegrationID.ValueString()
			}
			got.SecretPresent = strings.Contains(resp.State.Raw.String()+fmt.Sprint(resp.Diagnostics), "must-not-appear")
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("create mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReadRetainsStateUnlessExactNotFound(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Removed                bool
		ID, Slug, Installation string
		Version                int64
		Calls                  int
		Errors                 []string
	}
	tests := []struct {
		Name     string
		Setup    func(*fakeService)
		Expected Expectation
	}{
		{Name: "metadata_and_installation_refresh", Setup: func(s *fakeService) {
			s.get.Msg.Integration.Auth.ProprietaryApp.AppSlug = "renamed"
			s.get.Msg.Integration.ExternalInstallation = &v1.IntegrationExternalInstallation{Id: "42"}
		}, Expected: Expectation{ID: testID, Slug: "renamed", Installation: "42", Version: 1, Calls: 1}},
		{Name: "exact_notfound", Setup: func(s *fakeService) { s.getErr = connect.NewError(connect.CodeNotFound, errors.New("gone")) }, Expected: Expectation{Removed: true, Calls: 1}},
		{Name: "nil_response_retains_state", Setup: func(s *fakeService) { s.get = nil }, Expected: Expectation{ID: testID, Version: 1, Calls: 1, Errors: []string{"Unable to Read GitHub App Integration"}}},
		{Name: "nil_message_retains_state", Setup: func(s *fakeService) { s.get.Msg = nil }, Expected: Expectation{ID: testID, Version: 1, Calls: 1, Errors: []string{"Unable to Read GitHub App Integration"}}},
		{Name: "empty_integration_retains_state", Setup: func(s *fakeService) { s.get.Msg.Integration = nil }, Expected: Expectation{ID: testID, Version: 1, Calls: 1, Errors: []string{"Unable to Read GitHub App Integration"}}},
		{Name: "permission_denied_retains_state", Setup: func(s *fakeService) { s.getErr = connect.NewError(connect.CodePermissionDenied, errors.New("denied")) }, Expected: Expectation{ID: testID, Version: 1, Calls: 1, Errors: []string{"Unable to Read GitHub App Integration"}}},
		{Name: "metadata_now_matches_shared_app_retains_state", Setup: func(s *fakeService) { s.get.Msg.Integration = testSharedIntegration() }, Expected: Expectation{ID: testID, Version: 1, Calls: 1, Errors: []string{"Not an Organization-Owned GitHub App"}}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s := &fakeService{get: connect.NewResponse(&v1.GetIntegrationResponse{Integration: testIntegration()})}
			tc.Setup(s)
			prior := testConfig()
			prior.ID = types.StringValue(testID)
			prior.Credentials = types.ObjectNull(credentialsTypes)
			state := modelState(t, prior)
			resp := resource.ReadResponse{State: state, Identity: testIdentity(t)}
			testResource(s).Read(t.Context(), resource.ReadRequest{State: state}, &resp)
			got := Expectation{Removed: resp.State.Raw.IsNull(), Calls: s.getCalls, Errors: errorSummaries(resp.Diagnostics)}
			if !got.Removed {
				var m Model
				got.Errors = append(got.Errors, errorSummaries(resp.State.Get(t.Context(), &m))...)
				got.ID = m.ID.ValueString()
				got.Slug = m.AppSlug.ValueString()
				got.Version = m.CredentialsVersion.ValueInt64()
				if !m.Installation.IsNull() {
					got.Installation = testStringAttribute(t, m.Installation, "id")
				}
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("read mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateCredentialRotation(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		ID, Slug, Installation       string
		Version                      int64
		Auth, Enabled, SecretPresent bool
		Errors                       []string
	}
	tests := []struct {
		Name            string
		Rotate, Disable bool
		Setup           func(*fakeService)
		Expected        Expectation
	}{
		{Name: "full_bundle_rotation_preserves_identity_and_installation", Rotate: true, Expected: Expectation{ID: testID, Slug: "renamed", Installation: "42", Version: 2, Auth: true}},
		{Name: "enable_change_does_not_resubmit_credentials", Disable: true, Expected: Expectation{ID: testID, Slug: "owned-app", Installation: "42", Version: 1, Enabled: true}},
		{Name: "unchanged_marker_does_not_resubmit_credentials", Expected: Expectation{ID: testID, Slug: "owned-app", Installation: "42", Version: 1}},
		{Name: "nil_response_preserves_previous_state", Rotate: true, Setup: func(s *fakeService) { s.update = nil }, Expected: Expectation{ID: testID, Version: 1, Auth: true, Errors: []string{"Unable to Update GitHub App Integration"}}},
		{Name: "nil_message_preserves_previous_state", Rotate: true, Setup: func(s *fakeService) { s.update.Msg = nil }, Expected: Expectation{ID: testID, Version: 1, Auth: true, Errors: []string{"Unable to Update GitHub App Integration"}}},
		{Name: "mismatched_response_preserves_previous_state", Rotate: true, Setup: func(s *fakeService) { s.update.Msg.Integration.Id = "other-id" }, Expected: Expectation{ID: testID, Version: 1, Auth: true, Errors: []string{"Unable to Update GitHub App Integration"}}},
		{Name: "api_error_sanitized_preserves_previous_state", Rotate: true, Setup: func(s *fakeService) {
			s.updateErr = connect.NewError(connect.CodeInvalidArgument, errors.New("private-must-not-appear"))
		}, Expected: Expectation{ID: testID, Version: 1, Auth: true, Errors: []string{"Unable to Update GitHub App Integration"}}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			remote := testIntegration()
			remote.ExternalInstallation = &v1.IntegrationExternalInstallation{Id: "42"}
			if tc.Rotate {
				remote.Auth.ProprietaryApp.AppSlug = "renamed"
			}
			remote.Enabled = !tc.Disable
			s := &fakeService{update: connect.NewResponse(&v1.UpdateIntegrationResponse{Integration: remote})}
			if tc.Setup != nil {
				tc.Setup(s)
			}
			prior := testConfig()
			prior.ID = types.StringValue(testID)
			prior.Credentials = types.ObjectNull(credentialsTypes)
			config := testConfig()
			if tc.Rotate {
				config.CredentialsVersion = types.Int64Value(2)
			}
			config.Enabled = types.BoolValue(!tc.Disable)
			plan := config
			plan.ID = prior.ID
			plan.Credentials = types.ObjectNull(credentialsTypes)
			state := modelState(t, prior)
			resp := resource.UpdateResponse{State: state, Identity: testIdentity(t)}
			testResource(s).Update(t.Context(), resource.UpdateRequest{State: state, Plan: modelPlan(t, plan), Config: modelConfig(t, config)}, &resp)
			got := Expectation{Errors: errorSummaries(resp.Diagnostics), Auth: s.updateRequest.GetAuth() != nil, Enabled: s.updateRequest.Enabled != nil, SecretPresent: strings.Contains(resp.State.Raw.String()+fmt.Sprint(resp.Diagnostics), "must-not-appear")}
			var model Model
			got.Errors = append(got.Errors, errorSummaries(resp.State.Get(t.Context(), &model))...)
			got.ID = model.ID.ValueString()
			got.Slug = model.AppSlug.ValueString()
			got.Version = model.CredentialsVersion.ValueInt64()
			if !model.Installation.IsNull() {
				got.Installation = testStringAttribute(t, model.Installation, "id")
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("update mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeleteIntegrationOnly(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Removed              bool
		Calls                int
		Error, SecretPresent bool
	}
	tests := []struct {
		Name     string
		Err      error
		Expected Expectation
	}{
		{Name: "delete", Expected: Expectation{Removed: true, Calls: 1}},
		{Name: "already_deleted", Err: connect.NewError(connect.CodeNotFound, errors.New("gone")), Expected: Expectation{Removed: true, Calls: 1}},
		{Name: "api_failure_retains_state", Err: connect.NewError(connect.CodePermissionDenied, errors.New("must-not-appear")), Expected: Expectation{Calls: 1, Error: true}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s := &fakeService{deleteErr: tc.Err}
			model := emptyModel()
			model.ID = types.StringValue(testID)
			state := modelState(t, model)
			resp := resource.DeleteResponse{State: state}
			testResource(s).Delete(t.Context(), resource.DeleteRequest{State: state}, &resp)
			got := Expectation{Removed: resp.State.Raw.IsNull(), Calls: s.deleteCalls, Error: resp.Diagnostics.HasError(), SecretPresent: strings.Contains(fmt.Sprint(resp.Diagnostics), "must-not-appear")}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("delete mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDefinitionDiscovery(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		ID            string
		Calls         int
		Error         bool
		SecretPresent bool
	}
	page := func(next string, defs ...*v1.IntegrationDefinition) *connect.Response[v1.ListIntegrationDefinitionsResponse] {
		return connect.NewResponse(&v1.ListIntegrationDefinitionsResponse{Pagination: &v1.PaginationResponse{NextToken: next}, Definitions: defs})
	}
	tests := []struct {
		Name     string
		Pages    []*connect.Response[v1.ListIntegrationDefinitionsResponse]
		Err      error
		Expected Expectation
	}{
		{Name: "full_pagination", Pages: []*connect.Response[v1.ListIntegrationDefinitionsResponse]{page("next", &v1.IntegrationDefinition{Host: "github.com"}), page("", testDefinition())}, Expected: Expectation{ID: "github-definition", Calls: 2}},
		{Name: "absent", Pages: []*connect.Response[v1.ListIntegrationDefinitionsResponse]{page("")}, Expected: Expectation{Calls: 1, Error: true}},
		{Name: "ambiguous_later_page", Pages: []*connect.Response[v1.ListIntegrationDefinitionsResponse]{page("next", testDefinition()), page("", testDefinition())}, Expected: Expectation{Calls: 2, Error: true}},
		{Name: "nil_response", Pages: []*connect.Response[v1.ListIntegrationDefinitionsResponse]{nil}, Expected: Expectation{Calls: 1, Error: true}},
		{Name: "repeated_token", Pages: []*connect.Response[v1.ListIntegrationDefinitionsResponse]{page("next"), page("next")}, Expected: Expectation{Calls: 2, Error: true}},
		{Name: "api_error", Err: connect.NewError(connect.CodePermissionDenied, errors.New("must-not-appear")), Expected: Expectation{Calls: 1, Error: true}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s := &fakeService{definitionPages: tc.Pages, definitionErr: tc.Err}
			d, err := testResource(s).githubDefinition(t.Context())
			got := Expectation{ID: d.GetId(), Calls: s.definitionCalls, Error: err != nil}
			if err != nil {
				got.SecretPresent = strings.Contains(err.Error(), "must-not-appear")
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("definition mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestQueryPaginationAndSecretOmission(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		IDs                           []string
		ResourceCount, Calls          int
		SecretPresent, VersionPresent bool
		Errors                        []string
	}
	tests := []struct {
		Name     string
		Include  bool
		Limit    int64
		Expected Expectation
	}{
		{Name: "filtered_page_followed_by_pending_owned", Include: true, Expected: Expectation{IDs: []string{testID, testID}, ResourceCount: 2, Calls: 3}},
		{Name: "identity_only_limit", Limit: 1, Expected: Expectation{IDs: []string{testID}, Calls: 2}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			shared := testSharedIntegration()
			page := func(token string, remote *v1.Integration) *connect.Response[v1.ListIntegrationsResponse] {
				return connect.NewResponse(&v1.ListIntegrationsResponse{Integrations: []*v1.Integration{remote}, Pagination: &v1.PaginationResponse{NextToken: token}})
			}
			s := &fakeService{pages: []*connect.Response[v1.ListIntegrationsResponse]{page("next", shared), page("last", testIntegration()), page("", testIntegration())}}
			r := testResource(s)
			identity := testIdentity(t)
			req := list.ListRequest{IncludeResource: tc.Include, Limit: tc.Limit, ResourceSchema: resourceSchema(), ResourceIdentitySchema: identity.Schema}
			var resp list.ListResultsStream
			r.List(t.Context(), req, &resp)
			got := Expectation{}
			for item := range resp.Results {
				got.Errors = append(got.Errors, errorSummaries(item.Diagnostics)...)
				if item.Diagnostics.HasError() {
					continue
				}
				var id IdentityModel
				got.Errors = append(got.Errors, errorSummaries(item.Identity.Get(t.Context(), &id))...)
				got.IDs = append(got.IDs, id.IntegrationID.ValueString())
				if !item.Resource.Raw.IsNull() {
					got.ResourceCount++
					var model Model
					got.Errors = append(got.Errors, errorSummaries(item.Resource.Get(t.Context(), &model))...)
					got.VersionPresent = got.VersionPresent || !model.CredentialsVersion.IsNull()
					got.SecretPresent = got.SecretPresent || !model.Credentials.IsNull() || strings.Contains(item.Resource.Raw.String(), "must-not-appear")
				}
			}
			got.Calls = s.listCalls
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestQueryErrors(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Errors        int
		Calls         int
		SecretPresent bool
	}
	page := func(token string) *connect.Response[v1.ListIntegrationsResponse] {
		return connect.NewResponse(&v1.ListIntegrationsResponse{Pagination: &v1.PaginationResponse{NextToken: token}})
	}
	tests := []struct {
		Name     string
		Pages    []*connect.Response[v1.ListIntegrationsResponse]
		Err      error
		Expected Expectation
	}{
		{Name: "nil_response", Pages: []*connect.Response[v1.ListIntegrationsResponse]{nil}, Expected: Expectation{Errors: 1, Calls: 1}},
		{Name: "nil_message", Pages: []*connect.Response[v1.ListIntegrationsResponse]{{}}, Expected: Expectation{Errors: 1, Calls: 1}},
		{Name: "repeated_token", Pages: []*connect.Response[v1.ListIntegrationsResponse]{page("next"), page("next")}, Expected: Expectation{Errors: 1, Calls: 2}},
		{Name: "api_error_sanitized", Err: connect.NewError(connect.CodePermissionDenied, errors.New("must-not-appear")), Expected: Expectation{Errors: 1, Calls: 1}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s := &fakeService{pages: tc.Pages, listErr: tc.Err}
			var resp list.ListResultsStream
			testResource(s).List(t.Context(), list.ListRequest{ResourceSchema: resourceSchema(), ResourceIdentitySchema: testIdentity(t).Schema}, &resp)
			got := Expectation{}
			for item := range resp.Results {
				got.Errors += len(item.Diagnostics.Errors())
				got.SecretPresent = got.SecretPresent || strings.Contains(fmt.Sprint(item.Diagnostics), "must-not-appear")
			}
			got.Calls = s.listCalls
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("query error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

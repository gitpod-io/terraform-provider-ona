# Go Test Examples

## HTTP Handler Tests

Use `httptest` with the same handler shape used by provider acceptance-test fake services.

```go
func TestProjectHandlerNotFound(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Status int
		Body   string
	}

	tests := []struct {
		Name     string
		Path     string
		Expected Expectation
	}{
		{
			Name: "unknown_path_returns_not_found",
			Path: "/api/unknown",
			Expected: Expectation{
				Status: http.StatusNotFound,
				Body:   "404 page not found\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			_, projectHandler := v1connect.NewProjectServiceHandler(&fakeProjectService{
				projects: map[string]*v1.Project{},
			})
			handler := http.StripPrefix("/api", projectHandler)

			req := httptest.NewRequest(http.MethodGet, tc.Path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			got := Expectation{
				Status: rec.Code,
				Body:   rec.Body.String(),
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("handler mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

## Tests with Setup Functions

Use setup functions when each table case needs different fake API state or Terraform model values.

```go
func TestServiceAccountTokenRequest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result *v1.CreateServiceAccountTokenRequest
		Err    string
	}

	tests := []struct {
		Name     string
		Setup    func(t *testing.T) ServiceAccountTokenModel
		Expected Expectation
	}{
		{
			Name: "uses_service_account_id",
			Setup: func(t *testing.T) ServiceAccountTokenModel {
				t.Helper()
				return ServiceAccountTokenModel{
					ServiceAccountID: types.StringValue("service-account-1"),
				}
			},
			Expected: Expectation{
				Result: &v1.CreateServiceAccountTokenRequest{
					ServiceAccountId: "service-account-1",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, diags := createServiceAccountTokenRequest(tc.Setup(t))
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("createServiceAccountTokenRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

## Protobuf Tests

Use `protocmp.Transform()` when comparing protobuf messages.

```go
func TestCloneProject(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result *v1.Project
		Err    string
	}

	tests := []struct {
		Name     string
		Input    *v1.Project
		Expected Expectation
	}{
		{
			Name: "maps_project_metadata",
			Input: &v1.Project{
				Id: "project-1",
				Metadata: &v1.ProjectMetadata{
					OrganizationId: "org-1",
					Name:           "acme-api",
				},
			},
			Expected: Expectation{
				Result: &v1.Project{
					Id: "project-1",
					Metadata: &v1.ProjectMetadata{
						OrganizationId: "org-1",
						Name:           "acme-api",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			got.Result = cloneProject(tc.Input)

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("cloneProject() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

## Model to Request Mapping

Test Terraform model conversion directly. Include unknown/null cases and diagnostics.

```go
func TestCreateWarmPoolRequest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result *v1.CreateWarmPoolRequest
		Err    string
	}

	tests := []struct {
		Name     string
		Input    WarmPoolModel
		Expected Expectation
	}{
		{
			Name: "uses_min_and_max_size",
			Input: WarmPoolModel{
				ProjectID:          types.StringValue("project-1"),
				EnvironmentClassID: types.StringValue("class-1"),
				MinSize:            types.Int32Value(0),
				MaxSize:            types.Int32Value(5),
			},
			Expected: Expectation{
				Result: &v1.CreateWarmPoolRequest{
					ProjectId:          "project-1",
					EnvironmentClassId: "class-1",
					MinSize:            ptr[int32](0),
					MaxSize:            ptr[int32](5),
				},
			},
		},
		{
			Name: "rejects_unknown_min_size_before_apply",
			Input: WarmPoolModel{
				ProjectID:          types.StringValue("project-1"),
				EnvironmentClassID: types.StringValue("class-1"),
				MinSize:            types.Int32Unknown(),
				MaxSize:            types.Int32Value(5),
			},
			Expected: Expectation{
				Err: "Missing Warm Pool Minimum Size",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, diags := createWarmPoolRequest(tc.Input)
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("createWarmPoolRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

## Diagnostics Tests

Capture the diagnostic summary or detail in the expectation. Prefer the field that proves the user-facing behavior without making the test brittle.

```go
func TestValidateSecretScope(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Err string
	}

	tests := []struct {
		Name     string
		Input    SecretModel
		Expected Expectation
	}{
		{
			Name: "rejects_missing_project_id_for_project_secret",
			Input: SecretModel{
				Scope: types.StringValue("project"),
				Name:  types.StringValue("THIRD_PARTY_API_KEY"),
			},
			Expected: Expectation{
				Err: "Missing Secret Project ID",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			diags := validateSecretScope(tc.Input)
			if diags.HasError() {
				got.Err = diags[0].Summary()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateSecretScope() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

## Terraform Provider Lifecycle Tests

Use Terraform Plugin Testing for resource lifecycle behavior. Keep `resource.Test` checks even when they are not a single `cmp.Diff` assertion.

```go
func TestAccProjectResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newProjectAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.deleted("project-1") {
				return errors.New("project-1 was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccProjectResourceConfig(server.URL, "acme-api", "class-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_project.api", "id", "project-1"),
					resource.TestCheckResourceAttr("ona_project.api", "name", "acme-api"),
					resource.TestCheckResourceAttr("ona_project.api", "environment_class.0.environment_class_id", "class-1"),
				),
			},
			{
				Config: testAccProjectResourceConfig(server.URL, "acme-api", "class-1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_project.api",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccProjectResourceConfig(server.URL, "acme-api-updated", "class-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_project.api", "name", "acme-api-updated"),
				),
			},
		},
	})
}
```

## Helper Function Pattern

```go
func setupTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}
```

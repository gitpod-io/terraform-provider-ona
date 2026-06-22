# Go Test Examples

## HTTP Handler Tests

```go
func TestHTTPHandler(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Status int
		Body   string
	}

	tests := []struct {
		Name     string
		Method   string
		Path     string
		Expected Expectation
	}{
		{
			Name:   "successful_get",
			Method: http.MethodGet,
			Path:   "/health",
			Expected: Expectation{
				Status: http.StatusOK,
				Body:   `{"status":"ok"}`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.Method, tc.Path, nil)
			rec := httptest.NewRecorder()

			handler := NewHandler()
			handler.ServeHTTP(rec, req)

			got := Expectation{
				Status: rec.Code,
				Body:   rec.Body.String(),
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("Handler mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

## Database Tests with ExistingObjects

Create a fresh database per subtest to enable parallel execution.

```go
func TestDatabaseQuery(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result []*db.User
		Err    string
	}

	tests := []struct {
		Name            string
		ExistingObjects func(ctx context.Context, dbc *db.Client)
		Expected        Expectation
	}{
		{
			Name: "returns_active_users",
			ExistingObjects: func(ctx context.Context, dbc *db.Client) {
				dbc.User.Create().SetEmail("alice@example.com").SetActive(true).SaveX(ctx)
				dbc.User.Create().SetEmail("bob@example.com").SetActive(false).SaveX(ctx)
			},
			Expected: Expectation{
				Result: []*db.User{{Email: "alice@example.com", Active: true}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			dbc := enttest.NewTestDB(t)
			ctx := t.Context()
			tc.ExistingObjects(ctx, dbc)

			var got Expectation
			users, err := dbc.User.Query().Where(user.ActiveEQ(true)).All(ctx)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Result = users
			}

			if diff := cmp.Diff(tc.Expected, got,
				cmpopts.IgnoreFields(db.User{}, "ID", "CreatedAt", "UpdatedAt"),
			); diff != "" {
				t.Errorf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

## Tests with Setup Functions

```go
func TestServiceWithDependencies(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Response *Response
		Err      string
	}

	tests := []struct {
		Name     string
		Setup    func(t *testing.T, ctx context.Context) *Service
		Input    *Request
		Expected Expectation
	}{
		{
			Name: "cache_hit",
			Setup: func(t *testing.T, ctx context.Context) *Service {
				t.Helper()
				cache := NewMockCache()
				cache.Set(ctx, "key", &CachedData{Value: "cached"})
				return NewService(cache)
			},
			Input: &Request{Key: "key"},
			Expected: Expectation{
				Response: &Response{Value: "cached"},
			},
		},
		{
			Name: "cache_miss_fetches_api",
			Setup: func(t *testing.T, ctx context.Context) *Service {
				t.Helper()
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					json.NewEncoder(w).Encode(map[string]string{"value": "from_api"})
				}))
				t.Cleanup(server.Close)
				return NewService(NewMockCache(), WithBaseURL(server.URL))
			},
			Input: &Request{Key: "key"},
			Expected: Expectation{
				Response: &Response{Value: "from_api"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()

			svc := tc.Setup(t, ctx)

			var got Expectation
			resp, err := svc.Call(ctx, tc.Input)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Response = resp
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("Service.Call() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

## Protobuf Tests

```go
func TestProtoHandler(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Response *v1.MyResponse
		Err      string
	}

	tests := []struct {
		Name     string
		Request  *v1.MyRequest
		Expected Expectation
	}{
		{
			Name:    "successful_request",
			Request: &v1.MyRequest{Id: "123"},
			Expected: Expectation{
				Response: &v1.MyResponse{Status: v1.Status_STATUS_OK},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			resp, err := handler.Handle(t.Context(), tc.Request)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Response = resp
			}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("Handle() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

## Helper Function Pattern

```go
func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(server.Close)
	return server
}
```

package client

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestCodeFromError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{name: "nil error returns ok", err: nil, expected: "ok"},
		{name: "not_found", err: connect.NewError(connect.CodeNotFound, nil), expected: "not_found"},
		{name: "internal", err: connect.NewError(connect.CodeInternal, nil), expected: "internal"},
		{name: "unavailable", err: connect.NewError(connect.CodeUnavailable, nil), expected: "unavailable"},
		{name: "permission_denied", err: connect.NewError(connect.CodePermissionDenied, nil), expected: "permission_denied"},
		{name: "resource_exhausted", err: connect.NewError(connect.CodeResourceExhausted, nil), expected: "resource_exhausted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := codeFromError(tt.err)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Errorf("codeFromError mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAPICallMetrics_Registration(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	metrics := NewAPICallMetrics(registry)

	if metrics.requestsTotal == nil {
		t.Fatal("requestsTotal counter not initialized")
	}
	if metrics.requestDuration == nil {
		t.Fatal("requestDuration histogram not initialized")
	}

	// Simulate recording metrics directly
	metrics.requestsTotal.WithLabelValues("/gitpod.v1.RunnerService/GetRunner", "ok").Inc()
	metrics.requestsTotal.WithLabelValues("/gitpod.v1.RunnerService/GetRunner", "ok").Inc()
	metrics.requestsTotal.WithLabelValues("/gitpod.v1.RunnerService/GetRunner", "not_found").Inc()
	metrics.requestsTotal.WithLabelValues("/gitpod.v1.EnvironmentService/ListEnvironments", "ok").Inc()
	metrics.requestDuration.WithLabelValues("/gitpod.v1.RunnerService/GetRunner", "ok").Observe(0.05)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// Verify counter values
	got := findCounterValue(families, "gitpod_management_plane_api_requests_completed_total", "/gitpod.v1.RunnerService/GetRunner", "ok")
	if diff := cmp.Diff(float64(2), got); diff != "" {
		t.Errorf("GetRunner ok count (-want +got):\n%s", diff)
	}

	got = findCounterValue(families, "gitpod_management_plane_api_requests_completed_total", "/gitpod.v1.RunnerService/GetRunner", "not_found")
	if diff := cmp.Diff(float64(1), got); diff != "" {
		t.Errorf("GetRunner not_found count (-want +got):\n%s", diff)
	}

	got = findCounterValue(families, "gitpod_management_plane_api_requests_completed_total", "/gitpod.v1.EnvironmentService/ListEnvironments", "ok")
	if diff := cmp.Diff(float64(1), got); diff != "" {
		t.Errorf("ListEnvironments ok count (-want +got):\n%s", diff)
	}

	// Verify histogram was recorded
	histCount := findHistogramCount(families, "gitpod_management_plane_api_request_duration_seconds", "/gitpod.v1.RunnerService/GetRunner", "ok")
	if histCount != 1 {
		t.Errorf("expected 1 duration observation, got %d", histCount)
	}
}

func TestAPICallMetrics_InterceptorCreation(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	metrics := NewAPICallMetrics(registry)
	interceptor := metrics.Interceptor()

	if interceptor == nil {
		t.Fatal("Interceptor() returned nil")
	}
}

func TestAPICallMetrics_DuplicateRegistrationPanics(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	_ = NewAPICallMetrics(registry)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration, got none")
		}
	}()

	// Second registration with the same registry should panic (MustRegister)
	_ = NewAPICallMetrics(registry)
}

func findCounterValue(families []*dto.MetricFamily, name, procedure, code string) float64 {
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			var procMatch, codeMatch bool
			for _, l := range m.GetLabel() {
				if l.GetName() == "procedure" && l.GetValue() == procedure {
					procMatch = true
				}
				if l.GetName() == "code" && l.GetValue() == code {
					codeMatch = true
				}
			}
			if procMatch && codeMatch {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func findHistogramCount(families []*dto.MetricFamily, name, procedure, code string) uint64 {
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			var procMatch, codeMatch bool
			for _, l := range m.GetLabel() {
				if l.GetName() == "procedure" && l.GetValue() == procedure {
					procMatch = true
				}
				if l.GetName() == "code" && l.GetValue() == code {
					codeMatch = true
				}
			}
			if procMatch && codeMatch {
				return m.GetHistogram().GetSampleCount()
			}
		}
	}
	return 0
}

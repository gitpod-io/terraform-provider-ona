package client

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/types/known/durationpb"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
)

func TestNewDeadlineInterceptor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ctx  context.Context
		req  connect.AnyRequest
		next func(t *testing.T, ctx context.Context, ar connect.AnyRequest) (connect.AnyResponse, error)
	}{
		{
			name: "sets max 30s deadline if no deadline is set",
			ctx:  t.Context(),
			req:  connect.NewRequest(&v1.ListServicesRequest{}),
			next: func(t *testing.T, ctx context.Context, ar connect.AnyRequest) (connect.AnyResponse, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected deadline to be set")
				}
				if time.Until(deadline) > 30*time.Second {
					t.Fatalf("expected deadline to be less than or equal to 30s, got %v", time.Until(deadline))
				}
				return nil, nil
			},
		},
		{
			name: "does not change deadline if less than 30s",
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				return ctx
			}(),
			req: connect.NewRequest(&v1.ListServicesRequest{}),
			next: func(t *testing.T, ctx context.Context, ar connect.AnyRequest) (connect.AnyResponse, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected deadline to be set")
				}
				if time.Until(deadline) > 10*time.Second {
					t.Fatalf("expected deadline to be less than or equal to 10s, got %v", time.Until(deadline))
				}
				return nil, nil
			},
		},
		{
			name: "sets max 30s deadline if more than 30s",
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeout(t.Context(), 1*time.Minute)
				defer cancel()
				return ctx
			}(),
			req: connect.NewRequest(&v1.ListServicesRequest{}),
			next: func(t *testing.T, ctx context.Context, ar connect.AnyRequest) (connect.AnyResponse, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected deadline to be set")
				}
				if time.Until(deadline) > 30*time.Second {
					t.Fatalf("expected deadline to be less than or equal to 30s, got %v", time.Until(deadline))
				}
				return nil, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			interceptor := NewDeadlineInterceptor(30 * time.Second)
			_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
				return tt.next(t, ctx, request)
			})(tt.ctx, tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNewRateLimitRetryInterceptor(t *testing.T) {
	t.Parallel()
	t.Run("succeeds on first attempt", func(t *testing.T) {
		t.Parallel()
		var callCount int32
		interceptor := NewRateLimitRetryInterceptor(RateLimitRetryConfig{
			MaxRetries:    3,
			MaxRetryDelay: 10 * time.Second,
		})

		req := connect.NewRequest(&v1.ListServicesRequest{})
		_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			atomic.AddInt32(&callCount, 1)
			return connect.NewResponse(&v1.ListServicesResponse{}), nil
		})(t.Context(), req)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if atomic.LoadInt32(&callCount) != 1 {
			t.Fatalf("expected 1 call, got %d", callCount)
		}
	})

	t.Run("retries on rate limit error with header", func(t *testing.T) {
		t.Parallel()
		var callCount int32
		interceptor := NewRateLimitRetryInterceptor(RateLimitRetryConfig{
			MaxRetries:    3,
			MaxRetryDelay: 100 * time.Millisecond, // Cap for fast test
		})

		req := connect.NewRequest(&v1.ListServicesRequest{})
		_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			count := atomic.AddInt32(&callCount, 1)
			if count < 2 {
				// Return rate limit error with header (1 second, capped to 100ms)
				rateLimitErr := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
				rateLimitErr.Meta().Set(HeaderRateLimitRetryAfter, "1")
				return nil, rateLimitErr
			}
			return connect.NewResponse(&v1.ListServicesResponse{}), nil
		})(t.Context(), req)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if atomic.LoadInt32(&callCount) != 2 {
			t.Fatalf("expected 2 calls, got %d", callCount)
		}
	})

	t.Run("retries on rate limit error with RetryInfo detail", func(t *testing.T) {
		t.Parallel()
		var callCount int32
		interceptor := NewRateLimitRetryInterceptor(RateLimitRetryConfig{
			MaxRetries:    3,
			MaxRetryDelay: 10 * time.Second,
		})

		req := connect.NewRequest(&v1.ListServicesRequest{})
		_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			count := atomic.AddInt32(&callCount, 1)
			if count < 2 {
				// Return rate limit error with RetryInfo detail
				rateLimitErr := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
				detail, _ := connect.NewErrorDetail(&errdetails.RetryInfo{
					RetryDelay: durationpb.New(10 * time.Millisecond),
				})
				rateLimitErr.AddDetail(detail)
				return nil, rateLimitErr
			}
			return connect.NewResponse(&v1.ListServicesResponse{}), nil
		})(t.Context(), req)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if atomic.LoadInt32(&callCount) != 2 {
			t.Fatalf("expected 2 calls, got %d", callCount)
		}
	})

	t.Run("gives up after max retries", func(t *testing.T) {
		t.Parallel()
		var callCount int32
		interceptor := NewRateLimitRetryInterceptor(RateLimitRetryConfig{
			MaxRetries:    2,
			MaxRetryDelay: 50 * time.Millisecond, // Cap for fast test
		})

		req := connect.NewRequest(&v1.ListServicesRequest{})
		_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			atomic.AddInt32(&callCount, 1)
			// Always return rate limit error
			rateLimitErr := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
			rateLimitErr.Meta().Set(HeaderRateLimitRetryAfter, "1")
			return nil, rateLimitErr
		})(t.Context(), req)

		if err == nil {
			t.Fatal("expected error after max retries")
		}
		if connect.CodeOf(err) != connect.CodeResourceExhausted {
			t.Fatalf("expected ResourceExhausted error, got %v", connect.CodeOf(err))
		}
		// Initial attempt + 2 retries = 3 calls
		if atomic.LoadInt32(&callCount) != 3 {
			t.Fatalf("expected 3 calls (1 initial + 2 retries), got %d", callCount)
		}
	})

	t.Run("does not retry non-rate-limit errors", func(t *testing.T) {
		t.Parallel()
		var callCount int32
		interceptor := NewRateLimitRetryInterceptor(RateLimitRetryConfig{
			MaxRetries:    3,
			MaxRetryDelay: 10 * time.Second,
		})

		req := connect.NewRequest(&v1.ListServicesRequest{})
		_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			atomic.AddInt32(&callCount, 1)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal error"))
		})(t.Context(), req)

		if err == nil {
			t.Fatal("expected error")
		}
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("expected Internal error, got %v", connect.CodeOf(err))
		}
		if atomic.LoadInt32(&callCount) != 1 {
			t.Fatalf("expected 1 call (no retries for non-rate-limit errors), got %d", callCount)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()
		var callCount int32
		interceptor := NewRateLimitRetryInterceptor(RateLimitRetryConfig{
			MaxRetries:    3,
			MaxRetryDelay: 10 * time.Second,
		})

		ctx, cancel := context.WithCancel(t.Context())
		req := connect.NewRequest(&v1.ListServicesRequest{})

		_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			count := atomic.AddInt32(&callCount, 1)
			if count == 1 {
				// Cancel context after first call
				cancel()
				// Return rate limit error to trigger retry
				rateLimitErr := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
				rateLimitErr.Meta().Set(HeaderRateLimitRetryAfter, "5") // 5 seconds - should be interrupted
				return nil, rateLimitErr
			}
			return connect.NewResponse(&v1.ListServicesResponse{}), nil
		})(ctx, req)

		if err != context.Canceled {
			t.Fatalf("expected context.Canceled error, got %v", err)
		}
		if atomic.LoadInt32(&callCount) != 1 {
			t.Fatalf("expected 1 call (cancelled before retry), got %d", callCount)
		}
	})

	t.Run("caps retry delay at MaxRetryDelay", func(t *testing.T) {
		t.Parallel()
		var callCount int32
		var retryStartTime time.Time
		interceptor := NewRateLimitRetryInterceptor(RateLimitRetryConfig{
			MaxRetries:    1,
			MaxRetryDelay: 50 * time.Millisecond, // Cap at 50ms
		})

		req := connect.NewRequest(&v1.ListServicesRequest{})
		_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			count := atomic.AddInt32(&callCount, 1)
			if count == 1 {
				retryStartTime = time.Now()
				// Request 10 second delay, but should be capped at 50ms
				rateLimitErr := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
				rateLimitErr.Meta().Set(HeaderRateLimitRetryAfter, "10")
				return nil, rateLimitErr
			}
			return connect.NewResponse(&v1.ListServicesResponse{}), nil
		})(t.Context(), req)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		elapsed := time.Since(retryStartTime)
		// Should have waited ~50ms (capped), not 10 seconds
		if elapsed > 200*time.Millisecond {
			t.Fatalf("retry delay was not capped, elapsed: %v", elapsed)
		}
	})

	t.Run("uses default 1 second delay when no delay specified", func(t *testing.T) {
		t.Parallel()
		var callCount int32
		var retryStartTime time.Time
		interceptor := NewRateLimitRetryInterceptor(RateLimitRetryConfig{
			MaxRetries:    1,
			MaxRetryDelay: 100 * time.Millisecond, // Cap at 100ms for fast test
		})

		req := connect.NewRequest(&v1.ListServicesRequest{})
		_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			count := atomic.AddInt32(&callCount, 1)
			if count == 1 {
				retryStartTime = time.Now()
				// No retry delay specified - should use default 1s, capped at 100ms
				return nil, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
			}
			return connect.NewResponse(&v1.ListServicesResponse{}), nil
		})(t.Context(), req)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		elapsed := time.Since(retryStartTime)
		// Should have waited ~100ms (default 1s capped at MaxRetryDelay)
		if elapsed < 50*time.Millisecond || elapsed > 200*time.Millisecond {
			t.Fatalf("unexpected retry delay, elapsed: %v", elapsed)
		}
	})
}

func TestNewTransientRetryInterceptor(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		CallCount int32
		Err       string
	}

	tests := []struct {
		Name     string
		Config   TransientRetryConfig
		RPCFunc  func(callCount int32) (connect.AnyResponse, error)
		Expected Expectation
	}{
		{
			Name: "succeeds_on_first_attempt",
			Config: TransientRetryConfig{
				MaxRetries:     3,
				InitialBackoff: time.Millisecond,
				MaxBackoff:     10 * time.Millisecond,
			},
			RPCFunc: func(callCount int32) (connect.AnyResponse, error) {
				return connect.NewResponse(&v1.ListServicesResponse{}), nil
			},
			Expected: Expectation{CallCount: 1},
		},
		{
			Name: "retries_on_unavailable_and_succeeds",
			Config: TransientRetryConfig{
				MaxRetries:     3,
				InitialBackoff: time.Millisecond,
				MaxBackoff:     10 * time.Millisecond,
			},
			RPCFunc: func(callCount int32) (connect.AnyResponse, error) {
				if callCount < 3 {
					return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("connection reset by peer"))
				}
				return connect.NewResponse(&v1.ListServicesResponse{}), nil
			},
			Expected: Expectation{CallCount: 3},
		},
		{
			Name: "gives_up_after_max_retries",
			Config: TransientRetryConfig{
				MaxRetries:     2,
				InitialBackoff: time.Millisecond,
				MaxBackoff:     10 * time.Millisecond,
			},
			RPCFunc: func(callCount int32) (connect.AnyResponse, error) {
				return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("connection reset"))
			},
			// 1 initial + 2 retries = 3
			Expected: Expectation{CallCount: 3, Err: "unavailable: connection reset"},
		},
		{
			Name: "does_not_retry_non_transient_errors",
			Config: TransientRetryConfig{
				MaxRetries:     3,
				InitialBackoff: time.Millisecond,
				MaxBackoff:     10 * time.Millisecond,
			},
			RPCFunc: func(callCount int32) (connect.AnyResponse, error) {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("internal error"))
			},
			Expected: Expectation{CallCount: 1, Err: "internal: internal error"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var callCount int32
			interceptor := NewTransientRetryInterceptor(tc.Config)

			req := connect.NewRequest(&v1.ListServicesRequest{})
			_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
				count := atomic.AddInt32(&callCount, 1)
				return tc.RPCFunc(count)
			})(t.Context(), req)

			var got Expectation
			got.CallCount = atomic.LoadInt32(&callCount)
			if err != nil {
				got.Err = err.Error()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("NewTransientRetryInterceptor() mismatch (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("respects_context_cancellation", func(t *testing.T) {
		t.Parallel()

		var callCount int32
		interceptor := NewTransientRetryInterceptor(TransientRetryConfig{
			MaxRetries:     3,
			InitialBackoff: 5 * time.Second,
			MaxBackoff:     10 * time.Second,
		})

		ctx, cancel := context.WithCancel(t.Context())
		req := connect.NewRequest(&v1.ListServicesRequest{})

		_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			count := atomic.AddInt32(&callCount, 1)
			if count == 1 {
				cancel()
				return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("connection reset"))
			}
			return connect.NewResponse(&v1.ListServicesResponse{}), nil
		})(ctx, req)

		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if atomic.LoadInt32(&callCount) != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}
	})

	t.Run("backoff_is_capped_at_max", func(t *testing.T) {
		t.Parallel()

		var callCount int32
		interceptor := NewTransientRetryInterceptor(TransientRetryConfig{
			MaxRetries:     3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     2 * time.Millisecond,
		})

		start := time.Now()
		req := connect.NewRequest(&v1.ListServicesRequest{})
		_, err := interceptor(func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			count := atomic.AddInt32(&callCount, 1)
			if count <= 3 {
				return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("connection reset"))
			}
			return connect.NewResponse(&v1.ListServicesResponse{}), nil
		})(t.Context(), req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		elapsed := time.Since(start)
		// 3 retries with backoff capped at 2ms: 1ms + 2ms + 2ms = 5ms max
		if elapsed > 200*time.Millisecond {
			t.Errorf("backoff was not capped, elapsed: %v", elapsed)
		}
	})
}

func TestIsTransientError(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result bool
	}

	tests := []struct {
		Name     string
		Err      error
		Expected Expectation
	}{
		{
			Name:     "unavailable_error",
			Err:      connect.NewError(connect.CodeUnavailable, fmt.Errorf("connection reset")),
			Expected: Expectation{Result: true},
		},
		{
			Name:     "internal_error",
			Err:      connect.NewError(connect.CodeInternal, fmt.Errorf("internal error")),
			Expected: Expectation{Result: false},
		},
		{
			Name:     "resource_exhausted_error",
			Err:      connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited")),
			Expected: Expectation{Result: false},
		},
		{
			Name:     "nil_error",
			Err:      nil,
			Expected: Expectation{Result: false},
		},
		{
			Name:     "non_connect_error",
			Err:      fmt.Errorf("some error"),
			Expected: Expectation{Result: false},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{Result: isTransientError(tc.Err)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("isTransientError() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsRateLimitError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "ResourceExhausted error",
			err:      connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited")),
			expected: true,
		},
		{
			name:     "Internal error",
			err:      connect.NewError(connect.CodeInternal, fmt.Errorf("internal error")),
			expected: false,
		},
		{
			name:     "Unavailable error",
			err:      connect.NewError(connect.CodeUnavailable, fmt.Errorf("unavailable")),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "non-connect error",
			err:      fmt.Errorf("some error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isRateLimitError(tt.err)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractRetryDelay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected time.Duration
	}{
		{
			name: "extracts delay from header",
			err: func() error {
				err := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
				err.Meta().Set(HeaderRateLimitRetryAfter, "5")
				return err
			}(),
			expected: 5 * time.Second,
		},
		{
			name: "extracts delay from RetryInfo detail",
			err: func() error {
				err := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
				detail, _ := connect.NewErrorDetail(&errdetails.RetryInfo{
					RetryDelay: durationpb.New(3 * time.Second),
				})
				err.AddDetail(detail)
				return err
			}(),
			expected: 3 * time.Second,
		},
		{
			name: "detail takes precedence over header",
			err: func() error {
				err := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
				err.Meta().Set(HeaderRateLimitRetryAfter, "10")
				detail, _ := connect.NewErrorDetail(&errdetails.RetryInfo{
					RetryDelay: durationpb.New(3 * time.Second),
				})
				err.AddDetail(detail)
				return err
			}(),
			expected: 3 * time.Second,
		},
		{
			name:     "returns 0 for non-connect error",
			err:      fmt.Errorf("some error"),
			expected: 0,
		},
		{
			name:     "returns 0 for nil error",
			err:      nil,
			expected: 0,
		},
		{
			name: "returns 0 for error without delay info",
			err: func() error {
				return connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
			}(),
			expected: 0,
		},
		{
			name: "returns 0 for invalid header value",
			err: func() error {
				err := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
				err.Meta().Set(HeaderRateLimitRetryAfter, "invalid")
				return err
			}(),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractRetryDelay(tt.err)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

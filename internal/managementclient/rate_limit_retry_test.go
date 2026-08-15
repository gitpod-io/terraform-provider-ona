// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package managementclient

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestRateLimitRetryInterceptor(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Calls           int
		Waits           []time.Duration
		ResponseAttempt string
		Err             string
	}
	tests := []struct {
		Name              string
		Config            RateLimitRetryConfig
		Errors            []error
		CancelOnCall      int
		UseProductionWait bool
		Expected          Expectation
	}{
		{
			Name:     "retry_info_then_success",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithRetryInfo(durationpb.New(2*time.Second), "10"), nil},
			Expected: Expectation{Calls: 2, Waits: []time.Duration{2 * time.Second}, ResponseAttempt: "2"},
		},
		{
			Name:     "header_fallback_then_success",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithHeader("4"), nil},
			Expected: Expectation{Calls: 2, Waits: []time.Duration{4 * time.Second}, ResponseAttempt: "2"},
		},
		{
			Name:     "server_delay_is_capped",
			Config:   RateLimitRetryConfig{MaxRetries: 1, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithRetryInfo(durationpb.New(time.Minute), "1"), nil},
			Expected: Expectation{Calls: 2, Waits: []time.Duration{30 * time.Second}, ResponseAttempt: "2"},
		},
		{
			Name:     "retry_budget_is_exhausted",
			Config:   RateLimitRetryConfig{MaxRetries: 2, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithHeader("1"), rateLimitErrorWithHeader("1"), rateLimitErrorWithHeader("1")},
			Expected: Expectation{Calls: 3, Waits: []time.Duration{time.Second, time.Second}, ResponseAttempt: "3", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "zero_retries_disables_retrying",
			Config:   RateLimitRetryConfig{MaxRetries: 0, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithHeader("1")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "retry_info_without_delay_suppresses_header",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithRetryInfo(nil, "1")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "zero_retry_info_suppresses_header",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithRetryInfo(durationpb.New(0), "1")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "negative_retry_info_suppresses_header",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithRetryInfo(durationpb.New(-time.Second), "1")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "invalid_retry_info_suppresses_header",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithRetryInfo(&durationpb.Duration{Seconds: 315_576_000_001}, "1")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "undecodable_retry_info_suppresses_header",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithMalformedRetryInfo("1")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "missing_metadata_is_not_retried",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("not a rate limit"))},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: not a rate limit"},
		},
		{
			Name:     "malformed_header_is_not_retried",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithHeader("invalid")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "zero_header_is_not_retried",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithHeader("0")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "negative_header_is_not_retried",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithHeader("-1")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "overflowing_header_is_not_retried",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{rateLimitErrorWithHeader("9223372037")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "resource_exhausted: rate limited"},
		},
		{
			Name:     "other_connect_code_is_not_retried",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{connect.NewError(connect.CodeUnavailable, fmt.Errorf("unavailable"))},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "unavailable: unavailable"},
		},
		{
			Name:     "non_connect_error_is_not_retried",
			Config:   RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{fmt.Errorf("plain error")},
			Expected: Expectation{Calls: 1, ResponseAttempt: "1", Err: "plain error"},
		},
		{
			Name:     "wrapped_rate_limit_error_is_retried",
			Config:   RateLimitRetryConfig{MaxRetries: 1, MaxRetryDelay: 30 * time.Second},
			Errors:   []error{fmt.Errorf("wrapped: %w", rateLimitErrorWithHeader("1")), nil},
			Expected: Expectation{Calls: 2, Waits: []time.Duration{time.Second}, ResponseAttempt: "2"},
		},
		{
			Name:              "cancellation_interrupts_wait",
			Config:            RateLimitRetryConfig{MaxRetries: 5, MaxRetryDelay: 30 * time.Second},
			Errors:            []error{rateLimitErrorWithHeader("30")},
			CancelOnCall:      1,
			UseProductionWait: true,
			Expected:          Expectation{Calls: 1, Waits: []time.Duration{30 * time.Second}, Err: "context canceled"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			var cancel context.CancelFunc
			if tc.CancelOnCall > 0 {
				ctx, cancel = context.WithCancel(ctx)
				t.Cleanup(cancel)
			}

			var calls int
			var waits []time.Duration
			wait := func(ctx context.Context, delay time.Duration) error {
				waits = append(waits, delay)
				if tc.UseProductionWait {
					return waitForRetry(ctx, delay)
				}
				return nil
			}
			interceptor := newRateLimitRetryInterceptor(tc.Config, wait)
			request := connect.NewRequest(&v1.ListProjectsRequest{})
			response, err := interceptor(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
				calls++
				response := connect.NewResponse(&v1.ListProjectsResponse{})
				response.Header().Set("test-attempt", strconv.Itoa(calls))
				if calls == tc.CancelOnCall {
					cancel()
				}
				if calls <= len(tc.Errors) {
					return response, tc.Errors[calls-1]
				}
				return response, nil
			})(ctx, request)

			got := Expectation{Calls: calls, Waits: waits}
			if response != nil {
				got.ResponseAttempt = response.Header().Get("test-attempt")
			}
			if err != nil {
				got.Err = err.Error()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("rate-limit retry mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRateLimitRetryInterceptorConcurrentInvocations(t *testing.T) {
	t.Parallel()

	interceptor := newRateLimitRetryInterceptor(
		RateLimitRetryConfig{MaxRetries: 1, MaxRetryDelay: 30 * time.Second},
		func(context.Context, time.Duration) error { return nil },
	)

	var mu sync.Mutex
	calls := map[string]int{}
	wrapped := interceptor(func(_ context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		id := request.Header().Get("test-id")
		mu.Lock()
		calls[id]++
		call := calls[id]
		mu.Unlock()
		if call == 1 {
			return nil, rateLimitErrorWithHeader("1")
		}
		return connect.NewResponse(&v1.ListProjectsResponse{}), nil
	})

	results := make([]string, 2)
	var wg sync.WaitGroup
	for i, id := range []string{"first", "second"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			request := connect.NewRequest(&v1.ListProjectsRequest{})
			request.Header().Set("test-id", id)
			_, err := wrapped(t.Context(), request)
			if err != nil {
				results[i] = err.Error()
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	gotCalls := map[string]int{"first": calls["first"], "second": calls["second"]}
	mu.Unlock()
	got := struct {
		Calls  map[string]int
		Errors []string
	}{Calls: gotCalls, Errors: results}
	expected := struct {
		Calls  map[string]int
		Errors []string
	}{Calls: map[string]int{"first": 2, "second": 2}, Errors: []string{"", ""}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("concurrent retry mismatch (-want +got):\n%s", diff)
	}
}

func rateLimitErrorWithHeader(retryAfter string) error {
	err := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
	err.Meta().Set(headerRateLimitRetryAfter, retryAfter)
	return err
}

func rateLimitErrorWithRetryInfo(delay *durationpb.Duration, retryAfter string) error {
	err := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
	detail, detailErr := connect.NewErrorDetail(&errdetails.RetryInfo{RetryDelay: delay})
	if detailErr != nil {
		panic(detailErr)
	}
	err.AddDetail(detail)
	err.Meta().Set(headerRateLimitRetryAfter, retryAfter)
	return err
}

func rateLimitErrorWithMalformedRetryInfo(retryAfter string) error {
	err := connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("rate limited"))
	detail, detailErr := connect.NewErrorDetail(&anypb.Any{
		TypeUrl: "type.googleapis.com/" + retryInfoType,
		Value:   []byte{0xff},
	})
	if detailErr != nil {
		panic(detailErr)
	}
	err.AddDetail(detail)
	err.Meta().Set(headerRateLimitRetryAfter, retryAfter)
	return err
}

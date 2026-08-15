// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package managementclient

import (
	"context"
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

const (
	headerRateLimitRetryAfter = "RateLimit-RetryAfter"
	retryInfoType             = "google.rpc.RetryInfo"
)

// RateLimitRetryConfig configures retries for management API rate-limit rejections.
type RateLimitRetryConfig struct {
	MaxRetries    int64
	MaxRetryDelay time.Duration
}

type retryWaitFunc func(context.Context, time.Duration) error

// NewRateLimitRetryInterceptor retries unary requests rejected by Ona's rate limiter.
func NewRateLimitRetryInterceptor(config RateLimitRetryConfig) connect.UnaryInterceptorFunc {
	return newRateLimitRetryInterceptor(config, waitForRetry)
}

func newRateLimitRetryInterceptor(config RateLimitRetryConfig, wait retryWaitFunc) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			response, err := next(ctx, request)
			for range config.MaxRetries {
				delay, ok := rateLimitRetryDelay(err)
				if !ok {
					return response, err
				}
				if delay > config.MaxRetryDelay {
					delay = config.MaxRetryDelay
				}
				if err := wait(ctx, delay); err != nil {
					return nil, err
				}
				response, err = next(ctx, request)
				if err == nil {
					return response, nil
				}
			}
			return response, err
		}
	}
}

func rateLimitRetryDelay(err error) (time.Duration, bool) {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeResourceExhausted {
		return 0, false
	}

	retryInfoPresent := false
	for _, detail := range connectErr.Details() {
		if detail.Type() != retryInfoType {
			continue
		}
		retryInfoPresent = true
		message, valueErr := detail.Value()
		if valueErr != nil {
			continue
		}
		retryInfo, ok := message.(*errdetails.RetryInfo)
		if !ok || retryInfo.RetryDelay == nil || retryInfo.RetryDelay.CheckValid() != nil {
			continue
		}
		delay := retryInfo.RetryDelay.AsDuration()
		if delay > 0 {
			return delay, true
		}
	}
	if retryInfoPresent {
		return 0, false
	}

	retryAfter := connectErr.Meta().Get(headerRateLimitRetryAfter)
	seconds, parseErr := strconv.ParseInt(retryAfter, 10, 64)
	const maxDuration = int64(1<<63 - 1)
	if parseErr != nil || seconds <= 0 || seconds > maxDuration/int64(time.Second) {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

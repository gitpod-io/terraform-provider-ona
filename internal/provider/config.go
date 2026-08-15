// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	defaultHost                   = "https://app.gitpod.io"
	defaultRateLimitMaxRetries    = int64(5)
	defaultRateLimitMaxRetryDelay = 30 * time.Second
)

var errMissingToken = errors.New("missing Ona token: set provider token or ONA_TOKEN")

func newManagementPlane(host, token, userAgent string, retryConfig managementclient.RateLimitRetryConfig) (*managementclient.ManagementPlane, string, error) {
	if strings.TrimSpace(host) == "" {
		host = os.Getenv("ONA_HOST")
	}
	if strings.TrimSpace(host) == "" {
		host = defaultHost
	}
	host = strings.TrimSpace(host)
	if strings.TrimSpace(token) == "" {
		token = os.Getenv("ONA_TOKEN")
	}
	if strings.TrimSpace(token) == "" {
		return nil, "", errMissingToken
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return nil, "", fmt.Errorf("parse Ona host: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return nil, "", fmt.Errorf("invalid Ona host %q", host)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	baseURL := parsed.String()
	client, err := managementclient.New(
		baseURL,
		managementclient.WithAccessToken(token),
		managementclient.WithUserAgent(userAgent),
		managementclient.WithInterceptor(managementclient.NewRateLimitRetryInterceptor(retryConfig)),
	)
	if err != nil {
		return nil, "", fmt.Errorf("create Ona management client: %w", err)
	}
	return client, baseURL, nil
}

func configuredRateLimitMaxRetries(value types.Int64) (int64, error) {
	if value.IsNull() {
		return defaultRateLimitMaxRetries, nil
	}
	if value.IsUnknown() {
		return 0, errors.New("rate_limit_max_retries must be known")
	}
	if value.ValueInt64() < 0 {
		return 0, errors.New("rate_limit_max_retries must be greater than or equal to 0")
	}
	return value.ValueInt64(), nil
}

func configuredRateLimitMaxRetryDelay(value types.String) (time.Duration, error) {
	if value.IsNull() {
		return defaultRateLimitMaxRetryDelay, nil
	}
	if value.IsUnknown() {
		return 0, errors.New("rate_limit_max_retry_delay must be known")
	}
	return parsePositiveDuration(value.ValueString())
}

func parsePositiveDuration(value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("rate_limit_max_retry_delay must be a valid Go duration: %w", err)
	}
	if duration <= 0 {
		return 0, errors.New("rate_limit_max_retry_delay must be greater than 0")
	}
	return duration, nil
}

type positiveDurationValidator struct{}

func (positiveDurationValidator) Description(context.Context) string {
	return "value must be a valid, positive Go duration"
}

func (positiveDurationValidator) MarkdownDescription(ctx context.Context) string {
	return positiveDurationValidator{}.Description(ctx)
}

func (positiveDurationValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := parsePositiveDuration(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Rate Limit Retry Delay", err.Error())
	}
}

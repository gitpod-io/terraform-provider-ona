package replayproxy

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

var ignoredHeaderNames = map[string]struct{}{
	"accept":                   {},
	"accept-encoding":          {},
	"accept-language":          {},
	"authorization":            {},
	"baggage":                  {},
	"connection":               {},
	"content-length":           {},
	"content-type":             {},
	"connect-accept-encoding":  {},
	"connect-protocol-version": {},
	"connect-timeout-ms":       {},
	"date":                     {},
	"te":                       {},
	"trailer":                  {},
	"traceparent":              {},
	"tracestate":               {},
	"user-agent":               {},
	"x-ona-user-agent":         {},
	"x-forwarded-for":          {},
	"x-forwarded-host":         {},
	"x-forwarded-proto":        {},
	"x-request-id":             {},
	"x-real-ip":                {},
}

var ignoredHeaderPrefixes = []string{
	"grpc-",
	"sec-fetch-",
	"sec-ch-",
}

func normalizedHeaders(h http.Header) map[string]string {
	result := map[string]string{}
	for name, values := range h {
		key := strings.ToLower(name)
		if shouldIgnoreHeader(key) {
			continue
		}
		clean := make([]string, 0, len(values))
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" {
				clean = append(clean, value)
			}
		}
		if len(clean) == 0 {
			continue
		}
		slices.Sort(clean)
		result[key] = strings.Join(clean, ",")
	}
	return result
}

func shouldIgnoreHeader(key string) bool {
	if _, ok := ignoredHeaderNames[key]; ok {
		return true
	}
	return slices.ContainsFunc(ignoredHeaderPrefixes, func(prefix string) bool {
		return strings.HasPrefix(key, prefix)
	})
}

func validateSDKUserAgent(h http.Header, language string) (string, string, error) {
	userAgent := strings.TrimSpace(h.Get("User-Agent"))
	header := "User-Agent"
	if userAgent == "" {
		userAgent = strings.TrimSpace(h.Get("X-Ona-User-Agent"))
		header = "X-Ona-User-Agent"
	}
	if userAgent == "" {
		return "", "", fmt.Errorf("missing User-Agent or X-Ona-User-Agent")
	}

	product, expectedLanguage, err := userAgentExpectation(language)
	if err != nil {
		return "", "", err
	}
	pattern := fmt.Sprintf(`(?:^|\s)%s/[^\s]+ \(language=%s; layer=sdk\)(?:\s|$)`, regexp.QuoteMeta(product), regexp.QuoteMeta(expectedLanguage))
	if ok, err := regexp.MatchString(pattern, userAgent); err != nil {
		return "", "", fmt.Errorf("compile user-agent pattern: %w", err)
	} else if !ok {
		return "", "", fmt.Errorf("User-Agent %q does not include %s SDK token for language %s layer sdk", userAgent, product, expectedLanguage)
	}
	return userAgent, header, nil
}

func userAgentExpectation(language string) (product string, expectedLanguage string, err error) {
	switch language {
	case LanguageGo:
		return "ona-go-sdk", "go", nil
	case LanguageTypeScript:
		return "ona-ts-sdk", "typescript", nil
	case LanguagePython:
		return "ona-python-sdk", "python", nil
	default:
		return "", "", fmt.Errorf("unsupported SDK language %q", language)
	}
}

func validateProtocolHeaders(r *http.Request, trafficClass string) error {
	if trafficClass == TrafficClassAgentLiveStream {
		if r.Method != http.MethodGet {
			return fmt.Errorf("agent live stream uses %s, want GET", r.Method)
		}
		if !strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
			return fmt.Errorf("agent live stream missing Accept: text/event-stream")
		}
		return nil
	}

	if r.Method != http.MethodPost {
		return fmt.Errorf("connect request uses %s, want POST", r.Method)
	}
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "proto") {
		return fmt.Errorf("connect request Content-Type %q does not use protobuf", r.Header.Get("Content-Type"))
	}
	return nil
}

func copyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func responseHeadersForFixture(h http.Header) map[string]string {
	result := map[string]string{}
	for name, values := range h {
		key := strings.ToLower(name)
		if shouldIgnoreResponseHeader(key) {
			continue
		}
		clean := make([]string, 0, len(values))
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" {
				clean = append(clean, value)
			}
		}
		if len(clean) == 0 {
			continue
		}
		slices.Sort(clean)
		result[key] = strings.Join(clean, ",")
	}
	return result
}

func shouldIgnoreResponseHeader(key string) bool {
	switch key {
	case "accept-encoding",
		"authorization",
		"baggage",
		"connection",
		"content-length",
		"date",
		"traceparent",
		"tracestate",
		"x-request-id":
		return true
	default:
		return slices.ContainsFunc(ignoredHeaderPrefixes, func(prefix string) bool {
			return strings.HasPrefix(key, prefix)
		})
	}
}

func responseHeadersForClient(h http.Header, bodyLen int) http.Header {
	out := http.Header{}
	for key, values := range h {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			out.Add(key, value)
		}
	}
	if bodyLen >= 0 {
		out.Set("Content-Length", fmt.Sprintf("%d", bodyLen))
	}
	return out
}

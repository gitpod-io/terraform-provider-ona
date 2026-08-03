package replayproxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	supervisorconnect "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/supervisor/v1/v1connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestValidateSDKUserAgent(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		UserAgent string
		Header    string
		Err       string
	}

	tests := []struct {
		Name     string
		Headers  http.Header
		Language string
		Expected Expectation
	}{
		{
			Name: "go_sdk_user_agent",
			Headers: http.Header{
				"User-Agent": []string{"my-app/1.0 ona-go-sdk/dev (language=go; layer=sdk)"},
			},
			Language: LanguageGo,
			Expected: Expectation{
				UserAgent: "my-app/1.0 ona-go-sdk/dev (language=go; layer=sdk)",
				Header:    "User-Agent",
			},
		},
		{
			Name: "typescript_sdk_x_ona_user_agent",
			Headers: http.Header{
				"X-Ona-User-Agent": []string{"ona-ts-sdk/0.0.1 (language=typescript; layer=sdk)"},
			},
			Language: LanguageTypeScript,
			Expected: Expectation{
				UserAgent: "ona-ts-sdk/0.0.1 (language=typescript; layer=sdk)",
				Header:    "X-Ona-User-Agent",
			},
		},
		{
			Name: "raw_layer_rejected",
			Headers: http.Header{
				"User-Agent": []string{"ona-go-client/dev (language=go; layer=raw)"},
			},
			Language: LanguageGo,
			Expected: Expectation{
				Err: `User-Agent "ona-go-client/dev (language=go; layer=raw)" does not include ona-go-sdk SDK token for language go layer sdk`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			userAgent, header, err := validateSDKUserAgent(tc.Headers, tc.Language)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.UserAgent = userAgent
				got.Header = header
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateSDKUserAgent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNormalizedHeadersIgnoresRuntimeAndProtocolVariance(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Headers map[string]string
	}

	got := Expectation{
		Headers: normalizedHeaders(http.Header{
			"Accept":                   []string{"application/connect+proto"},
			"Authorization":            []string{"Bearer secret"},
			"Accept-Language":          []string{"*"},
			"Connect-Accept-Encoding":  []string{"gzip"},
			"Connect-Protocol-Version": []string{"1"},
			"Content-Type":             []string{"application/proto"},
			"Traceparent":              []string{"00-abc-def-01"},
			"User-Agent":               []string{"ona-go-sdk/dev (language=go; layer=sdk)"},
			"X-Stable":                 []string{"value"},
		}),
	}
	expected := Expectation{
		Headers: map[string]string{
			"x-stable": "value",
		},
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("normalizedHeaders() mismatch (-want +got):\n%s", diff)
	}
}

func TestResponseHeadersForFixtureKeepsContentType(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Headers map[string]string
	}

	got := Expectation{
		Headers: responseHeadersForFixture(http.Header{
			"Content-Type":   []string{"application/proto"},
			"Content-Length": []string{"123"},
			"Date":           []string{"Mon, 13 Jul 2026 17:00:00 GMT"},
			"X-Request-Id":   []string{"request-id"},
		}),
	}
	expected := Expectation{
		Headers: map[string]string{
			"content-type": "application/proto",
		},
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("responseHeadersForFixture() mismatch (-want +got):\n%s", diff)
	}
}

func TestEnsureConnectEndStream(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Body []byte
	}

	message := []byte{0x00, 0, 0, 0, 3, 'a', 'b', 'c'}
	got := Expectation{Body: ensureConnectEndStream(message)}
	expected := Expectation{Body: []byte{0x00, 0, 0, 0, 3, 'a', 'b', 'c', 0x02, 0, 0, 0, 2, '{', '}'}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ensureConnectEndStream() mismatch (-want +got):\n%s", diff)
	}

	got = Expectation{Body: ensureConnectEndStream(expected.Body)}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ensureConnectEndStream() should be idempotent (-want +got):\n%s", diff)
	}
}

func TestNormalizeResponseBodyRedactsTokensAndRewritesURLs(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Fixture *v1.GetEnvironmentResponse
		Client  *v1.GetEnvironmentResponse
		Tokens  map[string]string
	}

	rewriter := &fakeRewriter{
		runtimeBaseURL: "http://127.0.0.1:1234",
		tokens:         map[string]string{},
	}
	envBody, err := proto.Marshal(&v1.GetEnvironmentResponse{
		Environment: &v1.Environment{
			Id: "env-1",
			Status: &v1.EnvironmentStatus{
				EnvironmentUrls: &v1.EnvironmentStatus_EnvironmentURLs{
					Ops: "https://env.example.com/ops",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal environment response: %v", err)
	}
	tokenBody, err := proto.Marshal(&v1.CreateEnvironmentAccessTokenResponse{
		AccessToken: "real-env-token",
	})
	if err != nil {
		t.Fatalf("marshal token response: %v", err)
	}

	envBodies, err := normalizeResponseBody("/gitpod.v1.EnvironmentService/GetEnvironment", envBody, rewriter)
	if err != nil {
		t.Fatalf("normalize environment response: %v", err)
	}
	tokenBodies, err := normalizeResponseBody("/gitpod.v1.EnvironmentService/CreateEnvironmentAccessToken", tokenBody, rewriter)
	if err != nil {
		t.Fatalf("normalize token response: %v", err)
	}
	var fixture v1.GetEnvironmentResponse
	if err := proto.Unmarshal(envBodies.FixtureBody, &fixture); err != nil {
		t.Fatalf("unmarshal fixture response: %v", err)
	}
	var client v1.GetEnvironmentResponse
	if err := proto.Unmarshal(envBodies.ClientBody, &client); err != nil {
		t.Fatalf("unmarshal client response: %v", err)
	}
	var token v1.CreateEnvironmentAccessTokenResponse
	if err := proto.Unmarshal(tokenBodies.FixtureBody, &token); err != nil {
		t.Fatalf("unmarshal token response: %v", err)
	}

	got := Expectation{
		Fixture: &fixture,
		Client:  &client,
		Tokens:  rewriter.tokens,
	}
	expected := Expectation{
		Fixture: &v1.GetEnvironmentResponse{
			Environment: &v1.Environment{
				Id: "env-1",
				Status: &v1.EnvironmentStatus{
					EnvironmentUrls: &v1.EnvironmentStatus_EnvironmentURLs{
						Ops: "http://ona-replay.invalid/__replay/external/environment_ops",
					},
				},
			},
		},
		Client: &v1.GetEnvironmentResponse{
			Environment: &v1.Environment{
				Id: "env-1",
				Status: &v1.EnvironmentStatus{
					EnvironmentUrls: &v1.EnvironmentStatus_EnvironmentURLs{
						Ops: "http://127.0.0.1:1234/__replay/external/environment_ops",
					},
				},
			},
		},
		Tokens: map[string]string{
			envAccessTokenPlaceholder: "real-env-token",
		},
	}
	if token.AccessToken != envAccessTokenPlaceholder {
		t.Fatalf("token was not redacted: %q", token.AccessToken)
	}
	if diff := cmp.Diff(expected, got, protocmp.Transform()); diff != "" {
		t.Errorf("normalizeResponseBody() mismatch (-want +got):\n%s", diff)
	}
}

func TestNormalizeResponseBodyRewritesLocalDevOpsOrigin(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		ClientOpsURL string
		Origins      []string
	}

	rewriter := &fakeRewriter{
		runtimeBaseURL: "http://127.0.0.1:18100",
		tokens:         map[string]string{},
	}
	body, err := proto.Marshal(&v1.GetEnvironmentResponse{
		Environment: &v1.Environment{
			Id: "env-1",
			Status: &v1.EnvironmentStatus{
				EnvironmentUrls: &v1.EnvironmentStatus_EnvironmentURLs{
					Ops: "https://22999--env-1.localhost",
					Ssh: &v1.EnvironmentStatus_EnvironmentSSHURL{Url: "ssh://10.0.0.5:22"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal environment response: %v", err)
	}

	bodies, err := normalizeResponseBody("/gitpod.v1.EnvironmentService/GetEnvironment", body, rewriter)
	if err != nil {
		t.Fatalf("normalize environment response: %v", err)
	}
	var client v1.GetEnvironmentResponse
	if err := proto.Unmarshal(bodies.ClientBody, &client); err != nil {
		t.Fatalf("unmarshal client response: %v", err)
	}

	got := Expectation{
		ClientOpsURL: client.GetEnvironment().GetStatus().GetEnvironmentUrls().GetOps(),
		Origins:      rewriter.origins,
	}
	expected := Expectation{
		ClientOpsURL: "http://127.0.0.1:18100/__replay/external/environment_ops",
		Origins: []string{
			"http://10.0.0.5:22999",
			"http://10.0.0.5:22999",
		},
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("local dev ops rewrite mismatch (-want +got):\n%s", diff)
	}
}

func TestRecordExternalRouteOmitsLiveURLFromManifest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		ClientURL          string
		StableURL          string
		ForwardURL         string
		ManifestHasLiveURL bool
		ManifestHasOrigin  bool
		ValidateFixtureErr string
	}

	dir := t.TempDir()
	proxy, err := New(Options{
		Mode:             ModeRecord,
		FixtureDir:       dir,
		Scenario:         "unit",
		ExpectedLanguage: LanguageGo,
		UpstreamBaseURL:  "http://backend.example/api",
		PublicURL:        "http://proxy.example",
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	clientURL, err := proxy.rewriteExternalURL("https://environment.example/ops", TrafficClassEnvironmentOps, false)
	if err != nil {
		t.Fatalf("rewrite client URL: %v", err)
	}
	stableURL, err := proxy.rewriteExternalURL("https://environment.example/ops", TrafficClassEnvironmentOps, true)
	if err != nil {
		t.Fatalf("rewrite stable URL: %v", err)
	}
	requestURL, err := url.Parse(clientURL + supervisorconnect.EnvironmentOpsServiceExecProcedure + "?z=1")
	if err != nil {
		t.Fatalf("parse request URL: %v", err)
	}
	forwardURL, err := proxy.forwardURL(requestURL)
	if err != nil {
		t.Fatalf("forward URL: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var validateFixtureErr string
	if err := ValidateFixture(dir); err != nil {
		validateFixtureErr = err.Error()
	}

	got := Expectation{
		ClientURL:          clientURL,
		StableURL:          stableURL,
		ForwardURL:         forwardURL,
		ManifestHasLiveURL: strings.Contains(string(manifest), "https://environment.example/ops"),
		ManifestHasOrigin:  strings.Contains(string(manifest), "original_url"),
		ValidateFixtureErr: validateFixtureErr,
	}
	expected := Expectation{
		ClientURL:          "http://proxy.example/__replay/external/ops-1",
		StableURL:          "http://ona-replay.invalid/__replay/external/ops-1",
		ForwardURL:         "https://environment.example/ops/supervisor.v1.EnvironmentOpsService/Exec?z=1",
		ValidateFixtureErr: "",
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("external route metadata mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateFixtureRejectsDigestMismatch(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Err string
	}

	dir := recordUnitFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "0001.response.bin"), []byte("tampered-response"), 0o644); err != nil {
		t.Fatalf("tamper fixture response: %v", err)
	}

	var got Expectation
	if err := ValidateFixture(dir); err != nil {
		got.Err = err.Error()
	}
	expected := Expectation{
		Err: "interaction 0001 response body digest mismatch: got 749c501ecfebe6c94888871ba1dd2d4a982e5c752e17eb6817a2a278b6be93e3 want 5bcbe186d9720c63993ffcb48f3479a0c18ad97207a7056eda1acea730fee3e7",
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ValidateFixture() mismatch (-want +got):\n%s", diff)
	}
}

func TestProxyRecordsAndReplaysInteractionWithDifferentSDKLanguage(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		StatusCode int
		Body       string
		Remaining  int
		Err        string
	}

	dir := recordUnitFixture(t)

	replayer, err := New(Options{
		Mode:             ModeReplay,
		FixtureDir:       dir,
		ExpectedLanguage: LanguageTypeScript,
		PublicURL:        "http://proxy.example",
	})
	if err != nil {
		t.Fatalf("create replayer: %v", err)
	}
	replayServer := httptest.NewServer(replayer)
	t.Cleanup(replayServer.Close)

	replayResp, err := postProto(replayServer.URL+"/gitpod.v1.UnitService/GetThing", "ona-ts-sdk/0.0.1 (language=typescript; layer=sdk)", []byte("request"))
	if err != nil {
		t.Fatalf("replay request: %v", err)
	}
	defer func() { _ = replayResp.Body.Close() }()
	body, err := io.ReadAll(replayResp.Body)
	if err != nil {
		t.Fatalf("read replay body: %v", err)
	}

	got := Expectation{
		StatusCode: replayResp.StatusCode,
		Body:       string(body),
		Remaining:  replayer.RemainingInteractions(),
	}
	expected := Expectation{
		StatusCode: http.StatusCreated,
		Body:       "recorded-response",
		Remaining:  0,
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("replay response mismatch (-want +got):\n%s", diff)
	}

	if err := ValidateFixture(filepath.Clean(dir)); err != nil {
		t.Fatalf("validate fixture: %v", err)
	}
}

func recordUnitFixture(t *testing.T) string {
	t.Helper()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/proto")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("recorded-response"))
	}))
	t.Cleanup(upstream.Close)

	dir := t.TempDir()
	recorder, err := New(Options{
		Mode:             ModeRecord,
		FixtureDir:       dir,
		Scenario:         "unit",
		ExpectedLanguage: LanguageGo,
		UpstreamBaseURL:  upstream.URL,
		PublicURL:        "http://proxy.example",
	})
	if err != nil {
		t.Fatalf("create recorder: %v", err)
	}
	recordServer := httptest.NewServer(recorder)
	t.Cleanup(recordServer.Close)

	recordResp, err := postProto(recordServer.URL+"/gitpod.v1.UnitService/GetThing", "ona-go-sdk/dev (language=go; layer=sdk)", []byte("request"))
	if err != nil {
		t.Fatalf("record request: %v", err)
	}
	if _, err := io.Copy(io.Discard, recordResp.Body); err != nil {
		t.Fatalf("read record response: %v", err)
	}
	if err := recordResp.Body.Close(); err != nil {
		t.Fatalf("close record response: %v", err)
	}
	return dir
}

func postProto(target string, userAgent string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/proto")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Authorization", "Bearer redacted-by-fixture")
	return http.DefaultClient.Do(req)
}

type fakeRewriter struct {
	runtimeBaseURL string
	tokens         map[string]string
	origins        []string
}

func (f *fakeRewriter) rewriteExternalURL(originalURL string, trafficClass string, stable bool) (string, error) {
	f.origins = append(f.origins, originalURL)
	if stable {
		return stableReplayBaseURL + "/__replay/external/" + trafficClass, nil
	}
	return f.runtimeBaseURL + "/__replay/external/" + trafficClass, nil
}

func (f *fakeRewriter) rememberToken(placeholder string, token string) {
	f.tokens[placeholder] = token
}

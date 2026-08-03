package replayproxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
)

// Proxy records or replays SDK HTTP traffic for one fixture scenario.
type Proxy struct {
	mode             string
	expectedLanguage string
	upstreamBaseURL  string
	publicURL        string
	httpClient       *http.Client

	mu          sync.Mutex
	fixture     *fixtureStore
	nextIndex   int
	replayIndex int
	realTokens  map[string]string
	routeByURL  map[string]string
	routeOrigin map[string]string
}

// New creates a replay proxy from options.
func New(opts Options) (*Proxy, error) {
	mode := opts.Mode
	if mode == "" {
		mode = ModeReplay
	}
	if mode != ModeRecord && mode != ModeReplay {
		return nil, fmt.Errorf("unsupported mode %q", mode)
	}
	expectedLanguage := opts.ExpectedLanguage
	if expectedLanguage == "" {
		expectedLanguage = LanguageGo
	}
	if _, _, err := userAgentExpectation(expectedLanguage); err != nil {
		return nil, err
	}
	publicURL := strings.TrimRight(opts.PublicURL, "/")
	if publicURL == "" {
		publicURL = stableReplayBaseURL
	}

	var store *fixtureStore
	var err error
	if mode == ModeRecord {
		if opts.UpstreamBaseURL == "" {
			return nil, fmt.Errorf("upstream base URL is required in record mode")
		}
		store, err = newFixtureStore(opts.FixtureDir, opts.Scenario)
		if err != nil {
			return nil, err
		}
		store.manifest.SourceLanguage = expectedLanguage
		store.manifest.UpstreamBaseURL = opts.UpstreamBaseURL
		if err := store.writeManifest(); err != nil {
			return nil, err
		}
	} else {
		store, err = loadFixtureStore(opts.FixtureDir)
		if err != nil {
			return nil, err
		}
	}

	return &Proxy{
		mode:             mode,
		expectedLanguage: expectedLanguage,
		upstreamBaseURL:  strings.TrimRight(opts.UpstreamBaseURL, "/"),
		publicURL:        publicURL,
		httpClient:       http.DefaultClient,
		fixture:          store,
		nextIndex:        1,
		realTokens:       map[string]string{},
		routeByURL:       map[string]string{},
		routeOrigin:      map[string]string{},
	}, nil
}

// ServeHTTP implements http.Handler.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/__replay/status" {
		p.serveStatus(w)
		return
	}
	if err := p.serveHTTP(w, r); err != nil {
		fmt.Fprintf(os.Stderr, "replayproxy %s error: %s %s: %v\n", p.mode, r.Method, r.URL.Path, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

func (p *Proxy) serveStatus(w http.ResponseWriter) {
	status := struct {
		Mode      string `json:"mode"`
		Remaining int    `json:"remaining"`
	}{
		Mode:      p.mode,
		Remaining: p.RemainingInteractions(),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		fmt.Fprintf(os.Stderr, "replayproxy status encode error: %v\n", err)
	}
}

func (p *Proxy) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	switch p.mode {
	case ModeRecord:
		return p.record(w, r)
	case ModeReplay:
		return p.replay(w, r)
	default:
		return fmt.Errorf("unsupported mode %q", p.mode)
	}
}

func (p *Proxy) record(w http.ResponseWriter, r *http.Request) error {
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	_ = r.Body.Close()

	trafficClass := p.trafficClass(r.URL.Path, r.Header)
	if err := validateProtocolHeaders(r, trafficClass); err != nil {
		return err
	}
	userAgent, userAgentHeader, err := validateSDKUserAgent(r.Header, p.expectedLanguage)
	if err != nil {
		return err
	}
	normalizedRequestBody, err := normalizeRequestBody(r.URL.Path, requestBody)
	if err != nil {
		return err
	}

	index := p.nextInteractionIndex()
	targetURL, err := p.forwardURL(r.URL)
	if err != nil {
		return err
	}
	forwardReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("create forward request: %w", err)
	}
	copyHeaders(forwardReq.Header, r.Header)
	forwardReq.Header.Set("Accept-Encoding", "identity")
	p.translateAuthorization(forwardReq.Header)

	resp, err := p.httpClient.Do(forwardReq)
	if err != nil {
		return fmt.Errorf("forward %s %s: %w", r.Method, targetURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	meta := InteractionMetadata{
		Index:                  index,
		TrafficClass:           trafficClass,
		Method:                 r.Method,
		Path:                   r.URL.Path,
		Query:                  canonicalQuery(r.URL),
		StableRequestHeaders:   normalizedHeaders(r.Header),
		ResponseHeaders:        responseHeadersForFixture(resp.Header),
		StatusCode:             resp.StatusCode,
		RequestBodySHA256:      digestHex(normalizedRequestBody),
		RequestBodyFile:        fmt.Sprintf(defaultRequestBodyFileFormat, index),
		ResponseBodyFile:       fmt.Sprintf(defaultResponseBodyFileFormat, index),
		UserAgent:              userAgent,
		UserAgentHeader:        userAgentHeader,
		StableProtocolVerified: true,
	}
	if index == 1 {
		p.setSourceUserAgent(userAgent)
	}

	if shouldStreamResponse(r, resp) {
		return p.recordStreamingResponse(w, resp, meta, normalizedRequestBody)
	}

	responseBody, err := readForwardResponseBody(resp)
	if err != nil {
		return fmt.Errorf("read upstream response body: %w", err)
	}
	meta.ResponseHeaders = responseHeadersForFixture(resp.Header)
	bodies, err := normalizeResponseBody(r.URL.Path, responseBody, p)
	if err != nil {
		return err
	}
	meta.Redactions = append(meta.Redactions, bodies.Redactions...)
	meta.ResponseBodySHA256 = digestHex(bodies.FixtureBody)
	copyHeaders(w.Header(), responseHeadersForClient(resp.Header, len(bodies.ClientBody)))
	w.WriteHeader(resp.StatusCode)
	if _, err := w.Write(bodies.ClientBody); err != nil {
		return fmt.Errorf("write response body: %w", err)
	}

	return p.writeInteraction(meta, normalizedRequestBody, bodies.FixtureBody)
}

func (p *Proxy) recordStreamingResponse(w http.ResponseWriter, resp *http.Response, meta InteractionMetadata, normalizedRequestBody []byte) error {
	var buf bytes.Buffer
	copyHeaders(w.Header(), responseHeadersForClient(resp.Header, -1))
	w.WriteHeader(resp.StatusCode)
	_, copyErr := io.Copy(io.MultiWriter(flushWriter{w: w}, &buf), resp.Body)
	body := buf.Bytes()
	if isConnectStreamingResponse(resp) {
		body = ensureConnectEndStream(body)
	}
	if err := validateNoTokenLikeData(fmt.Sprintf("interaction %04d streaming response body", meta.Index), body); err != nil {
		return err
	}
	meta.ResponseBodySHA256 = digestHex(body)
	if err := p.writeInteraction(meta, normalizedRequestBody, body); err != nil {
		return err
	}
	if copyErr != nil && !errors.Is(copyErr, context.Canceled) {
		return fmt.Errorf("copy streaming response: %w", copyErr)
	}
	return nil
}

func isConnectStreamingResponse(resp *http.Response) bool {
	return strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/connect+proto")
}

func ensureConnectEndStream(body []byte) []byte {
	if connectStreamHasEnd(body) {
		return body
	}
	end := []byte{0x02, 0, 0, 0, 2, '{', '}'}
	out := make([]byte, 0, len(body)+len(end))
	out = append(out, body...)
	out = append(out, end...)
	return out
}

func connectStreamHasEnd(body []byte) bool {
	for len(body) >= 5 {
		flags := body[0]
		size := int(binary.BigEndian.Uint32(body[1:5]))
		if size < 0 || len(body)-5 < size {
			return false
		}
		if flags&0x02 != 0 {
			return true
		}
		body = body[5+size:]
	}
	return false
}

func readForwardResponseBody(resp *http.Response) ([]byte, error) {
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("open gzip response body: %w", err)
		}
		defer func() { _ = reader.Close() }()
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
		return io.ReadAll(reader)
	}
	return io.ReadAll(resp.Body)
}

type flushWriter struct {
	w http.ResponseWriter
}

func (w flushWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if flusher, ok := w.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}

func (p *Proxy) replay(w http.ResponseWriter, r *http.Request) error {
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	_ = r.Body.Close()

	trafficClass := p.trafficClass(r.URL.Path, r.Header)
	if err := validateProtocolHeaders(r, trafficClass); err != nil {
		return err
	}
	if _, _, err := validateSDKUserAgent(r.Header, p.expectedLanguage); err != nil {
		return err
	}
	normalizedRequestBody, err := normalizeRequestBody(r.URL.Path, requestBody)
	if err != nil {
		return err
	}
	got := InteractionMetadata{
		TrafficClass:         trafficClass,
		Method:               r.Method,
		Path:                 r.URL.Path,
		Query:                canonicalQuery(r.URL),
		StableRequestHeaders: normalizedHeaders(r.Header),
		RequestBodySHA256:    digestHex(normalizedRequestBody),
	}
	meta, responseBody, err := p.matchReplayInteraction(got)
	if err != nil {
		return err
	}

	bodies, err := normalizeResponseBody(meta.Path, responseBody, p)
	if err != nil {
		return err
	}
	headers := http.Header{}
	for key, value := range meta.ResponseHeaders {
		headers.Set(key, value)
	}
	copyHeaders(w.Header(), responseHeadersForClient(headers, len(bodies.ClientBody)))
	w.WriteHeader(meta.StatusCode)
	if _, err := w.Write(bodies.ClientBody); err != nil {
		return fmt.Errorf("write replay response: %w", err)
	}

	return nil
}

func (p *Proxy) matchReplayInteraction(got InteractionMetadata) (InteractionMetadata, []byte, error) {
	if p.replayIndex >= len(p.fixture.manifest.Interactions) {
		return InteractionMetadata{}, nil, fmt.Errorf("scenario %q unexpected %s request %s %s; fixture has no remaining interactions",
			p.fixture.manifest.Scenario, got.TrafficClass, got.Method, got.Path)
	}

	var firstErr error
	for idx := p.replayIndex; idx < len(p.fixture.manifest.Interactions) && idx < p.replayIndex+6; idx++ {
		item := p.fixture.manifest.Interactions[idx]
		meta, _, responseBody, err := p.fixture.readInteraction(item)
		if err != nil {
			return InteractionMetadata{}, nil, err
		}
		if err := compareRequest(p.fixture.manifest.Scenario, meta, got); err == nil {
			p.replayIndex = idx + 1
			return meta, responseBody, nil
		} else if firstErr == nil {
			firstErr = err
		}
		if !canSkipForLanguageVariance(meta) {
			break
		}
	}
	return InteractionMetadata{}, nil, firstErr
}

func compareRequest(scenario string, expected InteractionMetadata, got InteractionMetadata) error {
	var problems []string
	if expected.TrafficClass != got.TrafficClass {
		problems = append(problems, fmt.Sprintf("traffic class: got %s want %s", got.TrafficClass, expected.TrafficClass))
	}
	if expected.Method != got.Method {
		problems = append(problems, fmt.Sprintf("method: got %s want %s", got.Method, expected.Method))
	}
	if expected.Path != got.Path {
		problems = append(problems, fmt.Sprintf("path: got %s want %s", got.Path, expected.Path))
	}
	if expected.Query != got.Query {
		problems = append(problems, fmt.Sprintf("query: got %q want %q", got.Query, expected.Query))
	}
	if expected.RequestBodySHA256 != got.RequestBodySHA256 {
		problems = append(problems, fmt.Sprintf("request body sha256: got %s want %s", got.RequestBodySHA256, expected.RequestBodySHA256))
	}
	if diff := diffStringMap(expected.StableRequestHeaders, got.StableRequestHeaders); diff != "" {
		problems = append(problems, "stable headers: "+diff)
	}
	if len(problems) > 0 {
		return fmt.Errorf("scenario %q interaction %04d mismatch: expected %s %s%s, got %s %s%s; %s",
			scenario,
			expected.Index,
			expected.Method,
			expected.Path,
			formatQuery(expected.Query),
			got.Method,
			got.Path,
			formatQuery(got.Query),
			strings.Join(problems, "; "),
		)
	}
	return nil
}

func formatQuery(query string) string {
	if query == "" {
		return ""
	}
	return "?" + query
}

func diffStringMap(expected map[string]string, got map[string]string) string {
	for key, want := range expected {
		if got[key] != want {
			return fmt.Sprintf("%s got %q want %q", key, got[key], want)
		}
	}
	for key, value := range got {
		if _, ok := expected[key]; !ok {
			return fmt.Sprintf("unexpected %s=%q", key, value)
		}
	}
	return ""
}

func canSkipForLanguageVariance(meta InteractionMetadata) bool {
	if meta.Method != http.MethodPost || meta.TrafficClass != TrafficClassManagementPlane {
		return false
	}
	switch procedurePath(meta.Path) {
	case v1connect.AgentServiceGetAgentExecutionProcedure,
		v1connect.EnvironmentServiceGetEnvironmentProcedure:
		return true
	default:
		return false
	}
}

func (p *Proxy) nextInteractionIndex() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	index := p.nextIndex
	p.nextIndex++
	return index
}

func (p *Proxy) writeInteraction(meta InteractionMetadata, requestBody []byte, responseBody []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fixture.writeInteraction(meta, requestBody, responseBody)
}

func (p *Proxy) setSourceUserAgent(userAgent string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fixture.manifest.SourceUserAgent != "" {
		return
	}
	p.fixture.manifest.SourceUserAgent = userAgent
	_ = p.fixture.writeManifest()
}

func (p *Proxy) trafficClass(requestPath string, h http.Header) string {
	if strings.HasPrefix(requestPath, "/__replay/external/") {
		routeID, _, ok := externalRouteParts(requestPath)
		if ok {
			p.mu.Lock()
			route := p.fixture.manifest.ExternalRoutes[routeID]
			p.mu.Unlock()
			if route.Class != "" {
				return route.Class
			}
		}
	}
	if strings.Contains(strings.ToLower(h.Get("Accept")), "text/event-stream") {
		return TrafficClassAgentLiveStream
	}
	if strings.HasPrefix(procedurePath(requestPath), "/supervisor.v1.EnvironmentOpsService/") {
		return TrafficClassEnvironmentOps
	}
	return TrafficClassManagementPlane
}

func (p *Proxy) forwardURL(requestURL *url.URL) (string, error) {
	if strings.HasPrefix(requestURL.Path, "/__replay/external/") {
		routeID, suffix, ok := externalRouteParts(requestURL.Path)
		if !ok {
			return "", fmt.Errorf("invalid external replay route %s", requestURL.Path)
		}
		p.mu.Lock()
		route, ok := p.fixture.manifest.ExternalRoutes[routeID]
		original := p.routeOrigin[routeID]
		p.mu.Unlock()
		if !ok {
			return "", fmt.Errorf("unknown external replay route %s", routeID)
		}
		if original == "" {
			return "", fmt.Errorf("external replay route %s has no live origin in this recording process", routeID)
		}
		original = strings.TrimRight(original, "/")
		if route.Class == TrafficClassAgentLiveStream && suffix == "" {
			return original, nil
		}
		return original + suffix + querySuffix(requestURL), nil
	}
	return p.upstreamBaseURL + requestURL.Path + querySuffix(requestURL), nil
}

func (p *Proxy) rewriteExternalURL(originalURL string, trafficClass string, stable bool) (string, error) {
	if originalURL == "" {
		return "", nil
	}
	if strings.HasPrefix(originalURL, stableReplayBaseURL+"/__replay/external/") {
		routeID, _, ok := externalRouteParts(strings.TrimPrefix(originalURL, stableReplayBaseURL))
		if !ok {
			return "", fmt.Errorf("invalid stable replay URL %s", originalURL)
		}
		base := p.publicURL
		if stable {
			base = stableReplayBaseURL
		}
		return strings.TrimRight(base, "/") + "/__replay/external/" + routeID, nil
	}
	if p.mode == ModeReplay {
		return originalURL, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	routeID, ok := p.routeByURL[originalURL]
	if !ok {
		prefix := "external"
		switch trafficClass {
		case TrafficClassEnvironmentOps:
			prefix = "ops"
		case TrafficClassAgentLiveStream:
			prefix = "agent-live"
		}
		routeID = fmt.Sprintf("%s-%d", prefix, len(p.routeByURL)+1)
		p.routeByURL[originalURL] = routeID
		if p.fixture.manifest.ExternalRoutes == nil {
			p.fixture.manifest.ExternalRoutes = map[string]ExternalRoute{}
		}
		p.fixture.manifest.ExternalRoutes[routeID] = ExternalRoute{
			ID:    routeID,
			Class: trafficClass,
		}
		_ = p.fixture.writeManifest()
	}
	p.routeOrigin[routeID] = originalURL
	base := p.publicURL
	if stable {
		base = stableReplayBaseURL
	}
	return strings.TrimRight(base, "/") + "/__replay/external/" + routeID, nil
}

func (p *Proxy) rememberToken(placeholder string, token string) {
	if token == "" || token == placeholder {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.realTokens[placeholder] = token
}

func (p *Proxy) translateAuthorization(h http.Header) {
	value := h.Get("Authorization")
	if value == "" {
		return
	}
	kind, token, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(kind, "Bearer") {
		return
	}
	p.mu.Lock()
	realToken := p.realTokens[token]
	p.mu.Unlock()
	if realToken != "" {
		h.Set("Authorization", "Bearer "+realToken)
	}
}

func shouldStreamResponse(r *http.Request, resp *http.Response) bool {
	if r.Method == http.MethodGet && strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		return true
	}
	if isEventStreamProcedure(r.URL.Path) {
		return true
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	return strings.Contains(contentType, "text/event-stream")
}

func externalRouteParts(requestPath string) (routeID string, suffix string, ok bool) {
	rest := strings.TrimPrefix(requestPath, "/__replay/external/")
	if rest == requestPath || rest == "" {
		return "", "", false
	}
	routeID, suffix, _ = strings.Cut(rest, "/")
	if routeID == "" {
		return "", "", false
	}
	if suffix != "" {
		suffix = "/" + suffix
	}
	return routeID, suffix, true
}

func querySuffix(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}
	return "?" + u.RawQuery
}

func canonicalQuery(u *url.URL) string {
	if u == nil || u.RawQuery == "" {
		return ""
	}
	values := u.Query()
	return values.Encode()
}

func digestHex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// RemainingInteractions reports how many fixture interactions were not consumed in replay mode.
func (p *Proxy) RemainingInteractions() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.fixture.manifest.Interactions) - p.replayIndex
}

// ValidateFixture checks the fixture for obvious secret leaks.
func ValidateFixture(fixtureDir string) error {
	store, err := loadFixtureStore(fixtureDir)
	if err != nil {
		return err
	}
	manifestData, err := os.ReadFile(filepath.Join(fixtureDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read fixture manifest: %w", err)
	}
	if bytes.Contains(manifestData, []byte(`"original_url"`)) {
		return fmt.Errorf("fixture manifest contains live external route origins")
	}
	for id, route := range store.manifest.ExternalRoutes {
		if route.ID != id {
			return fmt.Errorf("external route %q has mismatched id %q", id, route.ID)
		}
		switch route.Class {
		case TrafficClassEnvironmentOps, TrafficClassAgentLiveStream:
		default:
			return fmt.Errorf("external route %q has unsupported traffic class %q", id, route.Class)
		}
	}
	for _, item := range store.manifest.Interactions {
		meta, requestBody, responseBody, err := store.readInteraction(item)
		if err != nil {
			return err
		}
		if err := validateNoTokenLikeData(fmt.Sprintf("interaction %04d request body", meta.Index), requestBody); err != nil {
			return err
		}
		if err := validateNoTokenLikeData(fmt.Sprintf("interaction %04d response body", meta.Index), responseBody); err != nil {
			return err
		}
		if got := digestHex(requestBody); meta.RequestBodySHA256 != "" && got != meta.RequestBodySHA256 {
			return fmt.Errorf("interaction %04d request body digest mismatch: got %s want %s", meta.Index, got, meta.RequestBodySHA256)
		}
		if got := digestHex(responseBody); meta.ResponseBodySHA256 != "" && got != meta.ResponseBodySHA256 {
			return fmt.Errorf("interaction %04d response body digest mismatch: got %s want %s", meta.Index, got, meta.ResponseBodySHA256)
		}
		for key, value := range meta.StableRequestHeaders {
			if strings.EqualFold(key, "authorization") || strings.Contains(value, "Bearer ") {
				return fmt.Errorf("interaction %04d metadata contains authorization data", meta.Index)
			}
		}
	}
	return nil
}

func validateNoTokenLikeData(description string, body []byte) error {
	if bytes.Contains(body, []byte("Bearer ")) ||
		bytes.Contains(body, []byte("ghp_")) ||
		bytes.Contains(body, []byte("github_pat_")) {
		return fmt.Errorf("%s contains token-looking data", description)
	}
	return nil
}

// ListenAndServe starts a replay proxy and blocks until the server exits.
func ListenAndServe(ctx context.Context, listenAddr string, opts Options) error {
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	if opts.PublicURL == "" {
		opts.PublicURL = publicURLFromListener(lis)
	}
	proxy, err := New(opts)
	if err != nil {
		_ = lis.Close()
		return err
	}
	fmt.Fprintf(os.Stderr, "replayproxy %s listening at %s\n", opts.Mode, opts.PublicURL)

	server := &http.Server{Handler: proxy}
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(lis)
	}()
	select {
	case <-ctx.Done():
		if err := server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("shutdown replay proxy: %w", err)
		}
		err := <-done
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func publicURLFromListener(lis net.Listener) string {
	host, port, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		return "http://" + lis.Addr().String()
	}
	if host == "" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return (&url.URL{Scheme: "http", Host: net.JoinHostPort(host, port), Path: path.Clean("/")}).String()
}

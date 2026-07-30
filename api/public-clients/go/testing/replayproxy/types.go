package replayproxy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	SchemaVersion = 1

	ModeRecord = "record"
	ModeReplay = "replay"

	LanguageGo         = "go"
	LanguageTypeScript = "typescript"
	LanguagePython     = "python"

	TrafficClassManagementPlane = "management_plane"
	TrafficClassEnvironmentOps  = "environment_ops"
	TrafficClassAgentLiveStream = "agent_live_stream"

	stableReplayBaseURL = "http://ona-replay.invalid"

	envAccessTokenPlaceholder            = "ona-replay-env-access-token"
	agentConversationTokenPlaceholder    = "ona-replay-agent-conversation-token"
	normalizedExecOperationID            = "00000000-0000-0000-0000-00000000e000"
	normalizedAgentUserInputID           = "00000000-0000-0000-0000-00000000a000"
	defaultInteractionMetadataFileFormat = "%04d.json"
	defaultRequestBodyFileFormat         = "%04d.request.bin"
	defaultResponseBodyFileFormat        = "%04d.response.bin"
)

// Options configures a replay proxy server.
type Options struct {
	Mode             string
	FixtureDir       string
	Scenario         string
	ExpectedLanguage string
	UpstreamBaseURL  string
	PublicURL        string
}

type Manifest struct {
	SchemaVersion   int                       `json:"schema_version"`
	Scenario        string                    `json:"scenario"`
	RecordedAt      string                    `json:"recorded_at,omitempty"`
	GitRevision     string                    `json:"git_revision,omitempty"`
	SourceLanguage  string                    `json:"source_language,omitempty"`
	SourceUserAgent string                    `json:"source_user_agent,omitempty"`
	UpstreamBaseURL string                    `json:"upstream_base_url,omitempty"`
	ExternalRoutes  map[string]ExternalRoute  `json:"external_routes,omitempty"`
	Interactions    []InteractionManifestItem `json:"interactions"`
}

type ExternalRoute struct {
	ID    string `json:"id"`
	Class string `json:"class"`
}

type InteractionManifestItem struct {
	Index        int    `json:"index"`
	MetadataFile string `json:"metadata_file"`
}

type InteractionMetadata struct {
	Index                  int               `json:"index"`
	TrafficClass           string            `json:"traffic_class"`
	Method                 string            `json:"method"`
	Path                   string            `json:"path"`
	Query                  string            `json:"query,omitempty"`
	StableRequestHeaders   map[string]string `json:"stable_request_headers,omitempty"`
	ResponseHeaders        map[string]string `json:"response_headers,omitempty"`
	StatusCode             int               `json:"status_code"`
	RequestBodySHA256      string            `json:"request_body_sha256,omitempty"`
	ResponseBodySHA256     string            `json:"response_body_sha256,omitempty"`
	RequestBodyFile        string            `json:"request_body_file,omitempty"`
	ResponseBodyFile       string            `json:"response_body_file,omitempty"`
	Redactions             []string          `json:"redactions,omitempty"`
	UserAgent              string            `json:"user_agent,omitempty"`
	UserAgentHeader        string            `json:"user_agent_header,omitempty"`
	StableProtocolVerified bool              `json:"stable_protocol_verified"`
}

type fixtureStore struct {
	dir      string
	manifest Manifest
}

func newFixtureStore(dir string, scenario string) (*fixtureStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("fixture directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create fixture directory: %w", err)
	}
	manifest := Manifest{
		SchemaVersion:  SchemaVersion,
		Scenario:       scenario,
		RecordedAt:     time.Now().UTC().Format(time.RFC3339),
		GitRevision:    gitRevision(),
		ExternalRoutes: map[string]ExternalRoute{},
	}
	return &fixtureStore{dir: dir, manifest: manifest}, nil
}

func gitRevision() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func loadFixtureStore(dir string) (*fixtureStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("fixture directory is required")
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read fixture manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse fixture manifest: %w", err)
	}
	if manifest.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported fixture schema version %d", manifest.SchemaVersion)
	}
	if manifest.ExternalRoutes == nil {
		manifest.ExternalRoutes = map[string]ExternalRoute{}
	}
	return &fixtureStore{dir: dir, manifest: manifest}, nil
}

func (s *fixtureStore) writeManifest() error {
	data, err := json.MarshalIndent(s.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal fixture manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(s.dir, "manifest.json"), data, 0o644); err != nil {
		return fmt.Errorf("write fixture manifest: %w", err)
	}
	return nil
}

func (s *fixtureStore) writeInteraction(meta InteractionMetadata, normalizedRequestBody []byte, responseBody []byte) error {
	meta.RequestBodyFile = fmt.Sprintf(defaultRequestBodyFileFormat, meta.Index)
	meta.ResponseBodyFile = fmt.Sprintf(defaultResponseBodyFileFormat, meta.Index)
	metaFile := fmt.Sprintf(defaultInteractionMetadataFileFormat, meta.Index)

	if err := os.WriteFile(filepath.Join(s.dir, meta.RequestBodyFile), normalizedRequestBody, 0o644); err != nil {
		return fmt.Errorf("write fixture request body: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, meta.ResponseBodyFile), responseBody, 0o644); err != nil {
		return fmt.Errorf("write fixture response body: %w", err)
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal interaction metadata: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(s.dir, metaFile), data, 0o644); err != nil {
		return fmt.Errorf("write interaction metadata: %w", err)
	}

	if !slices.ContainsFunc(s.manifest.Interactions, func(item InteractionManifestItem) bool {
		return item.Index == meta.Index
	}) {
		s.manifest.Interactions = append(s.manifest.Interactions, InteractionManifestItem{
			Index:        meta.Index,
			MetadataFile: metaFile,
		})
		slices.SortFunc(s.manifest.Interactions, func(a, b InteractionManifestItem) int {
			return a.Index - b.Index
		})
	}
	return s.writeManifest()
}

func (s *fixtureStore) readInteraction(item InteractionManifestItem) (InteractionMetadata, []byte, []byte, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, item.MetadataFile))
	if err != nil {
		return InteractionMetadata{}, nil, nil, fmt.Errorf("read interaction metadata %s: %w", item.MetadataFile, err)
	}
	var meta InteractionMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return InteractionMetadata{}, nil, nil, fmt.Errorf("parse interaction metadata %s: %w", item.MetadataFile, err)
	}
	requestBody, err := os.ReadFile(filepath.Join(s.dir, meta.RequestBodyFile))
	if err != nil {
		return InteractionMetadata{}, nil, nil, fmt.Errorf("read interaction request body %s: %w", meta.RequestBodyFile, err)
	}
	responseBody, err := os.ReadFile(filepath.Join(s.dir, meta.ResponseBodyFile))
	if err != nil {
		return InteractionMetadata{}, nil, nil, fmt.Errorf("read interaction response body %s: %w", meta.ResponseBodyFile, err)
	}
	return meta, requestBody, responseBody, nil
}

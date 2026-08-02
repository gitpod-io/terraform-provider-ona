package replayproxy

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strings"

	supervisorv1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/supervisor/v1"
	supervisorconnect "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/supervisor/v1/v1connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	"google.golang.org/protobuf/proto"
)

type responseBodies struct {
	FixtureBody []byte
	ClientBody  []byte
	Redactions  []string
}

type routeRewriter interface {
	rewriteExternalURL(originalURL string, trafficClass string, stable bool) (string, error)
	rememberToken(placeholder string, token string)
}

func normalizeRequestBody(path string, body []byte) ([]byte, error) {
	switch procedurePath(path) {
	case v1connect.EventServiceWatchEventsProcedure:
		flags := byte(0)
		payload := body
		framed := false
		if len(body) >= 5 {
			size := int(binary.BigEndian.Uint32(body[1:5]))
			if size >= 0 && len(body) == 5+size {
				flags = body[0]
				payload = body[5:]
				framed = true
			}
		}
		var msg v1.WatchEventsRequest
		if err := proto.Unmarshal(payload, &msg); err != nil {
			return nil, fmt.Errorf("decode watch events request: %w", err)
		}
		normalized, err := marshalDeterministic(&msg)
		if err != nil {
			return nil, err
		}
		if !framed {
			return normalized, nil
		}
		out := make([]byte, 5+len(normalized))
		out[0] = flags
		binary.BigEndian.PutUint32(out[1:5], uint32(len(normalized)))
		copy(out[5:], normalized)
		return out, nil
	case supervisorconnect.EnvironmentOpsServiceExecProcedure:
		var msg supervisorv1.ExecRequest
		if err := proto.Unmarshal(body, &msg); err != nil {
			return nil, fmt.Errorf("decode exec request: %w", err)
		}
		msg.OperationId = normalizedExecOperationID
		return marshalDeterministic(&msg)
	case v1connect.AgentServiceSendToAgentExecutionProcedure:
		var msg v1.SendToAgentExecutionRequest
		if err := proto.Unmarshal(body, &msg); err != nil {
			return nil, fmt.Errorf("decode send-to-agent request: %w", err)
		}
		if input := msg.GetUserInput(); input != nil {
			input.Id = normalizedAgentUserInputID
		}
		return marshalDeterministic(&msg)
	default:
		return body, nil
	}
}

func normalizeResponseBody(path string, body []byte, rewriter routeRewriter) (responseBodies, error) {
	if len(body) == 0 {
		return responseBodies{FixtureBody: body, ClientBody: body}, nil
	}

	switch procedurePath(path) {
	case v1connect.EnvironmentServiceCreateEnvironmentProcedure:
		var msg v1.CreateEnvironmentResponse
		if err := proto.Unmarshal(body, &msg); err != nil {
			return responseBodies{}, fmt.Errorf("decode create environment response: %w", err)
		}
		return rewriteEnvironmentResponse(&msg, rewriter)
	case v1connect.EnvironmentServiceCreateEnvironmentFromProjectProcedure:
		var msg v1.CreateEnvironmentFromProjectResponse
		if err := proto.Unmarshal(body, &msg); err != nil {
			return responseBodies{}, fmt.Errorf("decode create project environment response: %w", err)
		}
		return rewriteProjectEnvironmentResponse(&msg, rewriter)
	case v1connect.EnvironmentServiceGetEnvironmentProcedure:
		var msg v1.GetEnvironmentResponse
		if err := proto.Unmarshal(body, &msg); err != nil {
			return responseBodies{}, fmt.Errorf("decode get environment response: %w", err)
		}
		return rewriteGetEnvironmentResponse(&msg, rewriter)
	case v1connect.EnvironmentServiceCreateEnvironmentAccessTokenProcedure:
		var msg v1.CreateEnvironmentAccessTokenResponse
		if err := proto.Unmarshal(body, &msg); err != nil {
			return responseBodies{}, fmt.Errorf("decode environment access token response: %w", err)
		}
		redactions := []string{}
		if msg.AccessToken != "" && msg.AccessToken != envAccessTokenPlaceholder {
			rewriter.rememberToken(envAccessTokenPlaceholder, msg.AccessToken)
			msg.AccessToken = envAccessTokenPlaceholder
			redactions = append(redactions, "environment_access_token")
		}
		out, err := marshalDeterministic(&msg)
		if err != nil {
			return responseBodies{}, err
		}
		return responseBodies{FixtureBody: out, ClientBody: out, Redactions: redactions}, nil
	case v1connect.AgentServiceGetAgentExecutionProcedure:
		var msg v1.GetAgentExecutionResponse
		if err := proto.Unmarshal(body, &msg); err != nil {
			return responseBodies{}, fmt.Errorf("decode get agent execution response: %w", err)
		}
		return rewriteGetAgentExecutionResponse(&msg, rewriter)
	case v1connect.AgentServiceCreateAgentExecutionConversationTokenProcedure:
		var msg v1.CreateAgentExecutionConversationTokenResponse
		if err := proto.Unmarshal(body, &msg); err != nil {
			return responseBodies{}, fmt.Errorf("decode agent conversation token response: %w", err)
		}
		redactions := []string{}
		if msg.Token != "" && msg.Token != agentConversationTokenPlaceholder {
			rewriter.rememberToken(agentConversationTokenPlaceholder, msg.Token)
			msg.Token = agentConversationTokenPlaceholder
			redactions = append(redactions, "agent_conversation_token")
		}
		out, err := marshalDeterministic(&msg)
		if err != nil {
			return responseBodies{}, err
		}
		return responseBodies{FixtureBody: out, ClientBody: out, Redactions: redactions}, nil
	default:
		return responseBodies{FixtureBody: body, ClientBody: body}, nil
	}
}

func rewriteEnvironmentResponse(msg *v1.CreateEnvironmentResponse, rewriter routeRewriter) (responseBodies, error) {
	fixture := proto.Clone(msg).(*v1.CreateEnvironmentResponse)
	client := proto.Clone(msg).(*v1.CreateEnvironmentResponse)
	redactions, err := rewriteEnvironmentURLs(fixture.GetEnvironment(), client.GetEnvironment(), rewriter)
	if err != nil {
		return responseBodies{}, err
	}
	return marshalResponsePair(fixture, client, redactions)
}

func rewriteProjectEnvironmentResponse(msg *v1.CreateEnvironmentFromProjectResponse, rewriter routeRewriter) (responseBodies, error) {
	fixture := proto.Clone(msg).(*v1.CreateEnvironmentFromProjectResponse)
	client := proto.Clone(msg).(*v1.CreateEnvironmentFromProjectResponse)
	redactions, err := rewriteEnvironmentURLs(fixture.GetEnvironment(), client.GetEnvironment(), rewriter)
	if err != nil {
		return responseBodies{}, err
	}
	return marshalResponsePair(fixture, client, redactions)
}

func rewriteGetEnvironmentResponse(msg *v1.GetEnvironmentResponse, rewriter routeRewriter) (responseBodies, error) {
	fixture := proto.Clone(msg).(*v1.GetEnvironmentResponse)
	client := proto.Clone(msg).(*v1.GetEnvironmentResponse)
	redactions, err := rewriteEnvironmentURLs(fixture.GetEnvironment(), client.GetEnvironment(), rewriter)
	if err != nil {
		return responseBodies{}, err
	}
	return marshalResponsePair(fixture, client, redactions)
}

func rewriteGetAgentExecutionResponse(msg *v1.GetAgentExecutionResponse, rewriter routeRewriter) (responseBodies, error) {
	fixture := proto.Clone(msg).(*v1.GetAgentExecutionResponse)
	client := proto.Clone(msg).(*v1.GetAgentExecutionResponse)
	redactions, err := rewriteAgentExecutionURLs(fixture.GetAgentExecution(), client.GetAgentExecution(), rewriter)
	if err != nil {
		return responseBodies{}, err
	}
	return marshalResponsePair(fixture, client, redactions)
}

func rewriteEnvironmentURLs(fixture *v1.Environment, client *v1.Environment, rewriter routeRewriter) ([]string, error) {
	if fixture == nil || client == nil {
		return nil, nil
	}
	fixtureURLs := fixture.GetStatus().GetEnvironmentUrls()
	clientURLs := client.GetStatus().GetEnvironmentUrls()
	if fixtureURLs == nil || clientURLs == nil || fixtureURLs.GetOps() == "" {
		return nil, nil
	}

	routeOrigin := localDevOpsOrigin(fixture, fixtureURLs.GetOps())
	stableURL, err := rewriter.rewriteExternalURL(routeOrigin, TrafficClassEnvironmentOps, true)
	if err != nil {
		return nil, err
	}
	clientURL, err := rewriter.rewriteExternalURL(routeOrigin, TrafficClassEnvironmentOps, false)
	if err != nil {
		return nil, err
	}
	fixtureURLs.Ops = stableURL
	clientURLs.Ops = clientURL
	return []string{"environment_ops_url"}, nil
}

func localDevOpsOrigin(env *v1.Environment, opsURL string) string {
	parsedOps, err := url.Parse(opsURL)
	if err != nil || parsedOps.Scheme != "https" || !strings.HasSuffix(parsedOps.Hostname(), ".localhost") {
		return opsURL
	}
	portPrefix, suffix, _ := strings.Cut(parsedOps.Hostname(), "--")
	if suffix == "" || portPrefix == "" {
		return opsURL
	}
	for _, ch := range portPrefix {
		if ch < '0' || ch > '9' {
			return opsURL
		}
	}

	sshURL := env.GetStatus().GetEnvironmentUrls().GetSsh().GetUrl()
	parsedSSH, err := url.Parse(sshURL)
	if err != nil || parsedSSH.Hostname() == "" {
		return opsURL
	}
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(parsedSSH.Hostname(), portPrefix),
		Path:   parsedOps.Path,
	}).String()
}

func rewriteAgentExecutionURLs(fixture *v1.AgentExecution, client *v1.AgentExecution, rewriter routeRewriter) ([]string, error) {
	if fixture == nil || client == nil {
		return nil, nil
	}
	fixtureURLs := fixture.GetStatus().GetConversationUrls()
	clientURLs := client.GetStatus().GetConversationUrls()
	if fixtureURLs == nil || clientURLs == nil || fixtureURLs.GetLive() == "" {
		return nil, nil
	}

	stableURL, err := rewriter.rewriteExternalURL(fixtureURLs.GetLive(), TrafficClassAgentLiveStream, true)
	if err != nil {
		return nil, err
	}
	clientURL, err := rewriter.rewriteExternalURL(clientURLs.GetLive(), TrafficClassAgentLiveStream, false)
	if err != nil {
		return nil, err
	}
	fixtureURLs.Live = stableURL
	clientURLs.Live = clientURL
	return []string{"agent_live_url"}, nil
}

func marshalResponsePair(fixture proto.Message, client proto.Message, redactions []string) (responseBodies, error) {
	fixtureBody, err := marshalDeterministic(fixture)
	if err != nil {
		return responseBodies{}, err
	}
	clientBody, err := marshalDeterministic(client)
	if err != nil {
		return responseBodies{}, err
	}
	return responseBodies{FixtureBody: fixtureBody, ClientBody: clientBody, Redactions: redactions}, nil
}

func marshalDeterministic(msg proto.Message) ([]byte, error) {
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal protobuf message: %w", err)
	}
	return body, nil
}

func procedurePath(path string) string {
	if !strings.HasPrefix(path, "/__replay/external/") {
		return path
	}
	rest := strings.TrimPrefix(path, "/__replay/external/")
	_, suffix, ok := strings.Cut(rest, "/")
	if !ok {
		return path
	}
	return "/" + suffix
}

func isEventStreamProcedure(path string) bool {
	switch procedurePath(path) {
	case v1connect.EventServiceWatchEventsProcedure:
		return true
	default:
		return false
	}
}

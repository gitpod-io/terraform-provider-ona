// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package managementclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"

	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1/v1connect"
)

const (
	AuthorizationHeader = "authorization"
	UserAgentHeader     = "user-agent"
	BearerPrefix        = "Bearer"
)

type options struct {
	baseURL      string
	accessToken  string
	userAgent    string
	httpClient   connect.HTTPClient
	interceptors []connect.Interceptor
}

type Option func(*options) error

func WithUserAgent(userAgent string) Option {
	return func(o *options) error {
		o.userAgent = userAgent
		return nil
	}
}

func WithAccessToken(token string) Option {
	return func(o *options) error {
		token = strings.TrimSpace(token)
		o.accessToken = strings.TrimSpace(strings.TrimPrefix(token, BearerPrefix))
		return nil
	}
}

func WithHTTPClient(client connect.HTTPClient) Option {
	return func(o *options) error {
		o.httpClient = client
		return nil
	}
}

func WithInterceptor(interceptor connect.Interceptor) Option {
	return func(o *options) error {
		o.interceptors = append(o.interceptors, interceptor)
		return nil
	}
}

func New(baseURL string, opts ...Option) (*ManagementPlane, error) {
	o := options{
		httpClient: http.DefaultClient,
		baseURL:    baseURL,
	}
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, fmt.Errorf("cannot apply option: %w", err)
		}
	}

	interceptors := make([]connect.Interceptor, 0, len(o.interceptors)+2)
	if o.accessToken != "" {
		interceptors = append(interceptors, withStaticHeaderValue(AuthorizationHeader, BearerPrefix+" "+o.accessToken))
	}
	if o.userAgent != "" {
		interceptors = append(interceptors, withStaticHeaderValue(UserAgentHeader, o.userAgent))
	}
	interceptors = append(interceptors, o.interceptors...)

	var clientOpts []connect.ClientOption
	if len(interceptors) > 0 {
		clientOpts = append(clientOpts, connect.WithInterceptors(interceptors...))
	}

	return NewWithServices(Services{
		AccountService:               v1connect.NewAccountServiceClient(o.httpClient, o.baseURL, clientOpts...),
		AgentSecurityService:         v1connect.NewAgentSecurityServiceClient(o.httpClient, o.baseURL, clientOpts...),
		AgentService:                 v1connect.NewAgentServiceClient(o.httpClient, o.baseURL, clientOpts...),
		BillingService:               v1connect.NewBillingServiceClient(o.httpClient, o.baseURL, clientOpts...),
		EditorService:                v1connect.NewEditorServiceClient(o.httpClient, o.baseURL, clientOpts...),
		EnvironmentAutomationService: v1connect.NewEnvironmentAutomationServiceClient(o.httpClient, o.baseURL, clientOpts...),
		EnvironmentService:           v1connect.NewEnvironmentServiceClient(o.httpClient, o.baseURL, clientOpts...),
		ErrorsService:                v1connect.NewErrorsServiceClient(o.httpClient, o.baseURL, clientOpts...),
		EventService:                 v1connect.NewEventServiceClient(o.httpClient, o.baseURL, clientOpts...),
		GatewayService:               v1connect.NewGatewayServiceClient(o.httpClient, o.baseURL, clientOpts...),
		GroupService:                 v1connect.NewGroupServiceClient(o.httpClient, o.baseURL, clientOpts...),
		IdentityService:              v1connect.NewIdentityServiceClient(o.httpClient, o.baseURL, clientOpts...),
		InsightsService:              v1connect.NewInsightsServiceClient(o.httpClient, o.baseURL, clientOpts...),
		IntegrationService:           v1connect.NewIntegrationServiceClient(o.httpClient, o.baseURL, clientOpts...),
		OnaIntelligenceService:       v1connect.NewOnaIntelligenceServiceClient(o.httpClient, o.baseURL, clientOpts...),
		OrganizationService:          v1connect.NewOrganizationServiceClient(o.httpClient, o.baseURL, clientOpts...),
		PrebuildService:              v1connect.NewPrebuildServiceClient(o.httpClient, o.baseURL, clientOpts...),
		ProjectService:               v1connect.NewProjectServiceClient(o.httpClient, o.baseURL, clientOpts...),
		RunnerConfigurationService:   v1connect.NewRunnerConfigurationServiceClient(o.httpClient, o.baseURL, clientOpts...),
		RunnerInteractionService:     v1connect.NewRunnerInteractionServiceClient(o.httpClient, o.baseURL, clientOpts...),
		RunnerManagerService:         v1connect.NewRunnerManagerServiceClient(o.httpClient, o.baseURL, clientOpts...),
		RunnerService:                v1connect.NewRunnerServiceClient(o.httpClient, o.baseURL, clientOpts...),
		SecretService:                v1connect.NewSecretServiceClient(o.httpClient, o.baseURL, clientOpts...),
		SecurityService:              v1connect.NewSecurityServiceClient(o.httpClient, o.baseURL, clientOpts...),
		ServiceAccountService:        v1connect.NewServiceAccountServiceClient(o.httpClient, o.baseURL, clientOpts...),
		SessionService:               v1connect.NewSessionServiceClient(o.httpClient, o.baseURL, clientOpts...),
		UserService:                  v1connect.NewUserServiceClient(o.httpClient, o.baseURL, clientOpts...),
		WebhookService:               v1connect.NewWebhookServiceClient(o.httpClient, o.baseURL, clientOpts...),
		WorkflowService:              v1connect.NewWorkflowServiceClient(o.httpClient, o.baseURL, clientOpts...),
	}), nil
}

type Services struct {
	AccountService               v1connect.AccountServiceClient
	AgentSecurityService         v1connect.AgentSecurityServiceClient
	AgentService                 v1connect.AgentServiceClient
	BillingService               v1connect.BillingServiceClient
	EditorService                v1connect.EditorServiceClient
	EnvironmentAutomationService v1connect.EnvironmentAutomationServiceClient
	EnvironmentService           v1connect.EnvironmentServiceClient
	ErrorsService                v1connect.ErrorsServiceClient
	EventService                 v1connect.EventServiceClient
	GatewayService               v1connect.GatewayServiceClient
	GroupService                 v1connect.GroupServiceClient
	IdentityService              v1connect.IdentityServiceClient
	InsightsService              v1connect.InsightsServiceClient
	IntegrationService           v1connect.IntegrationServiceClient
	OnaIntelligenceService       v1connect.OnaIntelligenceServiceClient
	OrganizationService          v1connect.OrganizationServiceClient
	PrebuildService              v1connect.PrebuildServiceClient
	ProjectService               v1connect.ProjectServiceClient
	RunnerConfigurationService   v1connect.RunnerConfigurationServiceClient
	RunnerInteractionService     v1connect.RunnerInteractionServiceClient
	RunnerManagerService         v1connect.RunnerManagerServiceClient
	RunnerService                v1connect.RunnerServiceClient
	SecretService                v1connect.SecretServiceClient
	SecurityService              v1connect.SecurityServiceClient
	ServiceAccountService        v1connect.ServiceAccountServiceClient
	SessionService               v1connect.SessionServiceClient
	UserService                  v1connect.UserServiceClient
	WebhookService               v1connect.WebhookServiceClient
	WorkflowService              v1connect.WorkflowServiceClient
}

func NewWithServices(services Services) *ManagementPlane {
	return &ManagementPlane{
		accountService:               services.AccountService,
		agentSecurityService:         services.AgentSecurityService,
		agentService:                 services.AgentService,
		billingService:               services.BillingService,
		editorService:                services.EditorService,
		environmentAutomationService: services.EnvironmentAutomationService,
		environmentService:           services.EnvironmentService,
		errorsService:                services.ErrorsService,
		eventService:                 services.EventService,
		gatewayService:               services.GatewayService,
		groupService:                 services.GroupService,
		identityService:              services.IdentityService,
		insightsService:              services.InsightsService,
		integrationService:           services.IntegrationService,
		onaIntelligenceService:       services.OnaIntelligenceService,
		organizationService:          services.OrganizationService,
		prebuildService:              services.PrebuildService,
		projectService:               services.ProjectService,
		runnerConfigurationService:   services.RunnerConfigurationService,
		runnerInteractionService:     services.RunnerInteractionService,
		runnerManagerService:         services.RunnerManagerService,
		runnerService:                services.RunnerService,
		secretService:                services.SecretService,
		securityService:              services.SecurityService,
		serviceAccountService:        services.ServiceAccountService,
		sessionService:               services.SessionService,
		userService:                  services.UserService,
		webhookService:               services.WebhookService,
		workflowService:              services.WorkflowService,
	}
}

type ManagementPlane struct {
	accountService               v1connect.AccountServiceClient
	agentSecurityService         v1connect.AgentSecurityServiceClient
	agentService                 v1connect.AgentServiceClient
	billingService               v1connect.BillingServiceClient
	editorService                v1connect.EditorServiceClient
	environmentAutomationService v1connect.EnvironmentAutomationServiceClient
	environmentService           v1connect.EnvironmentServiceClient
	errorsService                v1connect.ErrorsServiceClient
	eventService                 v1connect.EventServiceClient
	gatewayService               v1connect.GatewayServiceClient
	groupService                 v1connect.GroupServiceClient
	identityService              v1connect.IdentityServiceClient
	insightsService              v1connect.InsightsServiceClient
	integrationService           v1connect.IntegrationServiceClient
	onaIntelligenceService       v1connect.OnaIntelligenceServiceClient
	organizationService          v1connect.OrganizationServiceClient
	prebuildService              v1connect.PrebuildServiceClient
	projectService               v1connect.ProjectServiceClient
	runnerConfigurationService   v1connect.RunnerConfigurationServiceClient
	runnerInteractionService     v1connect.RunnerInteractionServiceClient
	runnerManagerService         v1connect.RunnerManagerServiceClient
	runnerService                v1connect.RunnerServiceClient
	secretService                v1connect.SecretServiceClient
	securityService              v1connect.SecurityServiceClient
	serviceAccountService        v1connect.ServiceAccountServiceClient
	sessionService               v1connect.SessionServiceClient
	userService                  v1connect.UserServiceClient
	webhookService               v1connect.WebhookServiceClient
	workflowService              v1connect.WorkflowServiceClient
}

func (g *ManagementPlane) AccountService() v1connect.AccountServiceClient {
	return g.accountService
}

func (g *ManagementPlane) AgentSecurityService() v1connect.AgentSecurityServiceClient {
	return g.agentSecurityService
}

func (g *ManagementPlane) AgentService() v1connect.AgentServiceClient {
	return g.agentService
}

func (g *ManagementPlane) BillingService() v1connect.BillingServiceClient {
	return g.billingService
}

func (g *ManagementPlane) EditorService() v1connect.EditorServiceClient {
	return g.editorService
}

func (g *ManagementPlane) EnvironmentAutomationService() v1connect.EnvironmentAutomationServiceClient {
	return g.environmentAutomationService
}

func (g *ManagementPlane) EnvironmentService() v1connect.EnvironmentServiceClient {
	return g.environmentService
}

func (g *ManagementPlane) ErrorsService() v1connect.ErrorsServiceClient {
	return g.errorsService
}

func (g *ManagementPlane) EventService() v1connect.EventServiceClient {
	return g.eventService
}

func (g *ManagementPlane) GatewayService() v1connect.GatewayServiceClient {
	return g.gatewayService
}

func (g *ManagementPlane) GroupService() v1connect.GroupServiceClient {
	return g.groupService
}

func (g *ManagementPlane) IdentityService() v1connect.IdentityServiceClient {
	return g.identityService
}

func (g *ManagementPlane) InsightsService() v1connect.InsightsServiceClient {
	return g.insightsService
}

func (g *ManagementPlane) IntegrationService() v1connect.IntegrationServiceClient {
	return g.integrationService
}

func (g *ManagementPlane) OnaIntelligenceService() v1connect.OnaIntelligenceServiceClient {
	return g.onaIntelligenceService
}

func (g *ManagementPlane) OrganizationService() v1connect.OrganizationServiceClient {
	return g.organizationService
}

func (g *ManagementPlane) PrebuildService() v1connect.PrebuildServiceClient {
	return g.prebuildService
}

func (g *ManagementPlane) ProjectService() v1connect.ProjectServiceClient {
	return g.projectService
}

func (g *ManagementPlane) RunnerConfigurationService() v1connect.RunnerConfigurationServiceClient {
	return g.runnerConfigurationService
}

func (g *ManagementPlane) RunnerInteractionService() v1connect.RunnerInteractionServiceClient {
	return g.runnerInteractionService
}

func (g *ManagementPlane) RunnerManagerService() v1connect.RunnerManagerServiceClient {
	return g.runnerManagerService
}

func (g *ManagementPlane) RunnerService() v1connect.RunnerServiceClient {
	return g.runnerService
}

func (g *ManagementPlane) SecretService() v1connect.SecretServiceClient {
	return g.secretService
}

func (g *ManagementPlane) SecurityService() v1connect.SecurityServiceClient {
	return g.securityService
}

func (g *ManagementPlane) ServiceAccountService() v1connect.ServiceAccountServiceClient {
	return g.serviceAccountService
}

func (g *ManagementPlane) SessionService() v1connect.SessionServiceClient {
	return g.sessionService
}

func (g *ManagementPlane) UserService() v1connect.UserServiceClient {
	return g.userService
}

func (g *ManagementPlane) WebhookService() v1connect.WebhookServiceClient {
	return g.webhookService
}

func (g *ManagementPlane) WorkflowService() v1connect.WorkflowServiceClient {
	return g.workflowService
}

func withStaticHeaderValue(key, value string) connect.Interceptor {
	return &headerInterceptor{
		key:   key,
		value: value,
	}
}

type headerInterceptor struct {
	key   string
	value string
}

func (i *headerInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, ar connect.AnyRequest) (connect.AnyResponse, error) {
		if ar.Spec().IsClient && i.key != "" && i.value != "" {
			ar.Header().Set(i.key, i.value)
		}
		return next(ctx, ar)
	}
}

func (i *headerInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return connect.StreamingClientFunc(func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		if i.key != "" && i.value != "" {
			conn.RequestHeader().Set(i.key, i.value)
		}
		return conn
	})
}

func (i *headerInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

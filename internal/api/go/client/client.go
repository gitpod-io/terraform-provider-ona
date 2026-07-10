package client

import (
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"go.uber.org/mock/gomock"
	"golang.org/x/oauth2"

	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/client/mock"
	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1/v1connect"
)

type options struct {
	baseURL      string
	tokenSource  oauth2.TokenSource
	userAgent    string
	httpClient   *http.Client
	interceptors []connect.Interceptor
}

type Option func(*options) error

func WithUserAgent(userAgent string) Option {
	return func(o *options) error {
		o.userAgent = userAgent
		return nil
	}
}

func WithTracing(opts ...otelconnect.Option) Option {
	return func(o *options) error {
		interceptor, err := otelconnect.NewInterceptor(opts...)
		if err != nil {
			return fmt.Errorf("cannot create tracing interceptor: %w", err)
		}

		o.interceptors = append(o.interceptors, interceptor)

		return nil
	}
}

func WithHTTPClient(client *http.Client) Option {
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

// WithMetrics adds a metrics interceptor that counts requests per procedure and status code.
// Create APICallMetrics once via NewAPICallMetrics and share across multiple clients.
func WithMetrics(m *APICallMetrics) Option {
	return func(o *options) error {
		o.interceptors = append(o.interceptors, m.Interceptor())
		return nil
	}
}

func WithTokenSource(source oauth2.TokenSource) Option {
	return func(o *options) error {
		o.tokenSource = source
		return nil
	}
}

func WithAccessToken(token string) Option {
	return func(o *options) error {
		o.tokenSource = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		return nil
	}
}

type ManagementPlaneMock struct {
	AccountService               *mock.MockAccountServiceClient
	AgentSecurityService         *mock.MockAgentSecurityServiceClient
	AgentService                 *mock.MockAgentServiceClient
	BillingService               *mock.MockBillingServiceClient
	EditorService                *mock.MockEditorServiceClient
	EnvironmentAutomationService *mock.MockEnvironmentAutomationServiceClient
	EnvironmentService           *mock.MockEnvironmentServiceClient
	ErrorsService                *mock.MockErrorsServiceClient
	EventService                 *mock.MockEventServiceClient
	GroupService                 *mock.MockGroupServiceClient
	IdentityService              *mock.MockIdentityServiceClient
	InsightsService              *mock.MockInsightsServiceClient
	IntegrationService           *mock.MockIntegrationServiceClient
	OrganizationService          *mock.MockOrganizationServiceClient
	PrebuildService              *mock.MockPrebuildServiceClient
	ProjectService               *mock.MockProjectServiceClient
	RunnerConfigurationService   *mock.MockRunnerConfigurationServiceClient
	RunnerInteractionService     *mock.MockRunnerInteractionServiceClient
	RunnerService                *mock.MockRunnerServiceClient
	UserService                  *mock.MockUserServiceClient
	SecurityService              *mock.MockSecurityServiceClient
	SecretService                *mock.MockSecretServiceClient
	ServiceAccountService        *mock.MockServiceAccountServiceClient
	SessionService               *mock.MockSessionServiceClient
	GatewayService               *mock.MockGatewayServiceClient
	RunnerManagerService         *mock.MockRunnerManagerServiceClient
	OnaIntelligenceService       *mock.MockOnaIntelligenceServiceClient
	WorkflowService              *mock.MockWorkflowServiceClient
	WebhookService               *mock.MockWebhookServiceClient
}

// Client returns a client for the control plane API
func (m *ManagementPlaneMock) Client() *ManagementPlane {
	return &ManagementPlane{
		accountService:               m.AccountService,
		agentSecurityService:         m.AgentSecurityService,
		agentService:                 m.AgentService,
		billingService:               m.BillingService,
		editorService:                m.EditorService,
		environmentAutomationService: m.EnvironmentAutomationService,
		environmentService:           m.EnvironmentService,
		errorsService:                m.ErrorsService,
		eventService:                 m.EventService,
		groupService:                 m.GroupService,
		identityService:              m.IdentityService,
		insightsService:              m.InsightsService,
		integrationService:           m.IntegrationService,
		organizationService:          m.OrganizationService,
		prebuildService:              m.PrebuildService,
		projectService:               m.ProjectService,
		runnerConfigurationService:   m.RunnerConfigurationService,
		runnerInteractionService:     m.RunnerInteractionService,
		runnerService:                m.RunnerService,
		userService:                  m.UserService,
		securityService:              m.SecurityService,
		secretService:                m.SecretService,
		serviceAccountService:        m.ServiceAccountService,
		sessionService:               m.SessionService,
		gatewayService:               m.GatewayService,
		runnerManagerService:         m.RunnerManagerService,
		onaIntelligenceService:       m.OnaIntelligenceService,
		workflowService:              m.WorkflowService,
		webhookService:               m.WebhookService,
	}
}

// NewMock creates a new mock for the control plane API
func NewMock(ctrl *gomock.Controller) *ManagementPlaneMock {
	return &ManagementPlaneMock{
		AccountService:               mock.NewMockAccountServiceClient(ctrl),
		AgentSecurityService:         mock.NewMockAgentSecurityServiceClient(ctrl),
		AgentService:                 mock.NewMockAgentServiceClient(ctrl),
		BillingService:               mock.NewMockBillingServiceClient(ctrl),
		EditorService:                mock.NewMockEditorServiceClient(ctrl),
		EnvironmentAutomationService: mock.NewMockEnvironmentAutomationServiceClient(ctrl),
		EnvironmentService:           mock.NewMockEnvironmentServiceClient(ctrl),
		ErrorsService:                mock.NewMockErrorsServiceClient(ctrl),
		EventService:                 mock.NewMockEventServiceClient(ctrl),
		GroupService:                 mock.NewMockGroupServiceClient(ctrl),
		IdentityService:              mock.NewMockIdentityServiceClient(ctrl),
		InsightsService:              mock.NewMockInsightsServiceClient(ctrl),
		IntegrationService:           mock.NewMockIntegrationServiceClient(ctrl),
		OrganizationService:          mock.NewMockOrganizationServiceClient(ctrl),
		PrebuildService:              mock.NewMockPrebuildServiceClient(ctrl),
		ProjectService:               mock.NewMockProjectServiceClient(ctrl),
		RunnerConfigurationService:   mock.NewMockRunnerConfigurationServiceClient(ctrl),
		RunnerInteractionService:     mock.NewMockRunnerInteractionServiceClient(ctrl),
		RunnerService:                mock.NewMockRunnerServiceClient(ctrl),
		UserService:                  mock.NewMockUserServiceClient(ctrl),
		SecurityService:              mock.NewMockSecurityServiceClient(ctrl),
		SecretService:                mock.NewMockSecretServiceClient(ctrl),
		ServiceAccountService:        mock.NewMockServiceAccountServiceClient(ctrl),
		SessionService:               mock.NewMockSessionServiceClient(ctrl),
		GatewayService:               mock.NewMockGatewayServiceClient(ctrl),
		RunnerManagerService:         mock.NewMockRunnerManagerServiceClient(ctrl),
		OnaIntelligenceService:       mock.NewMockOnaIntelligenceServiceClient(ctrl),
		WorkflowService:              mock.NewMockWorkflowServiceClient(ctrl),
		WebhookService:               mock.NewMockWebhookServiceClient(ctrl),
	}
}

func New(baseURL string, opts ...Option) (*ManagementPlane, error) {
	o := options{
		httpClient: http.DefaultClient,
		baseURL:    baseURL,
	}
	for _, opt := range opts {
		err := opt(&o)
		if err != nil {
			return nil, fmt.Errorf("cannot apply option: %w", err)
		}
	}

	interceptors := append([]connect.Interceptor{
		TokenSourceInterceptor(o.tokenSource),
		WithCustomUserAgent(o.userAgent),
	}, o.interceptors...)

	clientOpts := []connect.ClientOption{
		connect.WithInterceptors(interceptors...),
	}

	return &ManagementPlane{
		accountService:               v1connect.NewAccountServiceClient(o.httpClient, o.baseURL, clientOpts...),
		agentSecurityService:         v1connect.NewAgentSecurityServiceClient(o.httpClient, o.baseURL, clientOpts...),
		agentService:                 v1connect.NewAgentServiceClient(o.httpClient, o.baseURL, clientOpts...),
		billingService:               v1connect.NewBillingServiceClient(o.httpClient, o.baseURL, clientOpts...),
		editorService:                v1connect.NewEditorServiceClient(o.httpClient, o.baseURL, clientOpts...),
		environmentAutomationService: v1connect.NewEnvironmentAutomationServiceClient(o.httpClient, o.baseURL, clientOpts...),
		environmentService:           v1connect.NewEnvironmentServiceClient(o.httpClient, o.baseURL, clientOpts...),
		errorsService:                v1connect.NewErrorsServiceClient(o.httpClient, o.baseURL, clientOpts...),
		eventService:                 v1connect.NewEventServiceClient(o.httpClient, o.baseURL, clientOpts...),
		groupService:                 v1connect.NewGroupServiceClient(o.httpClient, o.baseURL, clientOpts...),
		identityService:              v1connect.NewIdentityServiceClient(o.httpClient, o.baseURL, clientOpts...),
		insightsService:              v1connect.NewInsightsServiceClient(o.httpClient, o.baseURL, clientOpts...),
		integrationService:           v1connect.NewIntegrationServiceClient(o.httpClient, o.baseURL, clientOpts...),
		organizationService:          v1connect.NewOrganizationServiceClient(o.httpClient, o.baseURL, clientOpts...),
		prebuildService:              v1connect.NewPrebuildServiceClient(o.httpClient, o.baseURL, clientOpts...),
		projectService:               v1connect.NewProjectServiceClient(o.httpClient, o.baseURL, clientOpts...),
		runnerConfigurationService:   v1connect.NewRunnerConfigurationServiceClient(o.httpClient, o.baseURL, clientOpts...),
		runnerInteractionService:     v1connect.NewRunnerInteractionServiceClient(o.httpClient, o.baseURL, clientOpts...),
		runnerService:                v1connect.NewRunnerServiceClient(o.httpClient, o.baseURL, clientOpts...),
		userService:                  v1connect.NewUserServiceClient(o.httpClient, o.baseURL, clientOpts...),
		securityService:              v1connect.NewSecurityServiceClient(o.httpClient, o.baseURL, clientOpts...),
		secretService:                v1connect.NewSecretServiceClient(o.httpClient, o.baseURL, clientOpts...),
		serviceAccountService:        v1connect.NewServiceAccountServiceClient(o.httpClient, o.baseURL, clientOpts...),
		sessionService:               v1connect.NewSessionServiceClient(o.httpClient, o.baseURL, clientOpts...),
		gatewayService:               v1connect.NewGatewayServiceClient(o.httpClient, o.baseURL, clientOpts...),
		runnerManagerService:         v1connect.NewRunnerManagerServiceClient(o.httpClient, o.baseURL, clientOpts...),
		onaIntelligenceService:       v1connect.NewOnaIntelligenceServiceClient(o.httpClient, o.baseURL, clientOpts...),
		workflowService:              v1connect.NewWorkflowServiceClient(o.httpClient, o.baseURL, clientOpts...),
		webhookService:               v1connect.NewWebhookServiceClient(o.httpClient, o.baseURL, clientOpts...),
	}, nil
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
	groupService                 v1connect.GroupServiceClient
	identityService              v1connect.IdentityServiceClient
	insightsService              v1connect.InsightsServiceClient
	integrationService           v1connect.IntegrationServiceClient
	organizationService          v1connect.OrganizationServiceClient
	prebuildService              v1connect.PrebuildServiceClient
	projectService               v1connect.ProjectServiceClient
	runnerConfigurationService   v1connect.RunnerConfigurationServiceClient
	runnerInteractionService     v1connect.RunnerInteractionServiceClient
	runnerService                v1connect.RunnerServiceClient
	userService                  v1connect.UserServiceClient
	securityService              v1connect.SecurityServiceClient
	secretService                v1connect.SecretServiceClient
	serviceAccountService        v1connect.ServiceAccountServiceClient
	sessionService               v1connect.SessionServiceClient
	gatewayService               v1connect.GatewayServiceClient
	runnerManagerService         v1connect.RunnerManagerServiceClient
	onaIntelligenceService       v1connect.OnaIntelligenceServiceClient
	workflowService              v1connect.WorkflowServiceClient
	webhookService               v1connect.WebhookServiceClient
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

func (g *ManagementPlane) RunnerService() v1connect.RunnerServiceClient {
	return g.runnerService
}

func (g *ManagementPlane) UserService() v1connect.UserServiceClient {
	return g.userService
}

func (g *ManagementPlane) SecurityService() v1connect.SecurityServiceClient {
	return g.securityService
}

func (g *ManagementPlane) SecretService() v1connect.SecretServiceClient {
	return g.secretService
}

func (g *ManagementPlane) ServiceAccountService() v1connect.ServiceAccountServiceClient {
	return g.serviceAccountService
}

func (g *ManagementPlane) SessionService() v1connect.SessionServiceClient {
	return g.sessionService
}

func (g *ManagementPlane) GatewayService() v1connect.GatewayServiceClient {
	return g.gatewayService
}

func (g *ManagementPlane) RunnerManagerService() v1connect.RunnerManagerServiceClient {
	return g.runnerManagerService
}

func (g *ManagementPlane) OnaIntelligenceService() v1connect.OnaIntelligenceServiceClient {
	return g.onaIntelligenceService
}

func (g *ManagementPlane) WorkflowService() v1connect.WorkflowServiceClient {
	return g.workflowService
}

func (g *ManagementPlane) WebhookService() v1connect.WebhookServiceClient {
	return g.webhookService
}

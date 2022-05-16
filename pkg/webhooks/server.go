package webhooks

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	urinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	urlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	informers "k8s.io/client-go/informers/core/v1"
	rbacinformer "k8s.io/client-go/informers/rbac/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	rbaclister "k8s.io/client-go/listers/rbac/v1"
)

type Handlers interface {
	// Mutate performs the mutation of policy resources
	Mutate(logr.Logger, *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse
	// Validate performs the validation check on policy resources
	Validate(logr.Logger, *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse
}

// WebhookServer contains configured TLS server with MutationWebhook.
type WebhookServer struct {
	server *http.Server

	// clients
	client        client.Interface
	kyvernoClient kyvernoclient.Interface

	// listers
	urLister  urlister.UpdateRequestNamespaceLister
	rbLister  rbaclister.RoleBindingLister
	rLister   rbaclister.RoleLister
	crLister  rbaclister.ClusterRoleLister
	crbLister rbaclister.ClusterRoleBindingLister
	nsLister  listerv1.NamespaceLister

	// generate events
	eventGen event.Interface

	// policy cache
	pCache policycache.Interface

	// webhook registration client
	webhookRegister *webhookconfig.Register

	// helpers to validate against current loaded configuration
	configuration config.Configuration

	// channel for cleanup notification
	cleanUp chan<- struct{}

	// last request time
	webhookMonitor *webhookconfig.Monitor

	// policy report generator
	prGenerator policyreport.GeneratorInterface

	// update request generator
	urGenerator webhookgenerate.Interface

	auditHandler AuditHandler

	log logr.Logger

	openAPIController *openapi.Controller

	urController *background.Controller

	promConfig *metrics.PromConfig

	mu sync.RWMutex
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	policyHandlers Handlers,
	kyvernoClient kyvernoclient.Interface,
	client client.Interface,
	tlsPair func() ([]byte, []byte, error),
	urInformer urinformer.UpdateRequestInformer,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	rbInformer rbacinformer.RoleBindingInformer,
	crbInformer rbacinformer.ClusterRoleBindingInformer,
	rInformer rbacinformer.RoleInformer,
	crInformer rbacinformer.ClusterRoleInformer,
	namespace informers.NamespaceInformer,
	eventGen event.Interface,
	pCache policycache.Interface,
	webhookRegistrationClient *webhookconfig.Register,
	webhookMonitor *webhookconfig.Monitor,
	configHandler config.Configuration,
	prGenerator policyreport.GeneratorInterface,
	urGenerator webhookgenerate.Interface,
	auditHandler AuditHandler,
	cleanUp chan<- struct{},
	log logr.Logger,
	openAPIController *openapi.Controller,
	urc *background.Controller,
	promConfig *metrics.PromConfig,
) (*WebhookServer, error) {
	if tlsPair == nil {
		return nil, errors.New("NewWebhookServer is not initialized properly")
	}
	ws := &WebhookServer{
		client:            client,
		kyvernoClient:     kyvernoClient,
		urLister:          urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
		rbLister:          rbInformer.Lister(),
		rLister:           rInformer.Lister(),
		nsLister:          namespace.Lister(),
		crbLister:         crbInformer.Lister(),
		crLister:          crInformer.Lister(),
		eventGen:          eventGen,
		pCache:            pCache,
		webhookRegister:   webhookRegistrationClient,
		configuration:     configHandler,
		cleanUp:           cleanUp,
		webhookMonitor:    webhookMonitor,
		prGenerator:       prGenerator,
		urGenerator:       urGenerator,
		urController:      urc,
		auditHandler:      auditHandler,
		log:               log,
		openAPIController: openAPIController,
		promConfig:        promConfig,
	}
	mux := httprouter.New()
	resourceLogger := ws.log.WithName("resource")
	policyLogger := ws.log.WithName("policy")
	verifyLogger := ws.log.WithName("verify")
	mux.HandlerFunc("POST", config.MutatingWebhookServicePath, ws.admissionHandler(resourceLogger.WithName("mutate"), true, ws.resourceMutation))
	mux.HandlerFunc("POST", config.ValidatingWebhookServicePath, ws.admissionHandler(resourceLogger.WithName("validate"), true, ws.resourceValidation))
	mux.HandlerFunc("POST", config.PolicyMutatingWebhookServicePath, ws.admissionHandler(policyLogger.WithName("mutate"), true, policyHandlers.Mutate))
	mux.HandlerFunc("POST", config.PolicyValidatingWebhookServicePath, ws.admissionHandler(policyLogger.WithName("validate"), true, policyHandlers.Validate))
	mux.HandlerFunc("POST", config.VerifyMutatingWebhookServicePath, ws.admissionHandler(verifyLogger.WithName("mutate"), false, handlers.Verify(ws.webhookMonitor, ws.log.WithName("verifyHandler"))))
	mux.HandlerFunc("GET", config.LivenessServicePath, handlers.Probe(ws.webhookRegister.Check))
	mux.HandlerFunc("GET", config.ReadinessServicePath, handlers.Probe(nil))
	ws.server = &http.Server{
		Addr: ":9443", // Listen on port for HTTPS requests
		TLSConfig: &tls.Config{
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				certPem, keyPem, err := tlsPair()
				if err != nil {
					return nil, err
				}
				pair, err := tls.X509KeyPair(certPem, keyPem)
				if err != nil {
					return nil, err
				}
				return &pair, nil
			},
			MinVersion: tls.VersionTLS12,
		},
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	return ws, nil
}

func (ws *WebhookServer) buildPolicyContext(request *admissionv1.AdmissionRequest, addRoles bool) (*engine.PolicyContext, error) {
	userRequestInfo := v1beta1.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
	}

	if addRoles {
		var err error
		userRequestInfo.Roles, userRequestInfo.ClusterRoles, err = userinfo.GetRoleRef(ws.rbLister, ws.crbLister, request, ws.configuration)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch RBAC information for request")
		}
	}

	ctx, err := newVariablesContext(request, &userRequestInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create policy rule context")
	}

	resource, err := convertResource(request, request.Object.Raw)
	if err != nil {
		return nil, err
	}

	if err := ctx.AddImageInfos(&resource); err != nil {
		return nil, errors.Wrap(err, "failed to add image information to the policy rule context")
	}

	policyContext := &engine.PolicyContext{
		NewResource:         resource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    ws.configuration.GetExcludeGroupRole(),
		ExcludeResourceFunc: ws.configuration.ToFilter,
		JSONContext:         ctx,
		Client:              ws.client,
		AdmissionOperation:  true,
	}

	if request.Operation == admissionv1.Update {
		policyContext.OldResource, err = convertResource(request, request.OldObject.Raw)
		if err != nil {
			return nil, err
		}
	}

	return policyContext, nil
}

// convertResource converts RAW to unstructured
func convertResource(request *admissionv1.AdmissionRequest, resourceRaw []byte) (unstructured.Unstructured, error) {
	resource, err := utils.ConvertResource(resourceRaw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
	if err != nil {
		return unstructured.Unstructured{}, errors.Wrap(err, "failed to convert raw resource to unstructured format")
	}

	if request.Kind.Kind == "Secret" && request.Operation == admissionv1.Update {
		resource, err = utils.NormalizeSecret(&resource)
		if err != nil {
			return unstructured.Unstructured{}, errors.Wrap(err, "failed to convert secret to unstructured format")
		}
	}

	return resource, nil
}

// RunAsync TLS server in separate thread and returns control immediately
func (ws *WebhookServer) RunAsync(stopCh <-chan struct{}) {
	go func() {
		ws.log.V(3).Info("started serving requests", "addr", ws.server.Addr)
		if err := ws.server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			ws.log.Error(err, "failed to listen to requests")
		}
	}()
	ws.log.Info("starting service")
}

// Stop TLS server and returns control after the server is shut down
func (ws *WebhookServer) Stop(ctx context.Context) {
	// remove the static webhook configurations
	go ws.webhookRegister.Remove(ws.cleanUp)
	// shutdown http.Server with context timeout
	err := ws.server.Shutdown(ctx)
	if err != nil {
		// Error from closing listeners, or context timeout:
		ws.log.Error(err, "shutting down server")
		err = ws.server.Close()
		if err != nil {
			ws.log.Error(err, "server shut down failed")
		}
	}
}

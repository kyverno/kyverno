package webhooks

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	urinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	urlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	tlsutils "github.com/kyverno/kyverno/pkg/tls"
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
	"k8s.io/client-go/tools/cache"
)

// WebhookServer contains configured TLS server with MutationWebhook.
type WebhookServer struct {
	server        *http.Server
	client        *client.Client
	kyvernoClient *kyvernoclient.Clientset

	// grLister can list/get generate request from the shared informer's store
	grLister kyvernolister.GenerateRequestNamespaceLister

	// grSynced returns true if the Generate Request store has been synced at least once
	grSynced cache.InformerSynced

	// urLister can list/get update requests from the shared informer's store
	urLister urlister.UpdateRequestNamespaceLister

	// urSynced returns true if the Update Request store has been synced at least once
	urSynced cache.InformerSynced

	// returns true if the cluster policy store has synced atleast
	pSynced cache.InformerSynced

	// list/get role binding resource
	rbLister rbaclister.RoleBindingLister

	// list/get role binding resource
	rLister rbaclister.RoleLister

	// list/get role binding resource
	crLister rbaclister.ClusterRoleLister

	// return true if role bining store has synced atleast once
	rbSynced cache.InformerSynced

	// return true if role store has synced atleast once
	rSynced cache.InformerSynced

	// list/get cluster role binding resource
	crbLister rbaclister.ClusterRoleBindingLister

	// return true if cluster role binding store has synced atleast once
	crbSynced cache.InformerSynced

	// return true if cluster role  store has synced atleast once
	crSynced cache.InformerSynced

	// generate events
	eventGen event.Interface

	// policy cache
	pCache policycache.Interface

	// webhook registration client
	webhookRegister *webhookconfig.Register

	// helpers to validate against current loaded configuration
	configHandler config.Interface

	// channel for cleanup notification
	cleanUp chan<- struct{}

	// last request time
	webhookMonitor *webhookconfig.Monitor

	// policy report generator
	prGenerator policyreport.GeneratorInterface

	// generate request generator
	grGenerator webhookgenerate.Interface

	nsLister listerv1.NamespaceLister

	// nsListerSynced returns true if the namespace store has been synced at least once
	nsListerSynced cache.InformerSynced

	auditHandler AuditHandler

	log logr.Logger

	openAPIController *openapi.Controller

	grController *background.Controller

	promConfig *metrics.PromConfig

	mu sync.RWMutex
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	kyvernoClient *kyvernoclient.Clientset,
	client *client.Client,
	tlsPair *tlsutils.PemPair,
	grInformer kyvernoinformer.GenerateRequestInformer,
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
	configHandler config.Interface,
	prGenerator policyreport.GeneratorInterface,
	grGenerator webhookgenerate.Interface,
	auditHandler AuditHandler,
	cleanUp chan<- struct{},
	log logr.Logger,
	openAPIController *openapi.Controller,
	grc *background.Controller,
	promConfig *metrics.PromConfig,
) (*WebhookServer, error) {
	if tlsPair == nil {
		return nil, errors.New("NewWebhookServer is not initialized properly")
	}
	pair, err := tls.X509KeyPair(tlsPair.Certificate, tlsPair.PrivateKey)
	if err != nil {
		return nil, err
	}
	ws := &WebhookServer{
		client:            client,
		kyvernoClient:     kyvernoClient,
		grLister:          grInformer.Lister().GenerateRequests(config.KyvernoNamespace),
		grSynced:          grInformer.Informer().HasSynced,
		urLister:          urInformer.Lister().UpdateRequests(config.KyvernoNamespace),
		urSynced:          urInformer.Informer().HasSynced,
		pSynced:           pInformer.Informer().HasSynced,
		rbLister:          rbInformer.Lister(),
		rbSynced:          rbInformer.Informer().HasSynced,
		rLister:           rInformer.Lister(),
		rSynced:           rInformer.Informer().HasSynced,
		nsLister:          namespace.Lister(),
		nsListerSynced:    namespace.Informer().HasSynced,
		crbLister:         crbInformer.Lister(),
		crLister:          crInformer.Lister(),
		crbSynced:         crbInformer.Informer().HasSynced,
		crSynced:          crInformer.Informer().HasSynced,
		eventGen:          eventGen,
		pCache:            pCache,
		webhookRegister:   webhookRegistrationClient,
		configHandler:     configHandler,
		cleanUp:           cleanUp,
		webhookMonitor:    webhookMonitor,
		prGenerator:       prGenerator,
		grGenerator:       grGenerator,
		grController:      grc,
		auditHandler:      auditHandler,
		log:               log,
		openAPIController: openAPIController,
		promConfig:        promConfig,
	}
	mux := httprouter.New()
	mux.HandlerFunc("POST", config.MutatingWebhookServicePath, ws.admissionHandler(true, ws.resourceMutation))
	mux.HandlerFunc("POST", config.ValidatingWebhookServicePath, ws.admissionHandler(true, ws.resourceValidation))
	mux.HandlerFunc("POST", config.PolicyMutatingWebhookServicePath, ws.admissionHandler(true, ws.policyMutation))
	mux.HandlerFunc("POST", config.PolicyValidatingWebhookServicePath, ws.admissionHandler(true, ws.policyValidation))
	mux.HandlerFunc("POST", config.VerifyMutatingWebhookServicePath, ws.admissionHandler(false, handlers.Verify(ws.webhookMonitor, ws.log.WithName("verifyHandler"))))
	mux.HandlerFunc("GET", config.LivenessServicePath, handlers.Probe(ws.webhookRegister.Check))
	mux.HandlerFunc("GET", config.ReadinessServicePath, handlers.Probe(nil))
	ws.server = &http.Server{
		Addr:         ":9443", // Listen on port for HTTPS requests
		TLSConfig:    &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12},
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	return ws, nil
}

func (ws *WebhookServer) buildPolicyContext(request *admissionv1.AdmissionRequest, addRoles bool) (*engine.PolicyContext, error) {
	userRequestInfo := v1beta1.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
	}

	if addRoles {
		var err error
		userRequestInfo.Roles, userRequestInfo.ClusterRoles, err = userinfo.GetRoleRef(ws.rbLister, ws.crbLister, request, ws.configHandler)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch RBAC information for request")
		}
	}

	ctx, err := newVariablesContext(request, &userRequestInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create policy rule context")
	}

	// convert RAW to unstructured
	resource, err := utils.ConvertResource(request.Object.Raw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert raw resource to unstructured format")
	}

	if err := ctx.AddImageInfos(&resource); err != nil {
		return nil, errors.Wrap(err, "failed to add image information to the policy rule context")
	}

	if request.Kind.Kind == "Secret" && request.Operation == admissionv1.Update {
		resource, err = utils.NormalizeSecret(&resource)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert secret to unstructured format")
		}
	}

	policyContext := &engine.PolicyContext{
		NewResource:         resource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    ws.configHandler.GetExcludeGroupRole(),
		ExcludeResourceFunc: ws.configHandler.ToFilter,
		JSONContext:         ctx,
		Client:              ws.client,
		AdmissionOperation:  true,
	}

	if request.Operation == admissionv1.Update {
		policyContext.OldResource = resource
	}

	return policyContext, nil
}

// RunAsync TLS server in separate thread and returns control immediately
func (ws *WebhookServer) RunAsync(stopCh <-chan struct{}) {
	if !cache.WaitForCacheSync(stopCh, ws.grSynced, ws.urSynced, ws.pSynced, ws.rbSynced, ws.crbSynced, ws.rSynced, ws.crSynced) {
		ws.log.Info("failed to sync informer cache")
	}
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

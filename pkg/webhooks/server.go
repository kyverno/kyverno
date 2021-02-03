package webhooks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	context2 "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/generate"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/policystatus"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	tlsutils "github.com/kyverno/kyverno/pkg/tls"
	userinfo "github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/generate"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// list/get cluster policy resource
	pLister kyvernolister.ClusterPolicyLister

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

	// API to send policy stats for aggregation
	statusListener policystatus.Listener

	// helpers to validate against current loaded configuration
	configHandler config.Interface

	// channel for cleanup notification
	cleanUp chan<- struct{}

	// last request time
	webhookMonitor *webhookconfig.Monitor

	// policy report generator
	prGenerator policyreport.GeneratorInterface

	// generate request generator
	grGenerator *webhookgenerate.Generator

	nsLister listerv1.NamespaceLister

	// nsListerSynced returns true if the namespace store has been synced at least once
	nsListerSynced cache.InformerSynced

	auditHandler AuditHandler

	log logr.Logger

	openAPIController *openapi.Controller

	supportMutateValidate bool

	// resCache - controls creation and fetching of resource informer cache
	resCache resourcecache.ResourceCache

	grController *generate.Controller

	debug bool
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	kyvernoClient *kyvernoclient.Clientset,
	client *client.Client,
	tlsPair *tlsutils.PemPair,
	grInformer kyvernoinformer.GenerateRequestInformer,
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
	statusSync policystatus.Listener,
	configHandler config.Interface,
	prGenerator policyreport.GeneratorInterface,
	grGenerator *webhookgenerate.Generator,
	auditHandler AuditHandler,
	supportMutateValidate bool,
	cleanUp chan<- struct{},
	log logr.Logger,
	openAPIController *openapi.Controller,
	resCache resourcecache.ResourceCache,
	grc *generate.Controller,
	debug bool,
) (*WebhookServer, error) {

	if tlsPair == nil {
		return nil, errors.New("NewWebhookServer is not initialized properly")
	}

	var tlsConfig tls.Config
	pair, err := tls.X509KeyPair(tlsPair.Certificate, tlsPair.PrivateKey)
	if err != nil {
		return nil, err
	}
	tlsConfig.Certificates = []tls.Certificate{pair}

	ws := &WebhookServer{
		client:         client,
		kyvernoClient:  kyvernoClient,
		grLister:       grInformer.Lister().GenerateRequests(config.KyvernoNamespace),
		grSynced:       grInformer.Informer().HasSynced,
		pLister:        pInformer.Lister(),
		pSynced:        pInformer.Informer().HasSynced,
		rbLister:       rbInformer.Lister(),
		rbSynced:       rbInformer.Informer().HasSynced,
		rLister:        rInformer.Lister(),
		rSynced:        rInformer.Informer().HasSynced,
		nsLister:       namespace.Lister(),
		nsListerSynced: namespace.Informer().HasSynced,

		crbLister:             crbInformer.Lister(),
		crLister:              crInformer.Lister(),
		crbSynced:             crbInformer.Informer().HasSynced,
		crSynced:              crInformer.Informer().HasSynced,
		eventGen:              eventGen,
		pCache:                pCache,
		webhookRegister:       webhookRegistrationClient,
		statusListener:        statusSync,
		configHandler:         configHandler,
		cleanUp:               cleanUp,
		webhookMonitor:        webhookMonitor,
		prGenerator:           prGenerator,
		grGenerator:           grGenerator,
		grController:          grc,
		auditHandler:          auditHandler,
		log:                   log,
		openAPIController:     openAPIController,
		supportMutateValidate: supportMutateValidate,
		resCache:              resCache,
		debug:                 debug,
	}

	mux := httprouter.New()
	mux.HandlerFunc("POST", config.MutatingWebhookServicePath, ws.handlerFunc(ws.ResourceMutation, true))
	mux.HandlerFunc("POST", config.ValidatingWebhookServicePath, ws.handlerFunc(ws.resourceValidation, true))
	mux.HandlerFunc("POST", config.PolicyMutatingWebhookServicePath, ws.handlerFunc(ws.policyMutation, true))
	mux.HandlerFunc("POST", config.PolicyValidatingWebhookServicePath, ws.handlerFunc(ws.policyValidation, true))
	mux.HandlerFunc("POST", config.VerifyMutatingWebhookServicePath, ws.handlerFunc(ws.verifyHandler, false))

	// Handle Liveness responds to a Kubernetes Liveness probe
	// Fail this request if Kubernetes should restart this instance
	mux.HandlerFunc("GET", config.LivenessServicePath, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.WriteHeader(http.StatusOK)
	})

	// Handle Readiness responds to a Kubernetes Readiness probe
	// Fail this request if this instance can't accept traffic, but Kubernetes shouldn't restart it
	mux.HandlerFunc("GET", config.ReadinessServicePath, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.WriteHeader(http.StatusOK)
	})

	ws.server = &http.Server{
		Addr:         ":9443", // Listen on port for HTTPS requests
		TLSConfig:    &tlsConfig,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return ws, nil
}

func (ws *WebhookServer) handlerFunc(handler func(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse, filter bool) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		ws.webhookMonitor.SetTime(startTime)

		admissionReview := ws.bodyToAdmissionReview(r, rw)
		if admissionReview == nil {
			ws.log.Info("failed to parse admission review request", "request", r)
			return
		}

		logger := ws.log.WithName("handlerFunc").WithValues("kind", admissionReview.Request.Kind, "namespace", admissionReview.Request.Namespace,
			"name", admissionReview.Request.Name, "operation", admissionReview.Request.Operation, "uid", admissionReview.Request.UID)

		admissionReview.Response = &v1beta1.AdmissionResponse{
			Allowed: true,
			UID:     admissionReview.Request.UID,
		}

		// Do not process the admission requests for kinds that are in filterKinds for filtering
		request := admissionReview.Request
		if filter && ws.configHandler.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
			writeResponse(rw, admissionReview)
			return
		}

		admissionReview.Response = handler(request)
		writeResponse(rw, admissionReview)
		logger.V(4).Info("admission review request processed", "time", time.Since(startTime).String())

		return
	}
}

func writeResponse(rw http.ResponseWriter, admissionReview *v1beta1.AdmissionReview) {
	responseJSON, err := json.Marshal(admissionReview)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Could not encode response: %v", err), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := rw.Write(responseJSON); err != nil {
		http.Error(rw, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

// ResourceMutation mutates resource
func (ws *WebhookServer) ResourceMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {

	logger := ws.log.WithName("ResourceMutation").WithValues("uid", request.UID, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)

	if excludeKyvernoResources(request.Kind.Kind) {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status: "Success",
			},
		}
	}
	logger.V(6).Info("received an admission request in mutating webhook")
	mutatePolicies := ws.pCache.Get(policycache.Mutate, nil)
	validatePolicies := ws.pCache.Get(policycache.ValidateEnforce, nil)
	generatePolicies := ws.pCache.Get(policycache.Generate, nil)

	// Get namespace policies from the cache for the requested resource namespace
	nsMutatePolicies := ws.pCache.Get(policycache.Mutate, &request.Namespace)
	mutatePolicies = append(mutatePolicies, nsMutatePolicies...)

	// getRoleRef only if policy has roles/clusterroles defined
	var roles, clusterRoles []string
	var err error
	if containRBACInfo(mutatePolicies, validatePolicies, generatePolicies) {
		roles, clusterRoles, err = userinfo.GetRoleRef(ws.rbLister, ws.crbLister, request, ws.configHandler)
		if err != nil {
			logger.Error(err, "failed to get RBAC information for request")
		}
	}

	// convert RAW to unstructured
	resource, err := utils.ConvertResource(request.Object.Raw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
	if err != nil {
		logger.Error(err, "failed to convert RAW resource to unstructured format")
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: err.Error(),
			},
		}
	}

	userRequestInfo := v1.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: *request.UserInfo.DeepCopy()}

	// build context
	ctx := context2.NewContext()
	err = ctx.AddRequest(request)
	if err != nil {
		logger.Error(err, "failed to load incoming request in context")
	}

	err = ctx.AddUserInfo(userRequestInfo)
	if err != nil {
		logger.Error(err, "failed to load userInfo in context")
	}
	err = ctx.AddServiceAccount(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to load service account in context")
	}

	var patches []byte
	patchedResource := request.Object.Raw

	// MUTATION
	if ws.supportMutateValidate {
		if resource.GetDeletionTimestamp() == nil {
			patches = ws.HandleMutation(request, resource, mutatePolicies, ctx, userRequestInfo)
			logger.V(6).Info("", "generated patches", string(patches))

			// patch the resource with patches before handling validation rules
			patchedResource = processResourceWithPatches(patches, request.Object.Raw, logger)
			logger.V(6).Info("", "patchedResource", string(patchedResource))
		}
	} else {
		logger.Info("mutate rules are not supported prior to Kubernetes 1.14.0")
	}

	// GENERATE
	if request.Operation == v1beta1.Create || request.Operation == v1beta1.Update {
		newRequest := request.DeepCopy()
		newRequest.Object.Raw = patchedResource
		go ws.HandleGenerate(newRequest, generatePolicies, ctx, userRequestInfo, ws.configHandler)
	}

	patchType := v1beta1.PatchTypeJSONPatch
	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Status: "Success",
		},
		Patch:     patches,
		PatchType: &patchType,
	}
}

func (ws *WebhookServer) resourceValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithName("Validate").WithValues("uid", request.UID, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	if request.Operation == v1beta1.Delete {
		ws.handleDelete(request)
	}

	if !ws.supportMutateValidate {
		logger.Info("mutate and validate rules are not supported prior to Kubernetes 1.14.0")
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status: "Success",
			},
		}
	}

	if excludeKyvernoResources(request.Kind.Kind) {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status: "Success",
			},
		}
	}

	logger.V(6).Info("received an admission request in validating webhook")

	// push admission request to audit handler, this won't block the admission request
	ws.auditHandler.Add(request.DeepCopy())

	policies := ws.pCache.Get(policycache.ValidateEnforce, nil)
	// Get namespace policies from the cache for the requested resource namespace
	nsPolicies := ws.pCache.Get(policycache.ValidateEnforce, &request.Namespace)
	policies = append(policies, nsPolicies...)
	if len(policies) == 0 {
		logger.V(4).Info("no enforce validation policies; returning AdmissionResponse.Allowed: true")
		return &v1beta1.AdmissionResponse{Allowed: true}
	}

	var roles, clusterRoles []string
	var err error
	// getRoleRef only if policy has roles/clusterroles defined
	if containRBACInfo(policies) {
		roles, clusterRoles, err = userinfo.GetRoleRef(ws.rbLister, ws.crbLister, request, ws.configHandler)
		if err != nil {
			logger.Error(err, "failed to get RBAC information for request")
			return &v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Status:  "Failure",
					Message: err.Error(),
				},
			}
		}
		logger = logger.WithValues("username", request.UserInfo.Username,
			"groups", request.UserInfo.Groups, "roles", roles, "clusterRoles", clusterRoles)
	}

	userRequestInfo := v1.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: request.UserInfo}

	// build context
	ctx := context2.NewContext()
	err = ctx.AddRequest(request)
	if err != nil {
		logger.Error(err, "failed to load incoming request in context")
	}

	err = ctx.AddUserInfo(userRequestInfo)
	if err != nil {
		logger.Error(err, "failed to load userInfo in context")
	}
	err = ctx.AddServiceAccount(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to load service account in context")
	}

	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, ws.nsLister, logger)
	}

	ok, msg := HandleValidation(request, policies, nil, ctx, userRequestInfo, ws.statusListener, ws.eventGen, ws.prGenerator, ws.log, ws.configHandler, ws.resCache, ws.client, namespaceLabels)
	if !ok {
		logger.Info("admission request denied")
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: msg,
			},
		}
	}

	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Status: "Success",
		},
	}
}

// RunAsync TLS server in separate thread and returns control immediately
func (ws *WebhookServer) RunAsync(stopCh <-chan struct{}) {
	logger := ws.log
	if !cache.WaitForCacheSync(stopCh, ws.grSynced, ws.pSynced, ws.rbSynced, ws.crbSynced, ws.rSynced, ws.crSynced) {
		logger.Info("failed to sync informer cache")
	}

	go func() {
		logger.V(3).Info("started serving requests", "addr", ws.server.Addr)
		if err := ws.server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			logger.Error(err, "failed to listen to requests")
		}
	}()

	logger.Info("starting service")

	if !ws.debug {
		go ws.webhookMonitor.Run(ws.webhookRegister, ws.eventGen, ws.client, stopCh)
	}
}

// Stop TLS server and returns control after the server is shut down
func (ws *WebhookServer) Stop(ctx context.Context) {
	logger := ws.log

	// remove the static webhook configurations
	go ws.webhookRegister.Remove(ws.cleanUp)

	// shutdown http.Server with context timeout
	err := ws.server.Shutdown(ctx)
	if err != nil {
		// Error from closing listeners, or context timeout:
		logger.Error(err, "shutting down server")
		ws.server.Close()
	}
}

// bodyToAdmissionReview creates AdmissionReview object from request body
// Answers to the http.ResponseWriter if request is not valid
func (ws *WebhookServer) bodyToAdmissionReview(request *http.Request, writer http.ResponseWriter) *v1beta1.AdmissionReview {
	logger := ws.log
	if request.Body == nil {
		logger.Info("empty body", "req", request.URL.String())
		http.Error(writer, "empty body", http.StatusBadRequest)
		return nil
	}

	defer request.Body.Close()
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		logger.Info("failed to read HTTP body", "req", request.URL.String())
		http.Error(writer, "failed to read HTTP body", http.StatusBadRequest)
	}

	contentType := request.Header.Get("Content-Type")
	if contentType != "application/json" {
		logger.Info("invalid Content-Type", "contextType", contentType)
		http.Error(writer, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return nil
	}

	admissionReview := &v1beta1.AdmissionReview{}
	if err := json.Unmarshal(body, &admissionReview); err != nil {
		logger.Error(err, "failed to decode request body to type 'AdmissionReview")
		http.Error(writer, "Can't decode body as AdmissionReview", http.StatusExpectationFailed)
		return nil
	}

	return admissionReview
}

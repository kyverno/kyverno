package webhooks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/generate"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionRequests "github.com/kyverno/kyverno/pkg/metrics/admissionrequests"
	admissionReviewDuration "github.com/kyverno/kyverno/pkg/metrics/admissionreviewduration"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	tlsutils "github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/generate"
	"github.com/pkg/errors"
	"k8s.io/api/admission/v1beta1"
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

	// resCache - controls creation and fetching of resource informer cache
	resCache resourcecache.ResourceCache

	grController *generate.Controller

	promConfig *metrics.PromConfig
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
	configHandler config.Interface,
	prGenerator policyreport.GeneratorInterface,
	grGenerator *webhookgenerate.Generator,
	auditHandler AuditHandler,
	cleanUp chan<- struct{},
	log logr.Logger,
	openAPIController *openapi.Controller,
	resCache resourcecache.ResourceCache,
	grc *generate.Controller,
	promConfig *metrics.PromConfig,
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
		resCache:          resCache,
		promConfig:        promConfig,
	}

	mux := httprouter.New()
	mux.HandlerFunc("POST", config.MutatingWebhookServicePath, ws.handlerFunc(ws.resourceMutation, true))
	mux.HandlerFunc("POST", config.ValidatingWebhookServicePath, ws.handlerFunc(ws.resourceValidation, true))
	mux.HandlerFunc("POST", config.PolicyMutatingWebhookServicePath, ws.handlerFunc(ws.policyMutation, true))
	mux.HandlerFunc("POST", config.PolicyValidatingWebhookServicePath, ws.handlerFunc(ws.policyValidation, true))
	mux.HandlerFunc("POST", config.VerifyMutatingWebhookServicePath, ws.handlerFunc(ws.verifyHandler, false))

	// Handle Liveness responds to a Kubernetes Liveness probe
	// Fail this request if Kubernetes should restart this instance
	mux.HandlerFunc("GET", config.LivenessServicePath, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := ws.webhookRegister.Check(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
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

// resourceMutation mutates resource
func (ws *WebhookServer) resourceMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithName("MutateWebhook").WithValues("uid", request.UID, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation, "gvk", request.Kind.String())

	if excludeKyvernoResources(request.Kind.Kind) {
		return successResponse(nil)
	}

	logger.V(4).Info("received an admission request in mutating webhook")
	requestTime := time.Now().Unix()
	kind := request.Kind.Kind
	mutatePolicies := ws.pCache.GetPolicies(policycache.Mutate, kind, request.Namespace)
	verifyImagesPolicies := ws.pCache.GetPolicies(policycache.VerifyImages, kind, request.Namespace)

	if len(mutatePolicies) == 0 && len(verifyImagesPolicies) == 0 {
		logger.V(4).Info("no policies matched admission request")
		return successResponse(nil)
	}

	addRoles := containsRBACInfo(mutatePolicies)
	policyContext, err := ws.buildPolicyContext(request, addRoles)
	if err != nil {
		logger.Error(err, "failed to build policy context")
		return failureResponse(err.Error())
	}

	// update container images to a canonical form
	if err := enginectx.MutateResourceWithImageInfo(request.Object.Raw, policyContext.JSONContext); err != nil {
		ws.log.Error(err, "failed to patch images info to resource, policies that mutate images may be impacted")
	}

	mutatePatches := ws.applyMutatePolicies(request, policyContext, mutatePolicies, requestTime, logger)

	newRequest := patchRequest(mutatePatches, request, logger)
	imagePatches, err := ws.applyImageVerifyPolicies(newRequest, policyContext, verifyImagesPolicies, logger)
	if err != nil {
		logger.Error(err, "image verification failed")
		return failureResponse(err.Error())
	}

	var patches = append(mutatePatches, imagePatches...)
	return successResponse(patches)
}

// patchRequest applies patches to the request.Object and returns a new copy of the request
func patchRequest(patches []byte, request *v1beta1.AdmissionRequest, logger logr.Logger) *v1beta1.AdmissionRequest {
	patchedResource := processResourceWithPatches(patches, request.Object.Raw, logger)
	newRequest := request.DeepCopy()
	newRequest.Object.Raw = patchedResource
	return newRequest
}

func (ws *WebhookServer) buildPolicyContext(request *v1beta1.AdmissionRequest, addRoles bool) (*engine.PolicyContext, error) {
	userRequestInfo := v1.RequestInfo{
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

	if err := ctx.AddImageInfo(&resource); err != nil {
		return nil, errors.Wrap(err, "failed to add image information to the policy rule context")
	}

	policyContext := &engine.PolicyContext{
		NewResource:         resource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    ws.configHandler.GetExcludeGroupRole(),
		ExcludeResourceFunc: ws.configHandler.ToFilter,
		ResourceCache:       ws.resCache,
		JSONContext:         ctx,
		Client:              ws.client,
	}

	if request.Operation == v1beta1.Update {
		policyContext.OldResource = resource
	}

	return policyContext, nil
}

func successResponse(patch []byte) *v1beta1.AdmissionResponse {
	r := &v1beta1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Status: "Success",
		},
	}

	if len(patch) > 0 {
		patchType := v1beta1.PatchTypeJSONPatch
		r.PatchType = &patchType
		r.Patch = patch
	}

	return r
}

func errorResponse(logger logr.Logger, err error, message string) *v1beta1.AdmissionResponse {
	logger.Error(err, message)
	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Status:  "Failure",
			Message: message + ": " + err.Error(),
		},
	}
}

func failureResponse(message string) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Status:  "Failure",
			Message: message,
		},
	}
}

func registerAdmissionReviewDurationMetricMutate(logger logr.Logger, promConfig metrics.PromConfig, requestOperation string, engineResponses []*response.EngineResponse, admissionReviewLatencyDuration int64) {
	resourceRequestOperationPromAlias, err := admissionReviewDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
	if err := admissionReviewDuration.ParsePromConfig(promConfig).ProcessEngineResponses(engineResponses, admissionReviewLatencyDuration, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
}

func registerAdmissionRequestsMetricMutate(logger logr.Logger, promConfig metrics.PromConfig, requestOperation string, engineResponses []*response.EngineResponse) {
	resourceRequestOperationPromAlias, err := admissionReviewDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
	if err := admissionRequests.ParsePromConfig(promConfig).ProcessEngineResponses(engineResponses, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
}

func registerAdmissionReviewDurationMetricGenerate(logger logr.Logger, promConfig metrics.PromConfig, requestOperation string, latencyReceiver *chan int64, engineResponsesReceiver *chan []*response.EngineResponse) {
	defer close(*latencyReceiver)
	defer close(*engineResponsesReceiver)

	engineResponses := <-(*engineResponsesReceiver)

	resourceRequestOperationPromAlias, err := admissionReviewDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
	// this goroutine will keep on waiting here till it doesn't receive the admission review latency int64 from the other goroutine i.e. ws.HandleGenerate
	admissionReviewLatencyDuration := <-(*latencyReceiver)
	if err := admissionReviewDuration.ParsePromConfig(promConfig).ProcessEngineResponses(engineResponses, admissionReviewLatencyDuration, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
}

func registerAdmissionRequestsMetricGenerate(logger logr.Logger, promConfig metrics.PromConfig, requestOperation string, engineResponsesReceiver *chan []*response.EngineResponse) {
	defer close(*engineResponsesReceiver)
	engineResponses := <-(*engineResponsesReceiver)

	resourceRequestOperationPromAlias, err := admissionReviewDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
	if err := admissionRequests.ParsePromConfig(promConfig).ProcessEngineResponses(engineResponses, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
}

func (ws *WebhookServer) resourceValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithName("ValidateWebhook").WithValues("uid", request.UID, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	if request.Operation == v1beta1.Delete {
		ws.handleDelete(request)
	}

	if excludeKyvernoResources(request.Kind.Kind) {
		return successResponse(nil)
	}

	logger.V(6).Info("received an admission request in validating webhook")
	// timestamp at which this admission request got triggered
	admissionRequestTimestamp := time.Now().Unix()
	kind := request.Kind.Kind
	policies := ws.pCache.GetPolicies(policycache.ValidateEnforce, kind, "")
	// Get namespace policies from the cache for the requested resource namespace
	nsPolicies := ws.pCache.GetPolicies(policycache.ValidateEnforce, kind, request.Namespace)
	policies = append(policies, nsPolicies...)
	generatePolicies := ws.pCache.GetPolicies(policycache.Generate, kind, request.Namespace)

	if len(generatePolicies) == 0 && request.Operation == v1beta1.Update {
		// handle generate source resource updates
		go ws.handleUpdatesForGenerateRules(request, []*v1.ClusterPolicy{})
	}

	var roles, clusterRoles []string
	if containsRBACInfo(policies, generatePolicies) {
		var err error
		roles, clusterRoles, err = userinfo.GetRoleRef(ws.rbLister, ws.crbLister, request, ws.configHandler)
		if err != nil {
			return errorResponse(logger, err, "failed to fetch RBAC data")
		}
	}

	userRequestInfo := v1.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
	}

	ctx, err := newVariablesContext(request, &userRequestInfo)
	if err != nil {
		return errorResponse(logger, err, "failed create policy rule context")
	}

	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, ws.nsLister, logger)
	}

	newResource, oldResource, err := utils.ExtractResources(nil, request)
	if err != nil {
		return errorResponse(logger, err, "failed create parse resource")
	}

	if err := ctx.AddImageInfo(&newResource); err != nil {
		return errorResponse(logger, err, "failed add image information to policy rule context")
	}

	policyContext := &engine.PolicyContext{
		NewResource:         newResource,
		OldResource:         oldResource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    ws.configHandler.GetExcludeGroupRole(),
		ExcludeResourceFunc: ws.configHandler.ToFilter,
		ResourceCache:       ws.resCache,
		JSONContext:         ctx,
		Client:              ws.client,
	}

	vh := &validationHandler{
		log:         ws.log,
		eventGen:    ws.eventGen,
		prGenerator: ws.prGenerator,
	}

	ok, msg := vh.handleValidation(ws.promConfig, request, policies, policyContext, namespaceLabels, admissionRequestTimestamp)
	if !ok {
		logger.Info("admission request denied")
		return failureResponse(msg)
	}

	// push admission request to audit handler, this won't block the admission request
	ws.auditHandler.Add(request.DeepCopy())

	// process generate policies
	ws.applyGeneratePolicies(request, policyContext, generatePolicies, admissionRequestTimestamp, logger)

	return successResponse(nil)
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
		err = ws.server.Close()
		if err != nil {
			logger.Error(err, "server shut down failed")
		}
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

func newVariablesContext(request *v1beta1.AdmissionRequest, userRequestInfo *v1.RequestInfo) (*enginectx.Context, error) {
	ctx := enginectx.NewContext()
	if err := ctx.AddRequest(request); err != nil {
		return nil, errors.Wrap(err, "failed to load incoming request in context")
	}

	if err := ctx.AddUserInfo(*userRequestInfo); err != nil {
		return nil, errors.Wrap(err, "failed to load userInfo in context")
	}

	if err := ctx.AddServiceAccount(userRequestInfo.AdmissionUserInfo.Username); err != nil {
		return nil, errors.Wrap(err, "failed to load service account in context")
	}

	return ctx, nil
}

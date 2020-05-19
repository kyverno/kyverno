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
	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/checker"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	context2 "github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/openapi"
	"github.com/nirmata/kyverno/pkg/policystatus"
	"github.com/nirmata/kyverno/pkg/policystore"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	tlsutils "github.com/nirmata/kyverno/pkg/tls"
	userinfo "github.com/nirmata/kyverno/pkg/userinfo"
	"github.com/nirmata/kyverno/pkg/utils"
	"github.com/nirmata/kyverno/pkg/webhookconfig"
	"github.com/nirmata/kyverno/pkg/webhooks/generate"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacinformer "k8s.io/client-go/informers/rbac/v1"
	rbaclister "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
)

// WebhookServer contains configured TLS server with MutationWebhook.
type WebhookServer struct {
	server        http.Server
	client        *client.Client
	kyvernoClient *kyvernoclient.Clientset
	// list/get cluster policy resource
	pLister kyvernolister.ClusterPolicyLister
	// returns true if the cluster policy store has synced atleast
	pSynced cache.InformerSynced
	// list/get role binding resource
	rbLister rbaclister.RoleBindingLister
	// return true if role bining store has synced atleast once
	rbSynced cache.InformerSynced
	// list/get cluster role binding resource
	crbLister rbaclister.ClusterRoleBindingLister
	// return true if cluster role binding store has synced atleast once
	crbSynced cache.InformerSynced
	// generate events
	eventGen event.Interface
	// webhook registration client
	webhookRegistrationClient *webhookconfig.WebhookRegistrationClient
	// API to send policy stats for aggregation
	statusListener policystatus.Listener
	// helpers to validate against current loaded configuration
	configHandler config.Interface
	// channel for cleanup notification
	cleanUp chan<- struct{}
	// last request time
	lastReqTime *checker.LastReqTime
	// store to hold policy meta data for faster lookup
	pMetaStore policystore.LookupInterface
	// policy violation generator
	pvGenerator policyviolation.GeneratorInterface
	// generate request generator
	grGenerator            *generate.Generator
	resourceWebhookWatcher *webhookconfig.ResourceWebhookRegister
	log                    logr.Logger
	openAPIController      *openapi.Controller
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	kyvernoClient *kyvernoclient.Clientset,
	client *client.Client,
	tlsPair *tlsutils.TlsPemPair,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	rbInformer rbacinformer.RoleBindingInformer,
	crbInformer rbacinformer.ClusterRoleBindingInformer,
	eventGen event.Interface,
	webhookRegistrationClient *webhookconfig.WebhookRegistrationClient,
	statusSync policystatus.Listener,
	configHandler config.Interface,
	pMetaStore policystore.LookupInterface,
	pvGenerator policyviolation.GeneratorInterface,
	grGenerator *generate.Generator,
	resourceWebhookWatcher *webhookconfig.ResourceWebhookRegister,
	cleanUp chan<- struct{},
	log logr.Logger,
	openAPIController *openapi.Controller,
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
		client:                    client,
		kyvernoClient:             kyvernoClient,
		pLister:                   pInformer.Lister(),
		pSynced:                   pInformer.Informer().HasSynced,
		rbLister:                  rbInformer.Lister(),
		rbSynced:                  rbInformer.Informer().HasSynced,
		crbLister:                 crbInformer.Lister(),
		crbSynced:                 crbInformer.Informer().HasSynced,
		eventGen:                  eventGen,
		webhookRegistrationClient: webhookRegistrationClient,
		statusListener:            statusSync,
		configHandler:             configHandler,
		cleanUp:                   cleanUp,
		lastReqTime:               resourceWebhookWatcher.LastReqTime,
		pvGenerator:               pvGenerator,
		pMetaStore:                pMetaStore,
		grGenerator:               grGenerator,
		resourceWebhookWatcher:    resourceWebhookWatcher,
		log:                       log,
		openAPIController:         openAPIController,
	}

	mux := httprouter.New()
	webhookTimeout := ws.webhookRegistrationClient.GetWebhookTimeOut()
	mux.HandlerFunc("POST", config.MutatingWebhookServicePath, timeoutHandler(ws.handlerFunc(ws.resourceMutation, true), webhookTimeout))
	mux.HandlerFunc("POST", config.ValidatingWebhookServicePath, timeoutHandler(ws.handlerFunc(ws.resourceValidation, true), webhookTimeout))
	mux.HandlerFunc("POST", config.PolicyMutatingWebhookServicePath, timeoutHandler(ws.handlerFunc(ws.policyMutation, true), webhookTimeout))
	mux.HandlerFunc("POST", config.PolicyValidatingWebhookServicePath, timeoutHandler(ws.handlerFunc(ws.policyValidation, true), webhookTimeout))
	mux.HandlerFunc("POST", config.VerifyMutatingWebhookServicePath, timeoutHandler(ws.handlerFunc(ws.verifyHandler, false), webhookTimeout))

	ws.server = http.Server{
		Addr:         ":443", // Listen on port for HTTPS requests
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
		ws.lastReqTime.SetTime(startTime)

		admissionReview := ws.bodyToAdmissionReview(r, rw)
		if admissionReview == nil {
			ws.log.Info("failed to parse admission review request", "request", r)
			return
		}

		logger := ws.log.WithValues("kind", admissionReview.Request.Kind, "namespace", admissionReview.Request.Namespace, "name", admissionReview.Request.Name)

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
		logger.V(4).Info("request processed", "processingTime", time.Since(startTime).Milliseconds())

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

func (ws *WebhookServer) resourceMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithName("resourceMutation").WithValues("uid", request.UID, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	policies, err := ws.pMetaStore.ListAll()
	if err != nil {
		// Unable to connect to policy Lister to access policies
		logger.Error(err, "failed to list policies. Policies are NOT being applied")
		return &v1beta1.AdmissionResponse{Allowed: true}
	}

	// getRoleRef only if policy has roles/clusterroles defined
	var roles, clusterRoles []string
	if containRBACinfo(policies) {
		roles, clusterRoles, err = userinfo.GetRoleRef(ws.rbLister, ws.crbLister, request)
		if err != nil {
			// TODO(shuting): continue apply policy if error getting roleRef?
			logger.Error(err, "failed to get RBAC infromation for request")
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

	if checkPodTemplateAnnotation(resource) {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status: "Success",
			},
		}
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
	err = ctx.AddSA(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to load service account in context")
	}

	var patches []byte
	patchedResource := request.Object.Raw

	higherVersion := utils.HigherThanKubernetesVersion(ws.client, ws.log, 1, 14, 0)
	if higherVersion {
		// MUTATION
		// mutation failure should not block the resource creation
		// any mutation failure is reported as the violation
		patches := ws.HandleMutation(request, resource, policies, ctx, userRequestInfo)

		// patch the resource with patches before handling validation rules
		patchedResource = processResourceWithPatches(patches, request.Object.Raw, logger)

		if ws.resourceWebhookWatcher != nil && ws.resourceWebhookWatcher.RunValidationInMutatingWebhook == "true" {
			// VALIDATION
			ok, msg := ws.HandleValidation(request, policies, patchedResource, ctx, userRequestInfo)
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
		}
	}

	// GENERATE
	// Only applied during resource creation
	// Success -> Generate Request CR created successsfully
	// Failed -> Failed to create Generate Request CR
	if request.Operation == v1beta1.Create {
		ok, msg := ws.HandleGenerate(request, policies, ctx, userRequestInfo)
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
	}

	// Succesfful processing of mutation & validation rules in policy
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
	logger := ws.log.WithName("resourceValidation").WithValues("uid", request.UID, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	policies, err := ws.pMetaStore.ListAll()
	if err != nil {
		// Unable to connect to policy Lister to access policies
		logger.Error(err, "failed to list policies. Policies are NOT being applied")
		return &v1beta1.AdmissionResponse{Allowed: true}
	}

	var roles, clusterRoles []string

	// getRoleRef only if policy has roles/clusterroles defined
	if containRBACinfo(policies) {
		roles, clusterRoles, err = userinfo.GetRoleRef(ws.rbLister, ws.crbLister, request)
		if err != nil {
			// TODO(shuting): continue apply policy if error getting roleRef?
			logger.Error(err, "failed to get RBAC infromation for request")
		}
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
	err = ctx.AddSA(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to load service account in context")
	}

	raw := request.Object.Raw
	if request.Operation == v1beta1.Delete {
		raw = request.OldObject.Raw
	}

	resource, err := convertResource(raw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
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

	if checkPodTemplateAnnotation(resource) {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status: "Success",
			},
		}
	}

	higherVersion := utils.HigherThanKubernetesVersion(ws.client, ws.log, 1, 14, 0)
	if higherVersion {
		ok, msg := ws.HandleValidation(request, policies, nil, ctx, userRequestInfo)
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
	if !cache.WaitForCacheSync(stopCh, ws.pSynced, ws.rbSynced, ws.crbSynced) {
		logger.Info("failed to sync informer cache")
	}

	go func(ws *WebhookServer) {
		logger.V(3).Info("started serving requests", "addr", ws.server.Addr)
		if err := ws.server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			logger.Error(err, "failed to listen to requests")
		}
	}(ws)
	logger.Info("starting")

	// verifys if the admission control is enabled and active
	// resync: 60 seconds
	// deadline: 60 seconds (send request)
	// max deadline: deadline*3 (set the deployment annotation as false)
	go ws.lastReqTime.Run(ws.pLister, ws.eventGen, ws.client, checker.DefaultResync, checker.DefaultDeadline, stopCh)
}

// Stop TLS server and returns control after the server is shut down
func (ws *WebhookServer) Stop(ctx context.Context) {
	logger := ws.log
	// cleanUp
	// remove the static webhookconfigurations
	go ws.webhookRegistrationClient.RemoveWebhookConfigurations(ws.cleanUp)
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
	var body []byte
	if request.Body != nil {
		if data, err := ioutil.ReadAll(request.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		logger.Info("empty body")
		http.Error(writer, "empty body", http.StatusBadRequest)
		return nil
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

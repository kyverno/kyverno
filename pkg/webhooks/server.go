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

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/checker"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policystore"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/resourcewebhookwatcher"
	tlsutils "github.com/nirmata/kyverno/pkg/tls"
	userinfo "github.com/nirmata/kyverno/pkg/userinfo"
	"github.com/nirmata/kyverno/pkg/webhookconfig"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacinformer "k8s.io/client-go/informers/rbac/v1"
	rbaclister "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
)

// WebhookServer contains configured TLS server with MutationWebhook.
// MutationWebhook gets policies from policyController and takes control of the cluster with kubeclient.
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
	policyStatus policy.PolicyStatusInterface
	// helpers to validate against current loaded configuration
	configHandler config.Interface
	// channel for cleanup notification
	cleanUp chan<- struct{}
	// last request time
	lastReqTime *checker.LastReqTime
	// store to hold policy meta data for faster lookup
	pMetaStore policystore.LookupInterface
	// policy violation generator
	pvGenerator            policyviolation.GeneratorInterface
	resourceWebhookWatcher *resourcewebhookwatcher.ResourceWebhookWatcher
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
	policyStatus policy.PolicyStatusInterface,
	configHandler config.Interface,
	pMetaStore policystore.LookupInterface,
	pvGenerator policyviolation.GeneratorInterface,
	resourceWebhookWatcher *resourcewebhookwatcher.ResourceWebhookWatcher,
	lastReqTime *checker.LastReqTime,
	cleanUp chan<- struct{}) (*WebhookServer, error) {

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
		policyStatus:              policyStatus,
		configHandler:             configHandler,
		cleanUp:                   cleanUp,
		lastReqTime:               lastReqTime,
		pvGenerator:               pvGenerator,
		pMetaStore:                pMetaStore,
		resourceWebhookWatcher:    resourceWebhookWatcher,
	}
	mux := http.NewServeMux()
	mux.HandleFunc(config.MutatingWebhookServicePath, ws.serve)
	mux.HandleFunc(config.ValidatingWebhookServicePath, ws.serve)
	mux.HandleFunc(config.PolicyValidatingWebhookServicePath, ws.serve)
	mux.HandleFunc(config.PolicyMutatingWebhookServicePath, ws.serve)

	ws.server = http.Server{
		Addr:         ":443", // Listen on port for HTTPS requests
		TLSConfig:    &tlsConfig,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return ws, nil
}

// Main server endpoint for all requests
func (ws *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	// for every request recieved on the ep update last request time,
	// this is used to verify admission control
	ws.lastReqTime.SetTime(time.Now())
	admissionReview := ws.bodyToAdmissionReview(r, w)
	if admissionReview == nil {
		return
	}
	defer func() {
		glog.V(4).Infof("request: %v %s/%s/%s", time.Since(startTime), admissionReview.Request.Kind, admissionReview.Request.Namespace, admissionReview.Request.Name)
	}()

	admissionReview.Response = &v1beta1.AdmissionResponse{
		Allowed: true,
	}

	// Do not process the admission requests for kinds that are in filterKinds for filtering
	request := admissionReview.Request
	switch r.URL.Path {
	case config.VerifyMutatingWebhookServicePath:
		// we do not apply filters as this endpoint is used explicity
		// to watch kyveno deployment and verify if admission control is enabled
		admissionReview.Response = ws.handleVerifyRequest(request)
	case config.MutatingWebhookServicePath:
		if !ws.configHandler.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
			admissionReview.Response = ws.handleAdmissionRequest(request)
		}
	case config.PolicyValidatingWebhookServicePath:
		if !ws.configHandler.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
			admissionReview.Response = ws.handlePolicyValidation(request)
		}
	case config.PolicyMutatingWebhookServicePath:
		if !ws.configHandler.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
			admissionReview.Response = ws.handlePolicyMutation(request)
		}
	}
	admissionReview.Response.UID = request.UID

	responseJSON, err := json.Marshal(admissionReview)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not encode response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(responseJSON); err != nil {
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func (ws *WebhookServer) handleAdmissionRequest(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	policies, err := ws.pMetaStore.LookUp(request.Kind.Kind, request.Namespace)
	if err != nil {
		// Unable to connect to policy Lister to access policies
		glog.Errorf("Unable to connect to policy controller to access policies. Policies are NOT being applied: %v", err)
		return &v1beta1.AdmissionResponse{Allowed: true}
	}

	var roles, clusterRoles []string

	// getRoleRef only if policy has roles/clusterroles defined
	startTime := time.Now()
	if containRBACinfo(policies) {
		roles, clusterRoles, err = userinfo.GetRoleRef(ws.rbLister, ws.crbLister, request)
		if err != nil {
			// TODO(shuting): continue apply policy if error getting roleRef?
			glog.Errorf("Unable to get rbac information for request Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s: %v",
				request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation, err)
		}
	}
	glog.V(4).Infof("Time: webhook GetRoleRef %v", time.Since(startTime))

	// MUTATION
	ok, patches, msg := ws.HandleMutation(request, policies, roles, clusterRoles)
	if !ok {
		glog.V(4).Infof("Deny admission request:  %v/%s/%s", request.Kind, request.Namespace, request.Name)
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: msg,
			},
		}
	}

	// patch the resource with patches before handling validation rules
	patchedResource := processResourceWithPatches(patches, request.Object.Raw)

	// VALIDATION
	ok, msg = ws.HandleValidation(request, policies, patchedResource, roles, clusterRoles)
	if !ok {
		glog.V(4).Infof("Deny admission request: %v/%s/%s", request.Kind, request.Namespace, request.Name)
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: msg,
			},
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

// RunAsync TLS server in separate thread and returns control immediately
func (ws *WebhookServer) RunAsync(stopCh <-chan struct{}) {
	if !cache.WaitForCacheSync(stopCh, ws.pSynced, ws.rbSynced, ws.crbSynced) {
		glog.Error("webhook: failed to sync informer cache")
	}

	go func(ws *WebhookServer) {
		glog.V(3).Infof("serving on %s\n", ws.server.Addr)
		if err := ws.server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			glog.Infof("HTTP server error: %v", err)
		}
	}(ws)
	glog.Info("Started Webhook Server")
	// verifys if the admission control is enabled and active
	// resync: 60 seconds
	// deadline: 60 seconds (send request)
	// max deadline: deadline*3 (set the deployment annotation as false)
	go ws.lastReqTime.Run(ws.pLister, ws.eventGen, ws.client, checker.DefaultResync, checker.DefaultDeadline, stopCh)
}

// Stop TLS server and returns control after the server is shut down
func (ws *WebhookServer) Stop(ctx context.Context) {
	// cleanUp
	// remove the static webhookconfigurations
	go ws.webhookRegistrationClient.RemoveWebhookConfigurations(ws.cleanUp)
	// shutdown http.Server with context timeout
	err := ws.server.Shutdown(ctx)
	if err != nil {
		// Error from closing listeners, or context timeout:
		glog.Info("Server Shutdown error: ", err)
		ws.server.Close()
	}
}

// bodyToAdmissionReview creates AdmissionReview object from request body
// Answers to the http.ResponseWriter if request is not valid
func (ws *WebhookServer) bodyToAdmissionReview(request *http.Request, writer http.ResponseWriter) *v1beta1.AdmissionReview {
	var body []byte
	if request.Body != nil {
		if data, err := ioutil.ReadAll(request.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("Error: empty body")
		http.Error(writer, "empty body", http.StatusBadRequest)
		return nil
	}

	contentType := request.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Error("Error: invalid Content-Type: ", contentType)
		http.Error(writer, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return nil
	}

	admissionReview := &v1beta1.AdmissionReview{}
	if err := json.Unmarshal(body, &admissionReview); err != nil {
		glog.Errorf("Error: Can't decode body as AdmissionReview: %v", err)
		http.Error(writer, "Can't decode body as AdmissionReview", http.StatusExpectationFailed)
		return nil
	}

	return admissionReview
}

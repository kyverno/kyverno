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
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policystatus"
	"github.com/nirmata/kyverno/pkg/policystore"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	tlsutils "github.com/nirmata/kyverno/pkg/tls"
	userinfo "github.com/nirmata/kyverno/pkg/userinfo"
	"github.com/nirmata/kyverno/pkg/webhookconfig"
	"github.com/nirmata/kyverno/pkg/webhooks/generate"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbaclister "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
)

// WebhookServer contains configured TLS server with MutationWebhook.
// MutationWebhook gets policies from policyController and takes control of the cluster with kubeclient.
type WebhookServer struct {
	server        http.Server
	Client        *client.Client
	KyvernoClient *kyvernoclient.Clientset
	// list/get cluster policy resource
	PLister kyvernolister.ClusterPolicyLister
	// returns true if the cluster policy store has synced atleast
	PSynced cache.InformerSynced
	// list/get role binding resource
	RbLister rbaclister.RoleBindingLister
	// return true if role bining store has synced atleast once
	RbSynced cache.InformerSynced
	// list/get cluster role binding resource
	CrbLister rbaclister.ClusterRoleBindingLister
	// return true if cluster role binding store has synced atleast once
	CrbSynced cache.InformerSynced
	// generate events
	EventGen event.Interface
	// webhook registration client
	WebhookRegistrationClient *webhookconfig.WebhookRegistrationClient
	// API to send policy stats for aggregation
	StatusListener policystatus.Listener
	// helpers to validate against current loaded configuration
	ConfigHandler config.Interface
	// channel for cleanup notification
	CleanUp chan<- struct{}
	// last request time
	LastReqTime *checker.LastReqTime
	// store to hold policy meta data for faster lookup
	PMetaStore policystore.LookupInterface
	// policy violation generator
	PvGenerator policyviolation.GeneratorInterface
	// generate request generator
	GrGenerator            *generate.Generator
	ResourceWebhookWatcher *webhookconfig.ResourceWebhookRegister
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	ws *WebhookServer,
	tlsPair *tlsutils.TlsPemPair) (*WebhookServer, error) {

	if tlsPair == nil {
		return nil, errors.New("NewWebhookServer is not initialized properly")
	}
	var tlsConfig tls.Config
	pair, err := tls.X509KeyPair(tlsPair.Certificate, tlsPair.PrivateKey)
	if err != nil {
		return nil, err
	}
	tlsConfig.Certificates = []tls.Certificate{pair}

	mux := http.NewServeMux()
	mux.HandleFunc(config.MutatingWebhookServicePath, ws.handlerFunc(ws.handleMutateAdmissionRequest, true))
	mux.HandleFunc(config.ValidatingWebhookServicePath, ws.handlerFunc(ws.handleValidateAdmissionRequest, true))
	mux.HandleFunc(config.PolicyMutatingWebhookServicePath, ws.handlerFunc(ws.handlePolicyMutation, true))
	mux.HandleFunc(config.PolicyValidatingWebhookServicePath, ws.handlerFunc(ws.handlePolicyValidation, true))
	mux.HandleFunc(config.VerifyMutatingWebhookServicePath, ws.handlerFunc(ws.handleVerifyRequest, false))
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
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		// for every request received on the ep update last request time,
		// this is used to verify admission control
		ws.LastReqTime.SetTime(time.Now())
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
		if filter {
			if !ws.ConfigHandler.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
				admissionReview.Response = handler(request)
			}
		} else {
			admissionReview.Response = handler(request)
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
}

func (ws *WebhookServer) handleMutateAdmissionRequest(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	policies, err := ws.PMetaStore.ListAll()
	if err != nil {
		// Unable to connect to policy Lister to access policies
		glog.Errorf("Unable to connect to policy controller to access policies. Policies are NOT being applied: %v", err)
		return &v1beta1.AdmissionResponse{Allowed: true}
	}

	var roles, clusterRoles []string

	// getRoleRef only if policy has roles/clusterroles defined
	startTime := time.Now()
	if containRBACinfo(policies) {
		roles, clusterRoles, err = userinfo.GetRoleRef(ws.RbLister, ws.CrbLister, request)
		if err != nil {
			// TODO(shuting): continue apply policy if error getting roleRef?
			glog.Errorf("Unable to get rbac information for request Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s: %v",
				request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation, err)
		}
	}
	glog.V(4).Infof("Time: webhook GetRoleRef %v", time.Since(startTime))

	// convert RAW to unstructured
	resource, err := convertResource(request.Object.Raw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
	if err != nil {
		glog.Errorf(err.Error())

		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: err.Error(),
			},
		}
	}

	if checkPodTemplateAnn(resource) {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Status: "Success",
			},
		}
	}

	// MUTATION
	// mutation failure should not block the resource creation
	// any mutation failure is reported as the violation
	patches := ws.HandleMutation(request, resource, policies, roles, clusterRoles)

	// patch the resource with patches before handling validation rules
	patchedResource := processResourceWithPatches(patches, request.Object.Raw)

	if ws.ResourceWebhookWatcher != nil && ws.ResourceWebhookWatcher.RunValidationInMutatingWebhook == "true" {
		// VALIDATION
		ok, msg := ws.HandleValidation(request, policies, patchedResource, roles, clusterRoles)
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
	}

	// GENERATE
	// Only applied during resource creation
	// Success -> Generate Request CR created successsfully
	// Failed -> Failed to create Generate Request CR
	if request.Operation == v1beta1.Create {
		ok, msg := ws.HandleGenerate(request, policies, patchedResource, roles, clusterRoles)
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

func (ws *WebhookServer) handleValidateAdmissionRequest(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	policies, err := ws.PMetaStore.ListAll()
	if err != nil {
		// Unable to connect to policy Lister to access policies
		glog.Errorf("Unable to connect to policy controller to access policies. Policies are NOT being applied: %v", err)
		return &v1beta1.AdmissionResponse{Allowed: true}
	}

	var roles, clusterRoles []string

	// getRoleRef only if policy has roles/clusterroles defined
	startTime := time.Now()
	if containRBACinfo(policies) {
		roles, clusterRoles, err = userinfo.GetRoleRef(ws.RbLister, ws.CrbLister, request)
		if err != nil {
			// TODO(shuting): continue apply policy if error getting roleRef?
			glog.Errorf("Unable to get rbac information for request Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s: %v",
				request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation, err)
		}
	}
	glog.V(4).Infof("Time: webhook GetRoleRef %v", time.Since(startTime))

	// VALIDATION
	ok, msg := ws.HandleValidation(request, policies, nil, roles, clusterRoles)
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

	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Status: "Success",
		},
	}
}

// RunAsync TLS server in separate thread and returns control immediately
func (ws *WebhookServer) RunAsync(stopCh <-chan struct{}) {
	if !cache.WaitForCacheSync(stopCh, ws.PSynced, ws.RbSynced, ws.CrbSynced) {
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
	go ws.LastReqTime.Run(ws.PLister, ws.EventGen, ws.Client, checker.DefaultResync, checker.DefaultDeadline, stopCh)

}

// Stop TLS server and returns control after the server is shut down
func (ws *WebhookServer) Stop(ctx context.Context) {
	// cleanUp
	// remove the static webhookconfigurations
	go ws.WebhookRegistrationClient.RemoveWebhookConfigurations(ws.CleanUp)
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

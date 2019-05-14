package webhooks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nirmata/kube-policy/config"
	"github.com/nirmata/kube-policy/kubeclient"
	policylister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	engine "github.com/nirmata/kube-policy/pkg/engine"
	"github.com/nirmata/kube-policy/pkg/engine/mutation"
	tlsutils "github.com/nirmata/kube-policy/pkg/tls"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// WebhookServer contains configured TLS server with MutationWebhook.
// MutationWebhook gets policies from policyController and takes control of the cluster with kubeclient.
type WebhookServer struct {
	server       http.Server
	policyLister policylister.PolicyLister
	logger       *log.Logger
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	tlsPair *tlsutils.TlsPemPair,
	kubeClient *kubeclient.KubeClient,
	policyLister policylister.PolicyLister,
	logger *log.Logger) (*WebhookServer, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "Webhook Server:    ", log.LstdFlags)
	}

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
		policyLister: policyLister,
		logger:       logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(config.MutatingWebhookServicePath, ws.serve)
	mux.HandleFunc(config.ValidatingWebhookServicePath, ws.serve)

	ws.server = http.Server{
		Addr:         ":443", // Listen on port for HTTPS requests
		TLSConfig:    &tlsConfig,
		Handler:      mux,
		ErrorLog:     logger,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return ws, nil
}

// Main server endpoint for all requests
func (ws *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	admissionReview := ws.bodyToAdmissionReview(r, w)
	if admissionReview == nil {
		return
	}

	admissionReview.Response = &v1beta1.AdmissionResponse{
		Allowed: true,
	}

	if KindIsSupported(admissionReview.Request.Kind.Kind) {
		switch r.URL.Path {
		case config.MutatingWebhookServicePath:
			admissionReview.Response = ws.HandleMutation(admissionReview.Request)
		case config.ValidatingWebhookServicePath:
			admissionReview.Response = ws.HandleValidation(admissionReview.Request)
		}
	}

	admissionReview.Response.UID = admissionReview.Request.UID

	responseJson, err := json.Marshal(admissionReview)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not encode response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(responseJson); err != nil {
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

// RunAsync TLS server in separate thread and returns control immediately
func (ws *WebhookServer) RunAsync() {
	go func(ws *WebhookServer) {
		err := ws.server.ListenAndServeTLS("", "")
		if err != nil {
			ws.logger.Fatal(err)
		}
	}(ws)

	ws.logger.Printf("Started Webhook Server")
}

// Stop TLS server and returns control after the server is shut down
func (ws *WebhookServer) Stop() {
	err := ws.server.Shutdown(context.Background())
	if err != nil {
		// Error from closing listeners, or context timeout:
		ws.logger.Printf("Server Shutdown error: %v", err)
		ws.server.Close()
	}
}

// HandleMutation handles mutating webhook admission request
func (ws *WebhookServer) HandleMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	ws.logger.Printf("Handling mutation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	policies, err := ws.policyLister.List(labels.NewSelector())
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	var allPatches []mutation.PatchBytes
	for _, policy := range policies {
		ws.logger.Printf("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		policyPatches := engine.Mutate(*policy, request.Object.Raw, request.Kind)
		allPatches = append(allPatches, policyPatches...)

		if len(policyPatches) > 0 {
			namespace := mutation.ParseNamespaceFromObject(request.Object.Raw)
			name := mutation.ParseNameFromObject(request.Object.Raw)
			ws.logger.Printf("Policy %s applied to %s %s/%s", policy.Name, request.Kind.Kind, namespace, name)
		}
	}

	patchType := v1beta1.PatchTypeJSONPatch
	return &v1beta1.AdmissionResponse{
		Allowed:   true,
		Patch:     mutation.JoinPatches(allPatches),
		PatchType: &patchType,
	}
}

// HandleValidation handles validating webhook admission request
func (ws *WebhookServer) HandleValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	ws.logger.Printf("Handling validation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	policies, err := ws.policyLister.List(labels.NewSelector())
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	allowed := true
	for _, policy := range policies {
		ws.logger.Printf("Validating resource with policy %s with %d rules", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		if ok := engine.Validate(*policy, request.Object.Raw, request.Kind); !ok {
			ws.logger.Printf("Validation has failed: %v\n", err)
			utilruntime.HandleError(err)
			allowed = false
		} else {
			ws.logger.Println("Validation is successful")
		}
	}

	return &v1beta1.AdmissionResponse{
		Allowed: allowed,
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
		ws.logger.Print("Error: empty body")
		http.Error(writer, "empty body", http.StatusBadRequest)
		return nil
	}

	contentType := request.Header.Get("Content-Type")
	if contentType != "application/json" {
		ws.logger.Printf("Error: invalid Content-Type: %v", contentType)
		http.Error(writer, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return nil
	}

	admissionReview := &v1beta1.AdmissionReview{}
	if err := json.Unmarshal(body, &admissionReview); err != nil {
		ws.logger.Printf("Error: Can't decode body as AdmissionReview: %v", err)
		http.Error(writer, "Can't decode body as AdmissionReview", http.StatusExpectationFailed)
		return nil
	} else {
		return admissionReview
	}
}

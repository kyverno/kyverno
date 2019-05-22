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

	"github.com/nirmata/kyverno/client"
	"github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/config"
	engine "github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/mutation"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
	tlsutils "github.com/nirmata/kyverno/pkg/tls"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// WebhookServer contains configured TLS server with MutationWebhook.
// MutationWebhook gets policies from policyController and takes control of the cluster with kubeclient.
type WebhookServer struct {
	server       http.Server
	client       *client.Client
	policyLister v1alpha1.PolicyLister
	logger       *log.Logger
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	client *client.Client,
	tlsPair *tlsutils.TlsPemPair,
	shareInformer sharedinformer.PolicyInformer,
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
		client:       client,
		policyLister: shareInformer.GetLister(),
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
	switch r.URL.Path {
	case config.MutatingWebhookServicePath:
		admissionReview.Response = ws.HandleMutation(admissionReview.Request)
	case config.ValidatingWebhookServicePath:
		admissionReview.Response = ws.HandleValidation(admissionReview.Request)
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
		ws.logger.Printf("%v", err)
		return nil
	}

	var allPatches []mutation.PatchBytes
	for _, policy := range policies {
		ws.logger.Printf("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		policyPatches, _ := engine.Mutate(*policy, request.Object.Raw, request.Kind)
		allPatches = append(allPatches, policyPatches...)

		if len(policyPatches) > 0 {
			namespace := engine.ParseNamespaceFromObject(request.Object.Raw)
			name := engine.ParseNameFromObject(request.Object.Raw)
			ws.logger.Printf("Mutation from policy %s has applied to %s %s/%s", policy.Name, request.Kind.Kind, namespace, name)
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
		ws.logger.Printf("%v", err)
		return nil
	}

	for _, policy := range policies {
		// validation
		ws.logger.Printf("Validating resource with policy %s with %d rules", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		if err := engine.Validate(*policy, request.Object.Raw, request.Kind); err != nil {
			message := fmt.Sprintf("validation has failed: %s", err.Error())
			ws.logger.Println(message)

			return &v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: message,
				},
			}
		}

		// generation
		engine.Generate(ws.client, ws.logger, *policy, request.Object.Raw, request.Kind)
	}

	ws.logger.Println("Validation is successful")
	return &v1beta1.AdmissionResponse{
		Allowed: true,
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

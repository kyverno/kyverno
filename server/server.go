package server

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
	"sort"
	"time"

	"github.com/nirmata/kube-policy/config"
	"github.com/nirmata/kube-policy/kubeclient"
	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	policylister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/policyengine"
	"github.com/nirmata/kube-policy/pkg/policyengine/mutation"
	"github.com/nirmata/kube-policy/utils"
	"github.com/nirmata/kube-policy/webhooks"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// WebhookServer contains configured TLS server with MutationWebhook.
// MutationWebhook gets policies from policyController and takes control of the cluster with kubeclient.
type WebhookServer struct {
	server       http.Server
	policyEngine policyengine.PolicyEngine
	policyLister policylister.PolicyLister
	logger       *log.Logger
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	tlsPair *utils.TlsPemPair,
	kubeclient *kubeclient.KubeClient,
	policyLister policylister.PolicyLister,
	logger *log.Logger) (*WebhookServer, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "HTTPS Server: ", log.LstdFlags|log.Lshortfile)
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
	policyEngine := policyengine.NewPolicyEngine(kubeclient, logger)

	ws := &WebhookServer{
		policyEngine: policyEngine,
		policyLister: policyLister,
		logger:       logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(config.WebhookServicePath, ws.serve)

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
	if r.URL.Path == config.WebhookServicePath {
		admissionReview := ws.parseAdmissionReview(r, w)
		if admissionReview == nil {
			return
		}

		var admissionResponse *v1beta1.AdmissionResponse
		if webhooks.AdmissionIsRequired(admissionReview.Request) {
			admissionResponse = ws.Mutate(admissionReview.Request)
		}

		if admissionResponse == nil {
			admissionResponse = &v1beta1.AdmissionResponse{
				Allowed: true,
			}
		}

		admissionReview.Response = admissionResponse
		admissionReview.Response.UID = admissionReview.Request.UID

		responseJson, err := json.Marshal(admissionReview)
		if err != nil {
			http.Error(w, fmt.Sprintf("Could not encode response: %v", err), http.StatusInternalServerError)
			return
		}

		ws.logger.Printf("Response body\n:%v", string(responseJson))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err := w.Write(responseJson); err != nil {
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	} else {
		http.Error(w, fmt.Sprintf("Unexpected method path: %v", r.URL.Path), http.StatusNotFound)
	}
}

// Answers to the http.ResponseWriter if request is not valid
func (ws *WebhookServer) parseAdmissionReview(request *http.Request, writer http.ResponseWriter) *v1beta1.AdmissionReview {
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
		ws.logger.Printf("Request body:\n%v", string(body))
		return admissionReview
	}
}

// Runs TLS server in separate thread and returns control immediately
func (ws *WebhookServer) RunAsync() {
	go func(ws *WebhookServer) {
		err := ws.server.ListenAndServeTLS("", "")
		if err != nil {
			ws.logger.Fatal(err)
		}
	}(ws)
}

// Stops TLS server and returns control after the server is shut down
func (ws *WebhookServer) Stop() {
	err := ws.server.Shutdown(context.Background())
	if err != nil {
		// Error from closing listeners, or context timeout:
		ws.logger.Printf("Server Shutdown error: %v", err)
		ws.server.Close()
	}
}

func (ws *WebhookServer) Mutate(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	ws.logger.Printf("AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation, request.UserInfo)

	policies, err := ws.getPolicies()
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}
	if len(policies) == 0 {
		return nil
	}

	var allPatches []mutation.PatchBytes
	for _, policy := range policies {
		ws.logger.Printf("Applying policy %s with %d rules", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		policyPatches := ws.policyEngine.Mutate(policy, request.Object.Raw)
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

func (ws *WebhookServer) getPolicies() ([]kubepolicy.Policy, error) {
	selector := labels.NewSelector()
	cachedPolicies, err := ws.policyLister.List(selector)
	if err != nil {
		ws.logger.Printf("Error: %v", err)
		return nil, err
	}

	var policies []kubepolicy.Policy
	for _, elem := range cachedPolicies {
		policies = append(policies, *elem.DeepCopy())
	}

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].CreationTimestamp.Time.Before(policies[j].CreationTimestamp.Time)
	})
	return policies, nil
}

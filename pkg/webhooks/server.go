package webhooks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	engine "github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/info"
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
	filterKinds  []string
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	client *client.Client,
	tlsPair *tlsutils.TlsPemPair,
	shareInformer sharedinformer.PolicyInformer,
	filterKinds []string) (*WebhookServer, error) {

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
		filterKinds:  parseKinds(filterKinds),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(config.MutatingWebhookServicePath, ws.serve)
	mux.HandleFunc(config.ValidatingWebhookServicePath, ws.serve)

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
	admissionReview := ws.bodyToAdmissionReview(r, w)
	if admissionReview == nil {
		return
	}

	admissionReview.Response = &v1beta1.AdmissionResponse{
		Allowed: true,
	}
	// Do not process the admission requests for kinds that are in filterKinds for filtering
	if !StringInSlice(admissionReview.Request.Kind.Kind, ws.filterKinds) {

		switch r.URL.Path {
		case config.MutatingWebhookServicePath:
			admissionReview.Response = ws.HandleMutation(admissionReview.Request)
		case config.ValidatingWebhookServicePath:
			admissionReview.Response = ws.HandleValidation(admissionReview.Request)
		}
	}

	admissionReview.Response.UID = admissionReview.Request.UID

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

// RunAsync TLS server in separate thread and returns control immediately
func (ws *WebhookServer) RunAsync() {
	go func(ws *WebhookServer) {
		err := ws.server.ListenAndServeTLS("", "")
		if err != nil {
			glog.Fatal(err)
		}
	}(ws)
	glog.Info("Started Webhook Server")
}

// Stop TLS server and returns control after the server is shut down
func (ws *WebhookServer) Stop() {
	err := ws.server.Shutdown(context.Background())
	if err != nil {
		// Error from closing listeners, or context timeout:
		glog.Info("Server Shutdown error: ", err)
		ws.server.Close()
	}
}

// HandleMutation handles mutating webhook admission request
func (ws *WebhookServer) HandleMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {

	policies, err := ws.policyLister.List(labels.NewSelector())
	if err != nil {
		// Unable to connect to policy Lister to access policies
		glog.Error("Unable to connect to policy controller to access policies. Mutation Rules are NOT being applied")
		glog.Warning(err)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	var allPatches []engine.PatchBytes
	policyInfos := []*info.PolicyInfo{}
	for _, policy := range policies {

		// check if policy has a rule for the admission request kind
		if !StringInSlice(request.Kind.Kind, getApplicableKindsForPolicy(policy)) {
			continue
		}
		rname := engine.ParseNameFromObject(request.Object.Raw)
		rns := engine.ParseNamespaceFromObject(request.Object.Raw)
		rkind := engine.ParseKindFromObject(request.Object.Raw)
		policyInfo := info.NewPolicyInfo(policy.Name,
			rkind,
			rname,
			rns)

		glog.V(3).Infof("Handling mutation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			request.Kind.Kind, rns, rname, request.UID, request.Operation)

		glog.Infof("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		policyPatches, ruleInfos := engine.Mutate(*policy, request.Object.Raw, request.Kind)
		policyInfo.AddRuleInfos(ruleInfos)
		if !policyInfo.IsSuccessful() {
			glog.Infof("Failed to apply policy %s on resource %s/%s", policy.Name, rname, rns)
			for _, r := range ruleInfos {
				glog.Warning(r.Msgs)
			}
		} else if len(policyPatches) > 0 {
			allPatches = append(allPatches, policyPatches...)
			glog.Infof("Mutation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, rname, rns)
		}
		policyInfos = append(policyInfos, policyInfo)
	}

	ok, msg := isAdmSuccesful(policyInfos)
	if ok {
		patchType := v1beta1.PatchTypeJSONPatch
		return &v1beta1.AdmissionResponse{
			Allowed:   true,
			Patch:     engine.JoinPatches(allPatches),
			PatchType: &patchType,
		}
	}
	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: msg,
		},
	}
}

func isAdmSuccesful(policyInfos []*info.PolicyInfo) (bool, string) {
	var admSuccess = true
	var errMsgs []string
	for _, pi := range policyInfos {
		if !pi.IsSuccessful() {
			admSuccess = false
			errMsgs = append(errMsgs, fmt.Sprintf("\nPolicy %s failed with following rules", pi.Name))
			// Get the error rules
			errorRules := pi.ErrorRules()
			errMsgs = append(errMsgs, errorRules)
		}
	}
	return admSuccess, strings.Join(errMsgs, ";")
}

// HandleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
func (ws *WebhookServer) HandleValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	policyInfos := []*info.PolicyInfo{}

	policies, err := ws.policyLister.List(labels.NewSelector())
	if err != nil {
		// Unable to connect to policy Lister to access policies
		glog.Error("Unable to connect to policy controller to access policies. Validation Rules are NOT being applied")
		glog.Warning(err)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	for _, policy := range policies {

		if !StringInSlice(request.Kind.Kind, getApplicableKindsForPolicy(policy)) {
			continue
		}
		rname := engine.ParseNameFromObject(request.Object.Raw)
		rns := engine.ParseNamespaceFromObject(request.Object.Raw)
		rkind := engine.ParseKindFromObject(request.Object.Raw)

		policyInfo := info.NewPolicyInfo(policy.Name,
			rkind,
			rname,
			rns)

		glog.V(3).Infof("Handling validation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			request.Kind.Kind, rns, rname, request.UID, request.Operation)

		glog.Infof("Validating resource with policy %s with %d rules", policy.ObjectMeta.Name, len(policy.Spec.Rules))
		ruleInfos, err := engine.Validate(*policy, request.Object.Raw, request.Kind)
		if err != nil {
			// This is not policy error
			// but if unable to parse request raw resource
			// TODO : create event ? dont think so
			glog.Error(err)
			continue
		}
		policyInfo.AddRuleInfos(ruleInfos)

		if !policyInfo.IsSuccessful() {
			glog.Infof("Failed to apply policy %s on resource %s/%s", policy.Name, rname, rns)
			for _, r := range ruleInfos {
				glog.Warning(r.Msgs)
			}
		} else if len(ruleInfos) > 0 {
			glog.Infof("Validation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, rname, rns)
		}
		policyInfos = append(policyInfos, policyInfo)
	}

	// If Validation fails then reject the request
	ok, msg := isAdmSuccesful(policyInfos)
	if !ok {
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: msg,
			},
		}
	}
	// Process Generation
	return ws.HandleGeneration(request)
}

//HandleGeneration handles application of generation rules
func (ws *WebhookServer) HandleGeneration(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	if request.Kind.Kind != "Namespace" {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}
	policyInfos := []*info.PolicyInfo{}

	policies, err := ws.policyLister.List(labels.NewSelector())
	if err != nil {
		// Unable to connect to policy Lister to access policies
		glog.Error("Unable to connect to policy controller to access policies. Generation Rules are NOT being applied")
		glog.Warning(err)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}
	for _, policy := range policies {

		if !StringInSlice(request.Kind.Kind, getApplicableKindsForPolicy(policy)) {
			continue
		}
		rname := engine.ParseNameFromObject(request.Object.Raw)
		rns := engine.ParseNamespaceFromObject(request.Object.Raw)
		rkind := engine.ParseKindFromObject(request.Object.Raw)

		policyInfo := info.NewPolicyInfo(policy.Name,
			rkind,
			rname,
			rns)
		glog.V(3).Infof("Handling generation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			request.Kind.Kind, rns, rname, request.UID, request.Operation)
		glog.Infof("Applying  policy %s with generation %d rules", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		ruleInfos := engine.Generate(ws.client, *policy, request.Object.Raw, request.Kind, false)
		policyInfo.AddRuleInfos(ruleInfos)
		if !policyInfo.IsSuccessful() {
			glog.Infof("Failed to apply policy %s on resource %s/%s", policy.Name, rname, rns)
			for _, r := range ruleInfos {
				glog.Warning(r.Msgs)
			}
		} else {
			glog.Infof("Generation from policy %s has succesfully applied to %s %s/%s", policy.Name, request.Kind.Kind, rns, rname)
		}
		policyInfos = append(policyInfos, policyInfo)
	}
	ok, msg := isAdmSuccesful(policyInfos)
	if ok {
		glog.V(3).Info("Generation is successful")
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}
	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: msg,
		},
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

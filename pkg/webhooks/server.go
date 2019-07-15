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
	policyv1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	engine "github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
	tlsutils "github.com/nirmata/kyverno/pkg/tls"
	"github.com/nirmata/kyverno/pkg/utils"
	"github.com/nirmata/kyverno/pkg/violation"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const policyKind = "Policy"

// WebhookServer contains configured TLS server with MutationWebhook.
// MutationWebhook gets policies from policyController and takes control of the cluster with kubeclient.
type WebhookServer struct {
	server          http.Server
	client          *client.Client
	policyLister    v1alpha1.PolicyLister
	eventController event.Generator
	filterKinds     []string
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	client *client.Client,
	tlsPair *tlsutils.TlsPemPair,
	shareInformer sharedinformer.PolicyInformer,
	eventController event.Generator,
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
		client:          client,
		policyLister:    shareInformer.GetLister(),
		eventController: eventController,
		filterKinds:     parseKinds(filterKinds),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(config.MutatingWebhookServicePath, ws.serve)
	mux.HandleFunc(config.ValidatingWebhookServicePath, ws.serve)
	mux.HandleFunc(config.PolicyValidatingWebhookServicePath, ws.serve)

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
		case config.PolicyValidatingWebhookServicePath:
			admissionReview.Response = ws.HandlePolicyValidation(admissionReview.Request)
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

	var allPatches [][]byte
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
		} else {
			fmt.Println("cleanup")
			// CleanUp Violations if exists
			violation.RemoveViolation(policy, request.Kind.Kind, rns, rname)
			if len(policyPatches) > 0 {
				allPatches = append(allPatches, policyPatches...)
				glog.Infof("Mutation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, rname, rns)
			}
		}
		policyInfos = append(policyInfos, policyInfo)
	}

	if len(allPatches) > 0 {
		eventsInfo := newEventInfoFromPolicyInfo(policyInfos, (request.Operation == v1beta1.Update))
		ws.eventController.Add(eventsInfo...)
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
		} else {
			// CleanUp Violations if exists
			violation.RemoveViolation(policy, request.Kind.Kind, rns, rname)

			if len(ruleInfos) > 0 {
				glog.Infof("Validation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, rname, rns)
			}
		}
		policyInfos = append(policyInfos, policyInfo)
	}

	if len(policyInfos) > 0 && len(policyInfos[0].Rules) != 0 {
		eventsInfo := newEventInfoFromPolicyInfo(policyInfos, (request.Operation == v1beta1.Update))
		ws.eventController.Add(eventsInfo...)

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

	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
	// Generation rules applied via generation controller
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

//HandlePolicyValidation performs the validation check on policy resource
func (ws *WebhookServer) HandlePolicyValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	return ws.validateUniqueRuleName(request.Object.Raw)
}

func (ws *WebhookServer) validateUniqueRuleName(rawPolicy []byte) *v1beta1.AdmissionResponse {
	var policy *policyv1.Policy
	var ruleNames []string

	json.Unmarshal(rawPolicy, &policy)

	for _, rule := range policy.Spec.Rules {
		if utils.Contains(ruleNames, rule.Name) {
			msg := fmt.Sprintf(`The policy "%s" is invalid: duplicate rule name: "%s"`, policy.Name, rule.Name)
			glog.Errorln(msg)

			return &v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: msg,
				},
			}
		}
		ruleNames = append(ruleNames, rule.Name)
	}

	glog.V(3).Infof("Policy validation passed")
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}

func newEventInfoFromPolicyInfo(policyInfoList []*info.PolicyInfo, onUpdate bool) []*event.Info {
	var eventsInfo []*event.Info

	ok, msg := isAdmSuccesful(policyInfoList)
	// create events on operation UPDATE
	if onUpdate {
		if !ok {
			for _, pi := range policyInfoList {
				ruleNames := getRuleNames(*pi, false)
				eventsInfo = append(eventsInfo,
					event.NewEvent(pi.RKind, pi.RNamespace, pi.RName, event.RequestBlocked, event.FPolicyApplyBlockUpdate, ruleNames, pi.Name))

				eventsInfo = append(eventsInfo,
					event.NewEvent(policyKind, "", pi.Name, event.RequestBlocked, event.FPolicyBlockResourceUpdate, pi.RName, ruleNames))

				glog.V(3).Infof("Request blocked events info has prepared for %s/%s and %s/%s\n", policyKind, pi.Name, pi.RKind, pi.RName)
			}
		}
		return eventsInfo
	}

	// create events on operation CREATE
	if ok {
		for _, pi := range policyInfoList {
			ruleNames := getRuleNames(*pi, true)

			eventsInfo = append(eventsInfo,
				event.NewEvent(pi.RKind, pi.RNamespace, pi.RName, event.PolicyApplied, event.SRulesApply, ruleNames, pi.Name))

			glog.V(3).Infof("Success event info has prepared for %s/%s\n", pi.RKind, pi.RName)
		}
		return eventsInfo
	}

	for _, pi := range policyInfoList {
		ruleNames := getRuleNames(*pi, false)

		eventsInfo = append(eventsInfo,
			event.NewEvent(policyKind, "", pi.Name, event.RequestBlocked, event.FPolicyApplyBlockCreate, pi.RName, ruleNames))

		glog.V(3).Infof("Rule(s) %s of policy %s blocked resource creation, error: %s\n", ruleNames, pi.Name, msg)
	}
	return eventsInfo
}

func getRuleNames(policyInfo info.PolicyInfo, onSuccess bool) string {
	var ruleNames []string
	for _, rule := range policyInfo.Rules {
		if onSuccess {
			if rule.IsSuccessful() {
				ruleNames = append(ruleNames, rule.Name)
			}
		} else {
			if !rule.IsSuccessful() {
				ruleNames = append(ruleNames, rule.Name)
			}
		}
	}
	return strings.Join(ruleNames, ",")
}

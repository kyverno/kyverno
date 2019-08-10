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
	kyvernoclient "github.com/nirmata/kyverno/pkg/clientNew/clientset/versioned"
	informer "github.com/nirmata/kyverno/pkg/clientNew/informers/externalversions/kyverno/v1alpha1"
	lister "github.com/nirmata/kyverno/pkg/clientNew/listers/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	tlsutils "github.com/nirmata/kyverno/pkg/tls"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
)

// WebhookServer contains configured TLS server with MutationWebhook.
// MutationWebhook gets policies from policyController and takes control of the cluster with kubeclient.
type WebhookServer struct {
	server            http.Server
	client            *client.Client
	kyvernoClient     *kyvernoclient.Clientset
	pLister           lister.PolicyLister
	pvLister          lister.PolicyViolationLister
	eventGen          event.Interface
	filterK8Resources []utils.K8Resource
}

// NewWebhookServer creates new instance of WebhookServer accordingly to given configuration
// Policy Controller and Kubernetes Client should be initialized in configuration
func NewWebhookServer(
	kyvernoClient *kyvernoclient.Clientset,
	client *client.Client,
	tlsPair *tlsutils.TlsPemPair,
	pInformer informer.PolicyInformer,
	pvInormer informer.PolicyViolationInformer,
	eventGen event.Interface,
	filterK8Resources string) (*WebhookServer, error) {

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
		client:            client,
		kyvernoClient:     kyvernoClient,
		pLister:           pInformer.Lister(),
		pvLister:          pvInormer.Lister(),
		eventGen:          eventGen,
		filterK8Resources: utils.ParseKinds(filterK8Resources),
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
	if !utils.SkipFilteredResourcesReq(admissionReview.Request, ws.filterK8Resources) {
		// if the resource is being deleted we need to clear any existing Policy Violations
		// TODO: can report to the user that we clear the violation corresponding to this resource
		if admissionReview.Request.Operation == v1beta1.Delete {
			// Resource DELETE
			err := ws.removePolicyViolation(admissionReview.Request)
			if err != nil {
				glog.Info(err)
			}
			admissionReview.Response = &v1beta1.AdmissionResponse{
				Allowed: true,
			}
			admissionReview.Response.UID = admissionReview.Request.UID
		} else {
			// Resource CREATE
			// Resource UPDATE
			switch r.URL.Path {
			case config.MutatingWebhookServicePath:
				admissionReview.Response = ws.HandleMutation(admissionReview.Request)
			case config.ValidatingWebhookServicePath:
				admissionReview.Response = ws.HandleValidation(admissionReview.Request)
			case config.PolicyValidatingWebhookServicePath:
				admissionReview.Response = ws.HandlePolicyValidation(admissionReview.Request)
			}

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
		glog.V(3).Infof("serving on %s\n", ws.server.Addr)
		err := ws.server.ListenAndServeTLS("", "")
		if err != nil {
			glog.Fatalf("error serving TLS: %v\n", err)
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

package webhooks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// DebugModeOptions holds the options to configure debug mode
type DebugModeOptions struct {
	// DumpPayload is used to activate/deactivate debug mode.
	DumpPayload bool
}

// AdmissionRequestPayload holds a copy of the AdmissionRequest payload
type AdmissionRequestPayload struct {
	UID                types.UID                    `json:"uid"`
	Kind               metav1.GroupVersionKind      `json:"kind"`
	Resource           metav1.GroupVersionResource  `json:"resource"`
	SubResource        string                       `json:"subResource,omitempty"`
	RequestKind        *metav1.GroupVersionKind     `json:"requestKind,omitempty"`
	RequestResource    *metav1.GroupVersionResource `json:"requestResource,omitempty"`
	RequestSubResource string                       `json:"requestSubResource,omitempty"`
	Name               string                       `json:"name,omitempty"`
	Namespace          string                       `json:"namespace,omitempty"`
	Operation          string                       `json:"operation"`
	UserInfo           authenticationv1.UserInfo    `json:"userInfo"`
	Object             unstructured.Unstructured    `json:"object,omitempty"`
	OldObject          unstructured.Unstructured    `json:"oldObject,omitempty"`
	DryRun             *bool                        `json:"dryRun,omitempty"`
	Options            unstructured.Unstructured    `json:"options,omitempty"`
}

func newAdmissionRequestPayload(rq *admissionv1.AdmissionRequest) (*AdmissionRequestPayload, error) {
	newResource, oldResource, err := utils.ExtractResources(nil, rq)
	if err != nil {
		return nil, err
	}
	options := new(unstructured.Unstructured)
	if rq.Options.Raw != nil {
		options, err = engineutils.ConvertToUnstructured(rq.Options.Raw)
		if err != nil {
			return nil, err
		}
	}
	return redactPayload(&AdmissionRequestPayload{
		UID:                rq.UID,
		Kind:               rq.Kind,
		Resource:           rq.Resource,
		SubResource:        rq.SubResource,
		RequestKind:        rq.RequestKind,
		RequestResource:    rq.RequestResource,
		RequestSubResource: rq.RequestSubResource,
		Name:               rq.Name,
		Namespace:          rq.Namespace,
		Operation:          string(rq.Operation),
		UserInfo:           rq.UserInfo,
		Object:             newResource,
		OldObject:          oldResource,
		DryRun:             rq.DryRun,
		Options:            *options,
	})
}

func redactPayload(payload *AdmissionRequestPayload) (*AdmissionRequestPayload, error) {
	if strings.EqualFold(payload.Kind.Kind, "Secret") {
		if payload.Object.Object != nil {
			obj, err := utils.RedactSecret(&payload.Object)
			if err != nil {
				return nil, err
			}
			payload.Object = obj
		}
		if payload.OldObject.Object != nil {
			oldObj, err := utils.RedactSecret(&payload.OldObject)
			if err != nil {
				return nil, err
			}
			payload.OldObject = oldObj
		}
	}
	return payload, nil
}

type Server interface {
	// Run TLS server in separate thread and returns control immediately
	Run(<-chan struct{})
	// Stop TLS server and returns control after the server is shut down
	Stop(context.Context)
	// Cleanup returns the chanel used to wait for the server to clean up resources
	Cleanup() <-chan struct{}
}

type PolicyHandlers interface {
	// Mutate performs the mutation of policy resources
	Mutate(logr.Logger, *admissionv1.AdmissionRequest, time.Time) *admissionv1.AdmissionResponse
	// Validate performs the validation check on policy resources
	Validate(logr.Logger, *admissionv1.AdmissionRequest, time.Time) *admissionv1.AdmissionResponse
}

type ResourceHandlers interface {
	// Mutate performs the mutation of kube resources
	Mutate(logr.Logger, *admissionv1.AdmissionRequest, string, time.Time) *admissionv1.AdmissionResponse
	// Validate performs the validation check on kube resources
	Validate(logr.Logger, *admissionv1.AdmissionRequest, string, time.Time) *admissionv1.AdmissionResponse
}

type server struct {
	server      *http.Server
	runtime     runtimeutils.Runtime
	mwcClient   controllerutils.DeleteClient[*admissionregistrationv1.MutatingWebhookConfiguration]
	vwcClient   controllerutils.DeleteClient[*admissionregistrationv1.ValidatingWebhookConfiguration]
	leaseClient controllerutils.DeleteClient[*coordinationv1.Lease]
	cleanUp     chan struct{}
}

type TlsProvider func() ([]byte, []byte, error)

// NewServer creates new instance of server accordingly to given configuration
func NewServer(
	policyHandlers PolicyHandlers,
	resourceHandlers ResourceHandlers,
	configuration config.Configuration,
	debugModeOpts DebugModeOptions,
	tlsProvider TlsProvider,
	mwcClient controllerutils.DeleteClient[*admissionregistrationv1.MutatingWebhookConfiguration],
	vwcClient controllerutils.DeleteClient[*admissionregistrationv1.ValidatingWebhookConfiguration],
	leaseClient controllerutils.DeleteClient[*coordinationv1.Lease],
	runtime runtimeutils.Runtime,
) Server {
	mux := httprouter.New()
	resourceLogger := logger.WithName("resource")
	policyLogger := logger.WithName("policy")
	verifyLogger := logger.WithName("verify")
	registerWebhookHandlers(resourceLogger.WithName("mutate"), mux, config.MutatingWebhookServicePath, configuration, resourceHandlers.Mutate, debugModeOpts)
	registerWebhookHandlers(resourceLogger.WithName("validate"), mux, config.ValidatingWebhookServicePath, configuration, resourceHandlers.Validate, debugModeOpts)
	mux.HandlerFunc("POST", config.PolicyMutatingWebhookServicePath, admission(policyLogger.WithName("mutate"), filter(configuration, policyHandlers.Mutate), debugModeOpts))
	mux.HandlerFunc("POST", config.PolicyValidatingWebhookServicePath, admission(policyLogger.WithName("validate"), filter(configuration, policyHandlers.Validate), debugModeOpts))
	mux.HandlerFunc("POST", config.VerifyMutatingWebhookServicePath, admission(verifyLogger.WithName("mutate"), handlers.Verify(), DebugModeOptions{}))
	mux.HandlerFunc("GET", config.LivenessServicePath, handlers.Probe(runtime.IsLive))
	mux.HandlerFunc("GET", config.ReadinessServicePath, handlers.Probe(runtime.IsReady))
	return &server{
		server: &http.Server{
			Addr: ":9443",
			TLSConfig: &tls.Config{
				GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
					certPem, keyPem, err := tlsProvider()
					if err != nil {
						return nil, err
					}
					pair, err := tls.X509KeyPair(certPem, keyPem)
					if err != nil {
						return nil, err
					}
					return &pair, nil
				},
				MinVersion: tls.VersionTLS12,
			},
			Handler:           mux,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			ReadHeaderTimeout: 30 * time.Second,
			IdleTimeout:       5 * time.Minute,
			ErrorLog:          logging.StdLogger(logger.WithName("server"), ""),
		},
		mwcClient:   mwcClient,
		vwcClient:   vwcClient,
		leaseClient: leaseClient,
		runtime:     runtime,
		cleanUp:     make(chan struct{}),
	}
}

func (s *server) Run(stopCh <-chan struct{}) {
	go func() {
		logger.V(3).Info("started serving requests", "addr", s.server.Addr)
		if err := s.server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			logger.Error(err, "failed to listen to requests")
		}
	}()
	logger.Info("starting service")
}

func (s *server) Stop(ctx context.Context) {
	s.cleanup(ctx)
	err := s.server.Shutdown(ctx)
	if err != nil {
		logger.Error(err, "shutting down server")
		err = s.server.Close()
		if err != nil {
			logger.Error(err, "server shut down failed")
		}
	}
}

func (s *server) Cleanup() <-chan struct{} {
	return s.cleanUp
}

func (s *server) cleanup(ctx context.Context) {
	if s.runtime.IsGoingDown() {
		deleteLease := func(name string) {
			if err := s.leaseClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to clean up lease", "name", name)
			}
		}
		deleteVwc := func(name string) {
			if err := s.vwcClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to clean up validating webhook configuration", "name", name)
			}
		}
		deleteMwc := func(name string) {
			if err := s.mwcClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to clean up mutating webhook configuration", "name", name)
			}
		}
		deleteLease("kyvernopre-lock")
		deleteLease("kyverno-health")
		deleteVwc(config.ValidatingWebhookConfigurationName)
		deleteVwc(config.PolicyValidatingWebhookConfigurationName)
		deleteMwc(config.MutatingWebhookConfigurationName)
		deleteMwc(config.PolicyMutatingWebhookConfigurationName)
		deleteMwc(config.VerifyMutatingWebhookConfigurationName)
	}
	close(s.cleanUp)
}

func dumpPayload(logger logr.Logger, request *admissionv1.AdmissionRequest, response *admissionv1.AdmissionResponse) {
	reqPayload, err := newAdmissionRequestPayload(request)
	if err != nil {
		logger.Error(err, "Failed to extract resources")
	} else {
		logger.Info("Logging admission request and response payload ", "AdmissionRequest", reqPayload, "AdmissionResponse", response)
	}
}

func dump(inner handlers.AdmissionHandler, debugModeOpts DebugModeOptions) handlers.AdmissionHandler {
	// debug mode not enabled, no need to add debug middleware
	if !debugModeOpts.DumpPayload {
		return inner
	}
	return func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		response := inner(logger, request, startTime)
		dumpPayload(logger, request, response)
		return response
	}
}

func protect(inner handlers.AdmissionHandler) handlers.AdmissionHandler {
	if !toggle.ProtectManagedResources.Enabled() {
		return inner
	}
	return func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		newResource, oldResource, err := utils.ExtractResources(nil, request)
		if err != nil {
			logger.Error(err, "Failed to extract resources")
			return admissionutils.ResponseFailure(err.Error())
		}
		for _, resource := range []unstructured.Unstructured{newResource, oldResource} {
			resLabels := resource.GetLabels()
			if resLabels[kyvernov1.LabelAppManagedBy] == kyvernov1.ValueKyvernoApp {
				if request.UserInfo.Username != fmt.Sprintf("system:serviceaccount:%s:%s", config.KyvernoNamespace(), config.KyvernoServiceAccountName()) {
					logger.Info("Access to the resource not authorized, this is a kyverno managed resource and should be altered only by kyverno")
					return admissionutils.ResponseFailure("A kyverno managed resource can only be modified by kyverno")
				}
			}
		}
		return inner(logger, request, startTime)
	}
}

func filter(configuration config.Configuration, inner handlers.AdmissionHandler) handlers.AdmissionHandler {
	return handlers.Filter(configuration, inner)
}

func admission(logger logr.Logger, inner handlers.AdmissionHandler, debugModeOpts DebugModeOptions) http.HandlerFunc {
	return handlers.Admission(logger, dump(protect(inner), debugModeOpts))
}

func registerWebhookHandlers(
	logger logr.Logger,
	mux *httprouter.Router,
	basePath string,
	configuration config.Configuration,
	handlerFunc func(logr.Logger, *admissionv1.AdmissionRequest, string, time.Time) *admissionv1.AdmissionResponse,
	debugModeOpts DebugModeOptions,
) {
	mux.HandlerFunc("POST", basePath, admission(logger, filter(
		configuration,
		func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
			return handlerFunc(logger, request, "all", startTime)
		}), debugModeOpts),
	)
	mux.HandlerFunc("POST", basePath+"/fail", admission(logger, filter(
		configuration,
		func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
			return handlerFunc(logger, request, "fail", startTime)
		}), debugModeOpts),
	)
	mux.HandlerFunc("POST", basePath+"/ignore", admission(logger, filter(
		configuration,
		func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
			return handlerFunc(logger, request, "ignore", startTime)
		}), debugModeOpts),
	)
}

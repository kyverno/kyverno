package webhooks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Server interface {
	// Run TLS server in separate thread and returns control immediately
	Run(<-chan struct{})
	// Stop TLS server and returns control after the server is shut down
	Stop(context.Context)
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
	server          *http.Server
	webhookRegister *webhookconfig.Register
	cleanUp         chan<- struct{}
}

type TlsProvider func() ([]byte, []byte, error)

// NewServer creates new instance of server accordingly to given configuration
func NewServer(
	policyHandlers PolicyHandlers,
	resourceHandlers ResourceHandlers,
	tlsProvider TlsProvider,
	configuration config.Configuration,
	register *webhookconfig.Register,
	monitor *webhookconfig.Monitor,
	cleanUp chan<- struct{},
) Server {
	mux := httprouter.New()
	resourceLogger := logger.WithName("resource")
	policyLogger := logger.WithName("policy")
	verifyLogger := logger.WithName("verify")
	registerWebhookHandlers(resourceLogger.WithName("mutate"), mux, config.MutatingWebhookServicePath, monitor, configuration, resourceHandlers.Mutate)
	registerWebhookHandlers(resourceLogger.WithName("validate"), mux, config.ValidatingWebhookServicePath, monitor, configuration, resourceHandlers.Validate)
	mux.HandlerFunc("POST", config.PolicyMutatingWebhookServicePath, admission(policyLogger.WithName("mutate"), monitor, filter(configuration, policyHandlers.Mutate)))
	mux.HandlerFunc("POST", config.PolicyValidatingWebhookServicePath, admission(policyLogger.WithName("validate"), monitor, filter(configuration, policyHandlers.Validate)))
	mux.HandlerFunc("POST", config.VerifyMutatingWebhookServicePath, admission(verifyLogger.WithName("mutate"), monitor, handlers.Verify(monitor)))
	mux.HandlerFunc("GET", config.LivenessServicePath, handlers.Probe(register.Check))
	mux.HandlerFunc("GET", config.ReadinessServicePath, handlers.Probe(nil))
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
		},
		webhookRegister: register,
		cleanUp:         cleanUp,
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

func (s *server) cleanup(ctx context.Context) {
	cleanupKyvernoResource := s.webhookRegister.ShouldCleanupKyvernoResource()

	var wg sync.WaitGroup
	wg.Add(2)
	go s.webhookRegister.Remove(cleanupKyvernoResource, &wg)
	go s.webhookRegister.ResetPolicyStatus(cleanupKyvernoResource, &wg)
	wg.Wait()
	close(s.cleanUp)
}

func protect(inner handlers.AdmissionHandler) handlers.AdmissionHandler {
	return func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		if toggle.ProtectManagedResources.Enabled() {
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
		}
		return inner(logger, request, startTime)
	}
}

func filter(configuration config.Configuration, inner handlers.AdmissionHandler) handlers.AdmissionHandler {
	return handlers.Filter(configuration, inner)
}

func admission(logger logr.Logger, monitor *webhookconfig.Monitor, inner handlers.AdmissionHandler) http.HandlerFunc {
	return handlers.Monitor(monitor, handlers.Admission(logger, protect(inner)))
}

func registerWebhookHandlers(
	logger logr.Logger,
	mux *httprouter.Router,
	basePath string,
	monitor *webhookconfig.Monitor,
	configuration config.Configuration,
	handlerFunc func(logr.Logger, *admissionv1.AdmissionRequest, string, time.Time) *admissionv1.AdmissionResponse,
) {
	mux.HandlerFunc("POST", basePath, admission(logger, monitor, filter(
		configuration,
		func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
			return handlerFunc(logger, request, "all", startTime)
		})),
	)
	mux.HandlerFunc("POST", basePath+"/fail", admission(logger, monitor, filter(
		configuration,
		func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
			return handlerFunc(logger, request, "fail", startTime)
		})),
	)
	mux.HandlerFunc("POST", basePath+"/ignore", admission(logger, monitor, filter(
		configuration,
		func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
			return handlerFunc(logger, request, "ignore", startTime)
		})),
	)
}

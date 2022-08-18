package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/tracing"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	"go.opentelemetry.io/otel/attribute"
	admissionv1 "k8s.io/api/admission/v1"
)

type AdmissionHandler func(logr.Logger, *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse

func Admission(logger logr.Logger, inner AdmissionHandler) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		ctx := request.Context()
		startTime := time.Now()
		if request.Body == nil {
			logger.Info("empty body", "req", request.URL.String())
			http.Error(writer, "empty body", http.StatusBadRequest)
			return
		}
		defer request.Body.Close()
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			logger.Info("failed to read HTTP body", "req", request.URL.String())
			http.Error(writer, "failed to read HTTP body", http.StatusBadRequest)
			return
		}
		contentType := request.Header.Get("Content-Type")
		if contentType != "application/json" {
			logger.Info("invalid Content-Type", "contextType", contentType)
			http.Error(writer, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
			return
		}
		admissionReview := &admissionv1.AdmissionReview{}
		if err := json.Unmarshal(body, &admissionReview); err != nil {
			logger.Error(err, "failed to decode request body to type 'AdmissionReview")
			http.Error(writer, "Can't decode body as AdmissionReview", http.StatusExpectationFailed)
			return
		}
		logger := logger.WithValues(
			"kind", admissionReview.Request.Kind,
			"namespace", admissionReview.Request.Namespace,
			"name", admissionReview.Request.Name,
			"operation", admissionReview.Request.Operation,
			"uid", admissionReview.Request.UID,
		)
		admissionReview.Response = &admissionv1.AdmissionResponse{
			Allowed: true,
			UID:     admissionReview.Request.UID,
		}
		adminssionResponse := inner(logger, admissionReview.Request)
		if adminssionResponse != nil {
			admissionReview.Response = adminssionResponse
		}
		responseJSON, err := json.Marshal(admissionReview)
		if err != nil {
			http.Error(writer, fmt.Sprintf("Could not encode response: %v", err), http.StatusInternalServerError)
			return
		}

		// start span from request context
		attributes := []attribute.KeyValue{
			attribute.String("kind", admissionReview.Request.Kind.Kind),
			attribute.String("namespace", admissionReview.Request.Namespace),
			attribute.String("name", admissionReview.Request.Name),
			attribute.String("operation", string(admissionReview.Request.Operation)),
			attribute.String("uid", string(admissionReview.Request.UID)),
		}
		span := tracing.StartSpan(ctx, "admission_webhook_operations", string(admissionReview.Request.Operation), attributes)
		defer span.End()

		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err := writer.Write(responseJSON); err != nil {
			http.Error(writer, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
		if admissionReview.Request.Kind.Kind == "Lease" {
			logger.V(6).Info("admission review request processed", "time", time.Since(startTime).String())
		} else {
			logger.V(4).Info("admission review request processed", "time", time.Since(startTime).String())
		}
	}
}

func Filter(c config.Configuration, inner AdmissionHandler) AdmissionHandler {
	return func(logger logr.Logger, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
		if c.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
			return nil
		}
		return inner(logger, request)
	}
}

func Verify(m *webhookconfig.Monitor) AdmissionHandler {
	return func(logger logr.Logger, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
		logger.V(6).Info("incoming request", "last admission request timestamp", m.Time())
		return admissionutils.Response(true)
	}
}

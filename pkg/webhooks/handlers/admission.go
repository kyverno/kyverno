package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	admissionv1 "k8s.io/api/admission/v1"
)

type AdmissionHandler func(*admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse

func Admission(logger logr.Logger, inner AdmissionHandler) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
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
		admissionReview.Response = &admissionv1.AdmissionResponse{
			Allowed: true,
			UID:     admissionReview.Request.UID,
		}
		adminssionResponse := inner(admissionReview.Request)
		if adminssionResponse != nil {
			admissionReview.Response = adminssionResponse
		}
		responseJSON, err := json.Marshal(admissionReview)
		if err != nil {
			http.Error(writer, fmt.Sprintf("Could not encode response: %v", err), http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err := writer.Write(responseJSON); err != nil {
			http.Error(writer, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}

		if admissionReview.Request.Kind.Kind == "Lease" {
			if logger.V(6).Enabled() {
				logger := logger.WithValues(
					"kind", admissionReview.Request.Kind,
					"namespace", admissionReview.Request.Namespace,
					"name", admissionReview.Request.Name,
					"operation", admissionReview.Request.Operation,
					"uid", admissionReview.Request.UID,
				)
				logger.V(6).Info("admission review request processed", "time", time.Since(startTime).String())
			}
		} else {
			if logger.V(4).Enabled() {
				logger := logger.WithValues(
					"kind", admissionReview.Request.Kind,
					"namespace", admissionReview.Request.Namespace,
					"name", admissionReview.Request.Name,
					"operation", admissionReview.Request.Operation,
					"uid", admissionReview.Request.UID,
				)
				logger.V(4).Info("admission review request processed", "time", time.Since(startTime).String())
			}
		}
	}
}

func Filter(c config.Configuration, inner AdmissionHandler) AdmissionHandler {
	return func(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
		if c.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
			return nil
		}
		return inner(request)
	}
}

func Verify(m *webhookconfig.Monitor, logger logr.Logger) AdmissionHandler {
	return func(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
		if logger.V(6).Enabled() {
			logger := logger.WithValues(
				"action", "verify",
				"kind", request.Kind,
				"namespace", request.Namespace,
				"name", request.Name,
				"operation", request.Operation,
				"gvk", request.Kind.String(),
			)
			logger.V(6).Info("incoming request", "last admission request timestamp", m.Time())
		}
		return admissionutils.Response(true)
	}
}

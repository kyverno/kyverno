package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/tracing"
	admissionv1 "k8s.io/api/admission/v1"
)

func (h AdmissionHandler) WithAdmission(logger logr.Logger) HttpHandler {
	return withAdmission(logger, h)
}

func withAdmission(logger logr.Logger, inner AdmissionHandler) HttpHandler {
	return func(writer http.ResponseWriter, request *http.Request) {
		startTime := time.Now()
		if request.Body == nil {
			logger.Info("empty body", "req", request.URL.String())
			http.Error(writer, "empty body", http.StatusBadRequest)
			return
		}
		defer request.Body.Close()
		body, err := io.ReadAll(request.Body)
		if err != nil {
			logger.Info("failed to read HTTP body", "req", request.URL.String())
			http.Error(writer, "failed to read HTTP body", http.StatusBadRequest)
			return
		}
		contentType := request.Header.Get("Content-Type")
		if contentType != "application/json" {
			logger.Info("invalid Content-Type", "contentType", contentType)
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
			"user", admissionReview.Request.UserInfo,
		)
		admissionReview.Response = &admissionv1.AdmissionResponse{
			Allowed: true,
			UID:     admissionReview.Request.UID,
		}
		// start span from request context
		ctx, span := tracing.StartSpan(
			request.Context(),
			"webhooks/handlers",
			fmt.Sprintf("ADMISSION %s %s", admissionReview.Request.Operation, admissionReview.Request.Kind),
			admissionRequestAttributes(admissionReview.Request)...,
		)
		defer span.End()
		admissionResponse := inner(ctx, logger, admissionReview.Request, startTime)
		if admissionResponse != nil {
			admissionReview.Response = admissionResponse
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
			logger.V(6).Info("admission review request processed", "time", time.Since(startTime).String())
		} else {
			logger.V(4).Info("admission review request processed", "time", time.Since(startTime).String())
		}
	}
}

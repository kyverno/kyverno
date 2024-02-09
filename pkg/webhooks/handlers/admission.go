package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
)

func (inner AdmissionHandler) WithAdmission(logger logr.Logger) HttpHandler {
	return inner.withAdmission(logger).WithMetrics(logger).WithTrace("ADMISSION")
}

func (inner AdmissionHandler) withAdmission(logger logr.Logger) HttpHandler {
	return func(writer http.ResponseWriter, request *http.Request) {
		startTime := time.Now()
		if request.Body == nil {
			HttpError(request.Context(), writer, request, logger, errors.New("empty body"), http.StatusBadRequest)
			return
		}
		defer request.Body.Close()
		body, err := io.ReadAll(request.Body)
		if err != nil {
			HttpError(request.Context(), writer, request, logger, err, http.StatusBadRequest)
			return
		}
		contentType := request.Header.Get("Content-Type")
		if contentType != "application/json" {
			HttpError(request.Context(), writer, request, logger, errors.New("invalid Content-Type"), http.StatusUnsupportedMediaType)
			return
		}
		var admissionReview admissionv1.AdmissionReview
		if err := json.Unmarshal(body, &admissionReview); err != nil {
			HttpError(request.Context(), writer, request, logger, err, http.StatusExpectationFailed)
			return
		}
		logger := logger.WithValues(
			"gvk", admissionReview.Request.Kind,
			"gvr", admissionReview.Request.Resource,
			"namespace", admissionReview.Request.Namespace,
			"name", admissionReview.Request.Name,
			"operation", admissionReview.Request.Operation,
			"uid", admissionReview.Request.UID,
			"user", admissionReview.Request.UserInfo,
		)
		admissionRequest := AdmissionRequest{
			AdmissionRequest: *admissionReview.Request,
		}
		admissionResponse := inner(request.Context(), logger, admissionRequest, startTime)
		admissionReview.Response = &admissionResponse
		responseJSON, err := json.Marshal(admissionReview)
		if err != nil {
			HttpError(request.Context(), writer, request, logger, err, http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err := writer.Write(responseJSON); err != nil {
			HttpError(request.Context(), writer, request, logger, err, http.StatusInternalServerError)
			return
		}
	}
}

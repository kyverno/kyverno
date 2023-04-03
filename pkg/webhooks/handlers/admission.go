package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/userinfo"
	admissionv1 "k8s.io/api/admission/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

func (inner AdmissionHandler) WithAdmission(
	logger logr.Logger,
	rbLister rbacv1listers.RoleBindingLister,
	crbLister rbacv1listers.ClusterRoleBindingLister,
) HttpHandler {
	return inner.withAdmission(logger, rbLister, crbLister).WithTrace("ADMISSION")
}

func (inner AdmissionHandler) withAdmission(
	logger logr.Logger,
	rbLister rbacv1listers.RoleBindingLister,
	crbLister rbacv1listers.ClusterRoleBindingLister,
) HttpHandler {
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
		if rbLister != nil && crbLister != nil {
			if roles, clusterRoles, err := userinfo.GetRoleRef(rbLister, crbLister, admissionReview.Request.UserInfo); err != nil {
				HttpError(request.Context(), writer, request, logger, err, http.StatusInternalServerError)
				return
			} else {
				admissionRequest.Roles = roles
				admissionRequest.ClusterRoles = clusterRoles
				logger = logger.WithValues(
					"roles", roles,
					"clusterroles", clusterRoles,
				)
			}
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
		if admissionReview.Request.Kind.Kind == "Lease" {
			logger.V(6).Info("admission review request processed", "time", time.Since(startTime).String())
		} else {
			logger.V(4).Info("admission review request processed", "time", time.Since(startTime).String())
		}
	}
}

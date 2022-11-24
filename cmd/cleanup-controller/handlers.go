package main

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/cleanup-controller/validate"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
)

type cleanupPolicyHandlers struct {
	client dclient.Interface
}

func NewHandlers(client dclient.Interface) CleanupPolicyHandlers {
	return &cleanupPolicyHandlers{
		client: client,
	}
}

func (h *cleanupPolicyHandlers) Validate(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, _ time.Time) *admissionv1.AdmissionResponse {
	if request.SubResource != "" {
		logger.V(4).Info("skip policy validation on status update")
		return admissionutils.ResponseSuccess(request.UID)
	}
	policy, _, err := admissionutils.GetCleanupPolicies(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(request.UID, err)
	}
	err = validate.ValidateCleanupPolicy(policy, h.client, false)
	if err != nil {
		logger.Error(err, "policy validation errors")
		return admissionutils.Response(request.UID, err)
	}
	return admissionutils.Response(request.UID, err)
}

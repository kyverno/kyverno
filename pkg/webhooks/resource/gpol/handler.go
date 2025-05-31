package gpol

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
)

type handler struct {
	urGenerator updaterequest.Generator
}

func New(
	urGenerator updaterequest.Generator,
) *handler {
	return &handler{
		urGenerator: urGenerator,
	}
}

func (h *handler) Generate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	var policies []string
	if params := httprouter.ParamsFromContext(ctx); params != nil {
		if params := strings.Split(strings.TrimLeft(params.ByName("policies"), "/"), "/"); len(params) != 0 {
			policies = params
		}
	}

	go func(policies []string, request handlers.AdmissionRequest, logger logr.Logger) {
		admissionRequest := request.AdmissionRequest
		userInfo := kyvernov2.RequestInfo{
			AdmissionUserInfo: *request.UserInfo.DeepCopy(),
			Roles:             request.Roles,
			ClusterRoles:      request.ClusterRoles,
		}
		for _, policy := range policies {
			trigger, _, err := admissionutils.ExtractResources(nil, admissionRequest)
			if err != nil {
				logger.Error(err, "failed to extract resources from admission request")
				break
			}
			triggerSpec := kyvernov1.ResourceSpec{
				APIVersion: trigger.GetAPIVersion(),
				Kind:       trigger.GetKind(),
				Namespace:  trigger.GetNamespace(),
				Name:       trigger.GetName(),
				UID:        trigger.GetUID(),
			}
			logger.V(4).Info("creating the UR to generate downstream on trigger's operation", "operation", request.Operation, "policy", policy)
			urSpec := buildURSpecNew(kyvernov2.CELGenerate, policy, triggerSpec, false)
			urSpec.Context = buildURContext(admissionRequest, userInfo)
			if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
				logger.Error(err, "failed to create update request for generate policy", "policy", policy)
			} else {
				logger.V(4).Info("update request created for generate policy", "policy", policy)
			}
		}
	}(policies, request, logger)

	return admissionutils.Response(request.UID, nil)
}

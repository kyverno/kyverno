package handlers

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const namespaceControllerUsername = "system:serviceaccount:kube-system:namespace-controller"

const generateLabelPrefix = "generate.kyverno.io/"

var kyvernoUsernamePrefix = fmt.Sprintf("system:serviceaccount:%s:", config.KyvernoNamespace())

func (inner AdmissionHandler) WithProtection(enabled bool, controllerUsernames ...string) AdmissionHandler {
	inner = inner.withGenerateLabelProtection(controllerUsernames)
	if !enabled {
		return inner
	}
	return inner.withProtection().WithTrace("PROTECT")
}

func (inner AdmissionHandler) withGenerateLabelProtection(controllerUsernames []string) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		// Only configured controllers may change routing metadata. Other service
		// accounts in the installation namespace remain ordinary callers.
		if request.UserInfo.Username != "" && slices.Contains(controllerUsernames, request.UserInfo.Username) {
			return inner(ctx, logger, request, startTime)
		}
		if request.Operation != admissionv1.Create && request.Operation != admissionv1.Update {
			return inner(ctx, logger, request, startTime)
		}

		newResource, oldResource, err := admissionutils.ExtractResources(nil, request.AdmissionRequest)
		if err != nil {
			logger.Error(err, "failed to extract resources")
			return admissionutils.Response(request.UID, err)
		}
		if !maps.Equal(generateMetadata(newResource.GetLabels()), generateMetadata(oldResource.GetLabels())) {
			logger.V(2).Info("access to Kyverno generate labels is not authorized")
			return admissionutils.Response(request.UID, errors.New("Kyverno generate labels can only be set by Kyverno"))
		}
		return inner(ctx, logger, request, startTime)
	}
}

func generateMetadata(labels map[string]string) map[string]string {
	metadata := make(map[string]string)
	for key, value := range labels {
		// Clone sources remain user-owned and may be restored or replaced without
		// retaining the marker Kyverno adds to them.
		if strings.HasPrefix(key, generateLabelPrefix) && key != common.GenerateTypeCloneSourceLabel {
			metadata[key] = value
		}
	}
	// The generic managed-by label is reserved only alongside generation routing
	// metadata. The optional managed-resource protection handles it otherwise.
	if len(metadata) != 0 {
		if value, ok := labels[kyverno.LabelAppManagedBy]; ok {
			metadata[kyverno.LabelAppManagedBy] = value
		}
	}
	return metadata
}

func (inner AdmissionHandler) withProtection() AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		// Allows deletion of namespace containing managed resources
		if request.Operation == admissionv1.Delete && request.UserInfo.Username == namespaceControllerUsername {
			return inner(ctx, logger, request, startTime)
		}
		newResource, oldResource, err := admissionutils.ExtractResources(nil, request.AdmissionRequest)
		if err != nil {
			logger.Error(err, "failed to extract resources")
			return admissionutils.Response(request.UID, err)
		}
		for _, resource := range []unstructured.Unstructured{newResource, oldResource} {
			resLabels := resource.GetLabels()
			if resLabels[kyverno.LabelAppManagedBy] == kyverno.ValueKyvernoApp {
				if !strings.HasPrefix(request.UserInfo.Username, kyvernoUsernamePrefix) {
					logger.V(2).Info("access to the resource not authorized, this is a kyverno managed resource and should be altered only by kyverno")
					return admissionutils.Response(request.UID, errors.New("A kyverno managed resource can only be modified by kyverno"))
				}
			}
		}
		return inner(ctx, logger, request, startTime)
	}
}

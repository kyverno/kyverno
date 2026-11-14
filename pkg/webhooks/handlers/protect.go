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

const (
	endpointsControllerUsername           = "system:serviceaccount:kube-system:endpoint-controller"
	endpointSliceControllerUsername       = "system:serviceaccount:kube-system:endpointslice-controller"
	endpointSliceMirrorControllerUsername = "system:serviceaccount:kube-system:endpointslice-mirroring-controller"
)

const generateLabelPrefix = "generate.kyverno.io/"

var kyvernoUsernamePrefix = fmt.Sprintf("system:serviceaccount:%s:", config.KyvernoNamespace())

func (inner AdmissionHandler) WithProtection(enabled bool) AdmissionHandler {
	if !enabled {
		return inner
	}
	return inner.withProtection().WithTrace("PROTECT")
}

// WithGenerateLabelProtection guards routing metadata on the dedicated generation webhook.
func (inner AdmissionHandler) WithGenerateLabelProtection(controllerUsernames ...string) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		// Only configured controllers may change routing metadata. Other service
		// accounts in the installation namespace remain ordinary callers.
		if isControllerUsername(request.UserInfo.Username, controllerUsernames) {
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
		// Allow secondary child objects instantiated by controllers:
		// 1. On CREATE: objects carrying ownerReferences (e.g. CertificateRequest for Certificate, EndpointSlice for Service).
		// 2. On CREATE & UPDATE: core Endpoints and EndpointSlices managed by kube-controller-manager
		//    (endpoints-controller and endpointslice-controller reconcile Service labels along with endpoint subsets).
		if request.Operation == admissionv1.Create && len(newResource.GetOwnerReferences()) > 0 {
			return inner(ctx, logger, request, startTime)
		}
		if isEndpointsController(request.UserInfo.Username) &&
			((request.Kind.Group == "" && request.Kind.Kind == "Endpoints") ||
				(request.Kind.Group == "discovery.k8s.io" && request.Kind.Kind == "EndpointSlice")) {
			return inner(ctx, logger, request, startTime)
		}
		if !maps.Equal(extractKyvernoGenerateLabels(newResource.GetLabels()), extractKyvernoGenerateLabels(oldResource.GetLabels())) {
			logger.V(2).Info("access to Kyverno generate labels is not authorized")
			return admissionutils.Response(request.UID, errors.New("Kyverno generate labels can only be set by Kyverno"))
		}
		return inner(ctx, logger, request, startTime)
	}
}

func extractKyvernoGenerateLabels(labels map[string]string) map[string]string {
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

func isControllerUsername(username string, controllerUsernames []string) bool {
	return username != "" && slices.Contains(controllerUsernames, username)
}

func isEndpointsController(username string) bool {
	switch username {
	case endpointsControllerUsername, endpointSliceControllerUsername, endpointSliceMirrorControllerUsername:
		return true
	default:
		return false
	}
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

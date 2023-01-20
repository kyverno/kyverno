package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (inner AdmissionHandler) WithProtection(enabled bool, enablePolex bool, polexNamespace string) AdmissionHandler {
	if !enabled {
		return inner
	}
	return inner.withProtection(enablePolex, polexNamespace).WithTrace("PROTECT")
}

func (inner AdmissionHandler) withProtection(enablePolex bool, polexNamespace string) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		newResource, oldResource, err := admissionutils.ExtractResources(nil, request)
		if err != nil {
			logger.Error(err, "Failed to extract resources")
			return admissionutils.Response(request.UID, err)
		}
		for _, resource := range []unstructured.Unstructured{newResource, oldResource} {
			resLabels := resource.GetLabels()
			if resLabels[kyvernov1.LabelAppManagedBy] == kyvernov1.ValueKyvernoApp {
				if request.UserInfo.Username != fmt.Sprintf("system:serviceaccount:%s:%s", config.KyvernoNamespace(), config.KyvernoServiceAccountName()) {
					logger.Info("Access to the resource not authorized, this is a kyverno managed resource and should be altered only by kyverno")
					return admissionutils.ResponseSuccess(request.UID, "A kyverno managed resource can only be modified by kyverno")
				}
			}
			kind := resource.GetKind()
			if kind == "PolicyException" {
				warnings := []string{}
				if !enablePolex {
					warnings = append(warnings, string(disabledPolex))
				}
				polexns := strings.ToLower(polexNamespace)
				if polexns != "" {
					// check any and all
					if matchcases, ok, _ := unstructured.NestedMap(resource.Object, "spec", "match"); ok {
						for _, c := range matchcases {
							validateResourceNamespaces(&warnings, c, polexns)
						}
					}
				}
				return admissionutils.ResponseSuccess(request.UID, warnings...)
			}
		}

		return inner(ctx, logger, request, startTime)
	}
}

func validateResourceNamespaces(warnings *[]string, matchCase interface{}, polexns string) {
	if filters, ok := matchCase.([]interface{}); ok {
		for _, filter := range filters {
			if object, err := datautils.ToMap(filter); err != nil {
				if namespaces, ok, _ := unstructured.NestedSlice(object, "resources", "namespaces"); ok {
					for _, namespace := range namespaces {
						if ns, ok := namespace.(string); ok {
							if ns != polexns {
								if !warningExists(*warnings, namespacesDontMatch) {
									*warnings = append(*warnings, namespacesDontMatch)
								}
							}
						}
					}
				}
				if subjects, ok, _ := unstructured.NestedSlice(object, "subjects"); ok {
					for _, subject := range subjects {
						if m, err := datautils.ToMap(subject); err != nil {
							if m["namespace"] != polexns {
								if !warningExists(*warnings, namespacesDontMatch) {
									*warnings = append(*warnings, namespacesDontMatch)
								}
							}
						}
					}
				}
			}
		}
	}
}

func warningExists(ws []string, w string) bool {
	for _, wng := range ws {
		if wng == w {
			return true
		}
	}
	return false
}

var (
	namespacesDontMatch string = "PolicyException resource namespaces must match the defined namespace."
	disabledPolex       string = "PolicyException resources would not be processed until it is enabled."
)

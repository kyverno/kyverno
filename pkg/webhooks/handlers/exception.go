package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (inner AdmissionHandler) WithExceptionValidation(enablePolex bool, polexNamespace string) AdmissionHandler {
	return inner.withExceptionValidation(enablePolex, polexNamespace).WithTrace("EXCEPTION-VALIDATION")
}

func (inner AdmissionHandler) withExceptionValidation(enablePolex bool, polexNamespace string) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		newResource, oldResource, err := admissionutils.ExtractResources(nil, request)
		if err != nil {
			logger.Error(err, "Failed to extract resources")
			return admissionutils.Response(request.UID, err)
		}
		for _, resource := range []unstructured.Unstructured{newResource, oldResource} {
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

package handlers

import (
	"strings"
	"time"

	"github.com/go-logr/logr"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func dumpPayload(logger logr.Logger, request *admissionv1.AdmissionRequest, response *admissionv1.AdmissionResponse) {
	reqPayload, err := newAdmissionRequestPayload(request)
	if err != nil {
		logger.Error(err, "Failed to extract resources")
	} else {
		logger.Info("Logging admission request and response payload ", "AdmissionRequest", reqPayload, "AdmissionResponse", response)
	}
}

// admissionRequestPayload holds a copy of the AdmissionRequest payload
type admissionRequestPayload struct {
	UID                types.UID                    `json:"uid"`
	Kind               metav1.GroupVersionKind      `json:"kind"`
	Resource           metav1.GroupVersionResource  `json:"resource"`
	SubResource        string                       `json:"subResource,omitempty"`
	RequestKind        *metav1.GroupVersionKind     `json:"requestKind,omitempty"`
	RequestResource    *metav1.GroupVersionResource `json:"requestResource,omitempty"`
	RequestSubResource string                       `json:"requestSubResource,omitempty"`
	Name               string                       `json:"name,omitempty"`
	Namespace          string                       `json:"namespace,omitempty"`
	Operation          string                       `json:"operation"`
	UserInfo           authenticationv1.UserInfo    `json:"userInfo"`
	Object             unstructured.Unstructured    `json:"object,omitempty"`
	OldObject          unstructured.Unstructured    `json:"oldObject,omitempty"`
	DryRun             *bool                        `json:"dryRun,omitempty"`
	Options            unstructured.Unstructured    `json:"options,omitempty"`
}

func newAdmissionRequestPayload(rq *admissionv1.AdmissionRequest) (*admissionRequestPayload, error) {
	newResource, oldResource, err := utils.ExtractResources(nil, rq)
	if err != nil {
		return nil, err
	}
	options := new(unstructured.Unstructured)
	if rq.Options.Raw != nil {
		options, err = engineutils.ConvertToUnstructured(rq.Options.Raw)
		if err != nil {
			return nil, err
		}
	}
	return redactPayload(&admissionRequestPayload{
		UID:                rq.UID,
		Kind:               rq.Kind,
		Resource:           rq.Resource,
		SubResource:        rq.SubResource,
		RequestKind:        rq.RequestKind,
		RequestResource:    rq.RequestResource,
		RequestSubResource: rq.RequestSubResource,
		Name:               rq.Name,
		Namespace:          rq.Namespace,
		Operation:          string(rq.Operation),
		UserInfo:           rq.UserInfo,
		Object:             newResource,
		OldObject:          oldResource,
		DryRun:             rq.DryRun,
		Options:            *options,
	})
}

func redactPayload(payload *admissionRequestPayload) (*admissionRequestPayload, error) {
	if strings.EqualFold(payload.Kind.Kind, "Secret") {
		if payload.Object.Object != nil {
			obj, err := utils.RedactSecret(&payload.Object)
			if err != nil {
				return nil, err
			}
			payload.Object = obj
		}
		if payload.OldObject.Object != nil {
			oldObj, err := utils.RedactSecret(&payload.OldObject)
			if err != nil {
				return nil, err
			}
			payload.OldObject = oldObj
		}
	}
	return payload, nil
}

func Dump(inner AdmissionHandler) AdmissionHandler {
	return func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		response := inner(logger, request, startTime)
		dumpPayload(logger, request, response)
		return response
	}
}

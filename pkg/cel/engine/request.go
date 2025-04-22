package engine

import (
	"github.com/kyverno/kyverno/pkg/cel/libs"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

type EngineRequest struct {
	JsonPayload *unstructured.Unstructured
	Request     admissionv1.AdmissionRequest
	Context     libs.Context
}

func RequestFromAdmission(context libs.Context, request admissionv1.AdmissionRequest) EngineRequest {
	return EngineRequest{
		Request: request,
		Context: context,
	}
}

func RequestFromJSON(context libs.Context, jsonPayload *unstructured.Unstructured) EngineRequest {
	return EngineRequest{
		JsonPayload: jsonPayload,
		Context:     context,
	}
}

func Request(
	context libs.Context,
	gvk schema.GroupVersionKind,
	gvr schema.GroupVersionResource,
	subResource string,
	name string,
	namespace string,
	operation admissionv1.Operation,
	userInfo authenticationv1.UserInfo,
	object runtime.Object,
	oldObject runtime.Object,
	dryRun bool,
	options runtime.Object,
) EngineRequest {
	request := admissionv1.AdmissionRequest{
		Kind:               metav1.GroupVersionKind(gvk),
		Resource:           metav1.GroupVersionResource(gvr),
		SubResource:        subResource,
		RequestKind:        ptr.To(metav1.GroupVersionKind(gvk)),
		RequestResource:    ptr.To(metav1.GroupVersionResource(gvr)),
		RequestSubResource: subResource,
		Name:               name,
		Namespace:          namespace,
		Operation:          operation,
		UserInfo:           userInfo,
		Object:             runtime.RawExtension{Object: object},
		OldObject:          runtime.RawExtension{Object: oldObject},
		DryRun:             &dryRun,
		Options:            runtime.RawExtension{Object: options},
	}
	return RequestFromAdmission(context, request)
}

func (r *EngineRequest) AdmissionRequest() admissionv1.AdmissionRequest {
	return r.Request
}

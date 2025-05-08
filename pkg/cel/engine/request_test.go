package engine

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

func TestRequestFromAdmission(t *testing.T) {
	context := libs.NewFakeContextProvider()
	tests := []struct {
		name    string
		context libs.Context
		request admissionv1.AdmissionRequest
		want    EngineRequest
	}{{
		name:    "nil context",
		context: nil,
		request: admissionv1.AdmissionRequest{},
		want: EngineRequest{
			Context: nil,
			Request: admissionv1.AdmissionRequest{},
		},
	}, {
		name:    "test",
		context: context,
		request: admissionv1.AdmissionRequest{},
		want: EngineRequest{
			Context: context,
			Request: admissionv1.AdmissionRequest{},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RequestFromAdmission(tt.context, tt.request)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRequestFromJSON(t *testing.T) {
	context := libs.NewFakeContextProvider()
	tests := []struct {
		name        string
		context     libs.Context
		jsonPayload *unstructured.Unstructured
		want        EngineRequest
	}{{
		name:        "nil context",
		context:     nil,
		jsonPayload: &unstructured.Unstructured{},
		want: EngineRequest{
			Context:     nil,
			JsonPayload: &unstructured.Unstructured{},
		},
	}, {
		name:        "nil payload",
		context:     context,
		jsonPayload: nil,
		want: EngineRequest{
			Context:     context,
			JsonPayload: nil,
		},
	}, {
		name:        "test",
		context:     context,
		jsonPayload: &unstructured.Unstructured{},
		want: EngineRequest{
			Context:     context,
			JsonPayload: &unstructured.Unstructured{},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RequestFromJSON(tt.context, tt.jsonPayload)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRequest(t *testing.T) {
	context := libs.NewFakeContextProvider()
	type args struct {
		context     libs.Context
		gvk         schema.GroupVersionKind
		gvr         schema.GroupVersionResource
		subResource string
		name        string
		namespace   string
		operation   admissionv1.Operation
		userInfo    authenticationv1.UserInfo
		object      runtime.Object
		oldObject   runtime.Object
		dryRun      bool
		options     runtime.Object
	}
	tests := []struct {
		name string
		args args
		want EngineRequest
	}{{
		name: "nil context",
		args: args{
			context: nil,
		},
		want: EngineRequest{
			Context: nil,
			Request: admissionv1.AdmissionRequest{
				RequestKind:     &metav1.GroupVersionKind{},
				RequestResource: &metav1.GroupVersionResource{},
				DryRun:          ptr.To(false),
			},
		},
	}, {
		name: "test",
		args: args{
			context:     context,
			gvk:         schema.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"},
			gvr:         schema.GroupVersionResource{Group: "foo", Version: "bar", Resource: "baz"},
			subResource: "test",
			name:        "test-name",
			namespace:   "test-ns",
			operation:   "CREATE",
			userInfo: authenticationv1.UserInfo{
				Username: "test",
			},
			object:    nil,
			oldObject: nil,
			dryRun:    true,
			options:   nil,
		},
		want: EngineRequest{
			Context: context,
			Request: admissionv1.AdmissionRequest{
				Kind:               metav1.GroupVersionKind(schema.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"}),
				Resource:           metav1.GroupVersionResource(schema.GroupVersionResource{Group: "foo", Version: "bar", Resource: "baz"}),
				SubResource:        "test",
				RequestKind:        ptr.To(metav1.GroupVersionKind(schema.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"})),
				RequestResource:    ptr.To(metav1.GroupVersionResource(schema.GroupVersionResource{Group: "foo", Version: "bar", Resource: "baz"})),
				RequestSubResource: "test",
				Name:               "test-name",
				Namespace:          "test-ns",
				Operation:          "CREATE",
				UserInfo: authenticationv1.UserInfo{
					Username: "test",
				},
				DryRun: ptr.To(true),
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Request(
				tt.args.context,
				tt.args.gvk,
				tt.args.gvr,
				tt.args.subResource,
				tt.args.name,
				tt.args.namespace,
				tt.args.operation,
				tt.args.userInfo,
				tt.args.object,
				tt.args.oldObject,
				tt.args.dryRun,
				tt.args.options,
			)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEngineRequest_AdmissionRequest(t *testing.T) {
	tests := []struct {
		name    string
		request EngineRequest
		want    admissionv1.AdmissionRequest
	}{{
		name: "test",
		request: Request(
			nil,
			schema.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"},
			schema.GroupVersionResource{Group: "foo", Version: "bar", Resource: "baz"},
			"test",
			"test-name",
			"test-ns",
			"CREATE",
			authenticationv1.UserInfo{
				Username: "test",
			},
			nil,
			nil,
			true,
			nil,
		),
		want: admissionv1.AdmissionRequest{
			Kind:               metav1.GroupVersionKind(schema.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"}),
			Resource:           metav1.GroupVersionResource(schema.GroupVersionResource{Group: "foo", Version: "bar", Resource: "baz"}),
			SubResource:        "test",
			RequestKind:        ptr.To(metav1.GroupVersionKind(schema.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"})),
			RequestResource:    ptr.To(metav1.GroupVersionResource(schema.GroupVersionResource{Group: "foo", Version: "bar", Resource: "baz"})),
			RequestSubResource: "test",
			Name:               "test-name",
			Namespace:          "test-ns",
			Operation:          "CREATE",
			UserInfo: authenticationv1.UserInfo{
				Username: "test",
			},
			DryRun: ptr.To(true),
		},
	}, {
		name:    "json",
		request: RequestFromJSON(nil, &unstructured.Unstructured{}),
		want:    admissionv1.AdmissionRequest{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.request.AdmissionRequest()
			assert.Equal(t, tt.want, got)
		})
	}
}

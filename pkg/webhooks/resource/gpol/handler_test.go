package gpol

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

type mockURGenerator struct {
	called atomic.Int32
}

func (m *mockURGenerator) Apply(_ context.Context, _ kyvernov2.UpdateRequestSpec) error {
	m.called.Add(1)
	return nil
}

type mockGeneratingPolicyLister struct {
	policies map[string]*policiesv1beta1.GeneratingPolicy
}

func (m *mockGeneratingPolicyLister) List(_ labels.Selector) ([]*policiesv1beta1.GeneratingPolicy, error) {
	result := make([]*policiesv1beta1.GeneratingPolicy, 0, len(m.policies))
	for _, policy := range m.policies {
		result = append(result, policy)
	}
	return result, nil
}

func (m *mockGeneratingPolicyLister) Get(name string) (*policiesv1beta1.GeneratingPolicy, error) {
	if policy, ok := m.policies[name]; ok {
		return policy, nil
	}
	return nil, fmt.Errorf("policy %s not found", name)
}

func TestGenerate_DryRunDoesNotCreateUpdateRequests(t *testing.T) {
	mock := &mockURGenerator{}
	h := New(mock, nil, nil, "system:serviceaccount:kyverno:kyverno-background-controller")

	ctx := context.WithValue(context.Background(), httprouter.ParamsKey, httprouter.Params{
		{Key: "policies", Value: "/test-policy"},
	})
	req := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			DryRun:    ptr.To(true),
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}

	resp := h.Generate(ctx, logr.Discard(), req, "", time.Now())
	assert.True(t, resp.Allowed)

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), mock.called.Load(), "dry-run request must not create UpdateRequests")
}

func TestGenerate_BackgroundRequestSkippedByDefault(t *testing.T) {
	mock := &mockURGenerator{}
	backgroundSA := "system:serviceaccount:kyverno:kyverno-background-controller"
	h := New(
		mock,
		&mockGeneratingPolicyLister{
			policies: map[string]*policiesv1beta1.GeneratingPolicy{
				"test-policy": {
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
				},
			},
		},
		nil,
		backgroundSA,
	)

	ctx := context.WithValue(context.Background(), httprouter.ParamsKey, httprouter.Params{
		{Key: "policies", Value: "/test-policy"},
	})
	req := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: backgroundSA},
		},
	}

	resp := h.Generate(ctx, logr.Discard(), req, "", time.Now())
	assert.True(t, resp.Allowed)

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), mock.called.Load(), "background request must be skipped by default")
}

func TestGenerate_BackgroundRequestAllowedWhenDisabled(t *testing.T) {
	mock := &mockURGenerator{}
	backgroundSA := "system:serviceaccount:kyverno:kyverno-background-controller"
	h := New(
		mock,
		&mockGeneratingPolicyLister{
			policies: map[string]*policiesv1beta1.GeneratingPolicy{
				"test-policy": {
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					Spec: policiesv1beta1.GeneratingPolicySpec{
						EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
							SkipBackgroundRequests: ptr.To(false),
						},
					},
				},
			},
		},
		nil,
		backgroundSA,
	)

	ctx := context.WithValue(context.Background(), httprouter.ParamsKey, httprouter.Params{
		{Key: "policies", Value: "/test-policy"},
	})
	req := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test","namespace":"default"}}`)},
			UserInfo:  authenticationv1.UserInfo{Username: backgroundSA},
		},
	}

	resp := h.Generate(ctx, logr.Discard(), req, "", time.Now())
	assert.True(t, resp.Allowed)

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(1), mock.called.Load(), "background request must be processed when explicitly disabled")
}

func TestGenerate_NoOpUpdateDoesNotCreateUpdateRequest(t *testing.T) {
	mock := &mockURGenerator{}
	h := New(
		mock,
		&mockGeneratingPolicyLister{
			policies: map[string]*policiesv1beta1.GeneratingPolicy{
				"test-policy": {
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					Spec: policiesv1beta1.GeneratingPolicySpec{
						EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
							SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{
								Enabled: ptr.To(true),
							},
						},
					},
				},
			},
		},
		nil,
		"system:serviceaccount:kyverno:kyverno-background-controller",
	)

	ctx := context.WithValue(context.Background(), httprouter.ParamsKey, httprouter.Params{
		{Key: "policies", Value: "/test-policy"},
	})

	noOpObject := []byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"test","uid":"d4ba2952-e07b-46e0-9f33-bf5304e4466e","resourceVersion":"31796"}}`)

	req := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Update,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			Object:    runtime.RawExtension{Raw: noOpObject},
			OldObject: runtime.RawExtension{Raw: noOpObject},
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}

	resp := h.Generate(ctx, logr.Discard(), req, "", time.Now())
	assert.True(t, resp.Allowed)

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), mock.called.Load(),
		"a zero-delta UPDATE (object identical to oldObject) must not create an UpdateRequest, "+
			"or the background controller will re-evaluate the trigger and delete the synced downstream (#17452)")
}

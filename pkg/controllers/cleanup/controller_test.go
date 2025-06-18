package cleanup

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/kyverno/kyverno/pkg/config/mocks"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_SkipResourceDueToFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := mocks.NewMockConfiguration(ctrl)

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}

	mockConfig.EXPECT().
		ToFilter(gvk, "ConfigMap", "kube-system", "filtered-cm").
		Return(true).
		AnyTimes()

	c := &controller{
		configuration: mockConfig,
	}

	resource := unstructured.Unstructured{}
	resource.SetKind("ConfigMap")
	resource.SetNamespace("kube-system")
	resource.SetName("filtered-cm")

	filtered := c.configuration.ToFilter(
		gvk, resource.GetKind(), resource.GetNamespace(), resource.GetName(),
	)

	assert.True(t, filtered, "Expected resource to be filtered and skipped")
}

func TestIsRecoverableError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		shouldRecover bool
	}{
		{
			name:          "nil error",
			err:           nil,
			shouldRecover: false,
		},
		{
			name:          "forbidden error",
			err:           apierrors.NewForbidden(schema.GroupResource{Group: "projectcalico.org", Resource: "networkpolicies"}, "test", fmt.Errorf("Operation on Calico tiered policy is forbidden")),
			shouldRecover: true,
		},
		{
			name:          "unauthorized error",
			err:           apierrors.NewUnauthorized("access denied"),
			shouldRecover: true,
		},
		{
			name:          "not found error",
			err:           apierrors.NewNotFound(schema.GroupResource{Group: "example.com", Resource: "nonexistent"}, "test"),
			shouldRecover: true,
		},
		{
			name:          "method not supported error",
			err:           apierrors.NewMethodNotSupported(schema.GroupResource{Group: "example.com", Resource: "test"}, "list"),
			shouldRecover: true,
		},
		{
			name:          "calico tiered policy forbidden error message",
			err:           fmt.Errorf("networkpolicies.projectcalico.org is forbidden: Operation on Calico tiered policy is forbidden"),
			shouldRecover: true,
		},
		{
			name:          "resource not found message",
			err:           fmt.Errorf("the server could not find the requested resource"),
			shouldRecover: true,
		},
		{
			name:          "no matches for kind error",
			err:           fmt.Errorf("no matches for kind \"SomeCustomResource\" in version \"v1\""),
			shouldRecover: true,
		},
		{
			name:          "unable to recognize error",
			err:           fmt.Errorf("unable to recognize \"test.yaml\""),
			shouldRecover: true,
		},
		{
			name:          "failed to list error",
			err:           fmt.Errorf("failed to list resources"),
			shouldRecover: true,
		},
		{
			name:          "no Kind is registered error",
			err:           fmt.Errorf("no Kind is registered for the type"),
			shouldRecover: true,
		},
		{
			name:          "connection timeout - non-recoverable",
			err:           fmt.Errorf("connection timeout"),
			shouldRecover: false,
		},
		{
			name:          "internal server error - non-recoverable",
			err:           apierrors.NewInternalError(fmt.Errorf("internal server error")),
			shouldRecover: false,
		},
		{
			name:          "service unavailable - non-recoverable",
			err:           apierrors.NewServiceUnavailable("service unavailable"),
			shouldRecover: false,
		},
		{
			name:          "generic error - non-recoverable",
			err:           fmt.Errorf("some unexpected error"),
			shouldRecover: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRecoverableError(tt.err)
			if result != tt.shouldRecover {
				t.Errorf("isRecoverableError() = %v, want %v for error: %v", result, tt.shouldRecover, tt.err)
			}
		})
	}
}

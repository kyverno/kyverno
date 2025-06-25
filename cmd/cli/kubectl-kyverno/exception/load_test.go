package exception

import (
	"os"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_load(t *testing.T) {
	tests := []struct {
		name                string
		policies            string
		exceptionsLoaded    int
		celExceptionsLoaded int
		wantErr             bool
	}{{
		name:     "not a policy exception",
		policies: "../_testdata/resources/namespace.yaml",
		wantErr:  true,
	}, {
		name:                "policy exception",
		policies:            "../_testdata/exceptions/exception.yaml",
		exceptionsLoaded:    1,
		celExceptionsLoaded: 0,
	}, {
		name:     "policy exception and policy",
		policies: "../_testdata/exceptions/exception-and-policy.yaml",
		wantErr:  true,
	}, {
		name:                "cel policy exception",
		policies:            "../_testdata/exceptions/celexception.yaml",
		exceptionsLoaded:    0,
		celExceptionsLoaded: 1,
	}, {
		name:                "cel policy exception and policy",
		policies:            "../_testdata/exceptions/exception-and-celexception.yaml",
		exceptionsLoaded:    1,
		celExceptionsLoaded: 1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := os.ReadFile(tt.policies)
			require.NoError(t, err)
			require.NoError(t, err)
			if res, err := load(bytes); (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			} else if res != nil {
				if len(res.Exceptions) != tt.exceptionsLoaded {
					t.Errorf("Load() loaded amount = %v, wantLoaded %v", len(res.Exceptions), tt.exceptionsLoaded)
				} else if len(res.CELExceptions) != tt.celExceptionsLoaded {
					t.Errorf("Load() loaded amount = %v, wantLoaded %v", len(res.CELExceptions), tt.celExceptionsLoaded)
				}
			}
		})
	}
}

func Test_SelectFrom(t *testing.T) {
	resources := toUnstructured(t,
		&corev1.ConfigMap{TypeMeta: v1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"}},
		&kyvernov2.PolicyException{TypeMeta: v1.TypeMeta{
			Kind: exceptionV2.Kind, APIVersion: exceptionV2.GroupVersion().String()},
		},
		&kyvernov2beta1.PolicyException{TypeMeta: v1.TypeMeta{
			Kind: exceptionV2beta1.Kind, APIVersion: exceptionV2beta1.GroupVersion().String()},
		},
	)
	exceptions := SelectFrom(resources)
	require.Len(t, exceptions, 2)
}

func toUnstructured(t *testing.T, in ...interface{}) []*unstructured.Unstructured {
	var resources []*unstructured.Unstructured
	for _, r := range in {
		us, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
		require.NoError(t, err)
		resources = append(resources, &unstructured.Unstructured{Object: us})
	}

	return resources
}

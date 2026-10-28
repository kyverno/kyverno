package common

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewBackgroundContextOperation(t *testing.T) {
	tests := []struct {
		name             string
		storedOperation  admissionv1.Operation
		admissionRequest *admissionv1.AdmissionRequest
		wantOperation    kyvernov1.AdmissionOperation
		wantOldResource  bool
		wantOldSource    string
	}{
		{
			name:          "policy-generated UR defaults to CREATE",
			wantOperation: kyvernov1.Create,
		},
		{
			name:            "non-admission UR preserves explicit UPDATE",
			storedOperation: admissionv1.Update,
			wantOperation:   kyvernov1.Update,
		},
		{
			name:            "non-admission UR preserves explicit DELETE",
			storedOperation: admissionv1.Delete,
			wantOperation:   kyvernov1.Delete,
			wantOldResource: true,
			wantOldSource:   "trigger",
		},
		{
			name:            "admission UR takes operation from request",
			storedOperation: admissionv1.Create,
			admissionRequest: &admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "Secret"},
				Namespace: "repro",
				Object:    runtime.RawExtension{Raw: []byte("{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"name\":\"trigger-secret\",\"namespace\":\"repro\"}}")},
			},
			wantOperation: kyvernov1.Update,
		},
		{
			name:            "admission DELETE preserves request old resource",
			storedOperation: admissionv1.Create,
			admissionRequest: &admissionv1.AdmissionRequest{
				Operation: admissionv1.Delete,
				Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "Secret"},
				Name:      "trigger-secret",
				Namespace: "repro",
				OldObject: runtime.RawExtension{Raw: []byte("{\"apiVersion\":\"v1\",\"kind\":\"Secret\",\"metadata\":{\"name\":\"trigger-secret\",\"namespace\":\"repro\",\"annotations\":{\"source\":\"admission\"}}}")},
			},
			wantOperation:   kyvernov1.Delete,
			wantOldResource: true,
			wantOldSource:   "admission",
		},
	}

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	trigger := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"namespace": "repro",
			"name":      "trigger-secret",
			"annotations": map[string]interface{}{
				"source": "trigger",
			},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urContext := kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					Operation:        tt.storedOperation,
					AdmissionRequest: tt.admissionRequest,
				},
			}
			policyContext, err := NewBackgroundContext(
				logr.Discard(), nil, urContext, &kyvernov1.ClusterPolicy{}, trigger, cfg, jp, nil,
			)
			require.NoError(t, err)
			require.Equal(t, tt.wantOperation, policyContext.Operation())
			// These contexts always evaluate outside the admission webhook.
			require.False(t, policyContext.AdmissionOperation())
			operation, err := policyContext.JSONContext().Query("request.operation")
			require.NoError(t, err)
			require.Equal(t, string(tt.wantOperation), operation)

			if tt.wantOldResource {
				oldResource := policyContext.OldResource()
				require.Equal(t, trigger.GetName(), oldResource.GetName())
				require.Equal(t, trigger.GetNamespace(), oldResource.GetNamespace())
				oldName, err := policyContext.JSONContext().Query("request.oldObject.metadata.name")
				require.NoError(t, err)
				require.Equal(t, trigger.GetName(), oldName)
				oldSource, err := policyContext.JSONContext().Query("request.oldObject.metadata.annotations.source")
				require.NoError(t, err)
				require.Equal(t, tt.wantOldSource, oldSource)
			}
		})
	}
}

func TestNewBackgroundContextOperationsMatching(t *testing.T) {
	tests := []struct {
		name              string
		storedOperation   admissionv1.Operation
		matchOperations   []kyvernov1.AdmissionOperation
		excludeOperations []kyvernov1.AdmissionOperation
		wantMatch         bool
	}{
		{
			name:            "default CREATE matches CREATE UPDATE",
			matchOperations: []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
			wantMatch:       true,
		},
		{
			name:            "default CREATE does not match UPDATE only",
			matchOperations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
		},
		{
			name:            "default CREATE does not match DELETE only",
			matchOperations: []kyvernov1.AdmissionOperation{kyvernov1.Delete},
		},
		{
			name:      "operations omitted matches",
			wantMatch: true,
		},
		{
			name:              "CREATE exclusion applies to background evaluation",
			matchOperations:   []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
			excludeOperations: []kyvernov1.AdmissionOperation{kyvernov1.Create},
		},
		{
			name:              "UPDATE exclusion does not block synthetic CREATE",
			matchOperations:   []kyvernov1.AdmissionOperation{kyvernov1.Create, kyvernov1.Update},
			excludeOperations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
			wantMatch:         true,
		},
		{
			name:            "explicit UPDATE still matches UPDATE only",
			storedOperation: admissionv1.Update,
			matchOperations: []kyvernov1.AdmissionOperation{kyvernov1.Update},
			wantMatch:       true,
		},
		{
			name:            "explicit DELETE still matches DELETE only",
			storedOperation: admissionv1.Delete,
			matchOperations: []kyvernov1.AdmissionOperation{kyvernov1.Delete},
			wantMatch:       true,
		},
	}
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	trigger := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"namespace": "repro",
			"name":      "trigger-secret",
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urContext := kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{Operation: tt.storedOperation},
			}
			policyContext, err := NewBackgroundContext(
				logr.Discard(), nil, urContext, &kyvernov1.ClusterPolicy{}, trigger, cfg, jp, nil,
			)
			require.NoError(t, err)

			rule := kyvernov1.Rule{
				Name: "background-operations",
				MatchResources: kyvernov1.MatchResources{
					Any: kyvernov1.ResourceFilters{{ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: tt.matchOperations,
					}}},
				},
			}
			if len(tt.excludeOperations) > 0 {
				rule.ExcludeResources = &kyvernov1.MatchResources{
					Any: kyvernov1.ResourceFilters{{ResourceDescription: kyvernov1.ResourceDescription{
						Kinds:      []string{"Secret"},
						Operations: tt.excludeOperations,
					}}},
				}
			}

			err = engineutils.MatchesResourceDescription(
				policyContext.NewResource(), rule, policyContext.AdmissionInfo(),
				policyContext.NamespaceLabels(), "", trigger.GroupVersionKind(), "", policyContext.Operation(),
			)
			if tt.wantMatch {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

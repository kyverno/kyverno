package v2

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestGlobalContextEntrySpecValidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    GlobalContextEntrySpec
		wantErr bool
	}{
		{
			name: "valid KubernetesResource",
			spec: GlobalContextEntrySpec{
				KubernetesResource: &KubernetesResource{
					Group:    "apps",
					Version:  "v1",
					Resource: "deployments",
				},
			},
			wantErr: false,
		},
		{
			name: "valid APICall",
			spec: GlobalContextEntrySpec{
				APICall: &ExternalAPICall{
					APICall: kyverno.APICall{
						URLPath: "/api/v1/namespaces",
					},
					RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
				},
			},
			wantErr: false,
		},
		{
			name: "both KubernetesResource and APICall",
			spec: GlobalContextEntrySpec{
				KubernetesResource: &KubernetesResource{
					Group:    "apps",
					Version:  "v1",
					Resource: "deployments",
				},
				APICall: &ExternalAPICall{
					APICall: kyverno.APICall{
						URLPath: "/api/v1/namespaces",
					},
					RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
				},
			},
			wantErr: true,
		},
		{
			name:    "neither KubernetesResource nor APICall",
			spec:    GlobalContextEntrySpec{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.spec.Validate(field.NewPath("spec"), "")
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("GlobalContextEntrySpec.Validate() error = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestKubernetesResourceValidate(t *testing.T) {
	tests := []struct {
		name     string
		resource KubernetesResource
		wantErr  bool
	}{
		{
			name: "valid KubernetesResource",
			resource: KubernetesResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			wantErr: false,
		},
		{
			name: "missing group only if version is v1 or CoreGroup",
			resource: KubernetesResource{
				Group:    "",
				Version:  "v1",
				Resource: "deployments",
			},
			wantErr: false,
		},
		{
			name: "missing group with random version",
			resource: KubernetesResource{
				Group:    "",
				Version:  generateRandomVersion(),
				Resource: "deployments",
			},
			wantErr: true,
		},

		{
			name: "missing version",
			resource: KubernetesResource{
				Group:    "app",
				Resource: "deployments",
			},
			wantErr: true,
		},
		{
			name: "missing resource",
			resource: KubernetesResource{
				Group:   "apps",
				Version: "v1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.resource.Validate(field.NewPath("resource"))
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("KubernetesResource.Validate() error = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestExternalAPICallValidate(t *testing.T) {
	tests := []struct {
		name    string
		apiCall ExternalAPICall
		wantErr bool
	}{
		{
			name: "valid ExternalAPICall",
			apiCall: ExternalAPICall{
				APICall: kyverno.APICall{
					URLPath: "/api/v1/namespaces",
				},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},
			wantErr: false,
		},
		{
			name: "missing RefreshInterval",
			apiCall: ExternalAPICall{
				APICall: kyverno.APICall{
					URLPath: "/api/v1/namespaces",
				},
				RefreshInterval: &metav1.Duration{Duration: 0 * time.Second},
			},
			wantErr: true,
		},
		{
			name: "both Service and URLPath",
			apiCall: ExternalAPICall{
				APICall: kyverno.APICall{
					Service: &kyverno.ServiceCall{},
					URLPath: "/api/v1/namespaces",
				},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},
			wantErr: true,
		},
		{
			name: "missing Service and URLPath",
			apiCall: ExternalAPICall{
				APICall:         kyverno.APICall{},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},
			wantErr: true,
		},
		{
			name: "POST method without data",
			apiCall: ExternalAPICall{
				APICall: kyverno.APICall{
					Method: "POST",
				},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},
			wantErr: true,
		},

		{
			name: "non-POST method with data",
			apiCall: ExternalAPICall{
				APICall: kyverno.APICall{
					Method: "GET",
					Data: []kyverno.RequestData{
						{Key: "example-key", Value: &apiextv1.JSON{Raw: []byte(`{"field": "value"}`)}},
					},
				},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},

			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.apiCall.Validate(field.NewPath("apiCall"))
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("ExternalAPICall.Validate() error = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestGlobalContextEntryProjectionValidate(t *testing.T) {
	tests := []struct {
		name       string
		projection GlobalContextEntryProjection
		gctxName   string
		wantErr    bool
	}{
		{
			name: "valid projection",
			projection: GlobalContextEntryProjection{
				Name:     "example",
				JMESPath: "metadata.name",
			},
			gctxName: "globalContext",
			wantErr:  false,
		},
		{
			name: "missing name",
			projection: GlobalContextEntryProjection{
				JMESPath: "metadata.name",
			},
			gctxName: "globalContext",
			wantErr:  true,
		},
		{
			name: "name same as global context entry name",
			projection: GlobalContextEntryProjection{
				Name:     "globalContext",
				JMESPath: "metadata.name",
			},
			gctxName: "globalContext",
			wantErr:  true,
		},
		{
			name: "neither JMESPath nor Expression",
			projection: GlobalContextEntryProjection{
				Name: "example",
			},
			gctxName: "globalContext",
			wantErr:  true,
		},
		{
			name: "invalid JMESPath",
			projection: GlobalContextEntryProjection{
				Name:     "example",
				JMESPath: "invalid[",
			},
			gctxName: "globalContext",
			wantErr:  true,
		},
		{
			name: "valid Expression only",
			projection: GlobalContextEntryProjection{
				Name:       "example",
				Expression: "object.size()",
			},
			gctxName: "globalContext",
			wantErr:  false,
		},
		{
			name: "both JMESPath and Expression",
			projection: GlobalContextEntryProjection{
				Name:       "example",
				JMESPath:   "metadata.name",
				Expression: "object.size()",
			},
			gctxName: "globalContext",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.projection.Validate(field.NewPath("projection"), tt.gctxName)
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("GlobalContextEntryProjection.Validate() error = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

// TestGlobalContextEntryProjectionValidate_ErrorShape pins which field.Error each branch of
// the jmesPath/expression exactly-one-of check reports, since wantErr alone can't
// distinguish "both set" from "neither set" or tell an expression-only error from a
// JMESPath-compile error.
func TestGlobalContextEntryProjectionValidate_ErrorShape(t *testing.T) {
	tests := []struct {
		name         string
		projection   GlobalContextEntryProjection
		wantErrType  field.ErrorType
		wantErrField string
	}{
		{
			name: "neither set reports Required at the projection path",
			projection: GlobalContextEntryProjection{
				Name: "example",
			},
			wantErrType:  field.ErrorTypeRequired,
			wantErrField: "projection",
		},
		{
			name: "both set reports Forbidden at the expression field",
			projection: GlobalContextEntryProjection{
				Name:       "example",
				JMESPath:   "metadata.name",
				Expression: "object.size()",
			},
			wantErrType:  field.ErrorTypeForbidden,
			wantErrField: "projection.expression",
		},
		{
			name: "invalid JMESPath reports Invalid at the jmesPath field",
			projection: GlobalContextEntryProjection{
				Name:     "example",
				JMESPath: "invalid[",
			},
			wantErrType:  field.ErrorTypeInvalid,
			wantErrField: "projection.jmesPath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.projection.Validate(field.NewPath("projection"), "globalContext")
			var found *field.Error
			for _, e := range errs {
				if e.Type == tt.wantErrType && e.Field == tt.wantErrField {
					found = e
					break
				}
			}
			if found == nil {
				t.Fatalf("expected an error of type %v at field %q, got %v", tt.wantErrType, tt.wantErrField, errs)
			}
		})
	}
}

// TestGlobalContextEntryProjectionValidate_EmptyJMESPathDoesNotCompile ensures an
// explicitly empty JMESPath alongside a set Expression is treated as expression-only: the
// JMESPath compiler is never invoked, so an empty string can't surface as an "invalid
// JMESPath" error.
//
// This is defence in depth, not the user-facing contract. The CRD schema is stricter:
// minLength rejects `jmesPath: ""`, and the XValidation rule counts an explicitly present
// empty string, so the API server refuses this input before any webhook runs. Go cannot
// tell an absent string from an empty one, so Validate stays lenient rather than reporting
// a bogus "invalid JMESPath" for a field the user never set.
func TestGlobalContextEntryProjectionValidate_EmptyJMESPathDoesNotCompile(t *testing.T) {
	p := GlobalContextEntryProjection{
		Name:       "example",
		JMESPath:   "",
		Expression: "object.size()",
	}
	errs := p.Validate(field.NewPath("projection"), "globalContext")
	if len(errs) != 0 {
		t.Errorf("expected no errors for expression-only projection with empty JMESPath, got %v", errs)
	}
}

func TestExternalAPICallInlineEmbedding(t *testing.T) {
	in := GlobalContextEntry{
		Spec: GlobalContextEntrySpec{
			APICall: &ExternalAPICall{
				APICall: kyverno.APICall{
					URLPath: "/api/v1/namespaces",
					Method:  "GET",
				},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	apiCall, ok := m["spec"].(map[string]any)["apiCall"].(map[string]any)
	if !ok {
		t.Fatalf("expected spec.apiCall to be an object: %s", b)
	}
	for _, key := range []string{"urlPath", "method", "refreshInterval"} {
		if _, ok := apiCall[key]; !ok {
			t.Fatalf("expected inline key %q at apiCall level: %s", key, b)
		}
	}

	copyOut := in.DeepCopy()
	if !reflect.DeepEqual(in, *copyOut) {
		t.Fatalf("deepcopy not equal: in=%+v copy=%+v", in, *copyOut)
	}
}

func generateRandomVersion() string {
	rand.NewSource(time.Now().UnixNano())
	for {
		version := "v" + strconv.Itoa(rand.Intn(9)+2) // Generates a number between 2 and 10 (inclusive)
		if version != "v1" {
			return version
		}
	}
}

// TestGlobalContextEntryDeepCopyIsolatesSharedTypes proves GlobalContextEntry.DeepCopy still
// deep-copies the api/kyverno types it embeds. reflect.DeepEqual alone would pass on a shallow
// slice copy, so mutate the copy and assert the original is untouched.
func TestGlobalContextEntryDeepCopyIsolatesSharedTypes(t *testing.T) {
	in := GlobalContextEntry{
		Spec: GlobalContextEntrySpec{
			APICall: &ExternalAPICall{
				APICall: kyverno.APICall{
					URLPath: "/api/v1/namespaces",
					Method:  "POST",
					Data: []kyverno.RequestData{
						{Key: "original-key", Value: &apiextv1.JSON{Raw: []byte(`"original-value"`)}},
					},
					Service: &kyverno.ServiceCall{
						URL:      "https://svc.ns:443/path",
						CABundle: "original-bundle",
						Headers: []kyverno.HTTPHeader{
							{Key: "original-header", Value: "original-header-value"},
						},
					},
				},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},
		},
	}

	out := in.DeepCopy()
	if !reflect.DeepEqual(in, *out) {
		t.Fatalf("deepcopy not equal: in=%+v copy=%+v", in, *out)
	}

	copied := out.Spec.APICall
	copied.Data[0].Key = "mutated"
	copied.Data[0].Value.Raw[1] = 'X' // in place: a replacement would miss a shared backing array
	copied.Service.Headers[0].Value = "mutated"
	copied.Service.CABundle = "mutated"
	copied.RefreshInterval.Duration = time.Hour

	original := in.Spec.APICall
	if original.Data[0].Key != "original-key" {
		t.Errorf("Data slice is shared: key became %q", original.Data[0].Key)
	}
	if string(original.Data[0].Value.Raw) != `"original-value"` {
		t.Errorf("RequestData.Value.Raw buffer is shared: raw became %s", original.Data[0].Value.Raw)
	}
	if original.Service.Headers[0].Value != "original-header-value" {
		t.Errorf("Service.Headers slice is shared: value became %q", original.Service.Headers[0].Value)
	}
	if original.Service.CABundle != "original-bundle" {
		t.Errorf("Service pointer is shared: CABundle became %q", original.Service.CABundle)
	}
	if original.RefreshInterval.Duration != 10*time.Minute {
		t.Errorf("RefreshInterval pointer is shared: duration became %v", original.RefreshInterval.Duration)
	}
}

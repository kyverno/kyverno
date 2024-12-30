/*
Copyright 2022 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2alpha1

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
					APICall: kyvernov1.APICall{
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
					APICall: kyvernov1.APICall{
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
				APICall: kyvernov1.APICall{
					URLPath: "/api/v1/namespaces",
				},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},
			wantErr: false,
		},
		{
			name: "missing RefreshInterval",
			apiCall: ExternalAPICall{
				APICall: kyvernov1.APICall{
					URLPath: "/api/v1/namespaces",
				},
				RefreshInterval: &metav1.Duration{Duration: 0 * time.Second},
			},
			wantErr: true,
		},
		{
			name: "both Service and URLPath",
			apiCall: ExternalAPICall{
				APICall: kyvernov1.APICall{
					Service: &kyvernov1.ServiceCall{},
					URLPath: "/api/v1/namespaces",
				},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},
			wantErr: true,
		},
		{
			name: "missing Service and URLPath",
			apiCall: ExternalAPICall{
				APICall:         kyvernov1.APICall{},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},
			wantErr: true,
		},
		{
			name: "POST method without data",
			apiCall: ExternalAPICall{
				APICall: kyvernov1.APICall{
					Method: "POST",
				},
				RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			},
			wantErr: true,
		},

		{
			name: "non-POST method with data",
			apiCall: ExternalAPICall{
				APICall: kyvernov1.APICall{
					Method: "GET",
					Data: []kyvernov1.RequestData{
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
			name: "missing JMESPath",
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

func generateRandomVersion() string {
	rand.NewSource(time.Now().UnixNano())
	for {
		version := "v" + strconv.Itoa(rand.Intn(9)+2) // Generates a number between 2 and 10 (inclusive)
		if version != "v1" {
			return version
		}
	}
}

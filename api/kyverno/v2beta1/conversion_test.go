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

package v2beta1

import (
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
)

func TestConvert_v2alpha1_GlobalContextEntry_To_v2beta1_GlobalContextEntry(t *testing.T) {
	tests := []struct {
		name string
		in   *kyvernov2alpha1.GlobalContextEntry
		want *GlobalContextEntry
	}{
		{
			name: "basic conversion",
			in: &kyvernov2alpha1.GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-entry",
					Namespace: "test-ns",
				},
				Spec: kyvernov2alpha1.GlobalContextEntrySpec{
					KubernetesResource: &kyvernov2alpha1.KubernetesResource{
						Group:     "apps",
						Version:   "v1",
						Resource:  "deployments",
						Namespace: "default",
					},
				},
			},
			want: &GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-entry",
					Namespace: "test-ns",
				},
				Spec: GlobalContextEntrySpec{
					KubernetesResource: &KubernetesResource{
						Group:     "apps",
						Version:   "v1",
						Resource:  "deployments",
						Namespace: "default",
					},
				},
			},
		},
		{
			name: "with APICall",
			in: &kyvernov2alpha1.GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-apicall",
				},
				Spec: kyvernov2alpha1.GlobalContextEntrySpec{
					APICall: &kyvernov2alpha1.ExternalAPICall{
						URLPath:         "/api/v1/pods",
						Method:          "GET",
						RefreshInterval: metav1.Duration{Duration: 5 * time.Minute},
						RetryLimit:      3,
						Service: &kyvernov1.ServiceCall{
							URL: "https://example.com",
						},
						Data: []kyvernov2alpha1.RequestData{
							{
								Key:   "test-key",
								Value: apiextensionsv1.JSON{Raw: []byte(`"test-value"`)},
							},
						},
					},
				},
			},
			want: &GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-apicall",
				},
				Spec: GlobalContextEntrySpec{
					APICall: &ExternalAPICall{
						APICall: kyvernov1.APICall{
							URLPath: "/api/v1/pods",
							Method:  kyvernov1.Method("GET"),
							Service: &kyvernov1.ServiceCall{
								URL: "https://example.com",
							},
							Data: []kyvernov1.RequestData{
								{
									Key:   "test-key",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"test-value"`)},
								},
							},
						},
						RefreshInterval: &metav1.Duration{Duration: 5 * time.Minute},
						RetryLimit:      3,
					},
				},
			},
		},
		{
			name: "with projections",
			in: &kyvernov2alpha1.GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-projections",
				},
				Spec: kyvernov2alpha1.GlobalContextEntrySpec{
					KubernetesResource: &kyvernov2alpha1.KubernetesResource{
						Group:    "apps",
						Version:  "v1",
						Resource: "deployments",
					},
					Projections: []kyvernov2alpha1.GlobalContextEntryProjection{
						{
							Name:     "deployment-count",
							JMESPath: "length(@)",
						},
					},
				},
			},
			want: &GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-projections",
				},
				Spec: GlobalContextEntrySpec{
					KubernetesResource: &KubernetesResource{
						Group:    "apps",
						Version:  "v1",
						Resource: "deployments",
					},
					Projections: []GlobalContextEntryProjection{
						{
							Name:     "deployment-count",
							JMESPath: "length(@)",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &GlobalContextEntry{}
			err := Convert_v2alpha1_GlobalContextEntry_To_v2beta1_GlobalContextEntry(tt.in, out, conversion.Scope(nil))
			assert.NoError(t, err)
			assert.Equal(t, tt.want.ObjectMeta, out.ObjectMeta)
			assert.Equal(t, tt.want.Spec, out.Spec)
		})
	}
}

func TestConvert_v2beta1_GlobalContextEntry_To_v2alpha1_GlobalContextEntry(t *testing.T) {
	tests := []struct {
		name string
		in   *GlobalContextEntry
		want *kyvernov2alpha1.GlobalContextEntry
	}{
		{
			name: "basic conversion",
			in: &GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-entry",
					Namespace: "test-ns",
				},
				Spec: GlobalContextEntrySpec{
					KubernetesResource: &KubernetesResource{
						Group:     "apps",
						Version:   "v1",
						Resource:  "deployments",
						Namespace: "default",
					},
				},
			},
			want: &kyvernov2alpha1.GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-entry",
					Namespace: "test-ns",
				},
				Spec: kyvernov2alpha1.GlobalContextEntrySpec{
					KubernetesResource: &kyvernov2alpha1.KubernetesResource{
						Group:     "apps",
						Version:   "v1",
						Resource:  "deployments",
						Namespace: "default",
					},
				},
			},
		},
		{
			name: "with APICall",
			in: &GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-apicall",
				},
				Spec: GlobalContextEntrySpec{
					APICall: &ExternalAPICall{
						APICall: kyvernov1.APICall{
							URLPath: "/api/v1/pods",
							Method:  kyvernov1.Method("GET"),
							Service: &kyvernov1.ServiceCall{
								URL: "https://example.com",
							},
							Data: []kyvernov1.RequestData{
								{
									Key:   "test-key",
									Value: &apiextensionsv1.JSON{Raw: []byte(`"test-value"`)},
								},
							},
						},
						RefreshInterval: &metav1.Duration{Duration: 5 * time.Minute},
						RetryLimit:      3,
					},
				},
			},
			want: &kyvernov2alpha1.GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-apicall",
				},
				Spec: kyvernov2alpha1.GlobalContextEntrySpec{
					APICall: &kyvernov2alpha1.ExternalAPICall{
						URLPath:         "/api/v1/pods",
						Method:          "GET",
						RefreshInterval: metav1.Duration{Duration: 5 * time.Minute},
						RetryLimit:      3,
						Service: &kyvernov1.ServiceCall{
							URL: "https://example.com",
						},
						Data: []kyvernov2alpha1.RequestData{
							{
								Key:   "test-key",
								Value: apiextensionsv1.JSON{Raw: []byte(`"test-value"`)},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &kyvernov2alpha1.GlobalContextEntry{}
			err := Convert_v2beta1_GlobalContextEntry_To_v2alpha1_GlobalContextEntry(tt.in, out, conversion.Scope(nil))
			assert.NoError(t, err)
			assert.Equal(t, tt.want.ObjectMeta, out.ObjectMeta)
			assert.Equal(t, tt.want.Spec, out.Spec)
		})
	}
}

func TestConvert_v2alpha1_GlobalContextEntryStatus_To_v2beta1_GlobalContextEntryStatus(t *testing.T) {
	tests := []struct {
		name string
		in   *kyvernov2alpha1.GlobalContextEntryStatus
		want *GlobalContextEntryStatus
	}{
		{
			name: "basic status conversion",
			in: &kyvernov2alpha1.GlobalContextEntryStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: metav1.ConditionTrue,
						Reason: "Success",
					},
				},
				LastRefreshTime: metav1.Time{Time: time.Now()},
			},
			want: &GlobalContextEntryStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: metav1.ConditionTrue,
						Reason: "Success",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &GlobalContextEntryStatus{}
			err := Convert_v2alpha1_GlobalContextEntryStatus_To_v2beta1_GlobalContextEntryStatus(tt.in, out, conversion.Scope(nil))
			assert.NoError(t, err)
			assert.Equal(t, len(tt.want.Conditions), len(out.Conditions))
			if len(tt.want.Conditions) > 0 {
				assert.Equal(t, tt.want.Conditions[0].Type, out.Conditions[0].Type)
				assert.Equal(t, tt.want.Conditions[0].Status, out.Conditions[0].Status)
				assert.Equal(t, tt.want.Conditions[0].Reason, out.Conditions[0].Reason)
			}
			assert.Equal(t, tt.in.LastRefreshTime, out.LastRefreshTime)
		})
	}
}

func TestConvert_v2beta1_GlobalContextEntryStatus_To_v2alpha1_GlobalContextEntryStatus(t *testing.T) {
	tests := []struct {
		name string
		in   *GlobalContextEntryStatus
		want *kyvernov2alpha1.GlobalContextEntryStatus
	}{
		{
			name: "basic status conversion",
			in: &GlobalContextEntryStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: metav1.ConditionTrue,
						Reason: "Success",
					},
				},
				LastRefreshTime: metav1.Time{Time: time.Now()},
			},
			want: &kyvernov2alpha1.GlobalContextEntryStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "Ready",
						Status: metav1.ConditionTrue,
						Reason: "Success",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &kyvernov2alpha1.GlobalContextEntryStatus{}
			err := Convert_v2beta1_GlobalContextEntryStatus_To_v2alpha1_GlobalContextEntryStatus(tt.in, out, conversion.Scope(nil))
			assert.NoError(t, err)
			assert.Equal(t, len(tt.want.Conditions), len(out.Conditions))
			if len(tt.want.Conditions) > 0 {
				assert.Equal(t, tt.want.Conditions[0].Type, out.Conditions[0].Type)
				assert.Equal(t, tt.want.Conditions[0].Status, out.Conditions[0].Status)
				assert.Equal(t, tt.want.Conditions[0].Reason, out.Conditions[0].Reason)
			}
			assert.Equal(t, tt.in.LastRefreshTime, out.LastRefreshTime)
		})
	}
}

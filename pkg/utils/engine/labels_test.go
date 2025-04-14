package engine

import (
	"errors"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// mockNamespaceLister is a stub for the NamespaceLister interface.
type mockNamespaceLister struct {
	namespaces map[string]*corev1.Namespace
	err        error
}

// Get returns a namespace by name. It returns an error if m.err is set,
// or nil if the namespace does not exist.
func (m *mockNamespaceLister) Get(name string) (*corev1.Namespace, error) {
	if m.err != nil {
		return nil, m.err
	}
	if ns, exists := m.namespaces[name]; exists {
		return ns, nil
	}
	return nil, nil
}

// List is not used by the tests.
func (m *mockNamespaceLister) List(selector labels.Selector) ([]*corev1.Namespace, error) {
	var result []*corev1.Namespace
	for _, ns := range m.namespaces {
		if selector.Matches(labels.Set(ns.Labels)) {
			result = append(result, ns)
		}
	}
	return result, nil
}

func TestHasNamespaceSelector(t *testing.T) {
	tests := []struct {
		name     string
		policies []kyvernov1.PolicyInterface
		want     bool
	}{
		{
			name:     "no policies",
			policies: []kyvernov1.PolicyInterface{},
			want:     false,
		},
		{
			name: "policy with namespace selector in match",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										NamespaceSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{"env": "prod"},
										},
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "policy with namespace selector in exclude",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								ExcludeResources: &kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										NamespaceSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{"env": "dev"},
										},
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "policy with namespace selector in All slice",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									All: []kyvernov1.ResourceFilter{
										{
											ResourceDescription: kyvernov1.ResourceDescription{
												NamespaceSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{"foo": "bar"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "policy with namespace selector in Any slice",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									Any: []kyvernov1.ResourceFilter{
										{
											ResourceDescription: kyvernov1.ResourceDescription{
												NamespaceSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{"key": "value"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "policy without namespace selector anywhere",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds: []string{"Pod"},
									},
								},
							},
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasNamespaceSelector(tt.policies)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetNamespaceSelectorsFromNamespaceLister(t *testing.T) {
	tests := []struct {
		name                string
		kind                string
		namespaceOfResource string
		nsLister            corev1listers.NamespaceLister
		policies            []kyvernov1.PolicyInterface
		wantLabels          map[string]string
		wantErr             bool
	}{
		{
			name:                "no namespace selector in policies",
			kind:                "Pod",
			namespaceOfResource: "default",
			nsLister: &mockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{
					"default": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"env": "prod"},
						},
					},
				},
			},
			policies: []kyvernov1.PolicyInterface{
				// Policy without any namespace selector.
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds: []string{"Pod"},
									},
								},
							},
						},
					},
				},
			},
			wantLabels: map[string]string{},
			wantErr:    false,
		},
		{
			name:                "namespace resource; kind is Namespace, so no lookup",
			kind:                "Namespace",
			namespaceOfResource: "default",
			nsLister: &mockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{
					"default": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"env": "prod"},
						},
					},
				},
			},
			policies: []kyvernov1.PolicyInterface{
				// Even though policy defines a namespace selector, resource kind is Namespace.
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										NamespaceSelector: &metav1.LabelSelector{},
									},
								},
							},
						},
					},
				},
			},
			// Should return empty map because lookup is bypassed.
			wantLabels: map[string]string{},
			wantErr:    false,
		},
		{
			name:                "lookup error returned from lister",
			kind:                "Pod",
			namespaceOfResource: "default",
			nsLister: &mockNamespaceLister{
				namespaces: nil,
				err:        errors.New("lookup failure"),
			},
			policies: []kyvernov1.PolicyInterface{
				// Policy has a namespace selector.
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										NamespaceSelector: &metav1.LabelSelector{},
									},
								},
							},
						},
					},
				},
			},
			wantLabels: map[string]string{},
			wantErr:    true,
		},
		{
			name:                "namespace not found (nil namespace returned)",
			kind:                "Pod",
			namespaceOfResource: "nonexistent",
			nsLister: &mockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{},
			},
			policies: []kyvernov1.PolicyInterface{
				// Policy with namespace selector.
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										NamespaceSelector: &metav1.LabelSelector{},
									},
								},
							},
						},
					},
				},
			},
			wantLabels: map[string]string{},
			wantErr:    true,
		},
		{
			name:                "successful namespace labels retrieval",
			kind:                "Pod",
			namespaceOfResource: "default",
			nsLister: &mockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{
					"default": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"env": "prod", "tier": "frontend"},
						},
					},
				},
			},
			policies: []kyvernov1.PolicyInterface{
				// Policy with namespace selector in MatchResources.
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										NamespaceSelector: &metav1.LabelSelector{},
									},
								},
							},
						},
					},
				},
			},
			wantLabels: map[string]string{"env": "prod", "tier": "frontend"},
			wantErr:    false,
		},
		{
			name:                "successful lookup with namespace selector in All slice",
			kind:                "Pod",
			namespaceOfResource: "default",
			nsLister: &mockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{
					"default": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "myapp", "team": "devops"},
						},
					},
				},
			},
			policies: []kyvernov1.PolicyInterface{
				// Policy with namespace selector in the "All" slice.
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									All: []kyvernov1.ResourceFilter{
										{
											ResourceDescription: kyvernov1.ResourceDescription{
												NamespaceSelector: &metav1.LabelSelector{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantLabels: map[string]string{"app": "myapp", "team": "devops"},
			wantErr:    false,
		},
		{
			name:                "successful lookup with namespace selector in Any slice",
			kind:                "Pod",
			namespaceOfResource: "default",
			nsLister: &mockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{
					"default": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"role": "backend", "zone": "us-east"},
						},
					},
				},
			},
			policies: []kyvernov1.PolicyInterface{
				// Policy with namespace selector in the "Any" slice.
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								MatchResources: kyvernov1.MatchResources{
									Any: []kyvernov1.ResourceFilter{
										{
											ResourceDescription: kyvernov1.ResourceDescription{
												NamespaceSelector: &metav1.LabelSelector{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantLabels: map[string]string{"role": "backend", "zone": "us-east"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetNamespaceSelectorsFromNamespaceLister(
				tt.kind,
				tt.namespaceOfResource,
				tt.nsLister,
				tt.policies,
				logging.GlobalLogger(),
			)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantLabels, got)
		})
	}
}

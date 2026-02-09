package engine

import (
	"errors"
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// mockNamespaceLister is a mock implementation of the corev1listers.NamespaceLister interface.
type mockNamespaceLister struct {
	namespaces map[string]*corev1.Namespace
	err        error
}

func (m *mockNamespaceLister) Get(name string) (*corev1.Namespace, error) {
	if m.err != nil {
		return nil, m.err
	}
	if ns, ok := m.namespaces[name]; ok {
		return ns, nil
	}
	return nil, nil
}

func (m *mockNamespaceLister) List(selector labels.Selector) ([]*corev1.Namespace, error) {
	var nsList []*corev1.Namespace
	for _, ns := range m.namespaces {
		if selector.Matches(labels.Set(ns.Labels)) {
			nsList = append(nsList, ns)
		}
	}
	return nsList, nil
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
			name: "namespace selector in MatchResources",
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
			name: "namespace selector in ExcludeResources.ResourceDescription",
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
			name: "namespace selector in MatchResources.All slice",
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
			name: "namespace selector in MatchResources.Any slice",
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
			name: "namespace selector in ExcludeResources.All slice",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								ExcludeResources: &kyvernov1.MatchResources{
									All: []kyvernov1.ResourceFilter{
										{
											ResourceDescription: kyvernov1.ResourceDescription{
												NamespaceSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{"exclude": "all"},
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
			name: "namespace selector in ExcludeResources.Any slice",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								ExcludeResources: &kyvernov1.MatchResources{
									Any: []kyvernov1.ResourceFilter{
										{
											ResourceDescription: kyvernov1.ResourceDescription{
												NamespaceSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{"exclude": "any"},
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
			name: "policy with no namespace selector anywhere",
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
								ExcludeResources: &kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds: []string{"Deployment"},
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
			name:                "no namespace selector in policies returns empty map",
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
				// Policy with no namespace selector.
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
			name:                "resource kind Namespace bypass lookup",
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
				// Policy defines a namespace selector, but kind is "Namespace" so lookup is skipped.
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
			wantErr:    false,
		},
		{
			name:                "empty namespace returns empty map",
			kind:                "Pod",
			namespaceOfResource: "",
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
				// Policy has a namespace selector but namespaceOfResource is empty.
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
			wantErr:    false,
		},
		{
			name:                "lister returns error",
			kind:                "Pod",
			namespaceOfResource: "default",
			nsLister: &mockNamespaceLister{
				err: errors.New("lookup failure"),
			},
			policies: []kyvernov1.PolicyInterface{
				// Policy requires namespace selector.
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
			name:                "namespace not found (nil object returned)",
			kind:                "Pod",
			namespaceOfResource: "nonexistent",
			nsLister: &mockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{}, // lookup returns nil
			},
			policies: []kyvernov1.PolicyInterface{
				// Policy requires namespace selector.
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
			name:                "successful lookup from MatchResources",
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
			name:                "successful lookup from MatchResources.All slice",
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
				// Policy with namespace selector in MatchResources.All.
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
			name:                "successful lookup from MatchResources.Any slice",
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
				// Policy with namespace selector in MatchResources.Any.
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
		{
			name:                "successful lookup from ExcludeResources.All slice",
			kind:                "Pod",
			namespaceOfResource: "default",
			nsLister: &mockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{
					"default": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"exclude": "all", "region": "east"},
						},
					},
				},
			},
			policies: []kyvernov1.PolicyInterface{
				// Policy with namespace selector in ExcludeResources.All.
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								ExcludeResources: &kyvernov1.MatchResources{
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
			wantLabels: map[string]string{"exclude": "all", "region": "east"},
			wantErr:    false,
		},
		{
			name:                "successful lookup from ExcludeResources.Any slice",
			kind:                "Pod",
			namespaceOfResource: "default",
			nsLister: &mockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{
					"default": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"exclude": "any", "cluster": "main"},
						},
					},
				},
			},
			policies: []kyvernov1.PolicyInterface{
				// Policy with namespace selector in ExcludeResources.Any.
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								ExcludeResources: &kyvernov1.MatchResources{
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
			wantLabels: map[string]string{"exclude": "any", "cluster": "main"},
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
				assert.Error(t, err, "expected error but got nil")
			} else {
				assert.NoError(t, err, fmt.Sprintf("unexpected error: %v", err))
			}
			assert.Equal(t, tt.wantLabels, got)
		})
	}
}

package engine

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type mockNamespaceLister struct {
	namespaces map[string]*corev1.Namespace
}

func (m *mockNamespaceLister) Get(name string) (*corev1.Namespace, error) {
	if ns, exists := m.namespaces[name]; exists {
		return ns, nil
	}
	return nil, nil
}

func (m *mockNamespaceLister) List(selector labels.Selector) ([]*corev1.Namespace, error) {
	return nil, nil
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
			name: "policy without namespace selector",
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
			got := HasNamespaceSelector(tt.policies)
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
			name:                "namespace resource",
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
			name:                "namespace not found",
			kind:                "Pod",
			namespaceOfResource: "nonexistent",
			nsLister: &mockNamespaceLister{
				namespaces: map[string]*corev1.Namespace{},
			},
			policies: []kyvernov1.PolicyInterface{
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

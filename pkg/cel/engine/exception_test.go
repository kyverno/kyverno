package engine

import (
	"errors"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/labels"
)

type fakePolicyExceptionLister struct {
	err        error
	exceptions []*policiesv1alpha1.PolicyException
}

func (l *fakePolicyExceptionLister) List(selector labels.Selector) ([]*policiesv1alpha1.PolicyException, error) {
	return l.exceptions, l.err
}

func TestListExceptions(t *testing.T) {
	tests := []struct {
		name       string
		lister     policyExceptionLister
		policyKind string
		policyName string
		want       []*policiesv1alpha1.PolicyException
		wantErr    bool
	}{{
		name: "with error",
		lister: &fakePolicyExceptionLister{
			err: errors.New("dummy"),
		},
		wantErr: true,
	}, {
		name: "name doesn't match",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1alpha1.PolicyException{{
				Spec: policiesv1alpha1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1alpha1.PolicyRef{{
						Kind: "foo",
						Name: "bar",
					}},
				},
			}},
		},
		policyKind: "foo",
		policyName: "other",
	}, {
		name: "kind doesn't match",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1alpha1.PolicyException{{
				Spec: policiesv1alpha1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1alpha1.PolicyRef{{
						Kind: "foo",
						Name: "bar",
					}},
				},
			}},
		},
		policyKind: "other",
		policyName: "bar",
	}, {
		name: "match",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1alpha1.PolicyException{{
				Spec: policiesv1alpha1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1alpha1.PolicyRef{{
						Kind: "foo",
						Name: "bar",
					}},
				},
			}},
		},
		policyKind: "foo",
		policyName: "bar",
		want: []*policiesv1alpha1.PolicyException{{
			Spec: policiesv1alpha1.PolicyExceptionSpec{
				PolicyRefs: []policiesv1alpha1.PolicyRef{{
					Kind: "foo",
					Name: "bar",
				}},
			},
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ListExceptions(tt.lister, tt.policyKind, tt.policyName)
			assert.Equal(t, tt.want, got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

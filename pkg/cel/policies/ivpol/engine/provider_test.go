package engine

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewProviderExceptionExpiry(t *testing.T) {
	expired := metav1.NewTime(time.Now().Add(-1 * time.Hour))

	polex := func(name string, expiresAt *metav1.Time) *policiesv1beta1.PolicyException {
		return &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				ExpiresAt: expiresAt,
				PolicyRefs: []policiesv1beta1.PolicyRef{{
					Name: "ivpol",
					Kind: "ImageValidatingPolicy",
				}},
			},
		}
	}

	tests := []struct {
		name       string
		exceptions []*policiesv1beta1.PolicyException
		want       []string
	}{{
		name:       "live exception is attached",
		exceptions: []*policiesv1beta1.PolicyException{polex("live", nil)},
		want:       []string{"live"},
	}, {
		name:       "expired exception is not attached",
		exceptions: []*policiesv1beta1.PolicyException{polex("expired", &expired)},
		want:       nil,
	}, {
		name: "only the live exception is attached",
		exceptions: []*policiesv1beta1.PolicyException{
			polex("expired", &expired),
			polex("live", nil),
		},
		want: []string{"live"},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "ivpol"},
				TypeMeta:   metav1.TypeMeta{Kind: "ImageValidatingPolicy"},
			}

			provider, err := NewProvider([]policiesv1beta1.ImageValidatingPolicyLike{policy}, tt.exceptions)
			require.NoError(t, err)

			policies, err := provider.Fetch(context.Background())
			require.NoError(t, err)
			require.Len(t, policies, 1)

			names := make([]string, 0, len(policies[0].Exceptions))
			for _, e := range policies[0].Exceptions {
				names = append(names, e.GetName())
			}
			if tt.want == nil {
				assert.Empty(t, names)
				return
			}
			assert.Equal(t, tt.want, names)
		})
	}
}

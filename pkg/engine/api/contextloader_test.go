package api

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPolicyNamespace(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		policy    PolicyScope
		namespace string
		wantError string
	}{
		{name: "nil", wantError: "policy scope must not be nil"},
		{name: "typed nil", policy: (*kyvernov1.Policy)(nil), wantError: "policy scope must not be nil"},
		{name: "missing namespace", policy: &kyvernov1.Policy{}, wantError: "policy namespace must not be empty"},
		{name: "invalid namespace", policy: &kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: "tenant/a"}}, wantError: "is invalid"},
		{name: "namespaced", policy: &kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: "tenant-a"}}, namespace: "tenant-a"},
		{name: "cluster", policy: &kyvernov1.ClusterPolicy{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			namespace, err := PolicyNamespace(test.policy)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.namespace, namespace)
			}
		})
	}
}

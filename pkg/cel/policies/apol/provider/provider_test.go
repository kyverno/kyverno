package provider

import (
	"errors"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestSetStatus(t *testing.T) {
	t.Parallel()
	var pol policiesv1alpha1.AuthorizingPolicy
	SetStatus(&pol, nil)
	require.True(t, *pol.Status.ConditionStatus.Ready)
	require.Empty(t, pol.Status.ConditionStatus.Message)
	SetStatus(&pol, errors.New("bad expression"))
	require.False(t, *pol.Status.ConditionStatus.Ready)
	require.Equal(t, "bad expression", pol.Status.ConditionStatus.Message)
}

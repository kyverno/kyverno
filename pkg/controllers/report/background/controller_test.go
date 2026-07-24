package background

import (
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func ptrDuration(d time.Duration) *time.Duration {
	return &d
}

func Test_scanInterval(t *testing.T) {
	globalInterval := time.Hour

	clusterPolicy := func(name string, kinds []string, interval *time.Duration) engineapi.GenericPolicy {
		cpol := &kyvernov1.ClusterPolicy{}
		cpol.Name = name
		cpol.UID = types.UID(name)
		cpol.ResourceVersion = "1"
		cpol.Spec.Rules = []kyvernov1.Rule{
			{
				Name: "rule",
				MatchResources: kyvernov1.MatchResources{
					ResourceDescription: kyvernov1.ResourceDescription{Kinds: kinds},
				},
			},
		}
		if interval != nil {
			cpol.Spec.BackgroundScanInterval = &metav1.Duration{Duration: *interval}
		}
		return engineapi.NewKyvernoPolicy(cpol)
	}

	nonKyvernoPolicy := func(name string) engineapi.GenericPolicy {
		vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
		vap.Name = name
		return engineapi.NewValidatingAdmissionPolicy(vap)
	}

	fiveMinutes := 5 * time.Minute
	twoHours := 2 * time.Hour

	tests := []struct {
		name     string
		kind     string
		policies []engineapi.GenericPolicy
		want     time.Duration
	}{
		{
			name:     "no policies falls back to the global interval",
			kind:     "Pod",
			policies: nil,
			want:     globalInterval,
		},
		{
			name:     "policy without an override falls back to the global interval",
			kind:     "Pod",
			policies: []engineapi.GenericPolicy{clusterPolicy("no-override", []string{"Pod"}, nil)},
			want:     globalInterval,
		},
		{
			name:     "policy with a shorter override wins",
			kind:     "Pod",
			policies: []engineapi.GenericPolicy{clusterPolicy("fast", []string{"Pod"}, &fiveMinutes)},
			want:     fiveMinutes,
		},
		{
			name:     "policy with a longer override relaxes the global interval",
			kind:     "Pod",
			policies: []engineapi.GenericPolicy{clusterPolicy("slow", []string{"Pod"}, &twoHours)},
			want:     twoHours,
		},
		{
			name: "the shortest override among several policies matching the resource wins",
			kind: "Pod",
			policies: []engineapi.GenericPolicy{
				clusterPolicy("slow", []string{"Pod"}, &twoHours),
				clusterPolicy("fast", []string{"Pod"}, &fiveMinutes),
				clusterPolicy("no-override", []string{"Pod"}, nil),
			},
			want: fiveMinutes,
		},
		{
			name: "a policy matching a different kind is ignored",
			kind: "Pod",
			policies: []engineapi.GenericPolicy{
				clusterPolicy("fast-but-for-secrets", []string{"Secret"}, &fiveMinutes),
			},
			want: globalInterval,
		},
		{
			name: "a wildcard kind policy still counts",
			kind: "Pod",
			policies: []engineapi.GenericPolicy{
				clusterPolicy("fast-for-everything", []string{"*"}, &fiveMinutes),
			},
			want: fiveMinutes,
		},
		{
			name: "a non-positive override is ignored in favor of the global interval",
			kind: "Pod",
			policies: []engineapi.GenericPolicy{
				clusterPolicy("zero", []string{"Pod"}, ptrDuration(0)),
			},
			want: globalInterval,
		},
		{
			name: "a non-kyverno background policy floors a longer override at the global interval",
			kind: "Pod",
			policies: []engineapi.GenericPolicy{
				clusterPolicy("slow", []string{"Pod"}, &twoHours),
				nonKyvernoPolicy("some-vap"),
			},
			want: globalInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &controller{forceDelay: globalInterval}
			assert.Equal(t, tt.want, c.scanInterval(tt.kind, tt.policies...))
		})
	}
}

func Test_policyMatchKinds_cachesByResourceVersion(t *testing.T) {
	c := &controller{}
	cpol := &kyvernov1.ClusterPolicy{}
	cpol.UID = "policy-1"
	cpol.ResourceVersion = "1"
	cpol.Spec.Rules = []kyvernov1.Rule{
		{
			Name: "rule",
			MatchResources: kyvernov1.MatchResources{
				ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}},
			},
		},
	}

	assert.Equal(t, []string{"Pod"}, c.policyMatchKinds(cpol))

	// mutate the policy without bumping resourceVersion: the cached value must still be returned
	cpol.Spec.Rules[0].MatchResources.ResourceDescription.Kinds = []string{"Secret"}
	assert.Equal(t, []string{"Pod"}, c.policyMatchKinds(cpol), "cached kinds should not change until resourceVersion changes")

	// bump resourceVersion: now it should recompute and pick up the mutation
	cpol.ResourceVersion = "2"
	assert.Equal(t, []string{"Secret"}, c.policyMatchKinds(cpol), "changing resourceVersion should invalidate the cache")
}

package background

import (
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_scanInterval(t *testing.T) {
	globalInterval := time.Hour

	clusterPolicy := func(name string, interval *time.Duration) engineapi.GenericPolicy {
		cpol := &kyvernov1.ClusterPolicy{}
		cpol.Name = name
		if interval != nil {
			cpol.Spec.BackgroundScanInterval = &metav1.Duration{Duration: *interval}
		}
		return engineapi.NewKyvernoPolicy(cpol)
	}

	fiveMinutes := 5 * time.Minute
	twoHours := 2 * time.Hour

	tests := []struct {
		name     string
		policies []engineapi.GenericPolicy
		want     time.Duration
	}{
		{
			name:     "no policies falls back to the global interval",
			policies: nil,
			want:     globalInterval,
		},
		{
			name:     "policy without an override falls back to the global interval",
			policies: []engineapi.GenericPolicy{clusterPolicy("no-override", nil)},
			want:     globalInterval,
		},
		{
			name:     "policy with a shorter override wins",
			policies: []engineapi.GenericPolicy{clusterPolicy("fast", &fiveMinutes)},
			want:     fiveMinutes,
		},
		{
			name:     "policy with a longer override does not relax the global interval",
			policies: []engineapi.GenericPolicy{clusterPolicy("slow", &twoHours)},
			want:     globalInterval,
		},
		{
			name: "the shortest override among several policies wins",
			policies: []engineapi.GenericPolicy{
				clusterPolicy("slow", &twoHours),
				clusterPolicy("fast", &fiveMinutes),
				clusterPolicy("no-override", nil),
			},
			want: fiveMinutes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &controller{forceDelay: globalInterval}
			assert.Equal(t, tt.want, c.scanInterval(tt.policies...))
		})
	}
}

package admissionpolicygenerator

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
)

func TestReportingLabelUpdatesEnqueueSource(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		before, after map[string]string
		want          int
	}{
		{name: "status-only"},
		{name: "unrelated", after: map[string]string{"other": "value"}},
		{name: "enable", after: map[string]string{kyverno.LabelEnableVAPReporting: "true"}, want: 1},
		{name: "remove-enable", before: map[string]string{kyverno.LabelEnableVAPReporting: "true"}, want: 1},
		{name: "disable", after: map[string]string{kyverno.LabelExcludeReporting: ""}, want: 1},
		{name: "remove-disable", before: map[string]string{kyverno.LabelExcludeReporting: ""}, want: 1},
		{name: "true-to-false", before: map[string]string{kyverno.LabelEnableVAPReporting: "true"}, after: map[string]string{kyverno.LabelEnableVAPReporting: "false"}, want: 1},
	}
	for _, kind := range []string{"ValidatingPolicy", "MutatingPolicy", "ClusterPolicy"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any]())
					defer q.ShutDown()
					c := &controller{queue: q}
					old := metav1.ObjectMeta{Name: "test", ResourceVersion: "1", Labels: tc.before}
					updated := metav1.ObjectMeta{Name: "test", ResourceVersion: "2", Labels: tc.after}
					switch kind {
					case "ValidatingPolicy":
						c.updateVP(&policiesv1beta1.ValidatingPolicy{ObjectMeta: old}, &policiesv1beta1.ValidatingPolicy{ObjectMeta: updated})
					case "MutatingPolicy":
						c.updateMP(&policiesv1beta1.MutatingPolicy{ObjectMeta: old}, &policiesv1beta1.MutatingPolicy{ObjectMeta: updated})
					case "ClusterPolicy":
						c.updatePolicy(&kyvernov1.ClusterPolicy{ObjectMeta: old}, &kyvernov1.ClusterPolicy{ObjectMeta: updated})
					}
					assert.Equal(t, tc.want, q.Len())
					if q.Len() > 0 {
						item, _ := q.Get()
						assert.Equal(t, kind+"/test", item)
						q.Done(item)
					}
				})
			}
		})
	}
}

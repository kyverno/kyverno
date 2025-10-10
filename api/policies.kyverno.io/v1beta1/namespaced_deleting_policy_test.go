package v1beta1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_NamespacedDeletingPolicy(t *testing.T) {
	created := time.Now()
	ndpol := NamespacedDeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-namespaced-deleting-policy",
			Namespace:         "test-namespace",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: DeletingPolicySpec{
			Schedule: "*/1 * * * *",
		},
	}

	t.Run("GetKind", func(t *testing.T) {
		assert.Equal(t, "NamespacedDeletingPolicy", ndpol.GetKind())
	})

	t.Run("GetDeletingPolicySpec", func(t *testing.T) {
		spec := ndpol.GetDeletingPolicySpec()
		assert.NotNil(t, spec)
		assert.Equal(t, "*/1 * * * *", spec.Schedule)
	})

	t.Run("GetDeletingPolicySpec with nil policy", func(t *testing.T) {
		var nilPolicy *NamespacedDeletingPolicy
		spec := nilPolicy.GetDeletingPolicySpec()
		assert.Nil(t, spec)
	})

	t.Run("invalid schedule", func(t *testing.T) {
		ndpol := NamespacedDeletingPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind: "NamespacedDeletingPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-policy",
				Namespace:         "test-namespace",
				CreationTimestamp: metav1.NewTime(created),
			},
			Spec: DeletingPolicySpec{
				Schedule: "invalid",
			},
		}

		_, err := ndpol.GetNextExecutionTime(time.Now())
		assert.Error(t, err)
	})

	t.Run("get next execution time", func(t *testing.T) {
		now := time.Now()
		next, err := ndpol.GetNextExecutionTime(now)

		assert.NoError(t, err)
		assert.Equal(t, now.Add(1*time.Minute).Format("2006-01-02 15:04"), (*next).Format("2006-01-02 15:04"))
	})

	t.Run("fallback execution time", func(t *testing.T) {
		exec, err := ndpol.GetExecutionTime()

		assert.NoError(t, err)
		assert.Equal(t, created.Add(1*time.Minute).Format("2006-01-02 15:04"), exec.Format("2006-01-02 15:04"))
	})

	t.Run("last execution time", func(t *testing.T) {
		ndpol := NamespacedDeletingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: "test-namespace",
			},
			Spec: DeletingPolicySpec{
				Schedule: "*/1 * * * *",
			},
			Status: DeletingPolicyStatus{
				LastExecutionTime: metav1.NewTime(created),
			},
		}

		exec, err := ndpol.GetExecutionTime()

		assert.NoError(t, err)
		assert.Equal(t, created.Add(1*time.Minute).Format("2006-01-02 15:04"), exec.Format("2006-01-02 15:04"))
	})

	t.Run("namespace scoped", func(t *testing.T) {
		assert.Equal(t, "test-namespace", ndpol.GetNamespace())
		assert.Equal(t, "test-namespaced-deleting-policy", ndpol.GetName())
	})

	t.Run("different schedule formats", func(t *testing.T) {
		testCases := []struct {
			name     string
			schedule string
			wantErr  bool
		}{
			{
				name:     "every minute",
				schedule: "*/1 * * * *",
				wantErr:  false,
			},
			{
				name:     "every hour",
				schedule: "0 * * * *",
				wantErr:  false,
			},
			{
				name:     "daily at midnight",
				schedule: "0 0 * * *",
				wantErr:  false,
			},
			{
				name:     "weekly on Sunday",
				schedule: "0 0 * * 0",
				wantErr:  false,
			},
			{
				name:     "invalid schedule",
				schedule: "invalid cron",
				wantErr:  true,
			},
			{
				name:     "empty schedule",
				schedule: "",
				wantErr:  true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ndpol := NamespacedDeletingPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-policy",
						Namespace:         "test-namespace",
						CreationTimestamp: metav1.NewTime(created),
					},
					Spec: DeletingPolicySpec{
						Schedule: tc.schedule,
					},
				}

				_, err := ndpol.GetExecutionTime()
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func Test_NamespacedDeletingPolicy_DeletingPolicyLike(t *testing.T) {
	ndpol := &NamespacedDeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "test-namespace",
		},
		Spec: DeletingPolicySpec{
			Schedule: "*/1 * * * *",
		},
	}

	var _ DeletingPolicyLike = ndpol

	t.Run("interface compliance", func(t *testing.T) {
		assert.Equal(t, "NamespacedDeletingPolicy", ndpol.GetKind())
		assert.NotNil(t, ndpol.GetDeletingPolicySpec())
		assert.Equal(t, "test-namespace", ndpol.GetNamespace())
		assert.Equal(t, "test-policy", ndpol.GetName())
	})
}

func Test_NamespacedDeletingPolicyList(t *testing.T) {
	list := NamespacedDeletingPolicyList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NamespacedDeletingPolicyList",
			APIVersion: "policies.kyverno.io/v1alpha1",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1",
		},
		Items: []NamespacedDeletingPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy-1",
					Namespace: "namespace-1",
				},
				Spec: DeletingPolicySpec{
					Schedule: "*/1 * * * *",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy-2",
					Namespace: "namespace-2",
				},
				Spec: DeletingPolicySpec{
					Schedule: "0 * * * *",
				},
			},
		},
	}

	t.Run("list properties", func(t *testing.T) {
		assert.Equal(t, "NamespacedDeletingPolicyList", list.Kind)
		assert.Equal(t, "policies.kyverno.io/v1alpha1", list.APIVersion)
		assert.Equal(t, "1", list.ResourceVersion)
		assert.Len(t, list.Items, 2)
	})

	t.Run("list items", func(t *testing.T) {
		assert.Equal(t, "policy-1", list.Items[0].GetName())
		assert.Equal(t, "namespace-1", list.Items[0].GetNamespace())
		assert.Equal(t, "*/1 * * * *", list.Items[0].Spec.Schedule)

		assert.Equal(t, "policy-2", list.Items[1].GetName())
		assert.Equal(t, "namespace-2", list.Items[1].GetNamespace())
		assert.Equal(t, "0 * * * *", list.Items[1].Spec.Schedule)
	})
}

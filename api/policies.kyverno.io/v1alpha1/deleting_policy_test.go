package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_DeletingPolicy(t *testing.T) {
	created := time.Now()
	dpol := DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: DeletingPolicySpec{
			Schedule: "*/1 * * * *",
		},
	}

	t.Run("GetKind", func(t *testing.T) {
		assert.Equal(t, "DeletingPolicy", dpol.GetKind())
	})

	t.Run("invalid schedule", func(t *testing.T) {
		dpol := DeletingPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind: "DeletingPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.NewTime(created),
			},
			Spec: DeletingPolicySpec{
				Schedule: "invalud",
			},
		}

		_, err := dpol.GetNextExecutionTime(time.Now())

		assert.Error(t, err)
	})

	t.Run("get next execution time", func(t *testing.T) {
		now := time.Now()
		next, err := dpol.GetNextExecutionTime(now)

		assert.NoError(t, err)
		assert.Equal(t, now.Add(1*time.Minute).Format("2006-01-02 15:04"), (*next).Format("2006-01-02 15:04"))
	})

	t.Run("fallback execution time", func(t *testing.T) {
		exec, err := dpol.GetExecutionTime()

		assert.NoError(t, err)
		assert.Equal(t, created.Add(1*time.Minute).Format("2006-01-02 15:04"), exec.Format("2006-01-02 15:04"))
	})

	t.Run("last execution time", func(t *testing.T) {
		dpol := DeletingPolicy{
			Spec: DeletingPolicySpec{
				Schedule: "*/1 * * * *",
			},
			Status: DeletingPolicyStatus{
				LastExecutionTime: metav1.NewTime(created),
			},
		}

		exec, err := dpol.GetExecutionTime()

		assert.NoError(t, err)
		assert.Equal(t, created.Add(1*time.Minute).Format("2006-01-02 15:04"), exec.Format("2006-01-02 15:04"))
	})
}

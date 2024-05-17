package ttl

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockMetaObj struct {
	metav1.ObjectMeta
}

func TestParseDeletionTime(t *testing.T) {
	// Test cases
	tests := []struct {
		creationTime         time.Time
		ttlValue             string
		expectedDeletionTime time.Time
		expectError          bool
	}{
		// Test case 1: ttlValue is a valid duration
		{
			creationTime:         time.Date(2023, 7, 18, 12, 0, 0, 0, time.UTC),
			ttlValue:             "2h30m",
			expectedDeletionTime: time.Date(2023, 7, 18, 14, 30, 0, 0, time.UTC),
			expectError:          false,
		},
		// Test case 2: ttlValue is in RFC3339 format
		{
			creationTime:         time.Date(2023, 7, 18, 12, 0, 0, 0, time.UTC),
			ttlValue:             "2023-07-19T120000Z",
			expectedDeletionTime: time.Date(2023, 7, 19, 12, 0, 0, 0, time.UTC),
			expectError:          false,
		},
		// Test case 3: ttlValue is in custom date format
		{
			creationTime:         time.Date(2023, 7, 18, 12, 0, 0, 0, time.UTC),
			ttlValue:             "2023-07-19",
			expectedDeletionTime: time.Date(2023, 7, 19, 0, 0, 0, 0, time.UTC),
			expectError:          false,
		},
		// Test case 4: Invalid ttlValue
		{
			creationTime: time.Date(2023, 7, 18, 12, 0, 0, 0, time.UTC),
			ttlValue:     "invalid-value",
			expectError:  true,
		},
	}

	for _, test := range tests {
		var deletionTime time.Time
		metaObj := &mockMetaObj{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.NewTime(test.creationTime),
			},
		}
		err := parseDeletionTime(metaObj, &deletionTime, test.ttlValue)
		if test.expectError {
			if err == nil {
				t.Errorf("Expected an error but got nil for ttlValue: %s", test.ttlValue)
			}
		} else {
			if err != nil {
				t.Errorf("Expected no error but got: %v for ttlValue: %s", err, test.ttlValue)
			}
			if !deletionTime.Equal(test.expectedDeletionTime) {
				t.Errorf("Expected deletion time: %v but got: %v for ttlValue: %s",
					test.expectedDeletionTime, deletionTime, test.ttlValue)
			}
		}
	}
}

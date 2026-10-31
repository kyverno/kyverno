package v2

import (
	"encoding/json"
	"testing"
)

func TestGlobalContextEntryStatusPreservesLegacyReady(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"true", "false"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			var status GlobalContextEntryStatus
			if err := json.Unmarshal([]byte(`{"ready":`+value+`}`), &status); err != nil {
				t.Fatal(err)
			}
			if status.IsReady() {
				t.Fatal("deprecated ready field must not replace the Ready condition")
			}

			data, err := json.Marshal(status)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if got := string(fields["ready"]); got != value {
				t.Fatalf("deprecated ready field changed during round trip: got %q, want %s", got, value)
			}
		})
	}
}

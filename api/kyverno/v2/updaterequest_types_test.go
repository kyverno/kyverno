package v2

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
)

func TestUpdateRequestJSONRoundTrip(t *testing.T) {
	t.Parallel()
	in := UpdateRequest{
		Spec: UpdateRequestSpec{
			Type:   Generate,
			Policy: "my-policy",
			Rule:   "my-rule",
			RuleContext: []RuleContext{
				{Rule: "my-rule", Trigger: kyverno.ResourceSpec{Kind: "Pod", Name: "trigger-pod"}},
			},
			Resource: kyverno.ResourceSpec{Kind: "Pod", Namespace: "default", Name: "trigger-pod"},
		},
		Status: UpdateRequestStatus{
			State: Completed,
			GeneratedResources: []kyverno.ResourceSpec{
				{Kind: "ConfigMap", Namespace: "default", Name: "generated-1"},
				{Kind: "ConfigMap", Namespace: "default", Name: "generated-2"},
			},
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out UpdateRequest
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip mismatch: in=%+v out=%+v", in, out)
	}
}

func TestUpdateRequestStatusDeepCopyAliasing(t *testing.T) {
	t.Parallel()
	in := UpdateRequestStatus{
		State: Completed,
		GeneratedResources: []kyverno.ResourceSpec{
			{Kind: "ConfigMap", Namespace: "default", Name: "generated-1"},
		},
	}
	copyOut := in.DeepCopy()
	if !reflect.DeepEqual(in, *copyOut) {
		t.Fatalf("deepcopy not equal: in=%+v copy=%+v", in, *copyOut)
	}

	copyOut.GeneratedResources[0].Name = "mutated"
	if in.GeneratedResources[0].Name == "mutated" {
		t.Fatalf("mutating copy.GeneratedResources leaked into original")
	}
}

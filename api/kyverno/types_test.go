package kyverno

import (
	"encoding/json"
	"reflect"
	"testing"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestResourceSpecJSONRoundTrip(t *testing.T) {
	t.Parallel()
	in := ResourceSpec{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  "default",
		Name:       "my-deploy",
		UID:        types.UID("abc-123"),
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out ResourceSpec
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip mismatch: in=%+v out=%+v", in, out)
	}

	// empty optionals must be omitted, kind must be present
	empty := ResourceSpec{Kind: "Pod"}
	b, err = json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	if _, ok := m["kind"]; !ok {
		t.Fatalf("expected kind to be present: %s", b)
	}
	for _, k := range []string{"apiVersion", "namespace", "name", "uid"} {
		if _, ok := m[k]; ok {
			t.Fatalf("expected %q to be omitted for empty optional: %s", k, b)
		}
	}
}

func TestAPICallJSONRoundTrip(t *testing.T) {
	t.Parallel()
	in := APICall{
		URLPath: "/api/v1/namespaces",
		Method:  Method("POST"),
		Data: []RequestData{
			{Key: "key1", Value: &apiextv1.JSON{Raw: []byte(`"value1"`)}},
		},
		Service: &ServiceCall{
			URL: "https://svc.ns:443/path",
			Headers: []HTTPHeader{
				{Key: "Authorization", Value: "Bearer token"},
				{Key: "Content-Type", Value: "application/json"},
			},
			CABundle: "ca-bundle-data",
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out APICall
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip mismatch: in=%+v out=%+v", in, out)
	}
}

func TestAPICallDeepCopyAliasing(t *testing.T) {
	t.Parallel()
	in := APICall{
		URLPath: "/api/v1/namespaces",
		Method:  Method("POST"),
		Data: []RequestData{
			{Key: "key1", Value: &apiextv1.JSON{Raw: []byte(`"value1"`)}},
		},
		Service: &ServiceCall{
			URL:     "https://svc.ns:443/path",
			Headers: []HTTPHeader{{Key: "Authorization", Value: "Bearer token"}},
		},
	}
	copyOut := in.DeepCopy()
	if !reflect.DeepEqual(in, *copyOut) {
		t.Fatalf("deepcopy not equal: in=%+v copy=%+v", in, *copyOut)
	}

	// mutating the copy's nested fields must not affect the original
	copyOut.Data[0].Value.Raw[0] = 'X'
	if in.Data[0].Value.Raw[0] == 'X' {
		t.Fatalf("mutating copy.Data[0].Value.Raw leaked into original")
	}

	copyOut.Service.Headers[0].Value = "mutated"
	if in.Service.Headers[0].Value == "mutated" {
		t.Fatalf("mutating copy.Service.Headers[0] leaked into original")
	}

	copyOut.Service.URL = "mutated-url"
	if in.Service.URL == "mutated-url" {
		t.Fatalf("mutating copy.Service leaked into original")
	}
}

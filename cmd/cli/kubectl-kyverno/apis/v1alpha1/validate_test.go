package v1alpha1

import (
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestValidateAPICallResponses(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URL: "https://example.com",
			Response: APICallResponse{
				StatusCode: 200,
				Body:       runtime.RawExtension{Raw: []byte(`{"a":1}`)},
			},
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("empty url", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{URL: " ", Response: APICallResponse{}}})
		if err == nil || !strings.Contains(err.Error(), "url is required") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("bad status", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URL:      "https://x",
			Response: APICallResponse{StatusCode: 99},
		}})
		if err == nil || !strings.Contains(err.Error(), "statusCode") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("invalid json body", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URL:      "https://x",
			Response: APICallResponse{Body: runtime.RawExtension{Raw: []byte(`{`)}},
		}})
		if err == nil || !strings.Contains(err.Error(), "valid JSON") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestValidateGlobalContextEntries(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := ValidateGlobalContextEntries([]GlobalContextEntryValue{{
			Name: "g",
			Projections: []GlobalContextProjection{
				{Name: "items", Path: "deployments"},
			},
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("empty name", func(t *testing.T) {
		err := ValidateGlobalContextEntries([]GlobalContextEntryValue{{Name: " "}})
		if err == nil || !strings.Contains(err.Error(), "name is required") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("projection missing name", func(t *testing.T) {
		err := ValidateGlobalContextEntries([]GlobalContextEntryValue{{
			Name:        "g",
			Projections: []GlobalContextProjection{{Name: "", Path: "p"}},
		}})
		if err == nil || !strings.Contains(err.Error(), "projection name") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("projection missing path", func(t *testing.T) {
		err := ValidateGlobalContextEntries([]GlobalContextEntryValue{{
			Name:        "g",
			Projections: []GlobalContextProjection{{Name: "n", Path: " "}},
		}})
		if err == nil || !strings.Contains(err.Error(), "path is required") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestRawExtensionToObject(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		v, err := RawExtensionToObject(runtime.RawExtension{})
		if err != nil || v != nil {
			t.Fatalf("got %v, %v", v, err)
		}
	})
	t.Run("object", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]int{"k": 1})
		v, err := RawExtensionToObject(runtime.RawExtension{Raw: raw})
		if err != nil {
			t.Fatal(err)
		}
		m := v.(map[string]interface{})
		if int(m["k"].(float64)) != 1 {
			t.Fatalf("%#v", v)
		}
	})
}

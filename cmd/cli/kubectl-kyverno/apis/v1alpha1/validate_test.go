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
	t.Run("empty url and urlPath", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{URL: " ", URLPath: " ", Response: APICallResponse{}}})
		if err == nil || !strings.Contains(err.Error(), "either url or urlPath is required") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("both url and urlPath", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{URL: "https://x", URLPath: "/path", Response: APICallResponse{}}})
		if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("bad status", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URL:      "https://x",
			Response: APICallResponse{StatusCode: 99, Body: runtime.RawExtension{Raw: []byte(`{}`)}},
		}})
		if err == nil || !strings.Contains(err.Error(), "statusCode") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("missing body", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URL:      "https://x",
			Response: APICallResponse{Body: runtime.RawExtension{}},
		}})
		if err == nil || !strings.Contains(err.Error(), "body is required") {
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
	t.Run("body must be object not array", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URL:      "https://x",
			Response: APICallResponse{Body: runtime.RawExtension{Raw: []byte(`[]`)}},
		}})
		if err == nil || !strings.Contains(err.Error(), "JSON object") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("body must be object not scalar", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URL:      "https://x",
			Response: APICallResponse{Body: runtime.RawExtension{Raw: []byte(`"x"`)}},
		}})
		if err == nil || !strings.Contains(err.Error(), "JSON object") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("urlPath style path", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URLPath: "/apis/some/path",
			Response: APICallResponse{
				Body: runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid url not absolute https", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URL: "http://example.com/path",
			Response: APICallResponse{
				Body: runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("https URL missing host", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URL: "https://",
			Response: APICallResponse{
				Body: runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}})
		if err == nil || !strings.Contains(err.Error(), "has scheme but no host") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("https scheme fragment only", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URL: "https:",
			Response: APICallResponse{
				Body: runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}})
		if err == nil || !strings.Contains(err.Error(), "has scheme but no host") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("invalid urlPath not absolute", func(t *testing.T) {
		err := ValidateAPICallResponses([]APICallResponseEntry{{
			URLPath: "relative/path",
			Response: APICallResponse{
				Body: runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}})
		if err == nil || !strings.Contains(err.Error(), "absolute path starting") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestValidateGlobalContextEntries(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := ValidateGlobalContextEntries([]GlobalContextEntryValue{{
			Name: "g",
			Data: runtime.RawExtension{Raw: []byte(`{}`)},
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
			Data:        runtime.RawExtension{Raw: []byte(`{}`)},
			Projections: []GlobalContextProjection{{Name: "", Path: "p"}},
		}})
		if err == nil || !strings.Contains(err.Error(), "projection name") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("projection missing path", func(t *testing.T) {
		err := ValidateGlobalContextEntries([]GlobalContextEntryValue{{
			Name:        "g",
			Data:        runtime.RawExtension{Raw: []byte(`{}`)},
			Projections: []GlobalContextProjection{{Name: "n", Path: " "}},
		}})
		if err == nil || !strings.Contains(err.Error(), "path is required") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("projections without data", func(t *testing.T) {
		err := ValidateGlobalContextEntries([]GlobalContextEntryValue{{
			Name:        "g",
			Data:        runtime.RawExtension{},
			Projections: []GlobalContextProjection{{Name: "n", Path: "p"}},
		}})
		if err == nil || !strings.Contains(err.Error(), "data is required") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("missing data without projections", func(t *testing.T) {
		err := ValidateGlobalContextEntries([]GlobalContextEntryValue{{
			Name: "g",
			Data: runtime.RawExtension{},
		}})
		if err == nil || !strings.Contains(err.Error(), "data is required") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("duplicate name", func(t *testing.T) {
		err := ValidateGlobalContextEntries([]GlobalContextEntryValue{
			{Name: "g", Data: runtime.RawExtension{Raw: []byte(`{}`)}},
			{Name: "g", Data: runtime.RawExtension{Raw: []byte(`{"x":1}`)}},
		})
		if err == nil || !strings.Contains(err.Error(), "duplicate name") {
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

package store

import (
	"encoding/json"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestLookupMockResponse(t *testing.T) {
	idx := map[string]interface{}{
		"GET:https://a.example": "by-method",
		"https://b.example":     "plain",
	}
	t.Run("method key", func(t *testing.T) {
		v, ok := lookupMockResponse(idx, "GET", "https://a.example")
		if !ok || v != "by-method" {
			t.Fatalf("got %v, %v", v, ok)
		}
	})
	t.Run("plain fallback", func(t *testing.T) {
		v, ok := lookupMockResponse(idx, "POST", "https://b.example")
		if !ok || v != "plain" {
			t.Fatalf("got %v, %v", v, ok)
		}
	})
	t.Run("empty method uses plain only", func(t *testing.T) {
		_, ok := lookupMockResponse(idx, "", "https://a.example")
		if ok {
			t.Fatal("expected miss")
		}
	})
	t.Run("miss", func(t *testing.T) {
		_, ok := lookupMockResponse(idx, "GET", "https://none.example")
		if ok {
			t.Fatal("expected miss")
		}
	})
}

func TestBuildAPICallURLIndex(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		idx, err := buildAPICallURLIndex(nil)
		if err != nil || idx != nil {
			t.Fatalf("idx=%v err=%v", idx, err)
		}
	})
	t.Run("with method", func(t *testing.T) {
		body := map[string]interface{}{"ok": true}
		raw, _ := json.Marshal(body)
		idx, err := buildAPICallURLIndex([]v1alpha1.APICallResponseEntry{{
			URL:    "https://x",
			Method: "POST",
			Response: v1alpha1.APICallResponse{
				Body: runtime.RawExtension{Raw: raw},
			},
		}})
		if err != nil {
			t.Fatal(err)
		}
		v, ok := idx["POST:https://x"]
		if !ok {
			t.Fatalf("keys: %#v", idx)
		}
		if v.(map[string]interface{})["ok"] != true {
			t.Fatalf("%#v", v)
		}
	})
	t.Run("invalid body", func(t *testing.T) {
		_, err := buildAPICallURLIndex([]v1alpha1.APICallResponseEntry{{
			URL:      "https://x",
			Response: v1alpha1.APICallResponse{Body: runtime.RawExtension{Raw: []byte(`{`)}},
		}})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

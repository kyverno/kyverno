package store

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

func TestAddValuesReplacesFullObjects(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	jsonContext := enginecontext.NewContext(jp)
	if err := jsonContext.AddResource(map[string]interface{}{"spec": map[string]interface{}{"image": "new", "paused": "yes"}}); err != nil {
		t.Fatal(err)
	}
	if err := jsonContext.AddOldResource(map[string]interface{}{"spec": map[string]interface{}{"image": "old", "paused": "yes"}}); err != nil {
		t.Fatal(err)
	}

	values := map[string]interface{}{
		"request.object":            map[string]interface{}{"spec": map[string]interface{}{"image": "object"}},
		"request.oldObject":         map[string]interface{}{"spec": map[string]interface{}{"image": "oldObject"}},
		"request.object.spec.image": "dotted",
		"foo":                       "bar",
	}
	if err := AddValues(jsonContext, values); err != nil {
		t.Fatal(err)
	}

	// full-object values replace the resource rather than merging into it, and win over
	// dotted-path values targeting the same key
	for query, want := range map[string]string{
		"request.object.spec":    "object",
		"request.oldObject.spec": "oldObject",
	} {
		got, err := jsonContext.Query(query)
		if err != nil {
			t.Fatalf("query %s: %v", query, err)
		}
		spec, ok := got.(map[string]interface{})
		if !ok || len(spec) != 1 || spec["image"] != want {
			t.Errorf("query %s: got %v, want map[image:%s]", query, got, want)
		}
	}
	if got, err := jsonContext.Query("foo"); err != nil || got != "bar" {
		t.Errorf("query foo: got %v, %v, want bar", got, err)
	}
}

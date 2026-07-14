package apply

import (
	"context"
	"encoding/json"
	"fmt"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// staticEntry implements store.Entry for CLI offline use.
// It holds a pre-fetched snapshot of Kubernetes resources and
// optionally pre-computed JMESPath projections.
type staticEntry struct {
	objects     []interface{}
	projections map[string]interface{}
}

func (e *staticEntry) Get(projection string) (any, error) {
	if projection == "" {
		return e.objects, nil
	}
	if result, ok := e.projections[projection]; ok {
		return result, nil
	}
	return nil, fmt.Errorf("projection %q not found", projection)
}

func (e *staticEntry) Stop() {}

// buildGlobalContextStore creates a globalcontext store populated from
// GlobalContextEntry resources that reference KubernetesResource specs.
// The data is fetched once from the provided fake client.
func buildGlobalContextStore(
	ctx context.Context,
	gctxEntries []*kyvernov2.GlobalContextEntry,
	client dclient.Interface,
	jp jmespath.Interface,
) (gctxstore.Store, error) {
	s := gctxstore.New()
	for _, gce := range gctxEntries {
		if gce.Spec.KubernetesResource == nil {
			continue
		}
		kr := gce.Spec.KubernetesResource
		gvr := schema.GroupVersionResource{Group: kr.Group, Version: kr.Version, Resource: kr.Resource}

		// Use the dynamic client directly with the GVR from the spec so the
		// list targets kr.Resource exactly — avoids kind/apiVersion guessing
		// and handles irregular plurals and zero-instance resource types.
		dynClient := client.GetDynamicInterface().Resource(gvr)
		var unstructList *unstructured.UnstructuredList
		var err error
		if kr.Namespace != "" {
			unstructList, err = dynClient.Namespace(kr.Namespace).List(ctx, metav1.ListOptions{})
		} else {
			unstructList, err = dynClient.List(ctx, metav1.ListOptions{})
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list resources for GlobalContextEntry %s (%s): %w", gce.Name, gvr, err)
		}

		objects := make([]interface{}, 0, len(unstructList.Items))
		for i := range unstructList.Items {
			objects = append(objects, unstructList.Items[i].Object)
		}

		if len(gce.Spec.Projections) > 0 && jp == nil {
			return nil, fmt.Errorf("cannot build projections for GlobalContextEntry %s: JMESPath engine is required", gce.Name)
		}
		projections := make(map[string]interface{})
		for _, p := range gce.Spec.Projections {
			query, err := jp.Query(p.JMESPath)
			if err != nil {
				return nil, fmt.Errorf("failed to parse projection %q for GlobalContextEntry %s: %w", p.Name, gce.Name, err)
			}
			result, err := query.Search(objects)
			if err != nil {
				return nil, fmt.Errorf("failed to apply projection %q for GlobalContextEntry %s: %w", p.Name, gce.Name, err)
			}
			projections[p.Name] = result
		}

		s.Set(gce.Name, &staticEntry{
			objects:     objects,
			projections: projections,
		})
	}
	return s, nil
}

// extractGlobalContextEntries separates GlobalContextEntry resources from
// the main resource list and returns both sets.
func extractGlobalContextEntries(resources []*unstructured.Unstructured) ([]*unstructured.Unstructured, []*kyvernov2.GlobalContextEntry, error) {
	var regular []*unstructured.Unstructured
	var gctxEntries []*kyvernov2.GlobalContextEntry

	for _, r := range resources {
		gvk := r.GroupVersionKind()
		if gvk.Kind == "GlobalContextEntry" && gvk.Group == "kyverno.io" && gvk.Version == "v2" {
			gce := &kyvernov2.GlobalContextEntry{}
			if err := convertToGCE(r, gce); err != nil {
				return nil, nil, fmt.Errorf("failed to convert %s %q to GlobalContextEntry: %w", gvk, r.GetName(), err)
			}
			gctxEntries = append(gctxEntries, gce)
		} else {
			regular = append(regular, r)
		}
	}
	return regular, gctxEntries, nil
}

func convertToGCE(src *unstructured.Unstructured, dst *kyvernov2.GlobalContextEntry) error {
	data, err := json.Marshal(src.Object)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

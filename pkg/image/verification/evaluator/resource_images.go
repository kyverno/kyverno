package evaluator

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
)

func resourceImageReferences(attr admission.Attributes, request any) []string {
	object, _ := request.(map[string]any)
	objects := []map[string]any{object}
	if attr != nil {
		for _, resource := range []runtime.Object{attr.GetObject(), attr.GetOldObject()} {
			if resource != nil {
				fields, err := runtime.DefaultUnstructuredConverter.ToUnstructured(resource)
				if err == nil {
					objects = append(objects, fields)
				}
			}
		}
	}
	var images []string
	for _, object := range objects {
		for _, path := range [][]string{{"spec"}, {"spec", "template", "spec"}, {"spec", "jobTemplate", "spec", "template", "spec"}} {
			for _, category := range []string{"containers", "initContainers", "ephemeralContainers"} {
				containers, _, _ := unstructured.NestedSlice(object, append(path, category)...)
				for _, container := range containers {
					if fields, ok := container.(map[string]any); ok {
						if image, ok := fields["image"].(string); ok && image != "" {
							images = append(images, image)
						}
					}
				}
			}
		}
	}
	return images
}

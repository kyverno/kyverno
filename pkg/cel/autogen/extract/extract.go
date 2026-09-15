package extract

import "fmt"

// PathSegment is one step of a structured path into a parent object -
// either a map key or an array index. It is used to rebuild a real JSON Pointer
// for patch rebasing (see RebasePatch in patch.go), which the display-only
// Path string can't safely support.
type PathSegment struct {
	Key   string
	Index int
	IsIdx bool
}

// Extracted is a single pod-template-shaped subtree found inside a custom
// workload resource, together with the path it was found at (for
// diagnostics only).
type Extracted struct {
	Template map[string]any
	Path     string
	Segments []PathSegment // for patch rebasing
}

// ExtractPodTemplates walks obj's spec looking for every subtree that is
// structurally shaped like a corev1.PodTemplateSpec: an object with an
// optional "metadata" object and a "spec.containers" list where every
// container has a non-empty string "name" and a string "image". It does not
// know about any specific CRD - it works the same way for JobSet,
// PyTorchJob, a Deployment, or anything else, because CRD authors
// overwhelmingly embed the real Kubernetes PodTemplateSpec type verbatim.
//
// Discovery is deliberately scoped to obj["spec"], not the whole object:
// status commonly mirrors back an observed/last-applied template (rollout
// history, canary step state, ...), and a stale or informational template
// found there must not be able to deny an otherwise-valid spec update.
// metadata and other top-level fields are excluded for the same reason.
//
// Once a match is found at a node, ExtractPodTemplates does not descend
// further into it - a pod template does not contain a nested pod template.
func ExtractPodTemplates(obj map[string]any) []Extracted {
	spec, ok := obj["spec"].(map[string]any)
	if !ok {
		return nil
	}
	var out []Extracted
	walk(spec, "spec", []PathSegment{{Key: "spec"}}, &out)
	return out
}

func walk(node any, path string, segs []PathSegment, out *[]Extracted) {
	switch v := node.(type) {
	case map[string]any:
		if isPodTemplateSpec(v) {
			*out = append(*out, Extracted{Template: v, Path: path, Segments: append([]PathSegment{}, segs...)})
			return
		}
		for key, val := range v {
			walk(val, joinPath(path, key), appendSeg(segs, PathSegment{Key: key}), out)
		}
	case []any:
		for i, val := range v {
			walk(val, fmt.Sprintf("%s[%d]", path, i), appendSeg(segs, PathSegment{Index: i, IsIdx: true}), out)
		}
	}
}

func appendSeg(segs []PathSegment, s PathSegment) []PathSegment {
	out := make([]PathSegment, len(segs)+1)
	copy(out, segs)
	out[len(segs)] = s
	return out
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func isPodTemplateSpec(v map[string]any) bool {
	// metadata is optional (CRD authors frequently omit it entirely when
	// unset), but if present it must be an object.
	if metadata, ok := v["metadata"]; ok {
		if _, ok := metadata.(map[string]any); !ok {
			return false
		}
	}
	spec, ok := v["spec"].(map[string]any)
	if !ok {
		return false
	}
	containers, ok := spec["containers"].([]any)
	if !ok || len(containers) == 0 {
		return false
	}
	for _, c := range containers {
		container, ok := c.(map[string]any)
		if !ok {
			return false
		}
		name, ok := container["name"].(string)
		if !ok || name == "" {
			return false
		}
		if _, ok := container["image"].(string); !ok {
			return false
		}
	}
	return true
}

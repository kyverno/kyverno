package extract

import (
	"fmt"
	"strings"
)

// Extracted is a single pod-template-shaped subtree found inside a custom
// workload resource, together with the path it was found at (for
// diagnostics only).
type Extracted struct {
	Template map[string]any
	Path     string
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
	walk(spec, "spec", &out)
	return out
}

func walk(node any, path string, out *[]Extracted) {
	switch v := node.(type) {
	case map[string]any:
		if isPodTemplateSpec(v) {
			*out = append(*out, Extracted{Template: v, Path: path})
			return
		}
		for key, val := range v {
			walk(val, joinPath(path, key), out)
		}
	case []any:
		for i, val := range v {
			walk(val, fmt.Sprintf("%s[%d]", path, i), out)
		}
	}
}

// joinPath appends key to path, backslash-escaping any '.', '[', ']' or '\'
// in key itself so parsePath can invert the result unambiguously - a map key
// that legitimately contains one of those characters (e.g. a replica-type
// name used as a map key, "worker.pool") would otherwise be indistinguishable
// from a path separator or array-index syntax.
func joinPath(path, key string) string {
	escaped := escapePathSegment(key)
	if path == "" {
		return escaped
	}
	return path + "." + escaped
}

func escapePathSegment(key string) string {
	if !strings.ContainsAny(key, `\.[]`) {
		return key
	}
	var b strings.Builder
	for _, r := range key {
		switch r {
		case '\\', '.', '[', ']':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
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

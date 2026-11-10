package evaluator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	authzhttp "github.com/kyverno/kyverno-authz/pkg/cel/libs/authz/http"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/test"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/payload"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"google.golang.org/protobuf/encoding/protojson"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// Mode is the kind of payload an evaluation runs against.
type Mode string

const (
	// ModeKubernetes evaluates Kubernetes resources (VAP/MAP, ValidatingPolicy, MutatingPolicy,
	// GeneratingPolicy, ImageValidatingPolicy, DeletingPolicy).
	ModeKubernetes Mode = "Kubernetes"
	// ModeJSON evaluates arbitrary JSON documents against JSON evaluation-mode policies.
	ModeJSON Mode = "JSON"
	// ModeHTTP evaluates Kyverno Authz HTTP check requests.
	ModeHTTP Mode = "HTTP"
	// ModeEnvoy evaluates Envoy ext_authz CheckRequests.
	ModeEnvoy Mode = "Envoy"
)

// Modes lists every supported mode.
var Modes = []Mode{ModeKubernetes, ModeJSON, ModeHTTP, ModeEnvoy}

// ParseMode parses a mode name case-insensitively.
func ParseMode(s string) (Mode, error) {
	for _, m := range Modes {
		if strings.EqualFold(string(m), strings.TrimSpace(s)) {
			return m, nil
		}
	}
	return "", fmt.Errorf("unsupported mode %q, must be one of %v", s, Modes)
}

// DecodeInput decodes a raw JSON or YAML request body into the Input for mode:
//   - Kubernetes: one or more resources (multi-document YAML, a JSON object, or a v1 List)
//   - JSON: a single JSON/YAML object
//   - HTTP: a Kyverno Authz HTTP CheckRequest
//   - Envoy: an Envoy ext_authz CheckRequest (protojson)
//
// An empty body, a body that does not parse, or a body with no evaluable object is an error, so a
// malformed request can never be mistaken for one that matched nothing.
func DecodeInput(mode Mode, body []byte) (Input, error) {
	var in Input
	if len(bytes.TrimSpace(body)) == 0 {
		return in, fmt.Errorf("request body is empty")
	}
	switch mode {
	case ModeKubernetes:
		resources, err := resource.GetUnstructuredResources(body)
		if err != nil {
			return in, fmt.Errorf("failed to decode Kubernetes resources: %w", err)
		}
		resources = nonEmpty(resources)
		if len(resources) == 0 {
			return in, fmt.Errorf("request body contains no Kubernetes resource")
		}
		for i, r := range resources {
			if r.GetAPIVersion() == "" || r.GetKind() == "" {
				return in, fmt.Errorf("resource %d is missing apiVersion or kind", i)
			}
		}
		in.Resources = test.ProcessResources(resources)
	case ModeJSON:
		name := "payload.yaml"
		if trimmed := bytes.TrimSpace(body); trimmed[0] == '{' || trimmed[0] == '[' {
			name = "payload.json"
		}
		decoded, err := payload.Parse(name, body)
		if err != nil {
			return in, fmt.Errorf("failed to decode JSON payload: %w", err)
		}
		object, ok := decoded.(map[string]any)
		if !ok {
			return in, fmt.Errorf("JSON payload must be a single top-level object")
		}
		in.JSONPayloads = []*unstructured.Unstructured{{Object: object}}
	case ModeHTTP:
		data, err := yaml.YAMLToJSON(body)
		if err != nil {
			return in, fmt.Errorf("failed to decode HTTP check request: %w", err)
		}
		// reject unknown fields: a misspelled one (say "attribute") would otherwise be dropped and the
		// verdict given for a different request
		var request authzhttp.CheckRequest
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&request); err != nil {
			return in, fmt.Errorf("failed to decode HTTP check request: %w", err)
		}
		in.HTTPRequests = []*authzhttp.CheckRequest{&request}
	case ModeEnvoy:
		data, err := yaml.YAMLToJSON(body)
		if err != nil {
			return in, fmt.Errorf("failed to decode Envoy check request: %w", err)
		}
		var request authv3.CheckRequest
		if err := protojson.Unmarshal(data, &request); err != nil {
			return in, fmt.Errorf("failed to decode Envoy check request: %w", err)
		}
		in.EnvoyRequests = []*authv3.CheckRequest{&request}
	default:
		return in, fmt.Errorf("unsupported mode %q", mode)
	}
	return in, nil
}

// ProcessResources normalizes decoded resources the way apply does before evaluating them.
func ProcessResources(resources []*unstructured.Unstructured) []*unstructured.Unstructured {
	return test.ProcessResources(resources)
}

func nonEmpty(resources []*unstructured.Unstructured) []*unstructured.Unstructured {
	out := resources[:0]
	for _, r := range resources {
		if r != nil && len(r.Object) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// dedupe drops repeated objects (same apiVersion, kind, namespace and name), keeping the last one:
// the fake client panics on duplicates, and the same object may legitimately arrive both as a
// resource and as a target or cluster object.
func dedupe(resources []*unstructured.Unstructured) []*unstructured.Unstructured {
	index := map[string]int{}
	out := make([]*unstructured.Unstructured, 0, len(resources))
	for _, r := range resources {
		key := r.GetAPIVersion() + "/" + r.GetKind() + "/" + r.GetNamespace() + "/" + r.GetName()
		if r.GetName() == "" {
			out = append(out, r)
			continue
		}
		if i, ok := index[key]; ok {
			out[i] = r
			continue
		}
		index[key] = len(out)
		out = append(out, r)
	}
	return out
}

// NewFakeClient builds an in-memory cluster client seeded with the given resources (List kinds are
// flattened), so policies that look objects up (or match on namespaces) see them. It is what
// apply uses when it is not pointed at a real cluster.
func NewFakeClient(resources, targetResources, parameterResources []*unstructured.Unstructured) (dclient.Interface, error) {
	allResources := make([]*unstructured.Unstructured, 0, len(resources)+len(targetResources)+len(parameterResources))
	allResources = append(allResources, resources...)
	allResources = append(allResources, targetResources...)
	allResources = append(allResources, parameterResources...)

	flatResources, err := resource.FlattenResources(allResources)
	if err != nil {
		return nil, err
	}
	allResources = dedupe(flatResources)

	gvrToListKind := make(map[schema.GroupVersionResource]string)
	// gvrToGVK holds the authoritative GVR→GVK mapping derived directly from
	// each resource's TypeMeta. This avoids relying on inferKindFromResourceName,
	// which mishandles multi-word kinds such as ClusterRole ("clusterroles" → "Clusterrol").
	gvrToGVK := make(map[schema.GroupVersionResource]schema.GroupVersionKind)
	var discoveryGVRs []schema.GroupVersionResource
	seen := make(map[schema.GroupVersionResource]bool)

	objects := make([]runtime.Object, 0, len(allResources))
	for _, r := range allResources {
		objects = append(objects, r.DeepCopy())
		gvk := r.GroupVersionKind()
		if gvk.Kind == "" || gvk.Version == "" {
			return nil, fmt.Errorf("resource %s/%s is missing TypeMeta (apiVersion=%q kind=%q)", r.GetNamespace(), r.GetName(), r.GetAPIVersion(), gvk.Kind)
		}
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)
		if !seen[gvr] {
			seen[gvr] = true
			gvrToListKind[gvr] = gvk.Kind + "List"
			gvrToGVK[gvr] = gvk
			discoveryGVRs = append(discoveryGVRs, gvr)
		}
	}

	fakeClient, err := dclient.NewFakeClient(runtime.NewScheme(), gvrToListKind, objects...)
	if err != nil {
		return nil, err
	}
	disco := dclient.NewFakeDiscoveryClient(discoveryGVRs)
	for gvr, gvk := range gvrToGVK {
		disco.AddGVRToGVKMapping(gvr, gvk)
	}
	fakeClient.SetDiscovery(disco)
	return fakeClient, nil
}

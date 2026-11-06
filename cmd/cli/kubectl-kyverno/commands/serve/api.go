package serve

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/evaluator"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// APIVersion is the version of the kyverno serve HTTP contract. It is experimental: fields may be
// added, renamed or removed in any release without a deprecation cycle.
const APIVersion = "serve.cli.kyverno.io/v1alpha1"

// Decision is the overall outcome of one evaluation.
type Decision string

const (
	// DecisionAllow means at least one policy applied and none failed or errored.
	DecisionAllow Decision = "Allow"
	// DecisionDeny means at least one policy rule failed (a policy violation, or an authz deny).
	DecisionDeny Decision = "Deny"
	// DecisionNoMatch means no policy produced a pass or fail result for the payload. It is never
	// reported as Allow, so a typo in a policy's match block cannot read as compliance.
	DecisionNoMatch Decision = "NoMatch"
	// DecisionError means at least one policy errored, or the evaluation itself failed.
	DecisionError Decision = "Error"
)

// EvaluationRequest is the body of POST /v1alpha1/evaluate: a payload plus the context it is
// evaluated in. Every string field holds raw YAML or JSON (multi-document YAML where a field takes
// several objects), the same content the matching `kyverno apply` file flag would read.
type EvaluationRequest struct {
	APIVersion string         `json:"apiVersion,omitempty"`
	Kind       string         `json:"kind,omitempty"`
	Mode       evaluator.Mode `json:"mode"`
	// Payload is what is evaluated, in the format POST /v1alpha1/evaluate/{mode} takes.
	Payload string `json:"payload"`
	// Policies, when set, are evaluated instead of the server's --policies. Only accepted when the
	// server runs with --allow-request-policies.
	Policies string `json:"policies,omitempty"`
	// Exceptions are CEL PolicyExceptions applied in addition to those loaded with the policies.
	Exceptions string `json:"exceptions,omitempty"`
	// Context is a cli.kyverno.io/v1alpha1 Context document (what `kyverno apply --context-file`
	// reads): the resources CEL resource.get/resource.list lookups return, and image data.
	Context string `json:"context,omitempty"`
	// ClusterResources seed the in-memory cluster client alongside the payload, which namespace
	// lookups (namespaceSelector, namespaceObject) read. Use Context for resource.get lookups.
	ClusterResources string `json:"clusterResources,omitempty"`
	// ParameterResources are ValidatingAdmissionPolicy/MutatingAdmissionPolicy parameters.
	ParameterResources string `json:"parameterResources,omitempty"`
	// TargetResources are existing objects mutateExisting MutatingPolicies may mutate
	// (`kyverno apply --target-resource`).
	TargetResources string `json:"targetResources,omitempty"`
	// CustomResourceDefinitions describe custom resource kinds in the payload or targets
	// (`kyverno apply --crd-paths`). They only apply to this request.
	CustomResourceDefinitions string `json:"customResourceDefinitions,omitempty"`
	// Operation is the admission operation to simulate: CREATE (default), UPDATE or DELETE.
	Operation string `json:"operation,omitempty"`
}

// EvaluationResponse is the body of POST /v1alpha1/evaluate and POST /v1alpha1/evaluate/{mode}.
type EvaluationResponse struct {
	APIVersion      string         `json:"apiVersion"`
	Kind            string         `json:"kind"`
	Mode            evaluator.Mode `json:"mode"`
	KyvernoVersion  string         `json:"kyvernoVersion"`
	PolicySetDigest string         `json:"policySetDigest"`
	Decision        Decision       `json:"decision"`
	Summary         Summary        `json:"summary"`
	Results         []RuleResult   `json:"results"`
	Artifacts       *Artifacts     `json:"artifacts,omitempty"`
	Warnings        []string       `json:"warnings,omitempty"`
	Errors          []string       `json:"errors,omitempty"`
}

// Summary counts rule results by status.
type Summary struct {
	Pass  int `json:"pass"`
	Fail  int `json:"fail"`
	Warn  int `json:"warn"`
	Error int `json:"error"`
	Skip  int `json:"skip"`
}

// ObjectRef identifies a policy or a resource.
type ObjectRef struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name,omitempty"`
}

// RuleResult is one policy rule result.
type RuleResult struct {
	Policy     ObjectRef          `json:"policy"`
	Rule       string             `json:"rule,omitempty"`
	Type       engineapi.RuleType `json:"type,omitempty"`
	Status     string             `json:"status"`
	Message    string             `json:"message,omitempty"`
	Resource   *ObjectRef         `json:"resource,omitempty"`
	Exceptions []string           `json:"exceptions,omitempty"`
	Properties map[string]string  `json:"properties,omitempty"`
	// PatchedResource is the mutated resource (or JSON payload) this result produced, set on
	// passing Mutation results.
	PatchedResource map[string]any `json:"patchedResource,omitempty"`
	// PatchedTarget is the existing object a MutatingPolicy with targets mutated (mutate-existing).
	PatchedTarget map[string]any `json:"patchedTarget,omitempty"`
	// GeneratedResources are the resources this Generation result produced.
	GeneratedResources []map[string]any `json:"generatedResources,omitempty"`
}

// Artifacts are the objects produced by mutating and generating policies.
type Artifacts struct {
	// Patched are the mutated resources (or mutated JSON payload), one per mutated input.
	Patched []map[string]any `json:"patched,omitempty"`
	// Generated are the resources produced by GeneratingPolicies.
	Generated []map[string]any `json:"generated,omitempty"`
}

// PolicyList is the body of GET /v1alpha1/policies.
type PolicyList struct {
	APIVersion      string                `json:"apiVersion"`
	Kind            string                `json:"kind"`
	KyvernoVersion  string                `json:"kyvernoVersion"`
	PolicySetDigest string                `json:"policySetDigest"`
	Modes           []evaluator.Mode      `json:"modes"`
	Policies        []evaluator.PolicyRef `json:"policies"`
}

// Status is the body of every non-2xx response, and of /healthz and /readyz.
type Status struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Code       int    `json:"code"`
	Message    string `json:"message,omitempty"`
}

func newEvaluationResponse(mode evaluator.Mode, version, digest string, responses []engineapi.EngineResponse) *EvaluationResponse {
	resp := &EvaluationResponse{
		APIVersion:      APIVersion,
		Kind:            "EvaluationResponse",
		Mode:            mode,
		KyvernoVersion:  version,
		PolicySetDigest: digest,
		Results:         []RuleResult{},
	}
	artifacts := &Artifacts{}
	for _, er := range responses {
		var policyRef ObjectRef
		if pol := er.Policy(); pol != nil {
			policyRef = ObjectRef{APIVersion: pol.GetAPIVersion(), Kind: pol.GetKind(), Namespace: pol.GetNamespace(), Name: pol.GetName()}
		}
		resourceRef := resourceRefOf(mode, er.Resource)
		mutated := false
		for i := range er.PolicyResponse.Rules {
			rule := &er.PolicyResponse.Rules[i]
			result := RuleResult{
				Policy:     policyRef,
				Rule:       rule.Name(),
				Type:       rule.RuleType(),
				Status:     string(rule.Status()),
				Message:    rule.Message(),
				Resource:   resourceRef,
				Properties: rule.Properties(),
			}
			for _, ex := range rule.Exceptions() {
				result.Exceptions = append(result.Exceptions, ex.GetName())
			}
			switch rule.Status() {
			case engineapi.RuleStatusPass:
				resp.Summary.Pass++
			case engineapi.RuleStatusFail:
				resp.Summary.Fail++
			case engineapi.RuleStatusWarn:
				resp.Summary.Warn++
			case engineapi.RuleStatusError:
				resp.Summary.Error++
				resp.Errors = append(resp.Errors, describe(result))
			case engineapi.RuleStatusSkip:
				resp.Summary.Skip++
			}
			if rule.RuleType() == engineapi.Mutation && rule.Status() == engineapi.RuleStatusPass && len(er.PatchedResource.Object) > 0 {
				mutated = true
				result.PatchedResource = er.PatchedResource.Object
			}
			if target, _, _ := rule.PatchedTarget(); target != nil {
				result.PatchedTarget = target.Object
			}
			for _, gen := range rule.GeneratedResources() {
				if gen != nil {
					artifacts.Generated = append(artifacts.Generated, gen.Object)
					result.GeneratedResources = append(result.GeneratedResources, gen.Object)
				}
			}
			resp.Results = append(resp.Results, result)
		}
		if mutated {
			artifacts.Patched = append(artifacts.Patched, er.PatchedResource.Object)
		}
	}
	if len(artifacts.Patched) > 0 || len(artifacts.Generated) > 0 {
		resp.Artifacts = artifacts
	}
	resp.Decision = decide(resp.Summary)
	return resp
}

func resourceRefOf(mode evaluator.Mode, r unstructured.Unstructured) *ObjectRef {
	if mode != evaluator.ModeKubernetes || len(r.Object) == 0 {
		return nil
	}
	return &ObjectRef{APIVersion: r.GetAPIVersion(), Kind: r.GetKind(), Namespace: r.GetNamespace(), Name: r.GetName()}
}

func describe(r RuleResult) string {
	name := r.Policy.Name
	if r.Rule != "" && r.Rule != name {
		name += "/" + r.Rule
	}
	if r.Message == "" {
		return name
	}
	return name + ": " + r.Message
}

// decide folds a summary into a decision. Errors win over failures, failures over passes, and a
// payload no policy passed or failed is NoMatch, never Allow.
func decide(s Summary) Decision {
	switch {
	case s.Error > 0:
		return DecisionError
	case s.Fail > 0:
		return DecisionDeny
	case s.Pass > 0 || s.Warn > 0:
		return DecisionAllow
	default:
		return DecisionNoMatch
	}
}

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
)

type JSONPolicyStatus string

const (
	JSONPolicyApplied JSONPolicyStatus = "applied"
	JSONPolicySkipped JSONPolicyStatus = "skipped"
	JSONPolicyError   JSONPolicyStatus = "error"
)

type JSONPolicyResponse struct {
	Policy string
	Status JSONPolicyStatus
	// Document is the full document after this policy ran (unchanged for a
	// skipped policy). It is absent when Status is JSONPolicyError. Each entry
	// is an independent buffer, so callers may retain it.
	Document         json.RawMessage
	Exceptions       []*policiesv1beta1.PolicyException
	AuditAnnotations map[string]string
	Error            error
}

type JSONResponse struct {
	Document json.RawMessage
	Policies []JSONPolicyResponse
}

type jsonPolicy struct {
	key      string
	compiled *compiler.JSONPolicy
}

// JSONEngine evaluates a fixed policy set in caller-supplied order. It has no
// Kubernetes clients and is safe for concurrent calls.
type JSONEngine struct {
	policies []jsonPolicy
}

func NewJSONEngine(policies []policiesv1beta1.MutatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*JSONEngine, error) {
	result := &JSONEngine{}
	seen := make(map[string]bool)
	for i, policy := range policies {
		if policy == nil || (reflect.ValueOf(policy).Kind() == reflect.Pointer && reflect.ValueOf(policy).IsNil()) {
			return nil, fmt.Errorf("policy %d is nil", i)
		}
		key := PolicyKey(policy)
		if seen[key] {
			return nil, fmt.Errorf("duplicate JSON policy %q", key)
		}
		seen[key] = true
		var matched []*policiesv1beta1.PolicyException
		for _, exception := range exceptions {
			if exception == nil {
				return nil, fmt.Errorf("nil policy exception")
			}
			if exception.IsExpired() {
				continue
			}
			for _, ref := range exception.Spec.PolicyRefs {
				if ref.Name == policy.GetName() && ref.Kind == policy.GetKind() {
					matched = append(matched, exception)
					break
				}
			}
		}
		compiled, errs := compiler.CompileJSON(policy, matched)
		if len(errs) > 0 {
			return nil, fmt.Errorf("JSON policy %s: %w", key, errs.ToAggregate())
		}
		result.policies = append(result.policies, jsonPolicy{key: key, compiled: compiled})
	}
	return result, nil
}

// HandleJSON accepts exactly one JSON value, including arrays, scalars and null.
// On any error Document is absent: callers must not consume partial mutations.
func (e *JSONEngine) HandleJSON(ctx context.Context, document json.RawMessage) (JSONResponse, error) {
	var response JSONResponse
	if err := ctx.Err(); err != nil {
		return response, err
	}
	if len(document) > compiler.MaxJSONDocumentBytes {
		return response, fmt.Errorf("JSON document exceeds %d bytes", compiler.MaxJSONDocumentBytes)
	}
	if !json.Valid(document) {
		return response, fmt.Errorf("invalid JSON document")
	}
	patched := bytes.Clone(document)
	for _, policy := range e.policies {
		result, err := policy.compiled.EvaluateJSON(ctx, patched)
		outcome := JSONPolicyResponse{Policy: policy.key, Status: JSONPolicyApplied}
		if err != nil {
			outcome.Status, outcome.Error = JSONPolicyError, err
			response.Policies = append(response.Policies, outcome)
			return response, fmt.Errorf("JSON policy %s: %w", policy.key, err)
		}
		if result.Skipped {
			outcome.Status = JSONPolicySkipped
		}
		outcome.Document = result.Document
		outcome.Exceptions = result.Exceptions
		outcome.AuditAnnotations = result.AuditAnnotations
		response.Policies = append(response.Policies, outcome)
		patched = result.Document
	}
	if err := ctx.Err(); err != nil {
		return response, err
	}
	response.Document = patched
	return response, nil
}

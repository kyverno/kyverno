package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/payload"
	celpolicies "github.com/kyverno/kyverno/pkg/cel/policies"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func (p *PolicyProcessor) applyPoliciesOnJSON() ([]engineapi.EngineResponse, error) {
	if p.Out == nil {
		p.Out = io.Discard
	}
	if p.Rc == nil {
		p.Rc = &ResultCounts{}
	}
	ctx := p.Context
	if ctx == nil {
		ctx = context.Background()
	}
	var policies []policiesv1beta1.MutatingPolicyLike
	byName := make(map[string]policiesv1beta1.MutatingPolicyLike)
	var responses []engineapi.EngineResponse
	p.JSONPatchedDocuments = make(map[string]*payload.Document)
	for _, policy := range p.MutatingPolicies {
		if !celpolicies.IsJSONMutatingPolicy(policy) {
			message := "Kubernetes-mode MutatingPolicy skipped for JSON document; set spec.evaluation.mode to JSON"
			fmt.Fprintf(p.Out, "Skipping policy %s for JSON document %s: %s\n", policy.GetName(), p.JSONDocument.Name, message)
			response := engineapi.EngineResponse{PolicyResponse: engineapi.PolicyResponse{
				Rules: []engineapi.RuleResponse{*engineapi.RuleSkip("mutation", engineapi.Mutation, message, nil)},
			}}.WithPolicy(engineapi.NewMutatingPolicyFromLike(policy))
			p.Rc.addMutateResponse(response)
			responses = append(responses, response)
			continue
		}

		policies = append(policies, policy)
		key := mpolengine.PolicyKey(policy)
		if byName[key] != nil {
			p.Rc.IncrementError(1)
			return responses, fmt.Errorf("duplicate JSON policy %q", key)
		}
		byName[key] = policy
	}
	eng, err := mpolengine.NewJSONEngine(policies, p.CELExceptions)
	if err != nil {
		p.Rc.IncrementError(1)
		return responses, err
	}
	result, evaluationErr := eng.HandleJSON(ctx, p.JSONDocument.Raw)
	for _, outcome := range result.Policies {
		rule := engineapi.RulePass("mutation", engineapi.Mutation, "JSON document mutated", outcome.AuditAnnotations)
		switch outcome.Status {
		case mpolengine.JSONPolicySkipped:
			rule = engineapi.RuleSkip("mutation", engineapi.Mutation, "JSON mutation skipped by match condition, exception or failed JSON Patch test", outcome.AuditAnnotations)
		case mpolengine.JSONPolicyError:
			rule = engineapi.RuleError("mutation", engineapi.Mutation, "JSON mutation failed", outcome.Error, nil)
		}
		var exceptions []engineapi.GenericException
		for _, exception := range outcome.Exceptions {
			exceptions = append(exceptions, engineapi.NewCELPolicyException(exception))
		}
		rule = rule.WithExceptions(exceptions)
		response := engineapi.EngineResponse{PolicyResponse: engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{*rule},
		}}.WithPolicy(engineapi.NewMutatingPolicyFromLike(byName[outcome.Policy]))
		p.Rc.addMutateResponse(response)
		responses = append(responses, response)
	}
	if evaluationErr != nil {
		clear(p.JSONPatchedDocuments)
		hasErrorResult := false
		for _, outcome := range result.Policies {
			hasErrorResult = hasErrorResult || outcome.Status == mpolengine.JSONPolicyError
		}
		if !hasErrorResult {
			p.Rc.IncrementError(1)
		}
		return responses, evaluationErr
	}
	p.JSONDocument.Raw = result.Document
	for _, outcome := range result.Policies {
		p.JSONPatchedDocuments[outcome.Policy] = &payload.Document{Name: p.JSONDocument.Name, Raw: outcome.Document}
	}
	// The existing validation engines require object roots. Do not synthesize
	// Kubernetes identity or turn arrays/scalars into objects for those engines.
	if len(p.ValidatingPolicies) > 0 {
		object, err := p.JSONDocument.Object()
		if err != nil {
			p.Rc.IncrementError(1)
			return responses, err
		}
		validation := *p
		validation.JSONDocument = nil
		validation.JsonPayload = *object
		validation.MutatingPolicies = nil
		validation.Policies = nil
		validation.ValidatingAdmissionPolicies = nil
		validation.MutatingAdmissionPolicies = nil
		validation.GeneratingPolicies = nil
		validated, err := validation.ApplyPoliciesOnResource()
		responses = append(responses, validated...)
		if err != nil {
			p.Rc.IncrementError(1)
			return responses, err
		}
	}
	return responses, nil
}

// PrintJSONDocument uses the existing mutation output destination, without
// decoding through float64 or requiring Kubernetes metadata for filenames.
func (p *PolicyProcessor) PrintJSONDocument() error {
	if p.JSONDocument == nil || !json.Valid(p.JSONDocument.Raw) {
		return fmt.Errorf("no successful JSON document to print")
	}
	return p.printOutput(p.JSONDocument.Raw, engineapi.EngineResponse{}, p.JSONDocument.Name, false)
}

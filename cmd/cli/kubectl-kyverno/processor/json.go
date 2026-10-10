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

func (p *PolicyProcessor) applyPoliciesOnJSON(ctx context.Context) ([]engineapi.EngineResponse, error) {
	if p.Out == nil {
		p.Out = io.Discard
	}
	if p.Rc == nil {
		p.Rc = &ResultCounts{}
	}
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
	// Every other policy family keeps the existing object-root JSON payload
	// pipeline, now running on the mutated document. Kubernetes-mode
	// MutatingPolicies were already reported as skipped above.
	hasOtherPolicies := len(p.ValidatingPolicies) > 0 || len(p.Policies) > 0 ||
		len(p.ValidatingAdmissionPolicies) > 0 || len(p.MutatingAdmissionPolicies) > 0 ||
		len(p.GeneratingPolicies) > 0
	if !hasOtherPolicies {
		return responses, nil
	}
	// The existing engines require object roots. Do not synthesize Kubernetes
	// identity or turn arrays/scalars into objects for them.
	object, err := p.JSONDocument.Object()
	if err != nil {
		p.Rc.IncrementError(1)
		return responses, err
	}
	pipeline := *p
	pipeline.JSONDocument = nil
	pipeline.JsonPayload = *object
	pipeline.MutatingPolicies = nil
	processed, err := pipeline.ApplyPoliciesOnResourceWithContext(ctx)
	responses = append(responses, processed...)
	if err != nil {
		p.Rc.IncrementError(1)
		return responses, err
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

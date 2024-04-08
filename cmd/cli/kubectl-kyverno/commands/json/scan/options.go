package scan

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kyverno/kyverno-json/pkg/apis/policy/v1alpha1"
	"github.com/kyverno/kyverno-json/pkg/engine/template"
	jsonengine "github.com/kyverno/kyverno-json/pkg/json-engine"
	"github.com/kyverno/kyverno-json/pkg/payload"
	"github.com/kyverno/kyverno-json/pkg/policy"
	"github.com/kyverno/kyverno/ext/output/pluralize"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/labels"
)

type options struct {
	payload       string
	preprocessors []string
	policies      []string
	selectors     []string
	output        string
}

func (c *options) run(cmd *cobra.Command, _ []string) error {
	out := newOutput(cmd.OutOrStdout(), c.output)
	out.println("Loading policies ...")
	policies, err := policy.Load(c.policies...)
	if err != nil {
		return err
	}
	selector := labels.Everything()
	if len(c.selectors) != 0 {
		parsed, err := labels.Parse(strings.Join(c.selectors, ","))
		if err != nil {
			return err
		}
		selector = parsed
	}
	{
		var filteredPolicies []*v1alpha1.ValidatingPolicy
		for _, policy := range policies {
			if selector.Matches(labels.Set(policy.Labels)) {
				filteredPolicies = append(filteredPolicies, policy)
			}
		}
		policies = filteredPolicies
	}
	out.println("Loading payload ...")
	payload, err := payload.Load(c.payload)
	if err != nil {
		return err
	}
	if payload == nil {
		return errors.New("payload is `null`")
	}
	out.println("Pre processing ...")
	for _, preprocessor := range c.preprocessors {
		result, err := template.Execute(context.Background(), preprocessor, payload, nil)
		if err != nil {
			return err
		}
		if result == nil {
			return fmt.Errorf("prepocessor resulted in `null` payload (%s)", preprocessor)
		}
		payload = result
	}
	var resources []any
	if slice, ok := payload.([]any); ok {
		resources = slice
	} else {
		resources = append(resources, payload)
	}
	out.println("Running", "(", "evaluating", len(resources), pluralize.Pluralize(len(resources), "resource", "resources"), "against", len(policies), pluralize.Pluralize(len(policies), "policy", "policies"), ")", "...")
	e := jsonengine.New()
	var responses []jsonengine.Response
	for _, resource := range resources {
		responses = append(responses, e.Run(context.Background(), jsonengine.Request{
			Resource: resource,
			Policies: policies,
		}))
	}
	for _, response := range responses {
		for _, policy := range response.Policies {
			for _, rule := range policy.Rules {
				if rule.Error != nil {
					out.println("-", policy.Policy.Name, "/", rule.Rule.Name, "/", rule.Identifier, "ERROR:", rule.Error.Error())
				} else if len(rule.Violations) != 0 {
					out.println("-", policy.Policy.Name, "/", rule.Rule.Name, "/", rule.Identifier, "FAILED:", multierr.Combine(rule.Violations).Error())
				} else {
					// TODO: handle skip, warn
					out.println("-", policy.Policy.Name, "/", rule.Rule.Name, "/", rule.Identifier, "PASSED")
				}
			}
		}
	}
	out.responses(responses...)
	out.println("Done")
	return nil
}

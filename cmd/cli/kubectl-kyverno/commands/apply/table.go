package apply

import (
	"io"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy/annotations"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func printTable(out io.Writer, compact, auditWarn bool, engineResponses ...engineapi.EngineResponse) {
	var resultsTable table.Table
	id := 1
	for _, engineResponse := range engineResponses {
		policy := engineResponse.Policy()
		policyName := policy.GetName()
		policyNamespace := policy.GetNamespace()
		scored := annotations.Scored(policy.GetAnnotations())
		resourceKind := engineResponse.Resource.GetKind()
		resourceNamespace := engineResponse.Resource.GetNamespace()
		resourceName := engineResponse.Resource.GetName()

		for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
			var row table.Row
			row.ID = id
			id++
			row.Policy = color.Policy(policyNamespace, policyName)
			if policy.GetType() == engineapi.KyvernoPolicyType {
				row.Rule = color.Rule(ruleResponse.Name())
			}
			row.Resource = color.Resource(resourceKind, resourceNamespace, resourceName)
			if ruleResponse.Status() == engineapi.RuleStatusPass {
				row.Result = color.ResultPass()
			} else if ruleResponse.Status() == engineapi.RuleStatusFail {
				if !scored {
					row.Result = color.ResultWarn()
				} else if auditWarn && engineResponse.GetValidationFailureAction().Audit() {
					row.Result = color.ResultWarn()
				} else {
					row.Result = color.ResultFail()
				}
			} else if ruleResponse.Status() == engineapi.RuleStatusWarn {
				row.Result = color.ResultWarn()
			} else if ruleResponse.Status() == engineapi.RuleStatusError {
				row.Result = color.ResultError()
			} else if ruleResponse.Status() == engineapi.RuleStatusSkip {
				row.Result = color.ResultSkip()
			}
			row.Message = ruleResponse.Message()
			resultsTable.Add(row)
		}
	}
	printer := table.NewTablePrinter(out)
	printer.Print(resultsTable.Rows(compact))
}

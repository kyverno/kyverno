package apply

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

type Table struct {
	rows []Row
}

func (t *Table) Rows(compact bool) interface{} {
	if !compact {
		return t.rows
	}
	var rows []CompactRow
	for _, row := range t.rows {
		rows = append(rows, row.CompactRow)
	}
	return rows
}

func (t *Table) AddFailed(rows ...Row) {
	for _, row := range rows {
		if row.isFailure {
			t.rows = append(t.rows, row)
		}
	}
}

func (t *Table) Add(rows ...Row) {
	t.rows = append(t.rows, rows...)
}

type CompactRow struct {
	isFailure bool
	ID        int    `header:"id"`
	Policy    string `header:"policy"`
	Rule      string `header:"rule"`
	Resource  string `header:"resource"`
	Result    string `header:"result"`
}

type Row struct {
	CompactRow `header:"inline"`
	Message    string `header:"message"`
}

func printTable(compact, auditWarn bool, engineResponses ...engineapi.EngineResponse) {
	var table Table
	id := 1
	for _, engineResponse := range engineResponses {
		var policyNamespace, policyName string
		var ann map[string]string

		isVAP := engineResponse.IsValidatingAdmissionPolicy()

		if isVAP {
			policy := engineResponse.ValidatingAdmissionPolicy()
			policyNamespace = policy.GetNamespace()
			policyName = policy.GetName()
			ann = policy.GetAnnotations()
		} else {
			policy := engineResponse.Policy()
			policyNamespace = policy.GetNamespace()
			policyName = policy.GetName()
			ann = policy.GetAnnotations()
		}
		resourceKind := engineResponse.Resource.GetKind()
		resourceNamespace := engineResponse.Resource.GetNamespace()
		resourceName := engineResponse.Resource.GetName()

		for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
			var row Row
			row.ID = id
			id++
			if policyNamespace == "" {
				row.Policy = test.BoldFgCyan.Sprint(policyName)
			} else {
				row.Policy = test.BoldFgCyan.Sprint(policyNamespace) + "/" + test.BoldFgCyan.Sprint(policyName)
			}
			if !isVAP {
				row.Rule = test.BoldFgCyan.Sprint(ruleResponse.Name())
			}
			if resourceNamespace == "" {
				row.Resource = test.BoldFgCyan.Sprint(resourceKind) + "/" + test.BoldFgCyan.Sprint(resourceName)
			} else {
				row.Resource = test.BoldFgCyan.Sprint(resourceNamespace) + "/" + test.BoldFgCyan.Sprint(resourceKind) + "/" + test.BoldFgCyan.Sprint(resourceName)
			}
			if ruleResponse.Status() == engineapi.RuleStatusPass {
				row.Result = test.BoldGreen.Sprint(policyreportv1alpha2.StatusPass)
			} else if ruleResponse.Status() == engineapi.RuleStatusFail {
				if scored, ok := ann[kyvernov1.AnnotationPolicyScored]; ok && scored == "false" {
					row.Result = test.BoldYellow.Sprint(policyreportv1alpha2.StatusWarn)
				} else if auditWarn && engineResponse.GetValidationFailureAction().Audit() {
					row.Result = test.BoldYellow.Sprint(policyreportv1alpha2.StatusWarn)
				} else {
					row.Result = test.BoldRed.Sprint(policyreportv1alpha2.StatusFail)
				}
			} else if ruleResponse.Status() == engineapi.RuleStatusWarn {
				row.Result = test.BoldYellow.Sprint(policyreportv1alpha2.StatusWarn)
			} else if ruleResponse.Status() == engineapi.RuleStatusError {
				row.Result = test.BoldRed.Sprint(policyreportv1alpha2.StatusError)
			} else if ruleResponse.Status() == engineapi.RuleStatusSkip {
				row.Result = test.BoldFgCyan.Sprint(policyreportv1alpha2.StatusSkip)
			}
			row.Message = ruleResponse.Message()
			table.Add(row)
		}
	}
	printer := test.NewTablePrinter()
	printer.Print(table.Rows(compact))
}

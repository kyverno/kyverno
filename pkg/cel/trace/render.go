package trace

import (
	"fmt"
	"io"
	"strings"
)

// maxValueLen caps how much of a resolved value is printed on one line, so a node that resolved
// to a whole object (say, all containers) does not swamp the output.
const maxValueLen = 100

// Render writes d in the human-readable SCOPE / MATCH / VARIABLES / VERDICT form. A nil
// decision writes nothing, so callers can pass a result's Trace without checking it first.
//
// The default view is one line per expression: its source, an arrow, and what it resolved to.
// For a verdict that is not PASS, the per-node breakdown of the deciding expression is printed
// beneath it, since that is what shows which sub-expression produced which value.
func Render(w io.Writer, d *Decision) {
	if d == nil {
		return
	}
	if d.PolicyName != "" {
		fmt.Fprintf(w, "Policy:   %s (%s)\n", d.PolicyName, d.PolicyKind)
	}
	if d.ResourceName != "" || d.ResourceKind != "" {
		fmt.Fprintf(w, "Resource: %s\n", resourceLabel(d))
	}
	fmt.Fprintln(w)

	scope := "skipped"
	if d.Scope.Applied {
		scope = "applied"
	}
	if d.Scope.Reason != "" {
		row(w, "SCOPE", scope, d.Scope.Reason)
	}

	for _, m := range d.Match {
		row(w, "MATCH", "", named(m))
	}
	for _, v := range d.Variables {
		row(w, "VARIABLES", "", named(v))
	}

	v := d.Verdict
	if v.Status == "" {
		return
	}
	if v.Source != "" {
		row(w, "VERDICT", v.Status, expressionLine(v.ExpressionTrace))
	} else {
		row(w, "VERDICT", v.Status, v.Message)
		return
	}
	if v.Message != "" && v.Status != VerdictPass {
		fmt.Fprintf(w, "%-10s %-8s message: %q\n", "", "", v.Message)
	}
	if v.Status != VerdictPass && len(v.Nodes) > 0 {
		fmt.Fprintf(w, "%-10s %-8s evaluated:\n", "", "")
		for _, n := range v.Nodes {
			if n.Error != "" {
				fmt.Fprintf(w, "%-10s %-8s   %s  ->  ERROR: %s\n", "", "", n.Expression, clip(n.Error))
				continue
			}
			fmt.Fprintf(w, "%-10s %-8s   %s  ->  %s\n", "", "", n.Expression, clip(n.Value))
		}
	}
}

func row(w io.Writer, layer, status, rest string) {
	fmt.Fprintf(w, "%-10s %-8s %s\n", layer, status, rest)
}

// named prefixes the expression with its declared name, when it has one.
func named(n NamedExpressionTrace) string {
	if n.Name == "" {
		return expressionLine(n.ExpressionTrace)
	}
	return n.Name + ": " + expressionLine(n.ExpressionTrace)
}

func expressionLine(et ExpressionTrace) string {
	return fmt.Sprintf("%s  ->  %s", et.Source, clip(et.Result))
}

func resourceLabel(d *Decision) string {
	label := d.ResourceName
	if d.ResourceKind != "" {
		label = d.ResourceKind + "/" + d.ResourceName
	}
	if d.ResourceNamespace != "" {
		label += " (namespace: " + d.ResourceNamespace + ")"
	}
	return label
}

func clip(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxValueLen {
		return s[:maxValueLen] + "..."
	}
	return s
}

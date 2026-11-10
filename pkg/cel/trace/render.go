package trace

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// maxValueLen caps how much of a resolved value is printed on one line, so a node that resolved
// to a whole object (say, all containers) does not swamp the output.
const maxValueLen = 100

// Render writes d in the human-readable SCOPE / MATCH / VARIABLES / VALIDATION / VERDICT form. A nil
// decision writes nothing, so callers can pass a result's Trace without checking it first.
//
// The default view is one line per expression: its source, an arrow, and what it resolved to.
// For a verdict that is not PASS, the per-node breakdown of the deciding expression is printed
// beneath it, since that is what shows which sub-expression produced which value.
//
// When the resource is a Secret, all non-boolean resolved values are replaced with "(redacted)"
// so that Secret data is not printed to the terminal.
func Render(w io.Writer, d *Decision) {
	if d == nil {
		return
	}
	rf := renderFormat{secret: d.ResourceKind == "Secret"}
	if d.PolicyName != "" {
		fmt.Fprintf(w, "Policy:   %s (%s)\n", d.PolicyName, d.PolicyKind)
	}
	if d.ResourceName != "" || d.ResourceKind != "" {
		fmt.Fprintf(w, "Resource: %s\n", resourceLabel(d))
	}
	fmt.Fprintln(w)

	if rf.secret {
		fmt.Fprintf(w, "NOTE: values are redacted because this resource is a Secret.\n\n")
	}

	scope := "skipped"
	if d.Scope.Applied {
		scope = "applied"
	}
	if d.Scope.Reason != "" {
		row(w, "SCOPE", scope, d.Scope.Reason)
	}

	for _, m := range d.Match {
		row(w, "MATCH", "", rf.namedLine(m))
	}
	for _, v := range d.Variables {
		row(w, "VARIABLES", "", rf.namedLine(v))
	}
	if d.Images != nil {
		if len(d.Images.Found) == 0 {
			row(w, "IMAGES", "", "no images found on the resource")
		}
		for _, img := range d.Images.Found {
			if img.Checked {
				row(w, "IMAGES", "checked", img.Category+": "+rf.value(img.Image))
				continue
			}
			row(w, "IMAGES", "skipped", img.Category+": "+rf.value(img.Image)+"  (not matched by matchImageReferences)")
		}
	}
	// with a single validation the list would only repeat the verdict line
	listed := len(d.Validations) > 1
	if listed {
		for _, val := range d.Validations {
			if val.Status == VerdictNotRun {
				row(w, "VALIDATION", val.Status, fmt.Sprintf("[%d] %s  (an earlier validation did not pass)", val.Index, val.Source))
				continue
			}
			row(w, "VALIDATION", val.Status, fmt.Sprintf("[%d] %s", val.Index, rf.expressionLine(val.ExpressionTrace)))
		}
	}
	for _, m := range d.Mutations {
		if m.Error == "" {
			row(w, "MUTATIONS", "", rf.namedLine(NamedExpressionTrace{Name: m.Name, ExpressionTrace: m.ExpressionTrace}))
			if m.NotApplied != "" {
				fmt.Fprintf(w, "%-10s %-8s (not applied: %s)\n", "", "", rf.text(m.NotApplied))
			}
			continue
		}
		row(w, "MUTATIONS", VerdictError, m.Name+": "+rf.text(m.Error))
		rf.printNodes(w, m.ExpressionTrace)
	}
	for _, g := range d.Generations {
		if g.Error != "" {
			row(w, "GENERATE", VerdictError, g.Name+": "+rf.text(g.Error))
			rf.printNodes(w, g.ExpressionTrace)
			continue
		}
		if g.Source != "" {
			row(w, "GENERATE", "", rf.namedLine(NamedExpressionTrace{Name: g.Name, ExpressionTrace: g.ExpressionTrace}))
		} else {
			row(w, "GENERATE", "", g.Name)
		}
		if len(g.Generated) == 0 {
			fmt.Fprintf(w, "%-10s %-8s generated nothing\n", "", "")
		}
		for _, r := range g.Generated {
			fmt.Fprintf(w, "%-10s %-8s generated %s\n", "", "", rf.value(r))
		}
	}

	v := d.Verdict
	if v.Status == "" {
		return
	}
	if listed && v.Status == VerdictPass {
		// on a pass no single validation decided; the list above already shows each one
		row(w, "VERDICT", v.Status, fmt.Sprintf("all %d validations passed", len(d.Validations)))
		return
	}
	if v.Source != "" {
		row(w, "VERDICT", v.Status, rf.expressionLine(v.ExpressionTrace))
	} else {
		message := rf.text(v.Message)
		if message == "" && v.Status == VerdictPass {
			switch {
			case len(d.Mutations) > 0:
				// a mutating policy has no single expression that decides pass/fail -- see the
				// MUTATIONS lines above for what actually ran
				message = "completed; see MUTATIONS above for what ran"
			case len(d.Generations) > 0:
				message = "completed; see GENERATE above for what ran"
			default:
				// no validations ran to produce a verdict -- most likely the policy declares none
				message = "no validations to evaluate; the policy passes by default"
			}
		}
		row(w, "VERDICT", v.Status, message)
		return
	}
	if v.Message != "" && v.Status != VerdictPass {
		fmt.Fprintf(w, "%-10s %-8s message: %q\n", "", "", rf.text(v.Message))
	}
	if v.Status != VerdictPass {
		rf.printNodes(w, v.ExpressionTrace)
	}
}

// renderFormat carries the secret flag so every value written through Render applies the same
// redaction policy as LogValues does.
type renderFormat struct {
	secret bool
}

// value formats an evaluated CEL value for rendering: "true" and "false" are always kept; all
// other values are replaced with "(redacted)" in secret traces. Values are clipped to maxValueLen.
func (rf renderFormat) value(v string) string {
	if rf.secret && v != "" && v != "true" && v != "false" {
		return redacted
	}
	return clip(v)
}

// text formats free-form text (errors, messages, notApplied) for rendering: redacted in secret
// traces, printed in full otherwise. Unlike value, non-secret text is not clipped so that long
// error messages and rejection reasons are not silently truncated.
func (rf renderFormat) text(v string) string {
	if rf.secret && v != "" {
		return redacted
	}
	return v
}

// expressionLine formats "source  ->  result", redacting the result when secret.
func (rf renderFormat) expressionLine(et ExpressionTrace) string {
	return fmt.Sprintf("%s  ->  %s", et.Source, rf.value(et.Result))
}

// namedLine prefixes the expression line with its declared name, when it has one.
func (rf renderFormat) namedLine(n NamedExpressionTrace) string {
	if n.Name == "" {
		return rf.expressionLine(n.ExpressionTrace)
	}
	return n.Name + ": " + rf.expressionLine(n.ExpressionTrace)
}

// printNodes prints the per-node breakdown shared by VERDICT, MUTATIONS and GENERATE: one
// indented line per traced sub-expression, showing its resolved value or, if it failed, its
// error, then a note when values inside loops were left out.
func (rf renderFormat) printNodes(w io.Writer, et ExpressionTrace) {
	if len(et.Nodes) > 0 {
		fmt.Fprintf(w, "%-10s %-8s evaluated:\n", "", "")
		for _, n := range et.Nodes {
			if n.Error != "" {
				fmt.Fprintf(w, "%-10s %-8s   %s  ->  ERROR: %s\n", "", "", n.Expression, rf.value(n.Error))
				continue
			}
			fmt.Fprintf(w, "%-10s %-8s   %s  ->  %s\n", "", "", n.Expression, rf.value(n.Value))
		}
	}
	if et.LoopValuesOmitted {
		fmt.Fprintf(w, "%-10s %-8s (values inside loops such as all() and exists() are not shown: CEL keeps only the last item's)\n", "", "")
	}
	if et.NoBreakdown != "" {
		fmt.Fprintf(w, "%-10s %-8s (no breakdown: %s)\n", "", "", et.NoBreakdown)
	}
}

func row(w io.Writer, layer, status, rest string) {
	fmt.Fprintf(w, "%-10s %-8s %s\n", layer, status, rest)
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
		// maxValueLen is a byte offset; back up to a rune boundary so a multi-byte character
		// (e.g. in a label value) isn't split, which would leave invalid UTF-8 before the "...".
		cut := maxValueLen
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		return s[:cut] + "..."
	}
	return s
}

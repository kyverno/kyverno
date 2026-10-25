package apply

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kyvernoreports "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const divider = "----------------------------------------------------------------------"

func printSkippedAndInvalidPolicies(out io.Writer, skipInvalidPolicies SkippedInvalidPolicies) {
	if len(skipInvalidPolicies.skipped) > 0 {
		fmt.Fprintln(out, divider)
		fmt.Fprintln(out, color.BoldYellow("Policies Skipped:"))
		for i, policy := range skipInvalidPolicies.skipped {
			printPolicyDiagnostic(out, i+1, policy)
		}
		fmt.Fprintln(out, divider)
	}
	if len(skipInvalidPolicies.invalid) > 0 {
		fmt.Fprintln(out, divider)
		fmt.Fprintln(out, color.BoldRed("Invalid Policies:"))
		for i, policy := range skipInvalidPolicies.invalid {
			printPolicyDiagnostic(out, i+1, policy)
		}
		fmt.Fprintln(out, divider)
	}
}

func printPolicyDiagnostic(out io.Writer, index int, policy PolicyDiagnostic) {
	coloredPolicyName := color.Policy("", policy.name)
	if policy.reason == "" {
		fmt.Fprintf(out, "%d. %s\n", index, coloredPolicyName)
		return
	}
	fmt.Fprintf(out, "%d. %s: %s\n", index, coloredPolicyName, policy.reason)
}

func printReports(out io.Writer, engineResponses []engineapi.EngineResponse, auditWarn bool, outputFormat string) {
	clustered, namespaced := report.ComputePolicyReports(auditWarn, engineResponses...)

	printReport := func(report interface{}) {
		var output []byte
		if outputFormat == "json" {
			output, _ = json.Marshal(report)
		} else {
			output, _ = yaml.Marshal(report)
		}
		fmt.Fprintln(out, string(output))
	}

	if len(clustered) > 0 {
		clusterReport := report.MergeClusterReports(clustered)
		printReport(clusterReport)
	}

	for _, r := range namespaced {
		fmt.Fprintln(out, "---")
		printReport(r)
	}
}

func printExceptions(out io.Writer, engineResponses []engineapi.EngineResponse, auditWarn bool, outputFormat string, ttl time.Duration) {
	clustered, _ := report.ComputePolicyReports(auditWarn, engineResponses...)
	for _, report := range clustered {
		for _, result := range report.Results {
			if result.Result == "fail" {
				if err := printException(out, result, ttl, outputFormat); err != nil {
					log.Error(err)
				}
			}
		}
	}
}

// policyKindForSource maps a policy-report result's Source to the CEL policy Kind a suggested
// PolicyException's spec.policyRefs[].kind must match exactly (pkg/cel/engine.ListExceptions compares
// by exact Kind string). It assumes the cluster-scoped variant of each kind; a result from a Namespaced*
// policy needs that field hand-edited, since the report result alone doesn't say which one it was.
// Returns "" for sources a Kyverno PolicyException cannot exempt at all (native Kubernetes
// ValidatingAdmissionPolicy/MutatingAdmissionPolicy, which use their own binding-based exclusion).
func policyKindForSource(source string) string {
	switch source {
	case kyvernoreports.SourceValidatingPolicy:
		return "ValidatingPolicy"
	case kyvernoreports.SourceImageValidatingPolicy:
		return "ImageValidatingPolicy"
	case kyvernoreports.SourceMutatingPolicy:
		return "MutatingPolicy"
	case kyvernoreports.SourceGeneratingPolicy:
		return "GeneratingPolicy"
	default:
		return ""
	}
}

// celStringLiteral renders s as a CEL string literal.
func celStringLiteral(s string) string {
	return strconv.Quote(s)
}

// buildExceptionMatchExpression builds a CEL matchCondition expression equivalent to the given
// kind/name/namespace filter: object.kind must be one of kinds, object.metadata.name must equal (or,
// for a trailing "*", start with the prefix of) one of names, and object.metadata.namespace must equal
// namespace when set.
func buildExceptionMatchExpression(kinds, names []string, namespace string) string {
	kindLiterals := make([]string, len(kinds))
	for i, k := range kinds {
		kindLiterals[i] = celStringLiteral(k)
	}
	parts := []string{fmt.Sprintf("object.kind in [%s]", strings.Join(kindLiterals, ", "))}

	nameConds := make([]string, 0, len(names))
	for _, n := range names {
		if prefix, ok := strings.CutSuffix(n, "*"); ok {
			nameConds = append(nameConds, fmt.Sprintf("object.metadata.name.startsWith(%s)", celStringLiteral(prefix)))
		} else {
			nameConds = append(nameConds, fmt.Sprintf("object.metadata.name == %s", celStringLiteral(n)))
		}
	}
	if len(nameConds) > 0 {
		parts = append(parts, "("+strings.Join(nameConds, " || ")+")")
	}
	if namespace != "" {
		parts = append(parts, fmt.Sprintf("object.metadata.namespace == %s", celStringLiteral(namespace)))
	}
	return strings.Join(parts, " && ")
}

func printException(out io.Writer, result openreportsv1alpha1.ReportResult, ttl time.Duration, outputFormat string) error {
	kind := policyKindForSource(result.Source)
	if kind == "" {
		// Native Kubernetes ValidatingAdmissionPolicy/MutatingAdmissionPolicy results aren't
		// exempted via a Kyverno PolicyException; nothing useful to suggest.
		return nil
	}
	for _, r := range result.Subjects {
		name := strings.Join([]string{result.Policy, result.Rule, r.Namespace, r.Name}, "-")

		kinds := []string{r.Kind}
		names := []string{r.Name}
		if strings.HasPrefix(result.Rule, "autogen-") {
			if r.Kind == "CronJob" {
				kinds = append(kinds, "Job", "Pod")
			} else {
				kinds = append(kinds, "Pod")
			}
			names = append(names, r.Name+"-*")
		}

		exception := policiesv1beta1.PolicyException{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PolicyException",
				APIVersion: policiesv1beta1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				PolicyRefs: []policiesv1beta1.PolicyRef{
					{
						Name: result.Policy,
						Kind: kind,
					},
				},
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "match-resource",
						Expression: buildExceptionMatchExpression(kinds, names, r.Namespace),
					},
				},
			},
		}

		if ttl > 0 {
			exception.ObjectMeta.Labels = map[string]string{
				"cleanup.kyverno.io/ttl": ttl.String(),
			}
		}

		if controlList, ok := result.Properties["controlsJSON"]; ok {
			var controls []kyvernoreports.Control
			err := json.Unmarshal([]byte(controlList), &controls)
			if err != nil {
				return errors.Wrapf(err, "failed to unmarshall PSS controls %s", controlList)
			}
			controlNames := make([]string, 0, len(controls))
			for _, c := range controls {
				controlNames = append(controlNames, c.Name)
			}
			if len(controlNames) > 0 {
				if exception.Spec.Properties == nil {
					exception.Spec.Properties = map[string]string{}
				}
				exception.Spec.Properties["podSecurityControls"] = strings.Join(controlNames, ",")
			}
		}

		var exceptionReport []byte
		marshal := func(report interface{}) ([]byte, error) {
			if outputFormat == "json" {
				return json.Marshal(report)
			}
			return yaml.Marshal(report)
		}

		exceptionReport, err := marshal(exception)
		if err != nil {
			return err
		}

		fmt.Fprint(out, "---\n")
		fmt.Fprint(out, string(exceptionReport))
		fmt.Fprint(out, "\n")
	}

	return nil
}

func printViolations(out io.Writer, rc *processor.ResultCounts) {
	fmt.Fprintf(out, "\npass: %s, fail: %s, warn: %s, error: %s, skip: %s \n",
		color.BoldGreen(rc.Pass),
		color.BoldRed(rc.Fail),
		color.BoldYellow(rc.Warn),
		color.BoldRed(rc.Error),
		color.BoldCyan(rc.Skip),
	)
}

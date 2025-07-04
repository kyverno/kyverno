package apply

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kyvernoreports "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
	"sigs.k8s.io/yaml"
)

const divider = "----------------------------------------------------------------------"

func printSkippedAndInvalidPolicies(out io.Writer, skipInvalidPolicies SkippedInvalidPolicies) {
	if len(skipInvalidPolicies.skipped) > 0 {
		fmt.Fprintln(out, divider)
		fmt.Fprintln(out, "Policies Skipped (as required variables are not provided by the user):")
		for i, policyName := range skipInvalidPolicies.skipped {
			fmt.Fprintf(out, "%d. %s\n", i+1, policyName)
		}
		fmt.Fprintln(out, divider)
	}
	if len(skipInvalidPolicies.invalid) > 0 {
		fmt.Fprintln(out, divider)
		fmt.Fprintln(out, "Invalid Policies:")
		for i, policyName := range skipInvalidPolicies.invalid {
			fmt.Fprintf(out, "%d. %s\n", i+1, policyName)
		}
		fmt.Fprintln(out, divider)
	}
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

func printException(out io.Writer, result openreportsv1alpha1.ReportResult, ttl time.Duration, outputFormat string) error {
	for _, r := range result.Subjects {
		name := strings.Join([]string{result.Policy, result.Rule, r.Namespace, r.Name}, "-")

		kinds := []string{r.Kind}
		names := []string{r.Name}
		rules := []string{result.Rule}
		if strings.HasPrefix(result.Rule, "autogen-") {
			if r.Kind == "CronJob" {
				kinds = append(kinds, "Job")
				rules = append(rules, strings.ReplaceAll(result.Rule, "autogen-cronjob-", "autogen-"))
				kinds = append(kinds, "Pod")
				rules = append(rules, result.Rule[len("autogen-cronjob-"):])
			} else {
				kinds = append(kinds, "Pod")
				rules = append(rules, result.Rule[len("autogen-"):])
			}
			names = append(names, r.Name+"-*")
		}

		exception := kyvernov2beta1.PolicyException{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PolicyException",
				APIVersion: kyvernov2beta1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: kyvernov2beta1.PolicyExceptionSpec{
				Match: kyvernov2beta1.MatchResources{
					All: kyvernov1.ResourceFilters{
						kyvernov1.ResourceFilter{
							ResourceDescription: kyvernov1.ResourceDescription{
								Kinds:      kinds,
								Names:      names,
								Namespaces: []string{r.Namespace},
							},
						},
					},
				},
				Exceptions: []kyvernov2beta1.Exception{
					{
						PolicyName: result.Policy,
						RuleNames:  rules,
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
			pssList := make([]kyvernov1.PodSecurityStandard, 0)
			var controls []kyvernoreports.Control
			err := json.Unmarshal([]byte(controlList), &controls)
			if err != nil {
				return errors.Wrapf(err, "failed to unmarshall PSS controls %s", controlList)
			}
			for _, c := range controls {
				pss := kyvernov1.PodSecurityStandard{
					ControlName: c.Name,
				}
				if c.Images != nil {
					pss.Images = wildcardTagOrDigest(c.Images)
				}
				pssList = append(pssList, pss)
			}
			exception.Spec.PodSecurity = pssList
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

var regexpTagOrDigest = regexp.MustCompile(":.*|@.*")

func wildcardTagOrDigest(images []string) []string {
	for i, s := range images {
		images[i] = regexpTagOrDigest.ReplaceAllString(s, "*")
	}
	return images
}

func printViolations(out io.Writer, rc *processor.ResultCounts) {
	fmt.Fprintf(out, "\npass: %d, fail: %d, warn: %d, error: %d, skip: %d \n", rc.Pass, rc.Fail, rc.Warn, rc.Error, rc.Skip)
}

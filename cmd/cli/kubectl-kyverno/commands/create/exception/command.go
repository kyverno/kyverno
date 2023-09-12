package exception

import (
	"os"
	"strings"
	"text/template"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/templates"
	"github.com/spf13/cobra"
)

type options struct {
	Name       string
	Namespace  string
	Background bool
	Exceptions []v2alpha1.Exception
	Match      v2beta1.MatchResources
}

func Command() *cobra.Command {
	var path string
	var rules, any, all []string
	var options options
	cmd := &cobra.Command{
		Use:     "exception",
		Short:   "Create a Kyverno exception file.",
		Example: `kyverno create exception -n my-exception --namespace my-ns --any "kind=Pod,kind=Deployment,name=test-*"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tmpl, err := template.New("exception").Parse(templates.ExceptionTemplate)
			if err != nil {
				return err
			}
			for _, result := range rules {
				result := parseRule(result)
				if result != nil {
					options.Exceptions = append(options.Exceptions, *result)
				}
			}
			for _, result := range any {
				result := parseResourceFilter(result)
				if result != nil {
					options.Match.Any = append(options.Match.Any, *result)
				}
			}
			for _, result := range all {
				result := parseResourceFilter(result)
				if result != nil {
					options.Match.All = append(options.Match.All, *result)
				}
			}
			output := os.Stdout
			if path != "" {
				file, err := os.Create(path)
				if err != nil {
					return err
				}
				defer file.Close()
				output = file
			}
			return tmpl.Execute(output, options)
		},
	}
	cmd.Flags().StringVarP(&path, "output", "o", "", "Output path (uses standard console output if not set)")
	cmd.Flags().StringVarP(&options.Name, "name", "n", "", "Policy exception name")
	cmd.Flags().StringVar(&options.Namespace, "namespace", "", "Policy exception namespace")
	cmd.Flags().BoolVarP(&options.Background, "background", "b", true, "Set to false is policy should not be considered in background scans")
	cmd.Flags().StringArrayVarP(&rules, "rule", "r", nil, "List of policy rules")
	cmd.Flags().StringArrayVar(&any, "any", nil, "List of policy rules")
	cmd.Flags().StringArrayVar(&all, "all", nil, "List of policy rules")
	return cmd
}

func parseRule(in string) *v2alpha1.Exception {
	parts := strings.Split(in, ",")
	if len(parts) < 2 {
		return nil
	}
	return &v2alpha1.Exception{
		PolicyName: parts[0],
		RuleNames:  parts[1:],
	}
}

func parseResourceFilter(in string) *kyvernov1.ResourceFilter {
	parts := strings.Split(in, ",")
	if len(parts) == 0 {
		return nil
	}
	var result kyvernov1.ResourceFilter
	for _, part := range parts {
		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			return nil
		}
		switch kv[0] {
		case "kind":
			result.Kinds = append(result.Kinds, kv[1])
		case "name":
			result.Names = append(result.Names, kv[1])
		case "namespace":
			result.Namespaces = append(result.Namespaces, kv[1])
		case "operation":
			result.Operations = append(result.Operations, kyvernov1.AdmissionOperation(kv[1]))
		}
	}
	return &result
}

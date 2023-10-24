package test

import (
	"os"
	"strings"
	"text/template"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/templates"
	"github.com/spf13/cobra"
)

type result struct {
	Policy          string
	Rule            string
	Resource        string
	Namespace       string
	Kind            string
	PatchedResource string
	Result          string
}

type options struct {
	Name      string
	Policies  []string
	Resources []string
	Values    string
	Results   []*result
}

func Command() *cobra.Command {
	var path string
	var options options
	var pass, fail, skip []string
	cmd := &cobra.Command{
		Use:          "test",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tmpl, err := template.New("test").Parse(templates.TestTemplate)
			if err != nil {
				return err
			}
			for _, result := range pass {
				result := parseResult(result, "pass")
				if result != nil {
					options.Results = append(options.Results, result)
				}
			}
			for _, result := range fail {
				result := parseResult(result, "fail")
				if result != nil {
					options.Results = append(options.Results, result)
				}
			}
			for _, result := range skip {
				result := parseResult(result, "skip")
				if result != nil {
					options.Results = append(options.Results, result)
				}
			}
			output := cmd.OutOrStdout()
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
	cmd.Flags().StringVarP(&options.Name, "name", "n", "test-name", "Test name")
	cmd.Flags().StringSliceVarP(&options.Policies, "policy", "p", nil, "List of policy files")
	cmd.Flags().StringSliceVarP(&options.Resources, "resource", "r", nil, "List of resource files")
	cmd.Flags().StringVarP(&options.Values, "values", "f", "", "Values file")
	cmd.Flags().StringArrayVar(&pass, "pass", nil, "Expected `pass` results")
	cmd.Flags().StringArrayVar(&fail, "fail", nil, "Expected `fail` results")
	cmd.Flags().StringArrayVar(&skip, "skip", nil, "Expected `skip` results")
	return cmd
}

func parseResult(test string, status string) *result {
	parts := strings.Split(test, ",")
	if len(parts) == 5 {
		return &result{
			Policy:    parts[0],
			Rule:      parts[1],
			Resource:  parts[2],
			Namespace: parts[3],
			Kind:      parts[4],
			Result:    status,
		}
	} else if len(parts) == 6 {
		return &result{
			Policy:          parts[0],
			Rule:            parts[1],
			Resource:        parts[2],
			Namespace:       parts[3],
			Kind:            parts[4],
			PatchedResource: parts[5],
			Result:          status,
		}
	}
	return nil
}

package values

import (
	"os"
	"strings"
	"text/template"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/create/templates"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var path string
	var globalValues, namespaceSelector, rules, resources []string
	cmd := &cobra.Command{
		Use:     "values",
		Short:   "Create a Kyverno values file.",
		Example: "kyverno create values -g request.mode=dev -n prod,env=prod --rule policy,rule,env=demo --resource policy,resource,env=demo",
		RunE: func(cmd *cobra.Command, args []string) error {
			tmpl, err := template.New("values").Parse(templates.ValuesTemplate)
			if err != nil {
				return err
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
			values := api.Values{
				GlobalValues: map[string]string{},
			}
			for _, result := range namespaceSelector {
				result := parseNamespaceSelector(result)
				if result != nil {
					values.NamespaceSelectors = append(values.NamespaceSelectors, *result)
				}
			}
			for _, result := range globalValues {
				k, v := parseKeyValue(result)
				if k != "" && v != "" {
					values.GlobalValues[k] = v
				}
			}
			for _, result := range rules {
				result := parseRule(result)
				if result != nil {
					values.Policies = append(values.Policies, *result)
				}
			}
			for _, result := range resources {
				result := parseResource(result)
				if result != nil {
					values.Policies = append(values.Policies, *result)
				}
			}
			return tmpl.Execute(output, values)
		},
	}
	cmd.Flags().StringVarP(&path, "output", "o", "", "Output path (uses standard console output if not set)")
	cmd.Flags().StringArrayVarP(&namespaceSelector, "ns-selector", "n", nil, "Namespace selector")
	cmd.Flags().StringSliceVarP(&globalValues, "global", "g", nil, "Global value")
	cmd.Flags().StringArrayVar(&rules, "rule", nil, "Policy rule values")
	cmd.Flags().StringArrayVar(&resources, "resource", nil, "Policy resource values")
	return cmd
}

func parseNamespaceSelector(in string) *api.NamespaceSelector {
	parts := strings.Split(in, ",")
	if len(parts) < 2 {
		return nil
	}
	nsSelector := api.NamespaceSelector{
		Name:   parts[0],
		Labels: map[string]string{},
	}
	for _, label := range parts[1:] {
		k, v := parseKeyValue(label)
		if k != "" && v != "" {
			nsSelector.Labels[k] = v
		}
	}
	return &nsSelector
}

func parseKeyValue(in string) (string, string) {
	parts := strings.Split(in, "=")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func parseRule(in string) *api.Policy {
	parts := strings.Split(in, ",")
	if len(parts) < 2 {
		return nil
	}
	rule := api.Rule{
		Name:   parts[1],
		Values: map[string]interface{}{},
	}
	for _, value := range parts[2:] {
		k, v := parseKeyValue(value)
		if k != "" && v != "" {
			rule.Values[k] = v
		}
	}
	return &api.Policy{
		Name:  parts[0],
		Rules: []api.Rule{rule},
	}
}

func parseResource(in string) *api.Policy {
	parts := strings.Split(in, ",")
	if len(parts) < 2 {
		return nil
	}
	resource := api.Resource{
		Name:   parts[1],
		Values: map[string]interface{}{},
	}
	for _, value := range parts[2:] {
		k, v := parseKeyValue(value)
		if k != "" && v != "" {
			resource.Values[k] = v
		}
	}
	return &api.Policy{
		Name:      parts[0],
		Resources: []api.Resource{resource},
	}
}

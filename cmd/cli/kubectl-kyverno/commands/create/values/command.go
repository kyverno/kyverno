package values

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/templates"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var path string
	var globalValues, namespaceSelector, rules, resources []string
	cmd := &cobra.Command{
		Use:          "values",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tmpl, err := template.New("values").Parse(templates.ValuesTemplate)
			if err != nil {
				return err
			}
			values := v1alpha1.ValuesSpec{}
			values.GlobalValues = map[string]interface{}{}

			nsSelectors := map[string]map[string]string{}
			var nsOrder []string
			for _, result := range namespaceSelector {
				name, labels, err := parseNamespaceSelector(result)
				if err != nil {
					return err
				}
				if _, exists := nsSelectors[name]; !exists {
					nsSelectors[name] = map[string]string{}
					nsOrder = append(nsOrder, name)
				}
				for k, v := range labels {
					nsSelectors[name][k] = v
				}
			}
			for _, name := range nsOrder {
				values.NamespaceSelectors = append(values.NamespaceSelectors, v1alpha1.NamespaceSelector{
					Name:   name,
					Labels: nsSelectors[name],
				})
			}

			for _, result := range globalValues {
				k, v, err := parseKeyValue(result)
				if err != nil {
					return fmt.Errorf("invalid global value: %w", err)
				}
				values.GlobalValues[k] = v
			}

			type policyData struct {
				rules     []v1alpha1.Rule
				resources []v1alpha1.Resource
			}
			policiesMap := map[string]*policyData{}
			var policyOrder []string

			getOrCreatePolicy := func(name string) *policyData {
				if pd, ok := policiesMap[name]; ok {
					return pd
				}
				pd := &policyData{}
				policiesMap[name] = pd
				policyOrder = append(policyOrder, name)
				return pd
			}

			for _, result := range rules {
				policyName, rule, err := parseRule(result)
				if err != nil {
					return err
				}
				pd := getOrCreatePolicy(policyName)
				found := false
				for i, r := range pd.rules {
					if r.Name == rule.Name {
						for k, v := range rule.Values {
							pd.rules[i].Values[k] = v
						}
						found = true
						break
					}
				}
				if !found {
					pd.rules = append(pd.rules, rule)
				}
			}

			for _, result := range resources {
				policyName, resource, err := parseResource(result)
				if err != nil {
					return err
				}
				pd := getOrCreatePolicy(policyName)
				found := false
				for i, r := range pd.resources {
					if r.Name == resource.Name {
						for k, v := range resource.Values {
							pd.resources[i].Values[k] = v
						}
						found = true
						break
					}
				}
				if !found {
					pd.resources = append(pd.resources, resource)
				}
			}

			for _, name := range policyOrder {
				pd := policiesMap[name]
				values.Policies = append(values.Policies, v1alpha1.Policy{
					Name:      name,
					Rules:     pd.rules,
					Resources: pd.resources,
				})
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

func parseNamespaceSelector(in string) (string, map[string]string, error) {
	parts := strings.Split(in, ",")
	if len(parts) < 2 || parts[0] == "" {
		return "", nil, fmt.Errorf("invalid namespace selector %q: expected format <namespace>,<key>=<value>[,<key>=<value>...]", in)
	}
	labels := map[string]string{}
	for _, label := range parts[1:] {
		k, v, err := parseKeyValue(label)
		if err != nil {
			return "", nil, fmt.Errorf("invalid label in namespace selector %q: %w", in, err)
		}
		labels[k] = v
	}
	return parts[0], labels, nil
}

func parseKeyValue(in string) (string, string, error) {
	parts := strings.SplitN(in, "=", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid key=value pair %q: expected format <key>=<value>", in)
	}
	return parts[0], parts[1], nil
}

func parseRule(in string) (string, v1alpha1.Rule, error) {
	parts := strings.Split(in, ",")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", v1alpha1.Rule{}, fmt.Errorf("invalid rule %q: expected format <policy>,<rule>[,<key>=<value>...]", in)
	}
	rule := v1alpha1.Rule{
		Name:   parts[1],
		Values: map[string]interface{}{},
	}
	for _, value := range parts[2:] {
		k, v, err := parseKeyValue(value)
		if err != nil {
			return "", v1alpha1.Rule{}, fmt.Errorf("invalid value in rule %q: %w", in, err)
		}
		rule.Values[k] = v
	}
	return parts[0], rule, nil
}

func parseResource(in string) (string, v1alpha1.Resource, error) {
	parts := strings.Split(in, ",")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", v1alpha1.Resource{}, fmt.Errorf("invalid resource %q: expected format <policy>,<resource>[,<key>=<value>...]", in)
	}
	resource := v1alpha1.Resource{
		Name:   parts[1],
		Values: map[string]interface{}{},
	}
	for _, value := range parts[2:] {
		k, v, err := parseKeyValue(value)
		if err != nil {
			return "", v1alpha1.Resource{}, fmt.Errorf("invalid value in resource %q: %w", in, err)
		}
		resource.Values[k] = v
	}
	return parts[0], resource, nil
}

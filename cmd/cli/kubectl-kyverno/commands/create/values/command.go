package values

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/templates"
	"github.com/spf13/cobra"
)

// Command returns a cobra.Command for creating a Kyverno values file.
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
			values := v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{},
			}
			for _, result := range namespaceSelector {
				if err := addNamespaceSelector(&values, result); err != nil {
					return err
				}
			}
			for _, result := range globalValues {
				k, v, err := parseKeyValue(result)
				if err != nil {
					return err
				}
				values.GlobalValues[k] = v
			}
			for _, result := range rules {
				if err := addRule(&values, result); err != nil {
					return err
				}
			}
			for _, result := range resources {
				if err := addResource(&values, result); err != nil {
					return err
				}
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, values); err != nil {
				return err
			}

			output := cmd.OutOrStdout()
			if path != "" {
				file, err := os.Create(path) // #nosec G304
				if err != nil {
					return err
				}
				defer file.Close()
				output = file
			}
			_, err = output.Write(buf.Bytes())
			return err
		},
	}
	cmd.Flags().StringVarP(&path, "output", "o", "", "Output path (uses standard console output if not set)")
	cmd.Flags().StringArrayVarP(&namespaceSelector, "ns-selector", "n", nil, "Namespace selector")
	cmd.Flags().StringSliceVarP(&globalValues, "global", "g", nil, "Global value")
	cmd.Flags().StringArrayVar(&rules, "rule", nil, "Policy rule values")
	cmd.Flags().StringArrayVar(&resources, "resource", nil, "Policy resource values")
	return cmd
}

// formatValue formats a value for safe YAML template serialization.
// Empty values are represented as explicit double-quoted strings ("\"\"").
// Values containing YAML-sensitive sequences (such as inline comments " #" or "\t#",
// leading characters "#", "*", "&", mapping indicators ": ", line breaks,
// or leading/trailing whitespace) are double-quoted so that YAML parsers retain
// the full literal value while preserving native scalar typing.
func formatValue(val string) string {
	if val == "" {
		return `""`
	}
	if strings.Contains(val, " #") ||
		strings.Contains(val, "\t#") ||
		strings.HasPrefix(val, "#") ||
		strings.HasPrefix(val, "*") ||
		strings.HasPrefix(val, "&") ||
		strings.Contains(val, ": ") ||
		strings.ContainsAny(val, "\r\n") ||
		strings.TrimSpace(val) != val {
		return fmt.Sprintf("%q", val)
	}
	return val
}

// parseKeyValue parses a key=value string, preserving any '=' in the value.
// Empty values and YAML-sensitive values are safely quoted to prevent them
// from being parsed as null, comments, or mappings.
func parseKeyValue(in string) (string, string, error) {
	parts := strings.SplitN(in, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", fmt.Errorf("invalid key=value pair: %s", in)
	}
	key := strings.TrimSpace(parts[0])
	val := formatValue(parts[1])
	return key, val, nil
}

// addNamespaceSelector parses and merges namespace selector labels under the given namespace name.
func addNamespaceSelector(spec *v1alpha1.ValuesSpec, in string) error {
	parts := strings.Split(in, ",")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
		return fmt.Errorf("invalid namespace selector format: %s (expected <name>,<key>=<value>...)", in)
	}
	nsName := strings.TrimSpace(parts[0])

	var nsSelector *v1alpha1.NamespaceSelector
	for i := range spec.NamespaceSelectors {
		if spec.NamespaceSelectors[i].Name == nsName {
			nsSelector = &spec.NamespaceSelectors[i]
			break
		}
	}
	if nsSelector == nil {
		spec.NamespaceSelectors = append(spec.NamespaceSelectors, v1alpha1.NamespaceSelector{
			Name:   nsName,
			Labels: map[string]string{},
		})
		nsSelector = &spec.NamespaceSelectors[len(spec.NamespaceSelectors)-1]
	}
	if nsSelector.Labels == nil {
		nsSelector.Labels = map[string]string{}
	}

	for _, label := range parts[1:] {
		k, v, err := parseKeyValue(label)
		if err != nil {
			return err
		}
		nsSelector.Labels[k] = v
	}
	return nil
}

// getOrCreatePolicy finds an existing policy in spec by name, or creates a new one.
func getOrCreatePolicy(spec *v1alpha1.ValuesSpec, name string) *v1alpha1.Policy {
	for i := range spec.Policies {
		if spec.Policies[i].Name == name {
			return &spec.Policies[i]
		}
	}
	spec.Policies = append(spec.Policies, v1alpha1.Policy{
		Name: name,
	})
	return &spec.Policies[len(spec.Policies)-1]
}

// addRule parses and merges a rule and its values under the specified policy.
func addRule(spec *v1alpha1.ValuesSpec, in string) error {
	parts := strings.Split(in, ",")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("invalid rule format: %s (expected <policy>,<rule>,<key>=<value>...)", in)
	}
	policyName := strings.TrimSpace(parts[0])
	ruleName := strings.TrimSpace(parts[1])

	policy := getOrCreatePolicy(spec, policyName)

	var rule *v1alpha1.Rule
	for i := range policy.Rules {
		if policy.Rules[i].Name == ruleName {
			rule = &policy.Rules[i]
			break
		}
	}
	if rule == nil {
		policy.Rules = append(policy.Rules, v1alpha1.Rule{
			Name:   ruleName,
			Values: map[string]interface{}{},
		})
		rule = &policy.Rules[len(policy.Rules)-1]
	}
	if rule.Values == nil {
		rule.Values = map[string]interface{}{}
	}

	for _, value := range parts[2:] {
		k, v, err := parseKeyValue(value)
		if err != nil {
			return err
		}
		rule.Values[k] = v
	}
	return nil
}

// addResource parses and merges a resource and its values under the specified policy.
func addResource(spec *v1alpha1.ValuesSpec, in string) error {
	parts := strings.Split(in, ",")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("invalid resource format: %s (expected <policy>,<resource>,<key>=<value>...)", in)
	}
	policyName := strings.TrimSpace(parts[0])
	resourceName := strings.TrimSpace(parts[1])

	policy := getOrCreatePolicy(spec, policyName)

	var resource *v1alpha1.Resource
	for i := range policy.Resources {
		if policy.Resources[i].Name == resourceName {
			resource = &policy.Resources[i]
			break
		}
	}
	if resource == nil {
		policy.Resources = append(policy.Resources, v1alpha1.Resource{
			Name:   resourceName,
			Values: map[string]interface{}{},
		})
		resource = &policy.Resources[len(policy.Resources)-1]
	}
	if resource.Values == nil {
		resource.Values = map[string]interface{}{}
	}

	for _, value := range parts[2:] {
		k, v, err := parseKeyValue(value)
		if err != nil {
			return err
		}
		resource.Values[k] = v
	}
	return nil
}

package values

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/templates"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
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

			if path != "" {
				f, err := os.Create(path)
				if err != nil {
					return err
				}
				_, writeErr := f.Write(buf.Bytes())
				closeErr := f.Close()
				if writeErr != nil {
					return writeErr
				}
				return closeErr
			}
			_, err = cmd.OutOrStdout().Write(buf.Bytes())
			return err
		},
	}
	cmd.Flags().StringVarP(&path, "output", "o", "", "Output path (uses standard console output if not set)")
	cmd.Flags().StringArrayVarP(&namespaceSelector, "ns-selector", "n", nil, "Namespace selector")
	cmd.Flags().StringArrayVarP(&globalValues, "global", "g", nil, "Global value")
	cmd.Flags().StringArrayVar(&rules, "rule", nil, "Policy rule values")
	cmd.Flags().StringArrayVar(&resources, "resource", nil, "Policy resource values")
	return cmd
}

// formatValue formats a value for safe YAML template serialization.
// Native typed scalars (true, false, null, and numbers) are preserved as raw scalars.
// Empty string is represented as explicit `""`.
// Strings with line breaks are double-quoted (%q) to keep single-line scalar formatting.
// All other string values are formatted via a YAML encoder so that quotes, colons,
// tabs, and special characters are safely escaped and quoted.
func formatValue(val string) (string, error) {
	if val == "" {
		return `""`, nil
	}
	if val == "true" || val == "false" || val == "null" {
		return val, nil
	}
	if _, err := strconv.ParseInt(val, 10, 64); err == nil {
		return val, nil
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
		return val, nil
	}
	if strings.ContainsAny(val, "\r\n") {
		return fmt.Sprintf("%q", val), nil
	}

	b, err := yaml.Marshal(val)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// formatKey formats a key for safe YAML template serialization.
func formatKey(key string) (string, error) {
	if !utf8.ValidString(key) {
		return "", fmt.Errorf("invalid UTF-8: %s", key)
	}
	if strings.ContainsAny(key, "\r\n") {
		return fmt.Sprintf("%q", key), nil
	}
	b, err := yaml.Marshal(key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// parseKeyValue parses a key=value string, preserving any '=' in the value.
// Native typed scalars are preserved, while strings and keys are formatted
// via a YAML encoder to ensure valid YAML serialization.
func parseKeyValue(in string) (string, string, error) {
	if !utf8.ValidString(in) {
		return "", "", fmt.Errorf("invalid UTF-8 in key=value pair: %s", in)
	}
	parts := strings.SplitN(in, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", fmt.Errorf("invalid key=value pair: %s", in)
	}
	k, err := formatKey(strings.TrimSpace(parts[0]))
	if err != nil {
		return "", "", fmt.Errorf("invalid key: %w", err)
	}
	v, err := formatValue(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid value: %w", err)
	}
	return k, v, nil
}

// parseLabelKeyValue parses a key=value label pair, preserving any '=' in the value,
// and always formats the value as an explicit YAML double-quoted string. This ensures
// compatibility with Kubernetes namespace labels (map[string]string), preventing YAML
// unmarshal errors when values are boolean, numeric, or contain special characters.
func parseLabelKeyValue(in string) (string, string, error) {
	if !utf8.ValidString(in) {
		return "", "", fmt.Errorf("invalid UTF-8 in key=value pair: %s", in)
	}
	parts := strings.SplitN(in, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", fmt.Errorf("invalid key=value pair: %s", in)
	}
	k, err := formatKey(strings.TrimSpace(parts[0]))
	if err != nil {
		return "", "", fmt.Errorf("invalid key: %w", err)
	}
	val := fmt.Sprintf("%q", parts[1])
	return k, val, nil
}

// addNamespaceSelector parses and merges namespace selector labels under the given namespace name.
func addNamespaceSelector(spec *v1alpha1.ValuesSpec, in string) error {
	parts := strings.Split(in, ",")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
		return fmt.Errorf("invalid namespace selector format: %s (expected <name>,<key>=<value>...)", in)
	}
	nsName, err := formatKey(strings.TrimSpace(parts[0]))
	if err != nil {
		return fmt.Errorf("invalid namespace name: %w", err)
	}

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
		k, v, err := parseLabelKeyValue(label)
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
	policyName, err := formatKey(strings.TrimSpace(parts[0]))
	if err != nil {
		return fmt.Errorf("invalid policy name: %w", err)
	}
	ruleName, err := formatKey(strings.TrimSpace(parts[1]))
	if err != nil {
		return fmt.Errorf("invalid rule name: %w", err)
	}

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
	policyName, err := formatKey(strings.TrimSpace(parts[0]))
	if err != nil {
		return fmt.Errorf("invalid policy name: %w", err)
	}
	resourceName, err := formatKey(strings.TrimSpace(parts[1]))
	if err != nil {
		return fmt.Errorf("invalid resource name: %w", err)
	}

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

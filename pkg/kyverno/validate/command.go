package validate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	policyvalidate "github.com/nirmata/kyverno/pkg/engine/policy"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "Validates kyverno policies",
		Example: "kyverno validate /path/to/policy1 /path/to/policy2",
		RunE: func(cmd *cobra.Command, policyPaths []string) error {
			for _, policyPath := range policyPaths {
				policy, err := getPolicy(policyPath)
				if err != nil {
					return err
				}

				err = policyvalidate.Validate(*policy)
				if err != nil {
					return err
				}

				fmt.Println("Policy " + policy.Name + " is valid")
			}

			return nil
		},
	}

	return cmd
}

func getPolicy(path string) (*v1.ClusterPolicy, error) {
	policy := &v1.ClusterPolicy{}

	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load file: %v", err)
	}

	policyBytes, err := yaml.ToJSON(file)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(policyBytes, policy); err != nil {
		return nil, fmt.Errorf("failed to decode policy %s, err: %v", policy.Name, err)
	}

	if policy.TypeMeta.Kind != "ClusterPolicy" {
		return nil, fmt.Errorf("failed to parse policy")
	}

	return policy, nil
}

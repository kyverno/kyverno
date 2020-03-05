package validate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/nirmata/kyverno/pkg/kyverno/sanitizedError"

	"github.com/golang/glog"

	policyvalidate "github.com/nirmata/kyverno/pkg/policy"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "Validates kyverno policies",
		Example: "kyverno validate /path/to/policy.yaml /path/to/folderOfPolicies",
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {
			defer func() {
				if err != nil {
					if !sanitizedError.IsErrorSanitized(err) {
						glog.V(4).Info(err)
						err = fmt.Errorf("Internal error")
					}
				}
			}()

			policies, err := getPolicies(policyPaths)
			if err != nil {
				if !sanitizedError.IsErrorSanitized(err) {
					return sanitizedError.New("Could not parse policy paths")
				} else {
					return err
				}
			}

			for _, policy := range policies {
				err = policyvalidate.Validate(*policy)
				if err != nil {
					fmt.Println("Policy " + policy.Name + " is invalid")
				} else {
					fmt.Println("Policy " + policy.Name + " is valid")
				}
			}

			return nil
		},
	}

	return cmd
}

func getPoliciesInDir(path string) ([]*v1.ClusterPolicy, error) {
	var policies []*v1.ClusterPolicy

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			policiesFromDir, err := getPoliciesInDir(filepath.Join(path, file.Name()))
			if err != nil {
				return nil, err
			}

			policies = append(policies, policiesFromDir...)
		} else {
			policy, err := getPolicy(filepath.Join(path, file.Name()))
			if err != nil {
				return nil, err
			}

			policies = append(policies, policy)
		}
	}

	return policies, nil
}

func getPolicies(paths []string) ([]*v1.ClusterPolicy, error) {
	var policies = make([]*v1.ClusterPolicy, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)

		fileDesc, err := os.Stat(path)
		if err != nil {
			return nil, err
		}

		if fileDesc.IsDir() {
			policiesFromDir, err := getPoliciesInDir(path)
			if err != nil {
				return nil, err
			}

			policies = append(policies, policiesFromDir...)
		} else {
			policy, err := getPolicy(path)
			if err != nil {
				return nil, err
			}

			policies = append(policies, policy)
		}
	}

	for i := range policies {
		setFalse := false
		policies[i].Spec.Background = &setFalse
	}

	return policies, nil
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
		return nil, sanitizedError.New(fmt.Sprintf("failed to decode policy in %s", path))
	}

	if policy.TypeMeta.Kind != "ClusterPolicy" {
		return nil, sanitizedError.New(fmt.Sprintf("resource %v is not a cluster policy", policy.Name))
	}

	return policy, nil
}

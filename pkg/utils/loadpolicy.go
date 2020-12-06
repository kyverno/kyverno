package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// GetPolicy - extracts policies from YAML bytes
func GetPolicy(bytes []byte) (clusterPolicies []*v1.ClusterPolicy, errors []error) {
	policies, err := SplitYAMLDocuments(bytes)
	if err != nil {
		errors = append(errors, err)
		return clusterPolicies, errors
	}

	for _, thisPolicyBytes := range policies {
		policyBytes, err := yaml.ToJSON(thisPolicyBytes)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to convert json. error: %v", err))
			continue
		}

		policy := &v1.ClusterPolicy{}
		if err := json.Unmarshal(policyBytes, policy); err != nil {
			errors = append(errors, fmt.Errorf("failed to decode policy. error: %v", err))
			continue
		}

		if !(policy.TypeMeta.Kind == "ClusterPolicy" || policy.TypeMeta.Kind == "Policy") {
			msg := fmt.Sprintf("resource %s/%s is not a Policy or a ClusterPolicy", policy.Kind, policy.Name)
			errors = append(errors, fmt.Errorf(msg))
			continue
		}

		clusterPolicies = append(clusterPolicies, policy)
	}

	return clusterPolicies, errors
}

// SplitYAMLDocuments reads the YAML bytes per-document, unmarshals the TypeMeta information from each document
// and returns a map between the GroupVersionKind of the document and the document bytes
func SplitYAMLDocuments(yamlBytes []byte) (policies [][]byte, error error) {
	buf := bytes.NewBuffer(yamlBytes)
	reader := yaml.NewYAMLReader(bufio.NewReader(buf))
	for {
		// Read one YAML document at a time, until io.EOF is returned
		b, err := reader.Read()
		if err == io.EOF || len(b) == 0 {
			break
		} else if err != nil {
			return policies, fmt.Errorf("unable to read yaml")
		}

		policies = append(policies, b)
	}
	return policies, nil
}

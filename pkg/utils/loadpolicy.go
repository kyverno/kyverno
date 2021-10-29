package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GetPolicy - extracts policies from YAML bytes
func GetPolicy(bytes []byte) (clusterPolicies []*v1.ClusterPolicy, err error) {
	policies, err := SplitYAMLDocuments(bytes)
	if err != nil {
		return nil, err
	}

	for _, thisPolicyBytes := range policies {
		policyBytes, err := yaml.ToJSON(thisPolicyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to JSON: %v", err)
		}

		policy := &v1.ClusterPolicy{}
		if err := json.Unmarshal(policyBytes, policy); err != nil {
			return nil, fmt.Errorf("failed to decode policy: %v", err)
		}

		if policy.TypeMeta.Kind == "" {
			log.Log.V(3).Info("skipping file as policy.TypeMeta.Kind not found")
			continue
		}

		if !(policy.TypeMeta.Kind == "ClusterPolicy" || policy.TypeMeta.Kind == "Policy") {
			msg := fmt.Sprintf("resource %s/%s is not a Policy or a ClusterPolicy", policy.Kind, policy.Name)
			return nil, fmt.Errorf(msg)
		}

		if policy.Namespace != "" || (policy.Namespace == "" && policy.Kind == "Policy") {
			if policy.Namespace == "" {
				policy.Namespace = "default"
			}
			policy.Kind = "ClusterPolicy"
		}
		clusterPolicies = append(clusterPolicies, policy)
	}

	return clusterPolicies, nil
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

package common

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/kyverno/sanitizedError"
	"github.com/nirmata/kyverno/pkg/openapi"
	"k8s.io/apimachinery/pkg/util/yaml"
)

//GetPolicies - Extracting the policies from multiple YAML
func GetPolicies(paths []string) ([]*v1.ClusterPolicy, error) {
	var policies []*v1.ClusterPolicy
	for _, path := range paths {
		path = filepath.Clean(path)

		fileDesc, err := os.Stat(path)
		if err != nil {
			return nil, err
		}

		if fileDesc.IsDir() {
			files, err := ioutil.ReadDir(path)
			if err != nil {
				return nil, err
			}

			listOfFiles := make([]string, 0)
			for _, file := range files {
				listOfFiles = append(listOfFiles, filepath.Join(path, file.Name()))
			}

			policiesFromDir, err := GetPolicies(listOfFiles)
			if err != nil {
				return nil, err
			}

			policies = append(policies, policiesFromDir...)
		} else {
			getPolicies, getErrors := GetPolicy(path)
			var errString string
			for _, err := range getErrors {
				if err != nil {
					errString = errString + err.Error() + "\n"
				}
			}

			if errString != "" {
				return nil, errors.New(errString)
			}

			for _, policy := range getPolicies {
				policies = append(policies, policy)
			}
		}
	}

	for i := range policies {
		setFalse := false
		policies[i].Spec.Background = &setFalse
	}

	return policies, nil
}

// GetPolicy - Extracts policies from a YAML
func GetPolicy(path string) ([]*v1.ClusterPolicy, []error) {
	clusterPolicies := make([]*v1.ClusterPolicy, 0)
	errors := make([]error, 0)

	file, err := ioutil.ReadFile(path)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to load file: %v", err))
		return clusterPolicies, errors
	}

	policies := SplitYAMLDocuments(file)

	for _, thisPolicyBytes := range policies {
		policyBytes, err := yaml.ToJSON(thisPolicyBytes)
		if err != nil {
			return clusterPolicies, errors
		}

		policy := &v1.ClusterPolicy{}
		if err := json.Unmarshal(policyBytes, policy); err != nil {
			errors = append(errors, sanitizedError.New(fmt.Sprintf("failed to decode policy in %s", path)))
			continue
		}

		if policy.TypeMeta.Kind != "ClusterPolicy" {
			errors = append(errors, sanitizedError.New(fmt.Sprintf("resource %v is not a cluster policy", policy.Name)))
			continue
		}
		clusterPolicies = append(clusterPolicies, policy)
	}

	return clusterPolicies, errors
}

// SplitYAMLDocuments reads the YAML bytes per-document, unmarshals the TypeMeta information from each document
// and returns a map between the GroupVersionKind of the document and the document bytes
func SplitYAMLDocuments(yamlBytes []byte) [][]byte {
	policies := make([][]byte, 0)
	buf := bytes.NewBuffer(yamlBytes)
	reader := yaml.NewYAMLReader(bufio.NewReader(buf))
	for {

		// Read one YAML document at a time, until io.EOF is returned
		b, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Println(err)
		}
		if len(b) == 0 {
			break
		}
		policies = append(policies, b)
	}
	return policies
}

//GetPoliciesValidation - validating policies
func GetPoliciesValidation(policyPaths []string) ([]*v1.ClusterPolicy, *openapi.Controller, error) {
	policies, err := GetPolicies(policyPaths)
	if err != nil {
		if !sanitizedError.IsErrorSanitized(err) {
			return nil, nil, sanitizedError.New("Could not parse policy paths")
		} else {
			return nil, nil, err
		}
	}

	openAPIController, err := openapi.NewOpenAPIController()
	if err != nil {
		return nil, nil, err
	}
	return policies, openAPIController, nil
}

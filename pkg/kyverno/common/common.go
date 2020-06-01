package common

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/kyverno/sanitizedError"
	"github.com/nirmata/kyverno/pkg/openapi"
	"k8s.io/apimachinery/pkg/util/yaml"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// GetPolicies - Extracting the policies from multiple YAML
func GetPolicies(paths []string) (policies []*v1.ClusterPolicy, error error) {
	log := log.Log
	for _, path := range paths {
		path = filepath.Clean(path)

		fileDesc, err := os.Stat(path)
		if err != nil {
			log.Error(err, "failed to describe file")
			return nil, err
		}

		if fileDesc.IsDir() {
			files, err := ioutil.ReadDir(path)
			if err != nil {
				return nil, sanitizedError.New(fmt.Sprintf("failed to parse %v", path))
			}

			listOfFiles := make([]string, 0)
			for _, file := range files {
				listOfFiles = append(listOfFiles, filepath.Join(path, file.Name()))
			}

			policiesFromDir, err := GetPolicies(listOfFiles)
			if err != nil {
				log.Error(err, fmt.Sprintf("failed to extract policies from %v", listOfFiles))
				return nil, sanitizedError.New(("failed to extract policies"))
			}

			policies = append(policies, policiesFromDir...)
		} else {
			getPolicies, getErrors := GetPolicy(path)
			var errString string
			for _, err := range getErrors {
				if err != nil {
					errString += err.Error() + "\n"
				}
			}

			if errString != "" {
				return nil, sanitizedError.New(("falied to extract policies"))
			}

			policies = append(policies, getPolicies...)
		}
	}

	for i := range policies {
		setFalse := false
		policies[i].Spec.Background = &setFalse
	}

	return policies, nil
}

// GetPolicy - Extracts policies from a YAML
func GetPolicy(path string) (clusterPolicies []*v1.ClusterPolicy, errors []error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		errors = append(errors, fmt.Errorf(fmt.Sprintf("failed to load file: %v. error: %v", path, err)))
		return clusterPolicies, errors
	}

	policies, splitDocErrors := SplitYAMLDocuments(file)
	if splitDocErrors != nil {
		errors = append(errors, splitDocErrors)
		return clusterPolicies, errors
	}

	for _, thisPolicyBytes := range policies {
		policyBytes, err := yaml.ToJSON(thisPolicyBytes)
		if err != nil {
			errors = append(errors, fmt.Errorf(fmt.Sprintf("failed to convert json. error: %v", err)))
			continue
		}

		policy := &v1.ClusterPolicy{}
		if err := json.Unmarshal(policyBytes, policy); err != nil {
			errors = append(errors, fmt.Errorf(fmt.Sprintf("failed to decode policy in %s. error: %v", path, err)))
			continue
		}

		if policy.TypeMeta.Kind != "ClusterPolicy" {
			errors = append(errors, fmt.Errorf(fmt.Sprintf("resource %v is not a cluster policy", policy.Name)))
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
	return policies, error
}

//GetPoliciesValidation - validating policies
func GetPoliciesValidation(policyPaths []string) ([]*v1.ClusterPolicy, *openapi.Controller, error) {
	policies, err := GetPolicies(policyPaths)
	if err != nil {
		if !sanitizedError.IsErrorSanitized(err) {
			return nil, nil, sanitizedError.New((fmt.Sprintf("failed to parse %v path/s.", policyPaths)))
		}
		return nil, nil, err
	}

	openAPIController, err := openapi.NewOpenAPIController()
	if err != nil {
		return nil, nil, err
	}

	return policies, openAPIController, nil
}

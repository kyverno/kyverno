package common

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
	"regexp"
	yaml_v2 "sigs.k8s.io/yaml"
	"strings"

	"errors"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/kyverno/sanitizedError"
	"github.com/nirmata/kyverno/pkg/openapi"
	"github.com/nirmata/kyverno/pkg/policymutation"
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
				return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to parse %v", path), err)
			}

			listOfFiles := make([]string, 0)
			for _, file := range files {
				listOfFiles = append(listOfFiles, filepath.Join(path, file.Name()))
			}

			policiesFromDir, err := GetPolicies(listOfFiles)
			if err != nil {
				log.Error(err, fmt.Sprintf("failed to extract policies from %v", listOfFiles))
				return nil, sanitizedError.NewWithError(("failed to extract policies"), err)
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
				fmt.Printf("failed to extract policies: %s\n", errString)
				os.Exit(2)
			}

			policies = append(policies, getPolicies...)
		}
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

	policies, err := SplitYAMLDocuments(file)
	if err != nil {
		errors = append(errors, err)
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
			return nil, nil, sanitizedError.NewWithError((fmt.Sprintf("failed to parse %v path/s.", policyPaths)), err)
		}
		return nil, nil, err
	}

	openAPIController, err := openapi.NewOpenAPIController()
	if err != nil {
		return nil, nil, err
	}

	return policies, openAPIController, nil
}

// PolicyHasVariables - check for variables in the policy
func PolicyHasVariables(policy v1.ClusterPolicy) bool {
	policyRaw, _ := json.Marshal(policy)
	regex := regexp.MustCompile(`\{\{[^{}]*\}\}`)
	return len(regex.FindAllStringSubmatch(string(policyRaw), -1)) > 0
}

// PolicyHasNonAllowedVariables - checks for non whitelisted variables in the policy
func PolicyHasNonAllowedVariables(policy v1.ClusterPolicy) bool {
	policyRaw, _ := json.Marshal(policy)

	allVarsRegex := regexp.MustCompile(`\{\{[^{}]*\}\}`)

	allowedList := []string{`request\.`, `serviceAccountName`, `serviceAccountNamespace`}
	regexStr := `\{\{(` + strings.Join(allowedList, "|") + `)[^{}]*\}\}`
	matchedVarsRegex := regexp.MustCompile(regexStr)

	if len(allVarsRegex.FindAllStringSubmatch(string(policyRaw), -1)) > len(matchedVarsRegex.FindAllStringSubmatch(string(policyRaw), -1)) {
		return true
	}
	return false
}

// MutatePolicy - applies mutation to a policy
func MutatePolicy(policy *v1.ClusterPolicy, logger logr.Logger) (*v1.ClusterPolicy, error) {
	patches, _ := policymutation.GenerateJSONPatchesForDefaults(policy, logger)

	if len(patches) == 0 {
		return policy, nil
	}

	type jsonPatch struct {
		Path  string      `json:"path"`
		Op    string      `json:"op"`
		Value interface{} `json:"value"`
	}

	var jsonPatches []jsonPatch
	err := json.Unmarshal(patches, &jsonPatches)
	if err != nil {
		return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to unmarshal patches for %s policy", policy.Name), err)
	}
	patch, err := jsonpatch.DecodePatch(patches)
	if err != nil {
		return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to decode patch for %s policy", policy.Name), err)
	}

	policyBytes, _ := json.Marshal(policy)
	if err != nil {
		return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to marshal %s policy", policy.Name), err)
	}
	modifiedPolicy, err := patch.Apply(policyBytes)
	if err != nil {
		return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to apply %s policy", policy.Name), err)
	}

	var p v1.ClusterPolicy
	err = json.Unmarshal(modifiedPolicy, &p)
	if err != nil {
		return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to unmarshal %s policy", policy.Name), err)
	}

	return &p, nil
}



// ConvertFileToUnstructed - converting file to unstructured
func ConvertFileToUnstructed(crdPaths []string) (crds []*unstructured.Unstructured, err error) {
	crds, err = GetCRDs(crdPaths)
	if err != nil {
		if !sanitizedError.IsErrorSanitized(err) {
			return nil, sanitizedError.NewWithError((fmt.Sprintf("failed to parse %v path/s.", crdPaths)), err)
		}
		return nil, err
	}
	return crds, nil
}

// GetCRDs - Extracting the crds from multiple YAML
func GetCRDs(paths []string) (unstructuredCrds []*unstructured.Unstructured, err error) {
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
				return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to parse %v", path), err)
			}

			listOfFiles := make([]string, 0)
			for _, file := range files {
				listOfFiles = append(listOfFiles, filepath.Join(path, file.Name()))
			}

			policiesFromDir, err := GetCRDs(listOfFiles)
			if err != nil {
				log.Error(err, fmt.Sprintf("failed to extract crds from %v", listOfFiles))
				return nil, sanitizedError.NewWithError(("failed to extract crds"), err)
			}

			unstructuredCrds = append(unstructuredCrds, policiesFromDir...)
		} else {
			getCRDs, err := GetCRD(path)
			if err != nil {
				fmt.Printf("failed to extract crds: %s\n", err)
			}

			unstructuredCrds = append(unstructuredCrds, getCRDs...)
		}
	}
	return nil, nil
}

// GetCRD - Extracts crds from a YAML
func GetCRD(path string) (unstructuredCrds []*unstructured.Unstructured, err error) {
	log := log.Log
	path = filepath.Clean(path)

	fileDesc, err := os.Stat(path)
	if err != nil {
		log.Error(err, "failed to describe file", "file", path)
		return nil, err
	}

	if fileDesc.IsDir() {
		return nil, errors.New("path should be a file")
	}

	yamlbytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Error(err, "failed to read file", "file", path)
		return nil, err
	}

	var u unstructured.Unstructured
	buf := bytes.NewBuffer(yamlbytes)
	reader := yaml.NewYAMLReader(bufio.NewReader(buf))

	for {
		// Read one YAML document at a time, until io.EOF is returned
		b, err := reader.Read()
		if err == io.EOF || len(b) == 0 {
			break
		} else if err != nil {
			log.Error(err, "unable to read yaml")
		}
		err = yaml_v2.Unmarshal(b, &u)
		if err != nil {
			log.Error(err, "failed to convert file into unstructured object", "file", path)
			return nil, err
		}
		unstructuredCrds = append(unstructuredCrds, &u)
	}
	return unstructuredCrds, nil
}

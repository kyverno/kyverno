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
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	yaml_v2 "sigs.k8s.io/yaml"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	"github.com/kyverno/kyverno/pkg/policymutation"
	"github.com/kyverno/kyverno/pkg/utils"
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
			file, err := ioutil.ReadFile(path)
			if err != nil {
				log.Error(err, fmt.Sprintf("failed to load file: %v", path))
				return nil, sanitizedError.NewWithError(("failed to load file"), err)
			}
			getPolicies, getErrors := utils.GetPolicy(file)
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

//GetPoliciesValidation - validating policies
func GetPoliciesValidation(policyPaths []string) ([]*v1.ClusterPolicy, error) {
	policies, err := GetPolicies(policyPaths)
	if err != nil {
		if !sanitizedError.IsErrorSanitized(err) {
			return nil, sanitizedError.NewWithError((fmt.Sprintf("failed to parse %v path/s.", policyPaths)), err)
		}
		return nil, err
	}
	return policies, nil
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
		// If rules contains Context then skip this validation
		for _, rule := range policy.Spec.Rules {
			if len(rule.Context) > 0 {
				return false
			}
		}
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

// GetCRDs - Extracting the crds from multiple YAML
func GetCRDs(paths []string) (unstructuredCrds []*unstructured.Unstructured, err error) {
	log := log.Log
	unstructuredCrds = make([]*unstructured.Unstructured, 0)
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
				os.Exit(2)
			}
			unstructuredCrds = append(unstructuredCrds, getCRDs...)
		}
	}
	return unstructuredCrds, nil
}

// GetCRD - Extracts crds from a YAML
func GetCRD(path string) (unstructuredCrds []*unstructured.Unstructured, err error) {
	log := log.Log
	path = filepath.Clean(path)
	unstructuredCrds = make([]*unstructured.Unstructured, 0)
	yamlbytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Error(err, "failed to read file", "file", path)
		return nil, err
	}

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
		var u unstructured.Unstructured
		err = yaml_v2.Unmarshal(b, &u)
		if err != nil {
			log.Error(err, "failed to convert file into unstructured object", "file", path)
			return nil, err
		}
		unstructuredCrds = append(unstructuredCrds, &u)
	}

	return unstructuredCrds, nil
}

// IsInputFromPipe - check if input is passed using pipe
func IsInputFromPipe() bool {
	fileInfo, _ := os.Stdin.Stat()
	return fileInfo.Mode()&os.ModeCharDevice == 0
}

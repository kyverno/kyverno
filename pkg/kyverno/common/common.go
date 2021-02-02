package common

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kyverno/kyverno/pkg/utils"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	sanitizederror "github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	"github.com/kyverno/kyverno/pkg/policymutation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	yaml_v2 "sigs.k8s.io/yaml"
)

// GetPolicies - Extracting the policies from multiple YAML
func GetPolicies(paths []string) (policies []*v1.ClusterPolicy, errors []error) {
	for _, path := range paths {
		log.Log.V(5).Info("reading policies", "path", path)

		var (
			fileDesc os.FileInfo
			err      error
		)

		isHttpPath := strings.Contains(path, "http")
		if !isHttpPath {
			path = filepath.Clean(path)
			fileDesc, err = os.Stat(path)
			if err != nil {
				errors = append(errors, err)
				continue
			}
		}

		if !isHttpPath && fileDesc.IsDir() {
			files, err := ioutil.ReadDir(path)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to read %v: %v", path, err.Error()))
				continue
			}

			listOfFiles := make([]string, 0)
			for _, file := range files {
				ext := filepath.Ext(file.Name())
				if ext == "" || ext == ".yaml" || ext == ".yml" {
					listOfFiles = append(listOfFiles, filepath.Join(path, file.Name()))
				}
			}

			policiesFromDir, errorsFromDir := GetPolicies(listOfFiles)
			errors = append(errors, errorsFromDir...)
			policies = append(policies, policiesFromDir...)

		} else {
			var fileBytes []byte
			if isHttpPath {
				resp, err := http.Get(path)
				if err != nil {
					fmt.Errorf("failed to process %s", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					errors = append(errors, fmt.Errorf("failed to process %v: %v", path, err.Error()))
					continue
				}

				fileBytes, err = ioutil.ReadAll(resp.Body)
				if err != nil {
					fmt.Errorf("failed to process %s", err)
				}

				if err != nil {
					errors = append(errors, fmt.Errorf("failed to read %v: %v", path, err.Error()))
					continue
				}
			} else {
				fileBytes, err = ioutil.ReadFile(path)
				if err != nil {
					errors = append(errors, fmt.Errorf("failed to read %v: %v", path, err.Error()))
					continue
				}
			}

			policiesFromFile, errFromFile := utils.GetPolicy(fileBytes)
			if errFromFile != nil {
				err := fmt.Errorf("failed to process %s: %v", path, errFromFile.Error())
				errors = append(errors, err)
				continue
			}

			policies = append(policies, policiesFromFile...)

		}
	}

	log.Log.V(3).Info("read policies", "policies", len(policies), "errors", len(errors))
	return policies, errors
}

// PolicyHasVariables - check for variables in the policy
func PolicyHasVariables(policy v1.ClusterPolicy) [][]string {
	policyRaw, _ := json.Marshal(policy)
	matches := RegexVariables.FindAllStringSubmatch(string(policyRaw), -1)
	return matches
}

// PolicyHasNonAllowedVariables - checks for unexpected variables in the policy
func PolicyHasNonAllowedVariables(policy v1.ClusterPolicy) bool {
	policyRaw, _ := json.Marshal(policy)

	matchesAll := RegexVariables.FindAllStringSubmatch(string(policyRaw), -1)
	matchesAllowed := AllowedVariables.FindAllStringSubmatch(string(policyRaw), -1)

	if len(matchesAll) > len(matchesAllowed) {
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
		return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to unmarshal patches for %s policy", policy.Name), err)
	}

	patch, err := jsonpatch.DecodePatch(patches)
	if err != nil {
		return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to decode patch for %s policy", policy.Name), err)
	}

	policyBytes, _ := json.Marshal(policy)
	if err != nil {
		return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to marshal %s policy", policy.Name), err)
	}
	modifiedPolicy, err := patch.Apply(policyBytes)
	if err != nil {
		return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to apply %s policy", policy.Name), err)
	}

	var p v1.ClusterPolicy
	err = json.Unmarshal(modifiedPolicy, &p)
	if err != nil {
		return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to unmarshal %s policy", policy.Name), err)
	}

	return &p, nil
}

// GetCRDs - Extracting the crds from multiple YAML
func GetCRDs(paths []string) (unstructuredCrds []*unstructured.Unstructured, err error) {
	unstructuredCrds = make([]*unstructured.Unstructured, 0)
	for _, path := range paths {
		path = filepath.Clean(path)

		fileDesc, err := os.Stat(path)
		if err != nil {
			return nil, err
		}

		if fileDesc.IsDir() {
			files, err := ioutil.ReadDir(path)
			if err != nil {
				return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to parse %v", path), err)
			}

			listOfFiles := make([]string, 0)
			for _, file := range files {
				listOfFiles = append(listOfFiles, filepath.Join(path, file.Name()))
			}

			policiesFromDir, err := GetCRDs(listOfFiles)
			if err != nil {
				return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to extract crds from %v", listOfFiles), err)
			}

			unstructuredCrds = append(unstructuredCrds, policiesFromDir...)
		} else {
			getCRDs, err := GetCRD(path)
			if err != nil {
				fmt.Printf("\nError: failed to extract crds from %s.  \nCause: %s\n", path, err)
				os.Exit(2)
			}
			unstructuredCrds = append(unstructuredCrds, getCRDs...)
		}
	}
	return unstructuredCrds, nil
}

// GetCRD - Extracts crds from a YAML
func GetCRD(path string) (unstructuredCrds []*unstructured.Unstructured, err error) {
	path = filepath.Clean(path)
	unstructuredCrds = make([]*unstructured.Unstructured, 0)
	yamlbytes, err := ioutil.ReadFile(path)
	if err != nil {
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
			fmt.Printf("\nError: unable to read crd from %s. Cause: %s\n", path, err)
			os.Exit(2)
		}
		var u unstructured.Unstructured
		err = yaml_v2.Unmarshal(b, &u)
		if err != nil {
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

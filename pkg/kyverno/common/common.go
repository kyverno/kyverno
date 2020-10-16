package common

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"path/filepath"
	yaml_v2 "sigs.k8s.io/yaml"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	"github.com/kyverno/kyverno/pkg/policymutation"
	"github.com/kyverno/kyverno/pkg/utils"
	client "github.com/kyverno/kyverno/pkg/dclient"
)

// GetPolicies - Extracting the policies from multiple YAML
func GetPolicies(paths []string,cluster bool, dClient *client.Client) (policies []*v1.ClusterPolicy, error error) {
	for _, path := range paths {

		fmt.Println("------------------ -3 ")

		path = filepath.Clean(path)
		fmt.Println("------------------ -2 ")

		fileDesc, err := os.Stat(path)
		if err != nil {
			fmt.Println(err)
			p, err := getPolicyFromCluster(path, cluster, dClient)

			if err != nil {
				return nil, err
			}
			policies = append(policies, p)
			continue
		}
		fmt.Println("------------------ -1 ")

		if fileDesc.IsDir() {
			fmt.Println("------------------ 0 ")
			files, err := ioutil.ReadDir(path)
			if err != nil {
				fmt.Println("------------------1")
				return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to parse %v", path), err)
			}

			fmt.Println("------------------2")
			listOfFiles := make([]string, 0)
			for _, file := range files {
				listOfFiles = append(listOfFiles, filepath.Join(path, file.Name()))
			}

			fmt.Println("------------------3")
			policiesFromDir, err := GetPolicies(listOfFiles, cluster, dClient)
			if err != nil {
				return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to extract policies from %v", listOfFiles), err)
			}
			fmt.Println("------------------4")

			policies = append(policies, policiesFromDir...)
		} else {
			fmt.Println("------------------5")
			file, err := ioutil.ReadFile(path)
			if err != nil {
				fmt.Println("------------------6")
				// check if cluster flag is passed and get the policy from cluster
				getPolicyFromCluster(path, cluster, dClient)
				return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to load file %v", path), err)
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

func getPolicyFromCluster(policyName string,cluster bool,  dClient *client.Client) (*v1.ClusterPolicy, error){
	fmt.Println("getPolicyFromCluster:", policyName, cluster, dClient)

	if !cluster {
		return &v1.ClusterPolicy{}, nil
	}
	//var policy []*unstructured.Unstructured
	policy, err := dClient.GetResource("", "ClusterPolicy", "", policyName, "")

	fmt.Println("********************* 1")
	fmt.Println(policy)

	policyBytes, err := json.Marshal(policy.Object)
	if err != nil {
		fmt.Println("********************* 2")
		fmt.Println(err)
		return &v1.ClusterPolicy{}, err
	}

	var p v1.ClusterPolicy
	err = json.Unmarshal(policyBytes, &p)

	if err != nil {
		fmt.Println("********************* 3")
		fmt.Println(err)
		return &v1.ClusterPolicy{}, err
	}

	if err != nil {
		fmt.Println("Error Occurred while fetching policy from cluster ", err)
	}

	//for _, kind := range resourceTypes {
	//	resourceList, err := dClient.ListResource("", kind, "", nil)
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//	version := resourceList.GetAPIVersion()
	//	for _, resource := range resourceList.Items {
	//		resource.SetGroupVersionKind(schema.GroupVersionKind{
	//			Group:   "",
	//			Version: version,
	//			Kind:    kind,
	//		})
	//		resources = append(resources, resource.DeepCopy())
	//	}
	//}

	return &p, nil
}

//ValidateAndGetPolicies - validating policies
func ValidateAndGetPolicies(policyPaths []string, cluster bool, dClient *client.Client) ([]*v1.ClusterPolicy, error) {
	policies, err := GetPolicies(policyPaths, cluster, dClient)
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
	matches := REGEX_VARIABLES.FindAllStringSubmatch(string(policyRaw), -1)
	return len(matches) > 0
}

// PolicyHasNonAllowedVariables - checks for unexpected variables in the policy
func PolicyHasNonAllowedVariables(policy v1.ClusterPolicy) bool {
	policyRaw, _ := json.Marshal(policy)

	matchesAll := REGEX_VARIABLES.FindAllStringSubmatch(string(policyRaw), -1)
	matchesAllowed := ALLOWED_VARIABLES.FindAllStringSubmatch(string(policyRaw), -1)

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
				return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to parse %v", path), err)
			}

			listOfFiles := make([]string, 0)
			for _, file := range files {
				listOfFiles = append(listOfFiles, filepath.Join(path, file.Name()))
			}

			policiesFromDir, err := GetCRDs(listOfFiles)
			if err != nil {
				return nil, sanitizedError.NewWithError(fmt.Sprintf("failed to extract crds from %v", listOfFiles), err)
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

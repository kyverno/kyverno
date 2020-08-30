package common

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	//openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"
	//"github.com/googleapis/gnostic/compiler"
	yaml_v2 "sigs.k8s.io/yaml"

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

func ValidatePolicyAgainstCrd(policy *v1.ClusterPolicy, path string) error {
	log := log.Log
	path = filepath.Clean(path)

	fileDesc, err := os.Stat(path)
	if err != nil {
		log.Error(err, "failed to describe crd file")
		return err
	}

	if fileDesc.IsDir() {
		return errors.New("crd path should be a file")
	}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Error(err, "failed to crd read file")
		return err
	}

	var crd unstructured.Unstructured
	err = yaml_v2.Unmarshal(bytes, &crd)

	if err != nil {
		return err
	}
	log.Info("coming till here .................. 5")

	// crdDefinitionPrior represents CRDs version prior to 1.16
	var crdDefinitionPrior struct {
		Spec struct {
			Names struct {
				Kind string `json:"kind"`
			} `json:"names"`
			Validation struct {
				OpenAPIV3Schema interface{} `json:"openAPIV3Schema"`
			} `json:"validation"`
		} `json:"spec"`
	}

	// crdDefinitionNew represents CRDs version 1.16+
	var crdDefinitionNew struct {
		Spec struct {
			Names struct {
				Kind string `json:"kind"`
			} `json:"names"`
			Versions []struct {
				Schema struct {
					OpenAPIV3Schema interface{} `json:"openAPIV3Schema"`
				} `json:"schema"`
				Storage bool `json:"storage"`
			} `json:"versions"`
		} `json:"spec"`
	}

	log.Info("coming till here .................. 6")
	crdRaw, _ := json.Marshal(crd.Object)
	_ = json.Unmarshal(crdRaw, &crdDefinitionPrior)

	log.Info("coming till here .................. 7")
	openV3schema := crdDefinitionPrior.Spec.Validation.OpenAPIV3Schema
	crdName := crdDefinitionPrior.Spec.Names.Kind
	fmt.Println(crdName)

	log.Info("coming till here .................. 8")

	if openV3schema == nil {
		log.Info("coming till here .................. coming into openV3Schema = nil")
		_ = json.Unmarshal(crdRaw, &crdDefinitionNew)
		for _, crdVersion := range crdDefinitionNew.Spec.Versions {
			if crdVersion.Storage {
				openV3schema = crdVersion.Schema.OpenAPIV3Schema
				crdName = crdDefinitionNew.Spec.Names.Kind
				break
			}
		}
	}

	log.Info("coming till here .................. 9")
	log.Info("crd", "openV3schema", openV3schema)

	schemaRaw, _ := json.Marshal(openV3schema)
	if len(schemaRaw) < 1 {
		//log.Log.V(3).Info("could not parse crd schema", "name", crdName)
		return err
	}
	log.Info("coming till here .................. 10")

	//schemaRaw, err = addingDefaultFieldsToSchema(schemaRaw)
	//if err != nil {
	//	//log.Log.Error(err, "could not parse crd schema", "name", crdName)
	//	//return err
	//}
	log.Info("coming till here .................. 11")

	schema := new(spec.Schema)
	_ = json.Unmarshal(schemaRaw, schema)


	// strfmt.Default is the registry of recognized formats
	err = validate.AgainstSchema(schema, policy, strfmt.Default)
	if err != nil {
		fmt.Printf("JSON does not validate against schema: %v", err)
	} else {
		fmt.Printf("OK")
	}
	log.Info("coming till here .................. 14")

	//var schema yaml_v2.MapSlice
	//_ = yaml_v2.Unmarshal(schemaRaw, &schema)
	//
	//parsedSchema, err := openapi_v2.NewSchema(schema, compiler.NewContext("schema", nil))
	//if err != nil {
	//	//log.Log.Error(err, "could not parse crd schema", "name", crdName)
	//	return
	//}



	//var spec yaml_v2.MapSlice
	//err := yaml_v2.Unmarshal([]byte(data.SwaggerDoc), &spec)
	//if err != nil {
	//	return err
	//}
	//
	//crdDoc, err := openapi_v2.NewDocument(spec, compiler.NewContext("$root", nil))
	//if err != nil {
	//	return err
	//}
	//
	//crdDoc

	return nil
}

// addingDefaultFieldsToSchema will add any default missing fields like apiVersion, metadata
func addingDefaultFieldsToSchema(schemaRaw []byte) ([]byte, error) {
	var schema struct {
		Properties map[string]interface{} `json:"properties"`
	}
	_ = json.Unmarshal(schemaRaw, &schema)

	if len(schema.Properties) < 1 {
		return nil, errors.New("crd schema has no properties")
	}

	if schema.Properties["apiVersion"] == nil {
		apiVersionDefRaw := `{"description":"APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources","type":"string"}`
		apiVersionDef := make(map[string]interface{})
		_ = json.Unmarshal([]byte(apiVersionDefRaw), &apiVersionDef)
		schema.Properties["apiVersion"] = apiVersionDef
	}

	if schema.Properties["metadata"] == nil {
		metadataDefRaw := `{"$ref":"#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta","description":"Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata"}`
		metadataDef := make(map[string]interface{})
		_ = json.Unmarshal([]byte(metadataDefRaw), &metadataDef)
		schema.Properties["metadata"] = metadataDef
	}

	schemaWithDefaultFields, _ := json.Marshal(schema)

	return schemaWithDefaultFields, nil
}

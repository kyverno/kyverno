package apply

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/policymutation"

	"github.com/nirmata/kyverno/pkg/utils"

	"github.com/nirmata/kyverno/pkg/kyverno/common"
	"github.com/nirmata/kyverno/pkg/kyverno/sanitizedError"

	policy2 "github.com/nirmata/kyverno/pkg/policy"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/nirmata/kyverno/pkg/engine"

	engineutils "github.com/nirmata/kyverno/pkg/engine/utils"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/spf13/cobra"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	jsonpatch "github.com/evanphx/json-patch"
)

func Command() *cobra.Command {
	var cmd *cobra.Command
	var resourcePaths []string
	var cluster bool
	var mutatelogPath string

	kubernetesConfig := genericclioptions.NewConfigFlags(true)

	cmd = &cobra.Command{
		Use:     "apply",
		Short:   "Applies policies on resources",
		Example: fmt.Sprintf("To apply on a resource:\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2\n\nTo apply on a cluster\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster"),
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {
			defer func() {
				if err != nil {
					if !sanitizedError.IsErrorSanitized(err) {
						log.Log.Error(err, "failed to sanitize")
						err = fmt.Errorf("Internal error")
					}
				}
			}()

			if len(resourcePaths) == 0 && !cluster {
				return sanitizedError.NewWithError("resource file or cluster required", err)
			}

			var mutatelogPathIsDir bool
			if mutatelogPath != "" {
				spath := strings.Split(mutatelogPath, "/")
				sfileName := strings.Split(spath[len(spath)-1], ".")
				if sfileName[len(sfileName)-1] == "yml" || sfileName[len(sfileName)-1] == "yaml" {
					mutatelogPathIsDir = false
				} else {
					mutatelogPathIsDir = true
				}

				err = createFileOrFolder(mutatelogPath, mutatelogPathIsDir)
				if err != nil {
					if !sanitizedError.IsErrorSanitized(err) {
						return sanitizedError.NewWithError("failed to create file/folder.", err)
					}
					return err
				}
			}

			policies, openAPIController, err := common.GetPoliciesValidation(policyPaths)
			if err != nil {
				if !sanitizedError.IsErrorSanitized(err) {
					return sanitizedError.NewWithError("failed to mutate policies.", err)
				}
				return err
			}

			for _, policy := range policies {
				err := policy2.Validate(utils.MarshalPolicy(*policy), nil, true, openAPIController)
				if err != nil {
					fmt.Printf("Policy %v is not valid: %v\n", policy.Name, err)
					os.Exit(1)
				}

				if common.PolicyHasVariables(*policy) {
					return sanitizedError.NewWithError(fmt.Sprintf("invalid policy %s. 'apply' does not support policies with variables", policy.Name), err)
				}
			}

			var dClient *client.Client
			if cluster {
				restConfig, err := kubernetesConfig.ToRESTConfig()
				if err != nil {
					return err
				}
				dClient, err = client.NewClient(restConfig, 5*time.Minute, make(chan struct{}), log.Log)
				if err != nil {
					return err
				}
			}

			resources, err := getResources(policies, resourcePaths, dClient)
			if err != nil {
				return sanitizedError.NewWithError("failed to load resources", err)
			}

			newPolicies, err := mutatePolicy(policies)
			if err != nil {
				return sanitizedError.NewWithError("failed to mutate policy", err)
			}

			for i, policy := range newPolicies {
				for j, resource := range resources {
					if !(j == 0 && i == 0) {
						fmt.Printf("\n\n==========================================================================================\n")
					}

					err = applyPolicyOnResource(policy, resource, mutatelogPath, mutatelogPathIsDir)
					if err != nil {
						return sanitizedError.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.Name, resource.GetName()).Error(), err)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringArrayVarP(&resourcePaths, "resource", "r", []string{}, "Path to resource files")
	cmd.Flags().BoolVarP(&cluster, "cluster", "c", false, "Checks if policies should be applied to cluster in the current context")
	cmd.Flags().StringVarP(&mutatelogPath, "output", "o", "", "Prints the mutated resources in provided file/directory")
	return cmd
}

func getResources(policies []*v1.ClusterPolicy, resourcePaths []string, dClient *client.Client) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	var err error

	if dClient != nil {
		var resourceTypesMap = make(map[string]bool)
		var resourceTypes []string
		for _, policy := range policies {
			for _, rule := range policy.Spec.Rules {
				for _, kind := range rule.MatchResources.Kinds {
					resourceTypesMap[kind] = true
				}
			}
		}

		for kind := range resourceTypesMap {
			resourceTypes = append(resourceTypes, kind)
		}

		resources, err = getResourcesOfTypeFromCluster(resourceTypes, dClient)
		if err != nil {
			return nil, err
		}
	}

	for _, resourcePath := range resourcePaths {
		getResources, err := getResource(resourcePath)
		if err != nil {
			return nil, err
		}

		for _, resource := range getResources {
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func getResourcesOfTypeFromCluster(resourceTypes []string, dClient *client.Client) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured

	for _, kind := range resourceTypes {
		resourceList, err := dClient.ListResource(kind, "", nil)
		if err != nil {
			return nil, err
		}

		version := resourceList.GetAPIVersion()
		for _, resource := range resourceList.Items {
			resource.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "",
				Version: version,
				Kind:    kind,
			})
			resources = append(resources, resource.DeepCopy())
		}
	}

	return resources, nil
}

func getResource(path string) ([]*unstructured.Unstructured, error) {

	resources := make([]*unstructured.Unstructured, 0)
	getResourceErrors := make([]error, 0)

	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	files, splitDocError := common.SplitYAMLDocuments(file)
	if splitDocError != nil {
		return nil, splitDocError
	}

	for _, resourceYaml := range files {

		decode := scheme.Codecs.UniversalDeserializer().Decode
		resourceObject, metaData, err := decode(resourceYaml, nil, nil)
		if err != nil {
			getResourceErrors = append(getResourceErrors, err)
			continue
		}

		resourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&resourceObject)
		if err != nil {
			getResourceErrors = append(getResourceErrors, err)
			continue
		}

		resourceJSON, err := json.Marshal(resourceUnstructured)
		if err != nil {
			getResourceErrors = append(getResourceErrors, err)
			continue
		}

		resource, err := engineutils.ConvertToUnstructured(resourceJSON)
		if err != nil {
			getResourceErrors = append(getResourceErrors, err)
			continue
		}

		resource.SetGroupVersionKind(*metaData)

		if resource.GetNamespace() == "" {
			resource.SetNamespace("default")
		}

		resources = append(resources, resource)
	}

	var getErrString string
	for _, getResourceError := range getResourceErrors {
		getErrString = getErrString + getResourceError.Error() + "\n"
	}

	if getErrString != "" {
		return nil, errors.New(getErrString)
	}

	return resources, nil
}

// applyPolicyOnResource - function to apply policy on resource
func applyPolicyOnResource(policy *v1.ClusterPolicy, resource *unstructured.Unstructured, mutatelogPath string, mutatelogPathIsDir bool) error {
	fmt.Printf("\n\nApplying Policy %s on Resource %s/%s/%s\n", policy.Name, resource.GetNamespace(), resource.GetKind(), resource.GetName())

	mutateResponse := engine.Mutate(engine.PolicyContext{Policy: *policy, NewResource: *resource})
	if !mutateResponse.IsSuccessful() {
		fmt.Printf("\n\nMutation:")
		fmt.Printf("\nFailed to apply mutation")
		for i, r := range mutateResponse.PolicyResponse.Rules {
			fmt.Printf("\n%d. %s", i+1, r.Message)
		}
		fmt.Printf("\n\n")
	} else {
		if len(mutateResponse.PolicyResponse.Rules) > 0 {
			yamlEncodedResource, err := yamlv2.Marshal(mutateResponse.PatchedResource.Object)
			if err != nil {
				return err
			}

			if mutatelogPath == "" {
				fmt.Printf("\n\nMutation:\nMutation has been applied succesfully")
				fmt.Printf("\n\n" + string(yamlEncodedResource))
				fmt.Printf("\n\n")
			} else {
				err := printMutatedOutput(mutatelogPath, mutatelogPathIsDir, string(yamlEncodedResource), resource.GetName()+"-mutated")
				if err != nil {
					return sanitizedError.NewWithError("failed to print mutated result", err)
				}
				fmt.Printf("\n\nMutation:\nMutation has been applied succesfully. Check the files.")
			}

		} else {
			fmt.Printf("\n\nMutation:\nMutation skipped. Resource not matches the policy")
		}
	}

	validateResponse := engine.Validate(engine.PolicyContext{Policy: *policy, NewResource: mutateResponse.PatchedResource})
	if !validateResponse.IsSuccessful() {
		fmt.Printf("\n\nValidation:")
		fmt.Printf("\nResource is invalid")
		for i, r := range validateResponse.PolicyResponse.Rules {
			fmt.Printf("\n%d. %s", i+1, r.Message)
		}
		fmt.Printf("\n\n")
	} else {
		if len(validateResponse.PolicyResponse.Rules) > 0 {
			fmt.Printf("\n\nValidation:")
			fmt.Printf("\nResource is valid")
			fmt.Printf("\n\n")
		}
	}

	var policyHasGenerate bool
	for _, rule := range policy.Spec.Rules {
		if rule.HasGenerate() {
			policyHasGenerate = true
		}
	}

	if policyHasGenerate {
		generateResponse := engine.Generate(engine.PolicyContext{Policy: *policy, NewResource: *resource})
		if len(generateResponse.PolicyResponse.Rules) > 0 {
			fmt.Printf("\n\nGenerate:")
			fmt.Printf("\nResource is valid")
			fmt.Printf("\n\n")
		} else {
			fmt.Printf("\n\nGenerate:")
			fmt.Printf("\nResource is invalid")
			for i, r := range generateResponse.PolicyResponse.Rules {
				fmt.Printf("\n%d. %s", i+1, r.Message)
			}
			fmt.Printf("\n\n")
		}
	}

	return nil
}

// mutatePolicy - function to apply mutation on policies
func mutatePolicy(policies []*v1.ClusterPolicy) ([]*v1.ClusterPolicy, error) {
	newPolicies := make([]*v1.ClusterPolicy, 0)
	logger := log.Log.WithName("apply")

	for _, policy := range policies {
		patches, _ := policymutation.GenerateJSONPatchesForDefaults(policy, logger)

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
		json.Unmarshal(modifiedPolicy, &p)
		newPolicies = append(newPolicies, &p)
	}
	return newPolicies, nil
}

// printMutatedOutput - function to print output in provided file or directory
func printMutatedOutput(mutatelogPath string, mutatelogPathIsDir bool, yaml string, fileName string) error {
	var f *os.File
	var err error
	yaml = yaml + ("\n---\n\n")

	if !mutatelogPathIsDir {
		f, err = os.OpenFile(mutatelogPath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		f, err = os.OpenFile(mutatelogPath+"/"+fileName+".yaml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	}

	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(yaml)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

// createFileOrFolder - creating file or folder accoring to path provided
func createFileOrFolder(mutatelogPath string, mutatelogPathIsDir bool) error {
	mutatelogPath = filepath.Clean(mutatelogPath)
	_, err := os.Stat(mutatelogPath)

	if err != nil {
		if os.IsNotExist(err) {
			if !mutatelogPathIsDir {
				// check the folder existance, then create the file
				var folderPath string
				s := strings.Split(mutatelogPath, "/")

				if len(s) > 1 {
					folderPath = mutatelogPath[:len(mutatelogPath)-len(s[len(s)-1])-1]
					_, err := os.Stat(folderPath)
					fmt.Println(err)
					if os.IsNotExist(err) {
						errDir := os.MkdirAll(folderPath, 0755)
						if errDir != nil {
							return sanitizedError.NewWithError(fmt.Sprintf("failed to create directory"), err)
						}
					}

				}

				file, err := os.OpenFile(mutatelogPath, os.O_RDONLY|os.O_CREATE, 0644)
				if err != nil {
					return sanitizedError.NewWithError(fmt.Sprintf("failed to create file"), err)
				}

				err = file.Close()
				if err != nil {
					return sanitizedError.NewWithError(fmt.Sprintf("failed to close file"), err)
				}

			} else {
				errDir := os.MkdirAll(mutatelogPath, 0755)
				if errDir != nil {
					return sanitizedError.NewWithError(fmt.Sprintf("failed to create directory"), err)
				}
			}

		} else {
			return sanitizedError.NewWithError(fmt.Sprintf("failed to describe file"), err)
		}
	}

	return nil
}

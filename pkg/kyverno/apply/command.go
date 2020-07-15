package apply

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
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
				return sanitizedError.NewWithError(fmt.Sprintf("resource file or cluster required"), err)
			}

			policies, openAPIController, err := common.GetPoliciesValidation(policyPaths)
			if err != nil {
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

			newPolicies := make([]*v1.ClusterPolicy, 0)

			logger := log.Log.WithName("apply")

			for _, policy := range policies {
				patches, updateMsgs := policymutation.GenerateJSONPatchesForDefaults(policy, logger)
				fmt.Println("___________________________________________________________________________")
				fmt.Println(updateMsgs)

				type jsonPatch struct {
					Path  string      `json:"path"`
					Op    string      `json:"op"`
					Value interface{} `json:"value"`
				}

				var jsonPatches []jsonPatch
				err = json.Unmarshal(patches, &jsonPatches)
				if err != nil {
					return sanitizedError.NewWithError("failed to unmarshal patches", err)
				}
				patch, err := jsonpatch.DecodePatch(patches)
				if err != nil {
					return sanitizedError.NewWithError("failed to decode patch", err)
				}

				policyBytes, _ := json.Marshal(policy)
				if err != nil {
					return sanitizedError.NewWithError("failed to marshal policy", err)
				}
				modifiedPolicy, err := patch.Apply(policyBytes)
				if err != nil {
					return sanitizedError.NewWithError("failed to apply policy", err)
				}

				var p v1.ClusterPolicy
				json.Unmarshal(modifiedPolicy, &p)
				fmt.Printf("\nmutated %s policy after mutation:\n\n", p.Name)
				yamlPolicy, _ := yamlv2.Marshal(p)
				fmt.Println(string(yamlPolicy))
				fmt.Println("___________________________________________________________________________")
				newPolicies = append(newPolicies, &p)
			}

			for i, policy := range newPolicies {
				for j, resource := range resources {
					if !(j == 0 && i == 0) {
						fmt.Printf("\n\n==========================================================================================\n")
					}

					err = applyPolicyOnResource(policy, resource)
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

func applyPolicyOnResource(policy *v1.ClusterPolicy, resource *unstructured.Unstructured) error {
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
			fmt.Printf("\n\nMutation:")
			fmt.Printf("\nMutation has been applied succesfully")
			yamlEncodedResource, err := yamlv2.Marshal(mutateResponse.PatchedResource.Object)
			if err != nil {
				return err
			}

			fmt.Printf("\n\n" + string(yamlEncodedResource))
			fmt.Printf("\n\n")
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

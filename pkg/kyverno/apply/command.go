package apply

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/nirmata/kyverno/pkg/kyverno/sanitizedError"

	policy2 "github.com/nirmata/kyverno/pkg/policy"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/discovery"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/nirmata/kyverno/pkg/engine"

	engineutils "github.com/nirmata/kyverno/pkg/engine/utils"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/spf13/cobra"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
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
						glog.V(4).Info(err)
						err = fmt.Errorf("Internal error")
					}
				}
			}()

			if len(resourcePaths) == 0 && !cluster {
				return sanitizedError.New(fmt.Sprintf("Specify path to resource file or cluster name"))
			}

			policies, err := getPolicies(policyPaths)
			if err != nil {
				if !sanitizedError.IsErrorSanitized(err) {
					return sanitizedError.New("Could not parse policy paths")
				} else {
					return err
				}
			}

			for _, policy := range policies {
				err := policy2.Validate(*policy)
				if err != nil {
					return sanitizedError.New(fmt.Sprintf("Policy %v is not valid", policy.Name))
				}
			}

			var dClient discovery.CachedDiscoveryInterface
			if cluster {
				dClient, err = kubernetesConfig.ToDiscoveryClient()
				if err != nil {
					return sanitizedError.New(fmt.Errorf("Issues with kubernetes Config").Error())
				}
			}

			resources, err := getResources(policies, resourcePaths, dClient)
			if err != nil {
				return sanitizedError.New(fmt.Errorf("Issues fetching resources").Error())
			}

			for i, policy := range policies {
				for j, resource := range resources {
					if !(j == 0 && i == 0) {
						fmt.Printf("\n\n=======================================================================\n")
					}

					err = applyPolicyOnResource(policy, resource)
					if err != nil {
						return sanitizedError.New(fmt.Errorf("Issues applying policy %v on resource %v", policy.Name, resource.GetName()).Error())
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

func getResources(policies []*v1.ClusterPolicy, resourcePaths []string, dClient discovery.CachedDiscoveryInterface) ([]*unstructured.Unstructured, error) {
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
		resource, err := getResource(resourcePath)
		if err != nil {
			return nil, err
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func getResourcesOfTypeFromCluster(resourceTypes []string, dClient discovery.CachedDiscoveryInterface) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured

	for _, kind := range resourceTypes {
		endpoint, err := getListEndpointForKind(kind)
		if err != nil {
			return nil, err
		}

		listObjectRaw, err := dClient.RESTClient().Get().RequestURI(endpoint).Do().Raw()
		if err != nil {
			return nil, err
		}

		listObject, err := engineutils.ConvertToUnstructured(listObjectRaw)
		if err != nil {
			return nil, err
		}

		resourceList, err := listObject.ToList()
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

func getPoliciesInDir(path string) ([]*v1.ClusterPolicy, error) {
	var policies []*v1.ClusterPolicy

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			policiesFromDir, err := getPoliciesInDir(filepath.Join(path, file.Name()))
			if err != nil {
				return nil, err
			}

			policies = append(policies, policiesFromDir...)
		} else {
			policy, err := getPolicy(filepath.Join(path, file.Name()))
			if err != nil {
				return nil, err
			}

			policies = append(policies, policy)
		}
	}

	return policies, nil
}

func getPolicies(paths []string) ([]*v1.ClusterPolicy, error) {
	var policies = make([]*v1.ClusterPolicy, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)

		fileDesc, err := os.Stat(path)
		if err != nil {
			return nil, err
		}

		if fileDesc.IsDir() {
			policiesFromDir, err := getPoliciesInDir(path)
			if err != nil {
				return nil, err
			}

			policies = append(policies, policiesFromDir...)
		} else {
			policy, err := getPolicy(path)
			if err != nil {
				return nil, err
			}

			policies = append(policies, policy)
		}
	}

	for i := range policies {
		setFalse := false
		policies[i].Spec.Background = &setFalse
	}

	return policies, nil
}

func getPolicy(path string) (*v1.ClusterPolicy, error) {
	policy := &v1.ClusterPolicy{}

	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load file: %v", err)
	}

	policyBytes, err := yaml.ToJSON(file)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(policyBytes, policy); err != nil {
		return nil, sanitizedError.New(fmt.Sprintf("failed to decode policy in %s", path))
	}

	if policy.TypeMeta.Kind != "ClusterPolicy" {
		return nil, sanitizedError.New(fmt.Sprintf("resource %v is not a cluster policy", policy.Name))
	}

	return policy, nil
}

func getResource(path string) (*unstructured.Unstructured, error) {

	resourceYaml, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	resourceObject, metaData, err := decode(resourceYaml, nil, nil)
	if err != nil {
		return nil, err
	}

	resourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&resourceObject)
	if err != nil {
		return nil, err
	}

	resourceJSON, err := json.Marshal(resourceUnstructured)
	if err != nil {
		return nil, err
	}

	resource, err := engineutils.ConvertToUnstructured(resourceJSON)
	if err != nil {
		return nil, err
	}

	resource.SetGroupVersionKind(*metaData)

	if resource.GetNamespace() == "" {
		resource.SetNamespace("default")
	}

	return resource, nil
}

func applyPolicyOnResource(policy *v1.ClusterPolicy, resource *unstructured.Unstructured) error {

	fmt.Printf("\n\nApplying Policy %s on Resource %s/%s/%s\n", policy.Name, resource.GetNamespace(), resource.GetKind(), resource.GetName())

	mutateResponse := engine.Mutate(engine.PolicyContext{Policy: *policy, NewResource: *resource})
	if !mutateResponse.IsSuccesful() {
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
	if !validateResponse.IsSuccesful() {
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

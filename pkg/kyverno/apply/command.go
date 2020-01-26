package apply

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"

	policy2 "github.com/nirmata/kyverno/pkg/policy"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

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
	"k8s.io/client-go/kubernetes/scheme"
)

func Command() *cobra.Command {
	var resourcePath, kubeConfig, clusterName string

	cmd := &cobra.Command{
		Use:     "apply",
		Short:   "Applies policies on resources",
		Example: fmt.Sprintf("To apply on a resource:\nkyverno apply /path/to/policy1 /path/to/policy2 --resource=/path/to/resource\n\nTo apply on a cluster\nkyverno apply /path/to/policy1 /path/to/policy2 --kubeConfig=/path/to/kubeConfig"),
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {
			defer func() {
				if err != nil {
					err = fmt.Errorf("Failed to apply policies on resources : %v", err)
				}
			}()

			if resourcePath == "" && clusterName == "" {
				fmt.Println("Specify path to resource file or cluster name")
			}

			if kubeConfig == "" {
				kubeConfig = path.Join(homedir.HomeDir(), ".kube", "config")
			}

			var policies []*v1.ClusterPolicy
			for _, policyPath := range policyPaths {
				policy, err := getPolicy(policyPath)
				if err != nil {
					return err
				}

				err = policy2.Validate(*policy)
				if err != nil {
					return fmt.Errorf("Policy %v is not valid: %v", policy.Name, err)
				}

				policies = append(policies, policy)
			}

			var dClient *discovery.DiscoveryClient
			if clusterName != "" {
				dClient, err = getDiscoveryClient(kubeConfig, clusterName)
				if err != nil {
					return err
				}
			}

			resources, err := getResources(policies, resourcePath, dClient)
			if err != nil {
				return err
			}

			for i, policy := range policies {
				for j, resource := range resources {
					if !(j == 0 && i == 0) {
						fmt.Printf("\n\n=======================================================================\n")
					}

					err = applyPolicyOnResource(policy, resource)
					if err != nil {
						return err
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&resourcePath, "resource", "", "path to resource file")
	cmd.Flags().StringVar(&kubeConfig, "kubeConfig", "", "path to .kube/config file")
	cmd.Flags().StringVar(&clusterName, "cluster", "", "Name of the kubernetes cluster to which the policy will apply to")
	return cmd
}

func getDiscoveryClient(kubeConfig, clusterName string) (*discovery.DiscoveryClient, error) {
	apiConfig, err := clientcmd.LoadFromFile(kubeConfig)
	if err != nil {
		return nil, err
	}

	if apiConfig.Clusters[clusterName] == nil {
		return nil, fmt.Errorf("Cluster does not exist in kubeConfig")
	}

	clientConfig, err := clientcmd.BuildConfigFromFlags(apiConfig.Clusters[clusterName].Server, kubeConfig)
	if err != nil {
		return nil, err
	}

	return discovery.NewDiscoveryClientForConfig(clientConfig)
}

func getResources(policies []*v1.ClusterPolicy, resourcePath string, dClient *discovery.DiscoveryClient) ([]*unstructured.Unstructured, error) {
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

	if resourcePath != "" {
		resource, err := getResource(resourcePath)
		if err != nil {
			return nil, err
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func getResourcesOfTypeFromCluster(resourceTypes []string, dClient *discovery.DiscoveryClient) ([]*unstructured.Unstructured, error) {
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
		return nil, fmt.Errorf("failed to decode policy %s, err: %v", policy.Name, err)
	}

	if policy.TypeMeta.Kind != "ClusterPolicy" {
		return nil, fmt.Errorf("failed to parse policy")
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

	fmt.Printf("\n\nApplying Policy %s on Resource %s/%s/%s", policy.Name, resource.GetNamespace(), resource.GetKind(), resource.GetName())

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

	return nil
}

package report

import (
	"encoding/json"
	"errors"
	"fmt"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernov1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"io/ioutil"
	"os"
	"reflect"

	"github.com/nirmata/kyverno/pkg/engine/context"

	"strings"
	"time"

	client "github.com/nirmata/kyverno/pkg/dclient"

	"github.com/nirmata/kyverno/pkg/kyverno/common"
	"github.com/nirmata/kyverno/pkg/kyverno/sanitizedError"

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
)

type resultCounts struct {
	pass  int
	fail  int
	warn  int
	error int
	skip  int
}

func Command() *cobra.Command {
	var cmd *cobra.Command
	var namespace, kubeconfig string
	var cluster bool
	type Resource struct {
		Name   string            `json:"name"`
		Values map[string]string `json:"values"`
	}

	type Policy struct {
		Name      string     `json:"name"`
		Resources []Resource `json:"resources"`
	}

	type Values struct {
		Policies []Policy `json:"policies"`
	}

	kubernetesConfig := genericclioptions.NewConfigFlags(true)

	cmd = &cobra.Command{
		Use:     "report",
		Short:   "generate report",
		Example: fmt.Sprintf("To apply on a resource:\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2\n\nTo apply on a cluster\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster"),
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {
			defer func() {
				if err != nil {
					if !sanitizedError.IsErrorSanitized(err) {
						log.Log.Error(err, "failed to sanitize")
						err = fmt.Errorf("internal error")

					}
				}
			}()

			var dClient *client.Client
			var kclient *kyvernoclient.Clientset
			if cluster {
				restConfig, err := kubernetesConfig.ToRESTConfig()
				if err != nil {
					os.Exit(1)
				}

				dClient, err = client.NewClient(restConfig, 5*time.Minute, make(chan struct{}), log.Log)
				if err != nil {
					os.Exit(1)
				}

				kclient, err = kyvernoclient.NewForConfig(restConfig)
				if err != nil {
					os.Exit(1)
				}
			}

			ns, err := dClient.ListResource("", "Namespace", "", &kyvernov1.LabelSelector{})
			if err != nil {
				os.Exit(1)
			}
			var engineResponses []response.EngineResponse
			for _, n := range ns.Items {
				policies, err := kclient.KyvernoV1().Policies(n.GetName()).List(kyvernov1.ListOption{})
				if err != nil {
					os.Exit(1)
				}
				for _, p := range policies.Items {

					policyContext := engine.PolicyContext{
						NewResource:      newR,
						OldResource:      nil,
						Context:          context.Background(),
						Policy:           p,
						ExcludeGroupRole: excludeGroupROle,
					}
					engineResponse := engine.Validate(policyContext)
					if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
						// we get an empty response if old and new resources created the same response
						// allow updates if resource update doesnt change the policy evaluation
						continue
					}
					if len(engineResponse.PolicyResponse.Rules) > 0 {
						engineResponses = append(engineResponses, engineResponse)
					}

					engineResponse = engine.Mutate(policyContext)
					if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
						// we get an empty response if old and new resources created the same response
						// allow updates if resource update doesnt change the policy evaluation
						continue
					}
					if len(engineResponse.PolicyResponse.Rules) > 0 {
						engineResponses = append(engineResponses, engineResponse)
					}

					engineResponse = engine.Generate(policyContext)
					if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
						// we get an empty response if old and new resources created the same response
						// allow updates if resource update doesnt change the policy evaluation
						continue
					}
					if len(engineResponse.PolicyResponse.Rules) > 0 {
						engineResponses = append(engineResponses, engineResponse)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace")
	cmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "kubeconfig")
	cmd.Flags().StringVarP(&excludeGroupRole, "excludeGroupRole", "e", "", "excludeGroupRole")
	cmd.Flags().BoolVarP(&cluster, "cluster", "c", false, "Checks if policies should be applied to cluster in the current context")
	cmd.Flags().BoolVarP(&helm, "helm", "h", false, "Checks if policies should be applied to cluster in the current context")
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
		resourceList, err := dClient.ListResource("", kind, "", nil)
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
func applyPolicyOnResource(policy *v1.ClusterPolicy, resource *unstructured.Unstructured, mutateLogPath string, mutateLogPathIsDir bool, variables map[string]string, rc *resultCounts) error {
	responseError := false

	resPath := fmt.Sprintf("%s/%s/%s", resource.GetNamespace(), resource.GetKind(), resource.GetName())
	log.Log.V(3).Info("applying policy on resource", "policy", policy.Name, "resource", resPath)

	// build context
	ctx := context.NewContext()
	for key, value := range variables {
		startString := ""
		endString := ""
		for _, k := range strings.Split(key, ".") {
			startString += fmt.Sprintf(`{"%s":`, k)
			endString += `}`
		}

		midString := fmt.Sprintf(`"%s"`, value)
		finalString := startString + midString + endString
		var jsonData = []byte(finalString)
		ctx.AddJSON(jsonData)
	}

	mutateResponse := engine.Mutate(engine.PolicyContext{Policy: *policy, NewResource: *resource, Context: ctx})
	if !mutateResponse.IsSuccessful() {
		fmt.Printf("Failed to apply mutate policy %s -> resource %s", policy.Name, resPath)
		for i, r := range mutateResponse.PolicyResponse.Rules {
			fmt.Printf("\n%d. %s", i+1, r.Message)
		}
		responseError = true
	} else {
		if len(mutateResponse.PolicyResponse.Rules) > 0 {
			yamlEncodedResource, err := yamlv2.Marshal(mutateResponse.PatchedResource.Object)
			if err != nil {
				rc.error++
			}

			mutatedResource := string(yamlEncodedResource)
			if len(strings.TrimSpace(mutatedResource)) > 0 {
				fmt.Printf("\nmutate policy %s applied to %s:", policy.Name, resPath)
				fmt.Printf("\n" + mutatedResource)
				fmt.Printf("\n")
			}

		} else {
			fmt.Printf("\n\nMutation:\nMutation skipped. Resource not matches the policy\n")
		}
	}

	validateResponse := engine.Validate(engine.PolicyContext{Policy: *policy, NewResource: mutateResponse.PatchedResource, Context: ctx})
	if !validateResponse.IsSuccessful() {
		fmt.Printf("\npolicy %s -> resource %s failed: \n", policy.Name, resPath)
		for i, r := range validateResponse.PolicyResponse.Rules {
			if !r.Success {
				fmt.Printf("%d. %s: %s \n", i+1, r.Name, r.Message)
			}
		}

		responseError = true
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
			log.Log.V(3).Info("generate resource is valid", "policy", policy.Name, "resource", resPath)
		} else {
			fmt.Printf("generate policy %s resource %s is invalid \n", policy.Name, resPath)
			for i, r := range generateResponse.PolicyResponse.Rules {
				fmt.Printf("%d. %s \b", i+1, r.Message)
			}

			responseError = true
		}
	}

	if responseError == true {
		rc.fail++
	} else {
		rc.pass++
	}
	return nil
}

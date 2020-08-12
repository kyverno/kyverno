package apply

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/nirmata/kyverno/pkg/engine/context"
	"k8s.io/apimachinery/pkg/util/yaml"

	"os"
	"path/filepath"

	"strings"
	"time"

	client "github.com/nirmata/kyverno/pkg/dclient"

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
)


type resultCounts struct {
	pass int
	fail int
	warn int
	error int
	skip int
}

func Command() *cobra.Command {
	var cmd *cobra.Command
	var resourcePaths []string
	var cluster bool
	var mutatelogPath, variablesString, valuesFile string
	variables := make(map[string]string)

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

	valuesMap := make(map[string]map[string]*Resource)

	kubernetesConfig := genericclioptions.NewConfigFlags(true)

	cmd = &cobra.Command{
		Use:     "apply",
		Short:   "applies policies on resources",
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

			if valuesFile != "" && variablesString != "" {
				return sanitizedError.NewWithError("pass the values either using set flag or values_file flag", err)
			}

			if valuesFile != "" {
				yamlFile, err := ioutil.ReadFile(valuesFile)
				if err != nil {
					return sanitizedError.NewWithError("unable to read yaml", err)
				}

				valuesBytes, err := yaml.ToJSON(yamlFile)
				if err != nil {
					return sanitizedError.NewWithError("failed to convert json", err)
				}

				values := &Values{}
				if err := json.Unmarshal(valuesBytes, values); err != nil {
					return sanitizedError.NewWithError("failed to decode yaml", err)
				}

				for _, p := range values.Policies {
					pmap := make(map[string]*Resource)
					for _, r := range p.Resources {
						pmap[r.Name] = &r
					}
					valuesMap[p.Name] = pmap
				}
			}

			if variablesString != "" {
				kvpairs := strings.Split(strings.Trim(variablesString, " "), ",")
				for _, kvpair := range kvpairs {
					kvs := strings.Split(strings.Trim(kvpair, " "), "=")
					variables[strings.Trim(kvs[0], " ")] = strings.Trim(kvs[1], " ")
				}
			}

			if len(resourcePaths) == 0 && !cluster {
				return sanitizedError.NewWithError(fmt.Sprintf("resource file(s) or cluster required"), err)
			}

			var mutatelogPathIsDir bool
			if mutateLogPath != "" {
				spath := strings.Split(mutateLogPath, "/")
				sfileName := strings.Split(spath[len(spath)-1], ".")
				if sfileName[len(sfileName)-1] == "yml" || sfileName[len(sfileName)-1] == "yaml" {
					mutatelogPathIsDir = false
				} else {
					mutatelogPathIsDir = true
				}

				err = createFileOrFolder(mutateLogPath, mutatelogPathIsDir)
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
				if common.PolicyHasVariables(*policy) && variablesString == "" && valuesFile == "" {
					return sanitizedError.NewWithError(fmt.Sprintf("policy %s have variables. pass the values for the variables using set/values_file flag", policy.Name), err)
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

			mutatedPolicies, err := mutatePolices(policies)
			msgPolicies := "1 policy"
			if len(mutatedPolicies) > 1 {
				msgPolicies = fmt.Sprintf("%d policies", len(policies))
			}

			msgResources := "1 resource"
			if len(resources) > 1 {
				msgResources = fmt.Sprintf("%d resources", len(resources))
			}

			fmt.Printf("\napplying %s to %s \n", msgPolicies, msgResources)

			if len(mutatedPolicies) == 0 || len(resources) == 0 {
				return
			}

			rc := &resultCounts{}
			for _, policy := range mutatedPolicies {

				err := policy2.Validate(utils.MarshalPolicy(*policy), nil, true, openAPIController)
				if err != nil {
					rc.skip += len(resources)
					fmt.Printf("\nskipping policy %v as it is not valid: %v\n", policy.Name, err)
					continue
				}

				if common.PolicyHasVariables(*policy) {
					rc.skip += len(resources)
					fmt.Printf("\nskipping policy %s as policies with variables are not supported\n", policy.Name)
					continue
				}

				for _, resource := range resources {
					// get values from file for this policy resource combination
					thisPolicyResouceValues := make(map[string]string)
					if len(valuesMap[policy.GetName()]) != 0 && valuesMap[policy.GetName()][resource.GetName()] != nil {
						thisPolicyResouceValues = valuesMap[policy.GetName()][resource.GetName()].Values
					}

					for k, v := range variables {
						thisPolicyResouceValues[k] = v
					}

					if common.PolicyHasVariables(*policy) && len(thisPolicyResouceValues) == 0 {
						return sanitizedError.NewWithError(fmt.Sprintf("policy %s have variables. pass the values for the variables using set/values_file flag", policy.Name), err)
					}

					if !(j == 0 && i == 0) {
						fmt.Printf("\n\n==========================================================================================\n")
					}

					err = applyPolicyOnResource(policy, resource, mutatelogPath, mutatelogPathIsDir, thisPolicyResouceValues, rc)
					if err != nil {
						return sanitizedError.NewWithError(fmt.Errorf("failed to apply policy %v on resource %v", policy.Name, resource.GetName()).Error(), err)
					}
				}
			}

			fmt.Printf("\npass: %d, fail: %d, warn: %d, error: %d, skip: %d \n",
				rc.pass, rc.fail, rc.warn, rc.error, rc.skip)

			if rc.fail > 0 || rc.error > 0 {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringArrayVarP(&resourcePaths, "resource", "r", []string{}, "Path to resource files")
	cmd.Flags().BoolVarP(&cluster, "cluster", "c", false, "Checks if policies should be applied to cluster in the current context")
	cmd.Flags().StringVarP(&mutatelogPath, "output", "o", "", "Prints the mutated resources in provided file/directory")
	cmd.Flags().StringVarP(&variablesString, "set", "s", "", "Variables that are required")
	cmd.Flags().StringVarP(&valuesFile, "values_file", "f", "", "File containing values for policy variables")
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
func applyPolicyOnResource(policy *v1.ClusterPolicy, resource *unstructured.Unstructured, mutatelogPath string, mutatelogPathIsDir bool, variables map[string]string, rc *resultCounts) error {
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
}

// mutatePolicies - function to apply mutation on policies
func mutatePolices(policies []*v1.ClusterPolicy) ([]*v1.ClusterPolicy, error) {
	newPolicies := make([]*v1.ClusterPolicy, 0)
	logger := log.Log.WithName("apply")

	for _, policy := range policies {
		p, err := common.MutatePolicy(policy, logger)
		if err != nil {
			if !sanitizedError.IsErrorSanitized(err) {
				return nil, sanitizedError.NewWithError("failed to mutate policy.", err)
			}
			return nil, err
		}
		newPolicies = append(newPolicies, p)
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

// createFileOrFolder - creating file or folder according to path provided
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

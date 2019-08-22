package apply

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
	memory "k8s.io/client-go/discovery/cached/memory"
	dynamic "k8s.io/client-go/dynamic"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"
)

const (
	applyExample = `  # Apply a policy to the resource.
  kyverno apply @policy.yaml @resource.yaml
  kyverno apply @policy.yaml @resourceDir/
  kyverno apply @policy.yaml @resource.yaml --kubeconfig=$PATH_TO_KUBECONFIG_FILE`

	defaultYamlSeparator = "---"
)

// NewCmdApply returns the apply command for kyverno
func NewCmdApply(in io.Reader, out, errout io.Writer) *cobra.Command {
	var kubeconfig string
	cmd := &cobra.Command{
		Use:     "apply",
		Short:   "Apply policy on the resource(s)",
		Example: applyExample,
		Run: func(cmd *cobra.Command, args []string) {
			policy, resources := complete(kubeconfig, args)
			output := applyPolicy(policy, resources)
			fmt.Printf("%v\n", output)
		},
	}

	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file")
	return cmd
}

func complete(kubeconfig string, args []string) (*kyverno.Policy, []*resourceInfo) {
	policyDir, resourceDir, err := validateDir(args)
	if err != nil {
		glog.Errorf("Failed to parse file path, err: %v\n", err)
		os.Exit(1)
	}

	// extract policy
	policy, err := extractPolicy(policyDir)
	if err != nil {
		glog.Errorf("Failed to extract policy: %v\n", err)
		os.Exit(1)
	}

	// extract rawResource
	resources, err := extractResource(resourceDir, kubeconfig)
	if err != nil {
		glog.Errorf("Failed to parse resource: %v", err)
		os.Exit(1)
	}

	return policy, resources
}

func applyPolicy(policy *kyverno.Policy, resources []*resourceInfo) (output string) {
	for _, resource := range resources {
		patchedDocument, err := applyPolicyOnRaw(policy, resource.rawResource, resource.gvk)
		if err != nil {
			glog.Errorf("Error applying policy on resource %s, err: %v\n", resource.gvk.Kind, err)
			continue
		}

		out, err := prettyPrint(patchedDocument)
		if err != nil {
			glog.Errorf("JSON parse error: %v\n", err)
			continue
		}

		output = output + fmt.Sprintf("---\n%s", string(out))
	}
	return
}

func applyPolicyOnRaw(policy *kyverno.Policy, rawResource []byte, gvk *metav1.GroupVersionKind) ([]byte, error) {
	patchedResource := rawResource
	var err error

	rname := engine.ParseNameFromObject(rawResource)
	rns := engine.ParseNamespaceFromObject(rawResource)
	policyInfo := info.NewPolicyInfo(policy.Name,
		gvk.Kind,
		rname,
		rns,
		policy.Spec.ValidationFailureAction)

	resource, err := ConvertToUnstructured(rawResource)
	if err != nil {
		return nil, err
	}
	//TODO check if the kind information is present resource
	// Process Mutation
	engineResponse := engine.Mutate(*policy, *resource)
	policyInfo.AddRuleInfos(engineResponse.RuleInfos)
	if !policyInfo.IsSuccessful() {
		glog.Infof("Failed to apply policy %s on resource %s/%s", policy.Name, rname, rns)
		for _, r := range engineResponse.RuleInfos {
			glog.Warning(r.Msgs)
		}
	} else if len(engineResponse.Patches) > 0 {
		glog.Infof("Mutation from policy %s has applied succesfully to %s %s/%s", policy.Name, gvk.Kind, rname, rns)
		patchedResource, err = engine.ApplyPatches(rawResource, engineResponse.Patches)
		if err != nil {
			return nil, fmt.Errorf("Unable to apply mutation patches:\n%v", err)
		}
		// Process Validation
		engineResponse := engine.Validate(*policy, *resource)

		policyInfo.AddRuleInfos(engineResponse.RuleInfos)
		if !policyInfo.IsSuccessful() {
			glog.Infof("Failed to apply policy %s on resource %s/%s", policy.Name, rname, rns)
			for _, r := range engineResponse.RuleInfos {
				glog.Warning(r.Msgs)
			}
			return patchedResource, fmt.Errorf("Failed to apply policy %s on resource %s/%s", policy.Name, rname, rns)
		} else if len(engineResponse.RuleInfos) > 0 {
			glog.Infof("Validation from policy %s has applied succesfully to %s %s/%s", policy.Name, gvk.Kind, rname, rns)
		}
	}
	return patchedResource, nil
}

func extractPolicy(fileDir string) (*kyverno.Policy, error) {
	policy := &kyverno.Policy{}

	file, err := loadFile(fileDir)
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

	if policy.TypeMeta.Kind != "Policy" {
		return nil, fmt.Errorf("failed to parse policy")
	}

	return policy, nil
}

type resourceInfo struct {
	rawResource []byte
	gvk         *metav1.GroupVersionKind
}

func extractResource(fileDir, kubeconfig string) ([]*resourceInfo, error) {
	var files []string
	var resources []*resourceInfo

	// check if applied on multiple resources
	isDir, err := isDir(fileDir)
	if err != nil {
		return nil, err
	}

	if isDir {
		files, err = scanDir(fileDir)
		if err != nil {
			return nil, err
		}
	} else {
		files = []string{fileDir}
	}

	for _, dir := range files {
		data, err := loadFile(dir)
		if err != nil {
			glog.Warningf("Error while loading file: %v\n", err)
			continue
		}

		dd := bytes.Split(data, []byte(defaultYamlSeparator))

		for _, d := range dd {
			decode := scheme.Codecs.UniversalDeserializer().Decode
			obj, gvk, err := decode([]byte(d), nil, nil)
			if err != nil {
				glog.Warningf("Error while decoding YAML object, err: %s\n", err)
				continue
			}

			actualObj, err := convertToActualObject(kubeconfig, gvk, obj)
			if err != nil {
				glog.V(3).Infof("Failed to convert resource %s to actual k8s object: %v\n", gvk.Kind, err)
				glog.V(3).Infof("Apply policy on raw resource.\n")
			}

			raw, err := json.Marshal(actualObj)
			if err != nil {
				glog.Warningf("Error while marshalling manifest, err: %v\n", err)
				continue
			}

			gvkInfo := &metav1.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind}
			resources = append(resources, &resourceInfo{rawResource: raw, gvk: gvkInfo})
		}
	}

	return resources, err
}

func convertToActualObject(kubeconfig string, gvk *schema.GroupVersionKind, obj runtime.Object) (interface{}, error) {
	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		return obj, err
	}

	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return obj, err
	}

	kclient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return obj, err
	}

	asUnstructured := &unstructured.Unstructured{}
	if err := scheme.Scheme.Convert(obj, asUnstructured, nil); err != nil {
		return obj, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(kclient.Discovery()))
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
	if err != nil {
		return obj, err
	}

	actualObj, err := dynamicClient.Resource(mapping.Resource).Namespace("default").Create(asUnstructured, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
	if err != nil {
		return obj, err
	}

	return actualObj, nil
}

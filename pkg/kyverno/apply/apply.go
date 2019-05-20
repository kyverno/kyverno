package apply

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/engine"
	"github.com/spf13/cobra"
	yamlv2 "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
)

const applyExample = `  # Apply a policy to the resource.
  kyverno apply @policy.yaml @resource.yaml`

// NewCmdApply returns the apply command for kyverno
func NewCmdApply(in io.Reader, out, errout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apply",
		Short:   "Apply policy on the resource",
		Example: applyExample,
		Run: func(cmd *cobra.Command, args []string) {
			// TODO: add pre-checks for policy and resource manifest
			//  order for policy and resource in args could be disordered

			if len(args) != 2 {
				log.Printf("Missing policy and/or resource manifest.")
				return
			}

			// extract policy
			policyDir := validateDir(args[0])
			policy, err := extractPolicy(policyDir)
			if err != nil {
				log.Printf("failed to extract policy: %v", err)
				os.Exit(1)
			}

			// fmt.Printf("policy name=%s, rule name=%s, %s/%s\n", policy.ObjectMeta.Name, policy.Spec.Rules[0].Name,
			// policy.Spec.Rules[0].ResourceDescription.Kind, *policy.Spec.Rules[0].ResourceDescription.Name)

			// extract rawResource
			resourceDir := validateDir(args[1])
			rawResource, gvk, err := extractResource(resourceDir)
			if err != nil {
				log.Printf("failed to load resource: %v", err)
				os.Exit(1)
			}

			_, patchedDocument := engine.Mutate(*policy, rawResource, *gvk)
			out, err := prettyPrint(patchedDocument)
			if err != nil {
				fmt.Printf("JSON parse error: %v\n", err)
				fmt.Printf("%v\n", string(patchedDocument))
				return
			}

			fmt.Printf("%v\n", string(out))
		},
	}
	return cmd
}

func extractPolicy(fileDir string) (*kubepolicy.Policy, error) {
	policy := &kubepolicy.Policy{}

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

	return policy, nil
}

func extractResource(fileDir string) ([]byte, *metav1.GroupVersionKind, error) {
	file, err := loadFile(fileDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load file: %v", err)
	}

	data := make(map[interface{}]interface{})

	if err = yamlv2.Unmarshal([]byte(file), &data); err != nil {
		return nil, nil, fmt.Errorf("failed to parse resource: %v", err)
	}

	apiVersion, ok := data["apiVersion"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("failed to parse apiversion: %v", err)
	}

	kind, ok := data["kind"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("failed to parse kind of resource: %v", err)
	}

	var gvk *metav1.GroupVersionKind
	gv := strings.Split(apiVersion, "/")
	if len(gv) == 2 {
		gvk = &metav1.GroupVersionKind{Group: gv[0], Version: gv[1], Kind: kind}
	} else {
		gvk = &metav1.GroupVersionKind{Version: gv[0], Kind: kind}
	}

	json, err := yaml.ToJSON(file)

	return json, gvk, err
}

func loadFile(fileDir string) ([]byte, error) {
	if _, err := os.Stat(fileDir); os.IsNotExist(err) {
		return nil, err
	}

	return ioutil.ReadFile(fileDir)
}

func validateDir(dir string) string {
	if strings.HasPrefix(dir, "@") {
		return dir[1:]
	}
	return dir
}

func prettyPrint(data []byte) ([]byte, error) {
	out := make(map[interface{}]interface{})
	if err := yamlv2.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	return yamlv2.Marshal(&out)
}

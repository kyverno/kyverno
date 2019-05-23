package apply

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/spf13/cobra"
	yamlv2 "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
)

const applyExample = `  # Apply a policy to the resource.
  kyverno apply @policy.yaml @resource.yaml
  kyverno apply @policy.yaml @resourceDir/`

// NewCmdApply returns the apply command for kyverno
func NewCmdApply(in io.Reader, out, errout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apply",
		Short:   "Apply policy on the resource(s)",
		Example: applyExample,
		Run: func(cmd *cobra.Command, args []string) {
			var output string
			policy, resources := complete(args)

			for _, resource := range resources {
				patchedDocument, err := applyPolicy(policy, resource.rawResource, resource.gvk)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}

				out, err := prettyPrint(patchedDocument)
				if err != nil {
					fmt.Printf("JSON parse error: %v\n", err)
					fmt.Printf("%v\n", string(patchedDocument))
					return
				}

				output = output + fmt.Sprintf("---\n%s", string(out))
			}
			fmt.Printf("%v\n", output)
		},
	}
	return cmd
}

func complete(args []string) (*kubepolicy.Policy, []*resourceInfo) {

	policyDir, resourceDir, err := validateDir(args)
	if err != nil {
		fmt.Printf("Failed to parse file path, err: %v\n", err)
		os.Exit(1)
	}

	// extract policy
	policy, err := extractPolicy(policyDir)
	if err != nil {
		log.Printf("failed to extract policy: %v", err)
		os.Exit(1)
	}

	// extract rawResource
	resources, err := extractResource(resourceDir)
	if err != nil {
		log.Printf("failed to parse resource: %v", err)
		os.Exit(1)
	}

	return policy, resources
}

func applyPolicy(policy *kubepolicy.Policy, rawResource []byte, gvk *metav1.GroupVersionKind) ([]byte, error) {
	_, patchedDocument := engine.Mutate(*policy, rawResource, *gvk)

	if err := engine.Validate(*policy, patchedDocument, *gvk); err != nil {
		return nil, err
	}
	return patchedDocument, nil
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

	if policy.TypeMeta.Kind != "Policy" {
		return nil, fmt.Errorf("failed to parse policy")
	}

	return policy, nil
}

type resourceInfo struct {
	rawResource []byte
	gvk         *metav1.GroupVersionKind
}

func extractResource(fileDir string) ([]*resourceInfo, error) {
	var files []string
	var resources []*resourceInfo
	// check if applied on multiple resources
	isDir, err := isDir(fileDir)
	if err != nil {
		return nil, err
	}

	if isDir {
		files, err = ScanDir(fileDir)
		if err != nil {
			return nil, err
		}
	} else {
		files = []string{fileDir}
	}

	for _, dir := range files {
		file, err := loadFile(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to load file: %v", err)
		}

		data := make(map[interface{}]interface{})

		if err = yamlv2.Unmarshal([]byte(file), &data); err != nil {
			return nil, fmt.Errorf("failed to parse resource: %v", err)
		}

		apiVersion, ok := data["apiVersion"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse apiversion: %v", err)
		}

		kind, ok := data["kind"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse kind of resource: %v", err)
		}

		var gvkInfo *metav1.GroupVersionKind
		gv := strings.Split(apiVersion, "/")
		if len(gv) == 2 {
			gvkInfo = &metav1.GroupVersionKind{Group: gv[0], Version: gv[1], Kind: kind}
		} else {
			gvkInfo = &metav1.GroupVersionKind{Version: gv[0], Kind: kind}
		}

		json, err := yaml.ToJSON(file)

		resources = append(resources, &resourceInfo{rawResource: json, gvk: gvkInfo})
	}

	return resources, err
}

func loadFile(fileDir string) ([]byte, error) {
	if _, err := os.Stat(fileDir); os.IsNotExist(err) {
		return nil, err
	}

	return ioutil.ReadFile(fileDir)
}

func validateDir(args []string) (policyDir, resourceDir string, err error) {
	if len(args) != 2 {
		return "", "", fmt.Errorf("missing policy and/or resource manifest")
	}

	if strings.HasPrefix(args[0], "@") {
		policyDir = args[0][1:]
	}

	if strings.HasPrefix(args[1], "@") {
		resourceDir = args[1][1:]
	}
	return
}

func prettyPrint(data []byte) ([]byte, error) {
	out := make(map[interface{}]interface{})
	if err := yamlv2.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	return yamlv2.Marshal(&out)
}

func isDir(dir string) (bool, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return false, err
	}

	return fi.IsDir(), nil
}

func ScanDir(dir string) ([]string, error) {
	var res []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", dir, err)
			return err
		}
		/* 		if len(strings.Split(path, "/")) == 4 {
			fmt.Println(path)
		} */
		res = append(res, path)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking the path %q: %v", dir, err)
	}

	return res[1:], nil
}

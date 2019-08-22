package apply

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	yamlv2 "gopkg.in/yaml.v2"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

func createClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		defaultKC := defaultKubeconfigPath()
		if _, err := os.Stat(defaultKC); err == nil {
			kubeconfig = defaultKC
		}
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func defaultKubeconfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		glog.Warningf("Warning: failed to get home dir: %v\n", err)
		return ""
	}

	return filepath.Join(home, ".kube", "config")
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

func scanDir(dir string) ([]string, error) {
	var res []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("prevent panic by handling failure accessing a path %q: %v", dir, err)
		}

		res = append(res, path)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking the path %q: %v", dir, err)
	}

	return res[1:], nil
}
func ConvertToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	err := resource.UnmarshalJSON(data)
	if err != nil {
		glog.V(4).Infof("failed to unmarshall resource: %v", err)
		return nil, err
	}
	return resource, nil
}

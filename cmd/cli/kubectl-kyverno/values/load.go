package values

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func Load(f billy.Filesystem, filepath string) (*v1alpha1.Values, error) {
	yamlBytes, err := readFile(f, filepath)
	if err != nil {
		return nil, err
	}
		 var cm corev1.ConfigMap
		 if err := yaml.Unmarshal(yamlBytes, &cm); err == nil && cm.Kind == "ConfigMap" {
			 if cm.Data == nil {
				 return nil, errors.New("configmap manifest missing .data")
			 }
			 vals := &v1alpha1.Values{
				TypeMeta: metav1.TypeMeta{Kind: "Values", APIVersion: "cli.kyverno.io/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      cm.Name,
					Namespace: cm.Namespace,
				},
				ValuesSpec: v1alpha1.ValuesSpec{
					GlobalValues: make(map[string]interface{}, len(cm.Data)),
				},
			}
			for k, raw := range cm.Data {
				var val interface{} = raw
				trimmed := strings.TrimSpace(raw)
				if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
					var obj interface{}
					if err := json.Unmarshal([]byte(raw), &obj); err == nil {
						val = obj
					}
				}
			
				parts := strings.SplitN(k, ".", 2)
				if len(parts) == 2 {
					root, sub := parts[0], parts[1]
					m, _ := vals.ValuesSpec.GlobalValues[root].(map[string]interface{})
					if m == nil {
						m = make(map[string]interface{})
					}
					m[sub] = val
					vals.ValuesSpec.GlobalValues[root] = m
				} else {
					vals.ValuesSpec.GlobalValues[k] = val
				}
			}
			 return vals, nil
		 }
	
	vals := &v1alpha1.Values{}
	if err := yaml.UnmarshalStrict(yamlBytes, vals); err != nil {
		return nil, err
	}
	return vals, nil
}

func readFile(f billy.Filesystem, filepath string) ([]byte, error) {
	if f != nil {
		file, err := f.Open(filepath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(file)
	}
	return os.ReadFile(filepath)
}

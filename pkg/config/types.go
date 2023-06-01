package config

import (
	"encoding/json"
	"regexp"
	"strings"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WebhookConfig struct {
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
	ObjectSelector    *metav1.LabelSelector `json:"objectSelector,omitempty"`
}

func parseWebhooks(in string) ([]WebhookConfig, error) {
	webhookCfgs := make([]WebhookConfig, 0, 10)
	if err := json.Unmarshal([]byte(in), &webhookCfgs); err != nil {
		return nil, err
	}
	return webhookCfgs, nil
}

func parseExclusions(in string) (exclusions, inclusions []string) {
	for _, in := range strings.Split(in, ",") {
		in := strings.TrimSpace(in)
		if in == "" {
			continue
		}
		inclusion := strings.HasPrefix(in, "!")
		if inclusion {
			in = strings.TrimSpace(in[1:])
			if in == "" {
				continue
			}
		}
		if inclusion {
			inclusions = append(inclusions, in)
		} else {
			exclusions = append(exclusions, in)
		}
	}
	return
}

func parseWebhookAnnotations(in string) (map[string]string, error) {
	var out map[string]string
	if err := json.Unmarshal([]byte(in), &out); err != nil {
		return nil, err
	}
	return out, nil
}

type namespacesConfig struct {
	IncludeNamespaces []string `json:"include,omitempty"`
	ExcludeNamespaces []string `json:"exclude,omitempty"`
}

func parseIncludeExcludeNamespacesFromNamespacesConfig(in string) (namespacesConfig, error) {
	var namespacesConfigObject namespacesConfig
	err := json.Unmarshal([]byte(in), &namespacesConfigObject)
	return namespacesConfigObject, err
}

type filter struct {
	Group       string
	Version     string
	Kind        string
	Subresource string
	Namespace   string
	Name        string
}

func newFilter(kind, namespace, name string) filter {
	if kind == "" {
		return filter{}
	}
	g, v, k, s := kubeutils.ParseKindSelector(kind)
	return filter{
		Group:       g,
		Version:     v,
		Kind:        k,
		Subresource: s,
		Namespace:   namespace,
		Name:        name,
	}
}

// ParseKinds parses the kinds if a single string contains comma separated kinds
// {"1,2,3","4","5"} => {"1","2","3","4","5"}
func parseKinds(in string) []filter {
	resources := []filter{}
	var resource filter
	re := regexp.MustCompile(`\[([^\[\]]*)\]`)
	submatchall := re.FindAllString(in, -1)
	for _, element := range submatchall {
		element = strings.Trim(element, "[")
		element = strings.Trim(element, "]")
		elements := strings.Split(element, ",")
		if len(elements) == 0 {
			continue
		}
		if len(elements) == 3 {
			resource = newFilter(elements[0], elements[1], elements[2])
		}
		if len(elements) == 2 {
			resource = newFilter(elements[0], elements[1], "")
		}
		if len(elements) == 1 {
			resource = newFilter(elements[0], "", "")
		}
		resources = append(resources, resource)
	}
	return resources
}

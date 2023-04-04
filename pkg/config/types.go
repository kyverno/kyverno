package config

import (
	"encoding/json"
	"regexp"
	"strings"

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

func parseStrings(in string) []string {
	var out []string
	for _, in := range strings.Split(in, ",") {
		in := strings.TrimSpace(in)
		if in != "" {
			out = append(out, in)
		}
	}
	return out
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
	Kind      string // TODO: as we currently only support one GVK version, we use the kind only. But if we support multiple GVK, then GV need to be added
	Namespace string
	Name      string
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
			resource = filter{Kind: elements[0], Namespace: elements[1], Name: elements[2]}
		}
		if len(elements) == 2 {
			resource = filter{Kind: elements[0], Namespace: elements[1]}
		}
		if len(elements) == 1 {
			resource = filter{Kind: elements[0]}
		}
		resources = append(resources, resource)
	}
	return resources
}

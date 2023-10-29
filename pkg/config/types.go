package config

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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

func parseMatchConditions(in string) ([]admissionregistrationv1.MatchCondition, error) {
	var out []admissionregistrationv1.MatchCondition
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

type metricExposureConfig struct {
	Enabled                 *bool     `json:"enabled,omitempty"`
	DisabledLabelDimensions []string  `json:"disabledLabelDimensions,omitempty"`
	BucketBoundaries        []float64 `json:"bucketBoundaries,omitempty"`
}

func parseMetricExposureConfig(in string, defaultBoundaries []float64) (map[string]metricExposureConfig, error) {
	var metricExposureMap map[string]metricExposureConfig
	err := json.Unmarshal([]byte(in), &metricExposureMap)
	if err != nil {
		return nil, err
	}

	for key, config := range metricExposureMap {
		if config.Enabled == nil {
			b := true
			config.Enabled = &b
		}
		if config.DisabledLabelDimensions == nil {
			config.DisabledLabelDimensions = []string{}
		}
		if config.BucketBoundaries == nil {
			config.BucketBoundaries = defaultBoundaries
		}
		metricExposureMap[key] = config
	}

	return metricExposureMap, err
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

var submatchallRegex = regexp.MustCompile(`\[([^\[\]]*)\]`)

// ParseKinds parses the kinds if a single string contains comma separated kinds
// {"1,2,3","4","5"} => {"1","2","3","4","5"}
func parseKinds(in string) []filter {
	resources := []filter{}
	var resource filter
	submatchall := submatchallRegex.FindAllString(in, -1)
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

func parseBucketBoundariesConfig(boundariesString string) ([]float64, error) {
	var boundaries []float64
	boundariesString = strings.TrimSpace(boundariesString)

	if boundariesString != "" {
		boundaryStrings := strings.Split(boundariesString, ",")
		for _, boundaryStr := range boundaryStrings {
			boundaryStr = strings.TrimSpace(boundaryStr)
			boundary, err := strconv.ParseFloat(boundaryStr, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid boundary value '%s'", boundaryStr)
			}
			boundaries = append(boundaries, boundary)
		}
	}

	return boundaries, nil
}

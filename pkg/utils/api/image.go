package api

import (
	"fmt"
	"strconv"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/logging"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ImageInfo struct {
	imageutils.ImageInfo
	// Pointer is the path to the image object in the resource
	Pointer string `json:"jsonPointer"`
}

var (
	podExtractors               = BuildStandardExtractors("spec")
	podControllerExtractors     = BuildStandardExtractors("spec", "template", "spec")
	cronjobControllerExtractors = BuildStandardExtractors("spec", "jobTemplate", "spec", "template", "spec")
	registeredExtractors        = map[string][]imageExtractor{
		"Pod":                   podExtractors,
		"DaemonSet":             podControllerExtractors,
		"Deployment":            podControllerExtractors,
		"ReplicaSet":            podControllerExtractors,
		"ReplicationController": podControllerExtractors,
		"StatefulSet":           podControllerExtractors,
		"CronJob":               cronjobControllerExtractors,
		"Job":                   podControllerExtractors,
	}
)

type imageExtractor struct {
	Fields   []string
	Key      string
	Value    string
	Name     string
	JMESPath string
}

func (i *imageExtractor) ExtractFromResource(resource interface{}, cfg config.Configuration) (map[string]ImageInfo, error) {
	imageInfo := map[string]ImageInfo{}
	if err := extract(resource, []string{}, i.Key, i.Value, i.Fields, i.JMESPath, &imageInfo, cfg); err != nil {
		return nil, err
	}
	return imageInfo, nil
}

func extract(
	obj interface{},
	path []string,
	keyPath string,
	valuePath string,
	fields []string,
	jmesPath string,
	imageInfos *map[string]ImageInfo,
	cfg config.Configuration,
) error {
	if obj == nil {
		return nil
	}
	if len(fields) > 0 && fields[0] == "*" {
		switch typedObj := obj.(type) {
		case []interface{}:
			for i, v := range typedObj {
				if err := extract(v, append(path, strconv.Itoa(i)), keyPath, valuePath, fields[1:], jmesPath, imageInfos, cfg); err != nil {
					return err
				}
			}
		case map[string]interface{}:
			for i, v := range typedObj {
				if err := extract(v, append(path, i), keyPath, valuePath, fields[1:], jmesPath, imageInfos, cfg); err != nil {
					return err
				}
			}
		case interface{}:
			return fmt.Errorf("invalid type")
		}
		return nil
	}
	output, ok := obj.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid image config")
	}
	if len(fields) == 0 {
		pointer := fmt.Sprintf("/%s/%s", strings.Join(path, "/"), valuePath)
		key := pointer
		if keyPath != "" {
			key, ok = output[keyPath].(string)
			if !ok {
				return fmt.Errorf("invalid key")
			}
		}
		value, ok := output[valuePath].(string)
		if !ok || strings.TrimSpace(value) == "" {
			// the image may not be present
			logging.V(4).Info("image information is not present", "pointer", pointer)
			return nil
		}
		if jmesPath != "" {
			// TODO: should be injected
			jp := jmespath.New(cfg)
			q, err := jp.Query(jmesPath)
			if err != nil {
				return fmt.Errorf("invalid jmespath %s: %v", jmesPath, err)
			}
			result, err := q.Search(value)
			if err != nil {
				return fmt.Errorf("failed to apply jmespath %s: %v", jmesPath, err)
			}
			resultStr, ok := result.(string)
			if !ok {
				return fmt.Errorf("jmespath %s must produce a string, but produced %v", jmesPath, result)
			}
			value = resultStr
		}
		if imageInfo, err := imageutils.GetImageInfo(value, cfg); err != nil {
			return fmt.Errorf("invalid image '%s' (%s)", value, err.Error())
		} else {
			(*imageInfos)[key] = ImageInfo{*imageInfo, pointer}
		}
		return nil
	}
	currentPath := fields[0]
	return extract(output[currentPath], append(path, currentPath), keyPath, valuePath, fields[1:], jmesPath, imageInfos, cfg)
}

func BuildStandardExtractors(tags ...string) []imageExtractor {
	extractors := make([]imageExtractor, 0, 3)
	for _, tag := range []string{"initContainers", "containers", "ephemeralContainers"} {
		var t []string
		t = append(t, tags...)
		t = append(t, tag)
		t = append(t, "*")
		extractors = append(extractors, imageExtractor{Fields: t, Key: "name", Value: "image", Name: tag})
	}
	return extractors
}

func lookupImageExtractor(kind string, configs kyvernov1.ImageExtractorConfigs) []imageExtractor {
	if configs != nil {
		if extractorConfigs, ok := configs[kind]; ok {
			extractors := []imageExtractor{}
			for _, c := range extractorConfigs {
				fields := func(input []string) []string {
					output := []string{}
					for _, i := range input {
						o := strings.Trim(i, " ")
						if o != "" {
							output = append(output, o)
						}
					}
					return output
				}(strings.Split(c.Path, "/"))
				name := c.Name
				if name == "" {
					name = "custom"
				}
				value := c.Value
				if value == "" {
					value = fields[len(fields)-1]
					fields = fields[:len(fields)-1]
				}
				extractors = append(extractors, imageExtractor{
					Fields:   fields,
					Key:      c.Key,
					Name:     name,
					Value:    value,
					JMESPath: c.JMESPath,
				})
			}
			return extractors
		}
	}
	return registeredExtractors[kind]
}

func ExtractImagesFromResource(resource unstructured.Unstructured, configs kyvernov1.ImageExtractorConfigs, cfg config.Configuration) (map[string]map[string]ImageInfo, error) {
	infos := map[string]map[string]ImageInfo{}
	extractors := lookupImageExtractor(resource.GetKind(), configs)
	if extractors != nil && len(extractors) == 0 {
		return nil, fmt.Errorf("no extractors found for %s", resource.GetKind())
	}
	for _, extractor := range extractors {
		if infoMap, err := extractor.ExtractFromResource(resource.Object, cfg); err != nil {
			return nil, err
		} else if len(infoMap) > 0 {
			infos[extractor.Name] = infoMap
		}
	}
	return infos, nil
}

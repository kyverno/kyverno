package kube

import (
	"fmt"
	"strconv"
	"strings"

	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ImageExtractorConfigs map[string][]*ImageExtractorConfig

type ImageExtractorConfig struct {
	// Path is the path to the object containing the image field in a custom resource.
	// It should be slash-separated. Each slash-separated key must by a valid YAML key or a wildcard '*'.
	// Wildcard keys are expanded in case of arrays or objects.
	Path string `json:"path" yaml:"path"`
	// Value is an optional name of the field within 'path' that points to the image URI.
	// This is useful when a custom 'key' is also defined.
	// +optional
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
	// Name is the entry the image will be available under 'images.<name>' in the context.
	// If this field is not defined, image entries will appear under 'images.custom'.
	// +optional
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Key is an optional name of the field within 'path' that will be used to uniquely identify an image.
	// Note - this field MUST be unique.
	// +optional
	Key string `json:"key,omitempty" yaml:"key,omitempty"`
}

var (
	podExtractors               = BuildStandardExtractors("spec")
	podControllerExtractors     = BuildStandardExtractors("spec", "template", "spec")
	cronjobControllerExtractors = BuildStandardExtractors("spec", "jobTemplate", "spec", "template", "spec")
	registeredExtractors        = map[string][]imageExtractor{
		"Pod":         podExtractors,
		"DaemonSet":   podControllerExtractors,
		"Deployment":  podControllerExtractors,
		"ReplicaSet":  podControllerExtractors,
		"StatefulSet": podControllerExtractors,
		"CronJob":     cronjobControllerExtractors,
		"Job":         podControllerExtractors,
	}
)

type imageExtractor struct {
	Fields []string
	Key    string
	Value  string
	Name   string
}

func (i *imageExtractor) ExtractFromResource(resource interface{}) (map[string]imageutils.ImageInfo, error) {
	imageInfo := map[string]imageutils.ImageInfo{}
	if err := extract(resource, []string{}, i.Key, i.Value, i.Fields, &imageInfo); err != nil {
		return nil, err
	}
	return imageInfo, nil
}

func extract(obj interface{}, path []string, keyPath, valuePath string, fields []string, imageInfos *map[string]imageutils.ImageInfo) error {
	if obj == nil {
		return nil
	}
	if len(fields) > 0 && fields[0] == "*" {
		switch typedObj := obj.(type) {
		case []interface{}:
			for i, v := range typedObj {
				if err := extract(v, append(path, strconv.Itoa(i)), keyPath, valuePath, fields[1:], imageInfos); err != nil {
					return err
				}
			}
		case map[string]interface{}:
			for i, v := range typedObj {
				if err := extract(v, append(path, i), keyPath, valuePath, fields[1:], imageInfos); err != nil {
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
		if !ok {
			return fmt.Errorf("invalid value")
		}
		if imageInfo, err := imageutils.GetImageInfo(value, pointer); err != nil {
			return fmt.Errorf("invalid image %s", value)
		} else {
			(*imageInfos)[key] = *imageInfo
		}
		return nil
	}
	currentPath := fields[0]
	return extract(output[currentPath], append(path, currentPath), keyPath, valuePath, fields[1:], imageInfos)
}

func BuildStandardExtractors(tags ...string) []imageExtractor {
	var extractors []imageExtractor
	for _, tag := range []string{"initContainers", "containers", "ephemeralContainers"} {
		var t []string
		t = append(t, tags...)
		t = append(t, tag)
		t = append(t, "*")
		extractors = append(extractors, imageExtractor{Fields: t, Key: "name", Value: "image", Name: tag})
	}
	return extractors
}

func lookupImageExtractor(kind string, configs ImageExtractorConfigs) []imageExtractor {
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
					Fields: fields,
					Key:    c.Key,
					Name:   name,
					Value:  value,
				})
			}
			return extractors
		}
	}
	return registeredExtractors[kind]
}

func ExtractImagesFromResource(resource unstructured.Unstructured, configs ImageExtractorConfigs) (map[string]map[string]imageutils.ImageInfo, error) {
	infos := map[string]map[string]imageutils.ImageInfo{}
	for _, extractor := range lookupImageExtractor(resource.GetKind(), configs) {
		if infoMap, err := extractor.ExtractFromResource(resource.Object); err != nil {
			return nil, err
		} else if infoMap != nil && len(infoMap) > 0 {
			infos[extractor.Name] = infoMap
		}
	}
	return infos, nil
}

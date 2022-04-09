package kube

import (
	"fmt"
	"strconv"
	"strings"

	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	podExtractors               = BuildStandardExtractors("spec")
	podControllerExtractors     = BuildStandardExtractors("spec", "template", "spec")
	cronjobControllerExtractors = BuildStandardExtractors("spec", "jobTemplate", "spec", "template", "spec")
	registeredExtractors        = map[string][]ImageExtractor{
		"Pod":         podExtractors,
		"DaemonSet":   podControllerExtractors,
		"Deployment":  podControllerExtractors,
		"ReplicaSet":  podControllerExtractors,
		"StatefulSet": podControllerExtractors,
		"CronJob":     cronjobControllerExtractors,
		"Job":         podControllerExtractors,
	}
)

type ImageExtractor struct {
	fields []string
	key    string
	value  string
	name   string
}

func (i *ImageExtractor) ExtractFromResource(resource interface{}) (map[string]imageutils.ImageInfo, error) {
	imageInfo := map[string]imageutils.ImageInfo{}
	if err := extract(resource, []string{}, i.key, i.value, i.fields, &imageInfo); err != nil {
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

func BuildStandardExtractors(tags ...string) []ImageExtractor {
	var extractors []ImageExtractor
	for _, tag := range []string{"initContainers", "containers", "ephemeralContainers"} {
		var t []string
		t = append(t, tags...)
		t = append(t, tag)
		t = append(t, "*")
		extractors = append(extractors, ImageExtractor{fields: t, key: "name", value: "image", name: tag})
	}
	return extractors
}

func LookupImageExtractor(kind string) []ImageExtractor {
	return registeredExtractors[kind]
}

func ExtractImagesFromResource(resource unstructured.Unstructured) (map[string]map[string]imageutils.ImageInfo, error) {
	infos := map[string]map[string]imageutils.ImageInfo{}
	for _, extractor := range LookupImageExtractor(resource.GetKind()) {
		if infoMap, err := extractor.ExtractFromResource(resource.Object); err != nil {
			return nil, err
		} else if infoMap != nil && len(infoMap) > 0 {
			infos[extractor.name] = infoMap
		}
	}
	return infos, nil
}

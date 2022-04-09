package kube

import (
	"fmt"
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
		"CronJob":     cronjobControllerExtractors,
		"Deployment":  podControllerExtractors,
		"DaemonSet":   podControllerExtractors,
		"Job":         podControllerExtractors,
		"StatefulSet": podControllerExtractors,
	}
)

type ImageExtractor interface {
	ExtractFromResource(unstructured.Unstructured) map[string]imageutils.ImageInfo
}

type imageExtractor struct {
	fields []string
}

func (i *imageExtractor) ExtractFromResource(resource unstructured.Unstructured) map[string]imageutils.ImageInfo {
	if containers, ok, _ := unstructured.NestedSlice(resource.UnstructuredContent(), i.fields...); ok {
		return extractImageInfos(containers, "/"+strings.Join(i.fields, "/"))
	}
	return nil
}

func extractImageInfos(containers []interface{}, jsonPath string) map[string]imageutils.ImageInfo {
	infos := map[string]imageutils.ImageInfo{}
	var errs []string
	var index = 0
	for _, ctr := range containers {
		if container, ok := ctr.(map[string]interface{}); ok {
			imageInfo, err := imageutils.GetImageInfo(container["image"].(string))
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}
			infos[fmt.Sprintf("%s/%d/image", jsonPath, index)] = *imageInfo
		}
		index++
	}
	if len(errs) == 0 {
		return infos
	}
	return infos
}

func BuildStandardExtractors(tags ...string) []ImageExtractor {
	var extractors []ImageExtractor
	for _, tag := range []string{"initContainers", "containers", "ephemeralContainers"} {
		var t []string
		t = append(t, tags...)
		t = append(t, tag)
		extractors = append(extractors, &imageExtractor{fields: t})
	}
	return extractors
}

func LookupImageExtractor(kind string) []ImageExtractor {
	return registeredExtractors[kind]
}

func ExtractImagesFromResource(resource unstructured.Unstructured) map[string]imageutils.ImageInfo {
	infos := map[string]imageutils.ImageInfo{}
	for _, extractor := range LookupImageExtractor(resource.GetKind()) {
		for key, value := range extractor.ExtractFromResource(resource) {
			infos[key] = value
		}
	}
	return infos
}

package context

import (
	"strings"

	"github.com/distribution/distribution/reference"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type imageInfo struct {
	Registry string `json:"registry,omitempty"`
	Name     string `json:"name"`
	Tag      string `json:"tag,omitempty"`
	Digest   string `json:"digest,omitempty"`
}

type containerImage struct {
	Name  string
	Image imageInfo
}

type resourceImage struct {
	Containers     map[string]interface{} `json:"containers"`
	InitContainers map[string]interface{} `json:"initContainers,omitempty"`
}

func newResourceImage(initContainersImgs, containersImgs []*containerImage) resourceImage {
	initContainers := make(map[string]interface{})
	containers := make(map[string]interface{})

	for _, resource := range initContainersImgs {
		initContainers[resource.Name] = resource.Image
	}

	for _, resource := range containersImgs {
		containers[resource.Name] = resource.Image
	}

	return resourceImage{
		Containers:     containers,
		InitContainers: initContainers,
	}
}

func extractImageInfo(resource *unstructured.Unstructured, log logr.Logger) (initContainersImgs, containersImgs []*containerImage) {
	logger := log.WithName("extractImageInfo").WithValues("kind", resource.GetKind(), "ns", resource.GetNamespace(), "name", resource.GetName())

	switch resource.GetKind() {
	case "Pod":
		for i, tag := range []string{"initContainers", "containers"} {
			if initContainers, ok, _ := unstructured.NestedSlice(resource.UnstructuredContent(), "spec", tag); ok {
				img, err := convertToImageInfo(initContainers)
				if err != nil {
					logger.WithName(tag).Error(err, "failed to extract image info")
					continue
				}

				if i == 0 {
					initContainersImgs = append(initContainersImgs, img...)
				} else {
					containersImgs = append(containersImgs, img...)
				}
			}
		}

	case "Deployment", "DaemonSet", "Job", "StatefulSet":
		for i, tag := range []string{"initContainers", "containers"} {
			if initContainers, ok, _ := unstructured.NestedSlice(resource.UnstructuredContent(), "spec", "template", "spec", tag); ok {
				img, err := convertToImageInfo(initContainers)
				if err != nil {
					logger.WithName(tag).Error(err, "failed to extract image info")
					continue
				}

				if i == 0 {
					initContainersImgs = append(initContainersImgs, img...)
				} else {
					containersImgs = append(containersImgs, img...)
				}
			}
		}

	case "CronJob":
		for i, tag := range []string{"initContainers", "containers"} {
			if initContainers, ok, _ := unstructured.NestedSlice(resource.UnstructuredContent(), "spec", "jobTemplate", "spec", "template", "spec", tag); ok {
				img, err := convertToImageInfo(initContainers)
				if err != nil {
					logger.WithName(tag).Error(err, "failed to extract image info")
					continue
				}

				if i == 0 {
					initContainersImgs = append(initContainersImgs, img...)
				} else {
					containersImgs = append(containersImgs, img...)
				}
			}
		}
	}

	return
}

func convertToImageInfo(containers []interface{}) (images []*containerImage, err error) {
	var errs []string

	for _, ctr := range containers {
		if container, ok := ctr.(map[string]interface{}); ok {
			repo, err := reference.Parse(container["image"].(string))
			if err != nil {
				errs = append(errs, errors.Wrapf(err, "bad image: %s", container["image"].(string)).Error())
				continue
			}

			var registry, name, tag, digest string
			if named, ok := repo.(reference.Named); ok {
				registry, name = reference.SplitHostname(named)
			}

			if tagged, ok := repo.(reference.Tagged); ok {
				tag = tagged.Tag()
			}

			if digested, ok := repo.(reference.Digested); ok {
				digest = digested.Digest().String()
			}

			images = append(images, &containerImage{
				Name: container["name"].(string),
				Image: imageInfo{
					Registry: registry,
					Name:     name,
					Tag:      tag,
					Digest:   digest,
				},
			})
		}
	}

	if len(errs) == 0 {
		return images, nil
	}

	return images, errors.Errorf("%s", strings.Join(errs, ";"))
}

package context

import (
	"strings"

	"github.com/distribution/distribution/reference"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ImageInfo struct {
	Registry string `json:"registry,omitempty"`
	Name     string `json:"name"`
	Tag      string `json:"tag,omitempty"`
	Digest   string `json:"digest,omitempty"`
}

type ContainerImage struct {
	Name  string
	Image *ImageInfo
}

type Images struct {
	InitContainers map[string]*ImageInfo `json:"initContainers,omitempty"`
	Containers     map[string]*ImageInfo `json:"containers"`
}

func newImages(initContainersImgs, containersImgs []*ContainerImage) *Images {
	initContainers := make(map[string]*ImageInfo)
	for _, resource := range initContainersImgs {
		initContainers[resource.Name] = resource.Image
	}

	containers := make(map[string]*ImageInfo)
	for _, resource := range containersImgs {
		containers[resource.Name] = resource.Image
	}

	return &Images{
		InitContainers: initContainers,
		Containers:     containers,
	}
}

func extractImageInfo(resource *unstructured.Unstructured, log logr.Logger) (initContainersImgs, containersImgs []*ContainerImage) {
	logger := log.WithName("extractImageInfo").WithValues("kind", resource.GetKind(), "ns", resource.GetNamespace(), "name", resource.GetName())

	for _, tag := range []string{"initContainers", "containers"} {
		switch resource.GetKind() {
		case "Pod":
			if containers, ok, _ := unstructured.NestedSlice(resource.UnstructuredContent(), "spec", tag); ok {
				if tag == "initContainers" {
					initContainersImgs = extractImageInfos(containers, initContainersImgs, logger)
				} else {
					containersImgs = extractImageInfos(containers, containersImgs, logger)
				}
			}

		case "CronJob":
			if containers, ok, _ := unstructured.NestedSlice(resource.UnstructuredContent(), "spec", "jobTemplate", "spec", "template", "spec", tag); ok {
				if tag == "initContainers" {
					initContainersImgs = extractImageInfos(containers, initContainersImgs, logger)
				} else {
					containersImgs = extractImageInfos(containers, containersImgs, logger)
				}			}

		// handles "Deployment", "DaemonSet", "Job", "StatefulSet", and custom controllers with the same pattern
		default:
			if containers, ok, _ := unstructured.NestedSlice(resource.UnstructuredContent(), "spec", "template", "spec", tag); ok {
				if tag == "initContainers" {
					initContainersImgs = extractImageInfos(containers, initContainersImgs, logger)
				} else {
					containersImgs = extractImageInfos(containers, containersImgs, logger)
				}
			}
		}
	}

	return
}

func extractImageInfos(containers []interface{}, images []*ContainerImage, log logr.Logger) []*ContainerImage {
	img, err := convertToImageInfo(containers)
	if err != nil {
		log.Error(err, "failed to extract image info", "element", containers)
	}

	return append(images, img...)
}

func convertToImageInfo(containers []interface{}) (images []*ContainerImage, err error) {
	var errs []string
	for _, ctr := range containers {
		if container, ok := ctr.(map[string]interface{}); ok {
			image := container["image"].(string)
			imageInfo, err := newImageInfo(image)
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}

			images = append(images, &ContainerImage{
				Name: image,
				Image: imageInfo,
			})
		}
	}

	if len(errs) == 0 {
		return images, nil
	}

	return images, errors.Errorf("%s", strings.Join(errs, ";"))
}

func newImageInfo(image string) (*ImageInfo, error) {
	repo, err := reference.Parse(image)
	if err != nil {
		return nil, errors.Wrapf(err, "bad image: %s", image)
	}

	var registry, name, tag, digest string
	if named, ok := repo.(reference.Named); ok {
		registry = reference.Domain(named)
		name = reference.Path(named)
	}

	if tagged, ok := repo.(reference.Tagged); ok {
		tag = tagged.Tag()
	}

	if digested, ok := repo.(reference.Digested); ok {
		digest = digested.Digest().String()
	}

	// set default registry and tag
	if registry == "" {
		registry = "docker.io"
	}

	if tag == "" {
		tag = "latest"
	}

	return &ImageInfo{
		Registry: registry,
		Name:     name,
		Tag:      tag,
		Digest:   digest,
	}, nil
}

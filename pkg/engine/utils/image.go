package utils

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ImageMatches(image string, imagePatterns []string) bool {
	for _, imagePattern := range imagePatterns {
		if wildcard.Match(imagePattern, image) {
			return true
		}
	}
	return false
}

func GetMatchingImages(images map[string]map[string]apiutils.ImageInfo, rule kyvernov1.Rule) ([]apiutils.ImageInfo, string) {
	imageInfos := []apiutils.ImageInfo{}
	imageRefs := []string{}
	for _, infoMap := range images {
		for _, imageInfo := range infoMap {
			image := imageInfo.String()
			for _, verifyImage := range rule.VerifyImages {
				verifyImage = *verifyImage.Convert()
				imageRefs = append(imageRefs, verifyImage.ImageReferences...)
				if ImageMatches(image, verifyImage.ImageReferences) {
					imageInfos = append(imageInfos, imageInfo)
				}
			}
		}
	}
	return imageInfos, strings.Join(imageRefs, ",")
}

func ExtractMatchingImages(
	resource unstructured.Unstructured,
	context enginecontext.Interface,
	rule kyvernov1.Rule,
	cfg config.Configuration,
) ([]apiutils.ImageInfo, string, error) {
	var (
		images map[string]map[string]apiutils.ImageInfo
		err    error
	)
	images = context.ImageInfo()
	if rule.ImageExtractors != nil {
		images, err = context.GenerateCustomImageInfo(&resource, rule.ImageExtractors, cfg)
		if err != nil {
			// if we get an error while generating custom images from image extractors,
			// don't check for matching images in imageExtractors
			return nil, "", err
		}
	}
	matchingImages, imageRefs := GetMatchingImages(images, rule)
	return matchingImages, imageRefs, nil
}

func IsImageVerified(resource unstructured.Unstructured, image string, log logr.Logger) (engineapi.ImageVerificationMetadataStatus, error) {
	if resource.Object == nil {
		return engineapi.ImageVerificationFail, fmt.Errorf("nil resource")
	}
	if annotations := resource.GetAnnotations(); len(annotations) == 0 {
		return engineapi.ImageVerificationFail, nil
	} else if data, ok := annotations[kyverno.AnnotationImageVerify]; !ok {
		log.V(2).Info("missing image metadata in annotation", "key", kyverno.AnnotationImageVerify)
		return engineapi.ImageVerificationFail, fmt.Errorf("image is not verified")
	} else if ivm, err := engineapi.ParseImageMetadata(data); err != nil {
		log.Error(err, "failed to parse image verification metadata", "data", data)
		return engineapi.ImageVerificationFail, fmt.Errorf("failed to parse image metadata: %w", err)
	} else {
		return ivm.ImageVerificationStatus(image), nil
	}
}

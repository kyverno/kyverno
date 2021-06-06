package engine

import (
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/minio/minio/pkg/wildcard"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// VerifyImages ...
func VerifyImages(policyContext *PolicyContext) (resp *response.EngineResponse) {
	resp = &response.EngineResponse{}
	images := policyContext.JSONContext.ImageInfo()
	if images == nil {
		return resp
	}

	policy := policyContext.Policy
	patchedResource := policyContext.NewResource
	logger := log.Log.WithName("EngineVerifyImages").WithValues("policy", policy.Name,
		"kind", patchedResource.GetKind(), "namespace", patchedResource.GetNamespace(), "name", patchedResource.GetName())

	for _, rule := range policyContext.Policy.Spec.Rules {
		if len(rule.VerifyImages) == 0 {
			continue
		}

		for _, verifyImage := range rule.VerifyImages {
			image := verifyImage.Image
			key := verifyImage.Key

			for _, ctnr := range images.Containers {
				if wildcard.Match(image, ctnr.String()) {
					digest, err := cosign.Verify(image, []byte(key))
					if err != nil {

					}
				}
			}

			for _, ctnr := range images.InitContainers {
				if wildcard.Match(image, ctnr.String()) {

				}
			}
		}
	}

	return
}

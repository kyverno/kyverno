package eval

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func Evaluate(ctx context.Context, logger logr.Logger, ivpols []*v1alpha1.ImageVerificationPolicy, request interface{}, admissionAttr admission.Attributes, namespace runtime.Object, lister k8scorev1.SecretInterface, registryOpts ...imagedataloader.Option) ([]*EvaluationResult, error) {
	ictx, err := imagedataloader.NewImageContext(lister, registryOpts...)
	if err != nil {
		return nil, err
	}

	// TODO: use environmentconfig, add support for other controllers (autogen)
	isPod := false
	if r, ok := request.(*admissionv1.AdmissionRequest); ok && r.RequestKind.Group == "" && r.RequestKind.Version == "v1" && r.RequestKind.Kind == "Pod" {
		isPod = true
	}

	c := NewCompiler(ictx, lister, isPod)
	results := make([]*EvaluationResult, 0)
	for _, ivpol := range ivpols {
		p, errList := c.Compile(logger, ivpol)
		if errList != nil {
			return nil, fmt.Errorf("failed to compile policy %v", err)
		}

		result, err := p.Evaluate(ctx, ictx, admissionAttr, request, namespace)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

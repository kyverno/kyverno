package evaluator

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engine "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/image/verification/variables"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/kyverno/sdk/extensions/cel/libs/globalcontext"
	"github.com/kyverno/sdk/extensions/cel/libs/imagedata"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"go.uber.org/multierr"
	"gomodules.xyz/jsonpatch/v2"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type EvaluationResult struct {
	Error            error
	Message          string
	Index            int
	Result           bool
	AuditAnnotations map[string]string
	Exceptions       []*policiesv1beta1.PolicyException
	// MatchedImages is the set matchImageReferences selected -- what EnforceRequired
	// checks, once every policy in the request has been evaluated.
	MatchedImages []string
}

type CompiledPolicy interface {
	// Evaluate does not enforce validationConfigurations.required: the evidence may
	// still come from another policy later in the same request. Call
	// EnforceRequired on every passing policy only after all have evaluated.
	Evaluate(context.Context, imagedataloader.ImageContext, admission.Attributes, interface{}, runtime.Object, bool, libs.Context) (*EvaluationResult, error)
	EnforceRequired(images []string) error
	MutateDigest(context.Context, imagedataloader.ImageContext, admission.Attributes, interface{}, runtime.Object, unstructured.Unstructured, config.Configuration) ([]jsonpatch.JsonPatchOperation, error)
}

type compiledPolicy struct {
	failurePolicy        admissionregistrationv1.FailurePolicyType
	verifyDigest         bool
	matchConditions      []cel.Program
	matchImageReferences []engine.MatchImageReference
	validations          []engine.Validation
	imageExtractors      map[string]engine.ImageExtractor
	attestors            []*variables.CompiledAttestor
	attestationList      map[string]string
	auditAnnotations     map[string]cel.Program
	authOpts             []remote.Option
	nameOpts             []name.Option
	exceptions           []engine.Exception
	variables            map[string]cel.Program
	validationConfig     policiesv1alpha1.ValidationConfiguration
	verifications        *imageverify.ImageVerificationResults
}

func (c *compiledPolicy) Evaluate(ctx context.Context, ictx imagedataloader.ImageContext, attr admission.Attributes, request interface{}, namespace runtime.Object, isK8s bool, context libs.Context) (*EvaluationResult, error) {
	matched, err := c.match(ctx, attr, request, namespace, c.matchConditions)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, nil
	}
	// check if the resource matches an exception
	if len(c.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1beta1.PolicyException, 0)
		fullExemptionFound := false
		for _, polex := range c.exceptions {
			match, err := c.match(ctx, attr, request, namespace, polex.MatchConditions)
			if err != nil {
				if fullExemptionFound {
					// exception already granted; a broken later exception must not negate it
					continue
				}
				return nil, err
			}
			if match {
				// ImageValidatingPolicy does not yet expose exceptions.allowedImages /
				// exceptions.allowedValues to CEL the way vpol/gpol/mpol do. A partial
				// exception (Images or AllowedValues set) must not fully skip evaluation
				// or every image on the resource is exempted, including ones never listed.
				if len(polex.Exception.Spec.Images) > 0 || len(polex.Exception.Spec.AllowedValues) > 0 {
					continue
				}
				matchedExceptions = append(matchedExceptions, polex.Exception)
				fullExemptionFound = true
			}
		}
		if fullExemptionFound {
			return &EvaluationResult{Exceptions: matchedExceptions}, nil
		}
	}
	data := map[string]any{}
	vars := lazy.NewMapValue(engine.VariablesType)
	for name, variable := range c.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := variable.ContextEval(ctx, data)
			if out != nil {
				return out
			}
			if err != nil {
				return types.WrapErr(err)
			}
			return nil
		})
	}
	if isK8s {
		namespaceVal, err := objectToResolveVal(namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
		}
		requestVal, err := convertObjectToUnstructured(request)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
		}
		objectVal, err := objectToResolveVal(attr.GetObject())
		if err != nil {
			return nil, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
		}
		oldObjectVal, err := objectToResolveVal(attr.GetOldObject())
		if err != nil {
			return nil, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
		}
		data[engine.NamespaceObjectKey] = namespaceVal
		data[engine.RequestKey] = requestVal.Object
		data[engine.ObjectKey] = objectVal
		data[engine.OldObjectKey] = oldObjectVal
		data[engine.VariablesKey] = vars
		data[engine.GlobalContextKey] = globalcontext.Context{ContextInterface: context}
		data[engine.ImageDataKey] = imagedata.Context{ContextInterface: context} // the thing that actually does the fetching and validation of images
		data[engine.ResourceKey] = resource.Context{ContextInterface: context}
	} else {
		data[engine.ObjectKey] = request
	}

	images, err := engine.ExtractImages(data, c.imageExtractors)
	if err != nil {
		return nil, err
	}
	filteredImages := make(map[string][]string, len(images))
	imgList := []string{}
	for category, imgs := range images {
		filteredImages[category] = []string{} // ensure image.containers is always [] in CEL
		for _, img := range imgs {
			if apply, err := matching.MatchImage(img, c.matchImageReferences...); err != nil {
				return nil, err
			} else if apply {
				filteredImages[category] = append(filteredImages[category], img)
				imgList = append(imgList, img)
			}
		}
	}

	// not reset: verification results are shared across the request, an earlier
	// policy's verification must stay visible to the catch-all required policy

	// when we get here, we will be initialized with the global opts from the compiled policy
	// or from the credentials configured on the policy itself. the latter replaces the first
	result, err := c.checkDigests(imgList)
	if result != nil || err != nil {
		return result, err
	}

	// Prefetch image data through Get() one image at a time to avoid triggering
	// racy concurrent map writes in the SDK AddImages() implementation.
	for _, image := range imgList {
		if _, err := ictx.Get(ctx, image, c.authOpts, c.nameOpts); err != nil {
			return nil, err
		}
	}

	data[engine.ImagesKey] = filteredImages
	data[engine.AttestationsKey] = c.attestationList
	attestors := lazy.NewMapValue(cel.DynType)
	for _, attestor := range c.attestors {
		attestors.Append(attestor.Key, func(*lazy.MapValue) ref.Val {
			data, err := attestor.Evaluate(data)
			if err != nil {
				return types.WrapErr(err)
			}
			return data
		})
	}
	data[engine.AttestorsKey] = attestors

	for i, v := range c.validations {
		out, _, err := v.Program.ContextEval(ctx, data)
		if err != nil {
			return nil, err
		}
		// evaluate only when rule fails
		if outcome, err := utils.ConvertToNative[bool](out); err == nil && !outcome {
			message := v.Message
			if v.MessageExpression != nil {
				if out, _, err := v.MessageExpression.ContextEval(ctx, data); err != nil {
					message = fmt.Sprintf("failed to evaluate message expression: %s", err)
				} else if msg, err := utils.ConvertToNative[string](out); err != nil {
					message = fmt.Sprintf("failed to convert message expression to string: %s", err)
				} else {
					message = msg
				}
			}
			// Add default message if empty
			if message == "" {
				message = fmt.Sprintf("CEL expression validation failed at index %d", i)
			}
			auditAnnotations, err := c.evaluateAuditAnnotations(ctx, data)
			if err != nil {
				return nil, err
			}
			return &EvaluationResult{
				Result:           outcome,
				Message:          message,
				AuditAnnotations: auditAnnotations,
				Index:            i,
				Error:            err,
				MatchedImages:    imgList,
			}, nil
		} else if err != nil {
			return &EvaluationResult{Error: err}, nil
		}
	}

	auditAnnotations, err := c.evaluateAuditAnnotations(ctx, data)
	if err != nil {
		return nil, err
	}
	// required is enforced by the caller via EnforceRequired, not here
	return &EvaluationResult{Result: true, AuditAnnotations: auditAnnotations, MatchedImages: imgList}, nil
}

func (c *compiledPolicy) checkDigests(imgList []string) (*EvaluationResult, error) {
	if !c.verifyDigest {
		return nil, nil
	}

	for _, img := range imgList {
		ref, err := name.ParseReference(img, c.nameOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", img, err)
		}

		if _, ok := ref.(name.Digest); !ok {
			return &EvaluationResult{
				Result:  false,
				Message: fmt.Sprintf("image %s does not have a digest", img),
			}, nil
		}
	}

	return nil, nil
}

// EnforceRequired checks images (the policy's own matched set) against the
// request-scoped verification results, so the evidence can come from any policy
// in the request -- the catch-all model. Must be called only after every policy
// in the request has evaluated, or a catch-all run first would deny images a
// later policy verifies.
func (c *compiledPolicy) EnforceRequired(images []string) error {
	if c.validationConfig.Required != nil && !*c.validationConfig.Required {
		return nil
	}
	for _, image := range images {
		verified, attempted := c.verifications.Status(image)
		if verified {
			continue
		}
		if attempted {
			return fmt.Errorf("image %s failed signature or attestation verification", image)
		}
		return fmt.Errorf("image %s is not verified: no policy performed a signature or attestation check on it", image)
	}
	return nil
}

// MutateDigest pins the tag of every image matched by the policy's matchImageReferences
// to its resolved digest, provided the image does not already carry a digest. It only
// applies to images extracted from well-known container fields (containers, initContainers,
// ephemeralContainers) of the resource, mirroring the built-in CEL image extractors.
//
// An image that cannot be resolved is an error, not a silent skip: the caller records it
// against the policy and the webhook denies the request when the policy's validationActions
// include Deny, so an unreachable registry cannot quietly admit an unpinned image.
//
// Resolution is per image. The returned patches cover every image that could be pinned even
// when the returned error is non-nil, so one unresolvable image does not cost the others their
// digest, matching how ClusterPolicy pins each image independently.
func (c *compiledPolicy) MutateDigest(
	ctx context.Context,
	ictx imagedataloader.ImageContext,
	attr admission.Attributes,
	request interface{},
	namespace runtime.Object,
	resource unstructured.Unstructured,
	cfg config.Configuration,
) ([]jsonpatch.JsonPatchOperation, error) {
	matched, err := c.match(ctx, attr, request, namespace, c.matchConditions)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, nil
	}
	// skip mutation only for a full exemption (no Images / AllowedValues). Partial
	// exceptions must not skip digest pinning for the whole resource — same rule as
	// Evaluate, so validating and mutating paths stay aligned.
	for _, polex := range c.exceptions {
		match, err := c.match(ctx, attr, request, namespace, polex.MatchConditions)
		if err != nil {
			return nil, err
		}
		if match {
			if len(polex.Exception.Spec.Images) > 0 || len(polex.Exception.Spec.AllowedValues) > 0 {
				continue
			}
			return nil, nil
		}
	}

	// images are extracted from the well-known container fields directly (rather than
	// through the policy's CEL image extractors) because we need the JSON pointer to the
	// image reference within the resource in order to build a patch
	imagesByCategory, err := apiutils.ExtractImagesFromResource(resource, nil, cfg)
	if err != nil {
		return nil, err
	}

	var patches []jsonpatch.JsonPatchOperation
	var errs []error
	for _, infos := range imagesByCategory {
		for _, info := range infos {
			if info.Digest != "" {
				// already pinned to a digest, nothing to do
				continue
			}
			image := info.String()
			if apply, err := matching.MatchImage(image, c.matchImageReferences...); err != nil {
				return nil, err
			} else if !apply {
				continue
			}
			data, err := ictx.Get(ctx, image, c.authOpts, c.nameOpts)
			if err != nil {
				// Record the failure and carry on: an image that cannot be resolved must not
				// cost the images that can their digest. ClusterPolicy pins each image
				// independently for the same reason, appending a RuleError for the one it
				// could not resolve while keeping the patches for the rest
				// (pkg/engine/internal/imageverifier.go).
				errs = append(errs, fmt.Errorf("failed to resolve digest for image %s: %w", image, err))
				continue
			}
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "replace",
				Path:      info.Pointer,
				Value:     image + "@" + data.Digest,
			})
		}
	}
	return patches, multierr.Combine(errs...)
}

func (c *compiledPolicy) evaluateAuditAnnotations(ctx context.Context, data map[string]any) (map[string]string, error) {
	auditAnnotations := make(map[string]string, len(c.auditAnnotations))
	for key, annotation := range c.auditAnnotations {
		out, _, err := annotation.ContextEval(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate auditAnnotation '%s': %w", key, err)
		}
		if outcome, err := utils.ConvertToNative[string](out); err == nil && outcome != "" {
			auditAnnotations[key] = outcome
		} else if err != nil {
			return nil, fmt.Errorf("failed to convert auditAnnotation '%s' expression: %w", key, err)
		}
	}
	return auditAnnotations, nil
}

func (p *compiledPolicy) match(
	ctx context.Context,
	attr admission.Attributes,
	request interface{},
	namespace runtime.Object,
	matchConditions []cel.Program,
) (bool, error) {
	data := make(map[string]any)
	if isK8s(request) {
		namespaceVal, err := objectToResolveVal(namespace)
		if err != nil {
			return false, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
		}
		requestVal, err := convertObjectToUnstructured(request)
		if err != nil {
			return false, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
		}
		objectVal, err := objectToResolveVal(attr.GetObject())
		if err != nil {
			return false, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
		}
		oldObjectVal, err := objectToResolveVal(attr.GetOldObject())
		if err != nil {
			return false, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
		}
		data[engine.NamespaceObjectKey] = namespaceVal
		data[engine.RequestKey] = requestVal.Object
		data[engine.ObjectKey] = objectVal
		data[engine.OldObjectKey] = oldObjectVal
	} else {
		data[engine.ObjectKey] = request
	}
	var errs []error
	for _, matchCondition := range matchConditions {
		// evaluate the condition
		out, _, err := matchCondition.ContextEval(ctx, data)
		// check error
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// try to convert to a bool
		result, err := utils.ConvertToNative[bool](out)
		// check error
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// if condition is false, skip
		if !result {
			return false, nil
		}
	}
	if err := multierr.Combine(errs...); err == nil {
		return true, nil
	} else if p.failurePolicy == admissionregistrationv1.Ignore {
		return false, nil
	} else {
		return false, err
	}
}

func convertObjectToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	if obj == nil || reflect.ValueOf(obj).IsNil() {
		return &unstructured.Unstructured{Object: nil}, nil
	}
	ret, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: ret}, nil
}

func objectToResolveVal(r runtime.Object) (interface{}, error) {
	if r == nil || reflect.ValueOf(r).IsNil() {
		return nil, nil
	}
	v, err := convertObjectToUnstructured(r)
	if err != nil {
		return nil, err
	}
	return v.Object, nil
}

package evaluator

import (
	"context"
	"fmt"
	"slices"
	"strings"

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
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/kyverno/kyverno/pkg/config"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/image/verification/variables"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/kyverno/sdk/extensions/cel/libs/globalcontext"
	"github.com/kyverno/sdk/extensions/cel/libs/http"
	"github.com/kyverno/sdk/extensions/cel/libs/imagedata"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"go.uber.org/multierr"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// Trace is the decision trace for this evaluation. It is nil unless the policy was compiled
	// with tracing on (NewCompilerWithTrace), so callers must nil-check it. The policy/resource
	// header and Scope are unknown at this level and are left for the caller to fill in.
	Trace *trace.Decision
	// Skipped is set when a match condition excluded the resource. Without tracing that case
	// returns a nil result, and it still does; a non-nil skipped result is only returned when
	// tracing is on, so the match traces are not lost. Consumers must treat it exactly like nil.
	Skipped bool
}

type CompiledPolicy interface {
	// Evaluate does not enforce validationConfigurations.required: the evidence may
	// still come from another policy later in the same request. Call
	// EnforceRequired on every passing policy only after all have evaluated.
	Evaluate(context.Context, imagedataloader.ImageContext, imageverifycache.Client, *imageverify.ImageVerificationResults, admission.Attributes, interface{}, runtime.Object, bool, func() (map[string]any, error), libs.Context) (*EvaluationResult, error)
	EnforceRequired(images []string, verifications *imageverify.ImageVerificationResults) error
	// Tracing reports whether Evaluate fills EvaluationResult.Trace.
	Tracing() bool
	MutateDigest(context.Context, imagedataloader.ImageContext, imageverifycache.Client, *imageverify.ImageVerificationResults, admission.Attributes, interface{}, runtime.Object, unstructured.Unstructured, func() (map[string]any, error), config.Configuration, libs.Context) ([]jsonpatch.JsonPatchOperation, error)
}

type compiledPolicy struct {
	namespace            string
	failurePolicy        admissionregistrationv1.FailurePolicyType
	verifyDigest         bool
	matchConditions      []cel.Program
	matchImageReferences []engine.MatchImageReference
	validations          []engine.Validation
	imageExtractors      engine.ImageExtractorProfiles
	attestors            []*variables.CompiledAttestor
	attestationList      map[string]string
	auditAnnotations     map[string]cel.Program
	authOpts             []remote.Option
	nameOpts             []name.Option
	exceptions           []engine.Exception
	variables            map[string]cel.Program
	validationConfig     policiesv1alpha1.ValidationConfiguration
	ivFuncs              *imageverify.IvFuncs
	// trace is whether this policy was compiled for decision tracing. tracedMatchConditions is
	// index-aligned with matchConditions and, like tracedVariables, is empty when trace is off.
	trace                 bool
	tracedMatchConditions []engine.TracedProgram
	tracedVariables       map[string]engine.TracedProgram
}

func (c *compiledPolicy) Tracing() bool {
	return c.trace
}

func (c *compiledPolicy) Evaluate(ctx context.Context, imgCtx imagedataloader.ImageContext, cache imageverifycache.Client, results *imageverify.ImageVerificationResults, attr admission.Attributes, request interface{}, namespace runtime.Object, isK8s bool, requestMapFn func() (map[string]any, error), context libs.Context) (*EvaluationResult, error) {
	data, err := prepareK8sData(attr, request, namespace, isK8s, requestMapFn)
	if err != nil {
		return nil, err
	}
	boundRuntime := imageverify.NewRuntimeForPolicy(c.ivFuncs, imgCtx, cache, results)
	data[imageverify.RuntimeKey] = boundRuntime
	// override the compile-time http context so reused programs see this call's CLI HTTP mocks
	if context != nil {
		data["http"] = http.Context{ContextInterface: libs.NewMockAwareHTTPContext(engine.NewLazyCELHTTPContext(c.namespace), context.GetHTTPMocks())}
	}
	// The traces below are only appended to when c.trace is set, and decision() returns nil
	// otherwise, so with tracing off nothing here changes what Evaluate returns.
	var matchTraces, variableTraces []trace.NamedExpressionTrace
	var validationTraces []trace.ValidationTrace
	// imagesTrace is set once images are extracted, so a failure before that shows no IMAGES
	var imagesTrace *trace.ImagesTrace
	// excludedBy is the match condition that came out false, if any; erroredMatches are the ones
	// that errored or did not return a bool. With failurePolicy Ignore, errors alone skip the
	// policy after every condition has run, so the skip message needs these.
	var excludedBy string
	var erroredMatches []string
	// validating is set once the validations start, so only then are the ones never reached
	// listed as not run
	var validating bool
	// verdict tracks the validation that decides the outcome, or the step that failed before any
	// validation ran
	verdict := trace.VerdictTrace{Status: trace.VerdictPass}
	decision := func() *trace.Decision {
		if !c.trace {
			return nil
		}
		validations := validationTraces
		if validating {
			// evaluation stops at the first validation that does not pass; the rest are listed so
			// the reader sees them, but they are never evaluated just for the trace
			for i := len(validationTraces); i < len(c.validations); i++ {
				validations = append(validations, trace.ValidationTrace{
					Index:           i,
					Status:          trace.VerdictNotRun,
					ExpressionTrace: buildExpressionTrace(c.validations[i].AST, nil, nil, nil),
				})
			}
		}
		return &trace.Decision{Match: matchTraces, Variables: variableTraces, Images: imagesTrace, Validations: validations, Verdict: verdict}
	}
	// failed is every error return from here on: unchanged without tracing, and with tracing the
	// error stays the second return value while the result carries what was traced before it
	failed := func(err error) (*EvaluationResult, error) {
		if !c.trace {
			return nil, err
		}
		verdict.Status, verdict.Message = trace.VerdictError, err.Error()
		return &EvaluationResult{Trace: decision()}, err
	}
	var recordMatch func(int, ref.Val, error)
	if c.trace {
		// out and err are the deciding evaluation's; the tracking twin is only re-run to collect
		// node values for the trace (see compiler.TracedProgram)
		recordMatch = func(i int, out ref.Val, err error) {
			if i >= len(c.tracedMatchConditions) {
				return
			}
			t := c.tracedMatchConditions[i]
			details := engine.TraceDetails(ctx, t.Traced, data, err)
			matchTraces = append(matchTraces, trace.NamedExpressionTrace{
				Name:            t.Name,
				ExpressionTrace: buildExpressionTrace(t.AST, out, details, err),
			})
			if err != nil {
				erroredMatches = append(erroredMatches, t.Name)
			} else if result, err := utils.ConvertToNative[bool](out); err != nil {
				erroredMatches = append(erroredMatches, t.Name)
			} else if !result {
				excludedBy = t.Name
			}
		}
	}
	matched, err := c.match(ctx, data, c.matchConditions, recordMatch)
	if err != nil {
		return failed(err)
	}
	if !matched {
		if !c.trace {
			return nil, nil
		}
		return &EvaluationResult{Skipped: true, Trace: &trace.Decision{
			Match:   matchTraces,
			Verdict: trace.VerdictTrace{Status: trace.VerdictSkip, Message: skipMessage(len(matchTraces), excludedBy, erroredMatches)},
		}}, nil
	}
	// check if the resource matches an exception
	if len(c.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1beta1.PolicyException, 0)
		fullExemptionFound := false
		for _, polex := range c.exceptions {
			match, err := c.match(ctx, data, polex.MatchConditions, nil)
			if err != nil {
				if fullExemptionFound {
					// exception already granted; a broken later exception must not negate it
					continue
				}
				return failed(err)
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
			verdict = trace.VerdictTrace{Status: trace.VerdictSkip, Message: exemptMessage(matchedExceptions)}
			return &EvaluationResult{Exceptions: matchedExceptions, Trace: decision()}, nil
		}
	}
	vars := lazy.NewMapValue(engine.VariablesType)
	for name, variable := range c.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := variable.ContextEval(ctx, data)
			if c.trace {
				if t, ok := c.tracedVariables[name]; ok {
					// out is still the deciding program's value; the twin only explains it. Any
					// variable the re-run reads comes from this same lazy map, so it matches.
					details := engine.TraceDetails(ctx, t.Traced, data, err)
					// variables are lazy, so this records them in the order they are first read
					variableTraces = append(variableTraces, trace.NamedExpressionTrace{
						Name:            name,
						ExpressionTrace: buildExpressionTrace(t.AST, out, details, err),
					})
				}
			}
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
		data[engine.VariablesKey] = vars
		// The activation overrides the Lib's cel.Globals binding, so confine here too
		// or a namespaced policy would reach the raw context at evaluation time.
		data[engine.GlobalContextKey] = globalcontext.Context{ContextInterface: engine.ConfineGlobalContext(context, c.namespace)}
		data[engine.ImageDataKey] = imagedata.Context{ContextInterface: context} // the thing that actually does the fetching and validation of images
		data[engine.ResourceKey] = resource.Context{ContextInterface: context}
	}

	var gvr *metav1.GroupVersionResource
	if req, ok := request.(*admissionv1.AdmissionRequest); ok {
		gvr = req.RequestResource
	}
	images, err := engine.ExtractImages(data, c.imageExtractors.ForResource(gvr))
	if err != nil {
		return failed(err)
	}
	filteredImages := make(map[string][]string, len(images))
	imgList := []string{}
	for category, imgs := range images {
		filteredImages[category] = []string{} // ensure image.containers is always [] in CEL
		for _, img := range imgs {
			apply, err := matching.MatchImage(img, c.matchImageReferences...)
			if err != nil {
				return failed(err)
			}
			if c.trace {
				if imagesTrace == nil {
					imagesTrace = &trace.ImagesTrace{}
				}
				imagesTrace.Found = append(imagesTrace.Found, trace.ImageTrace{Category: category, Image: img, Checked: apply})
			}
			if apply {
				filteredImages[category] = append(filteredImages[category], img)
				imgList = append(imgList, img)
			}
		}
	}
	if c.trace {
		if imagesTrace == nil {
			imagesTrace = &trace.ImagesTrace{}
		}
		// images is a map, so put the categories in a stable order; within one, keep the
		// extractor's own order
		slices.SortStableFunc(imagesTrace.Found, func(a, b trace.ImageTrace) int { return strings.Compare(a.Category, b.Category) })
	}

	// not reset: verification results are shared across the request, an earlier
	// policy's verification must stay visible to the catch-all required policy

	// when we get here, we will be initialized with the global opts from the compiled policy
	// or from the credentials configured on the policy itself. the latter replaces the first
	result, err := c.checkDigests(imgList)
	if err != nil {
		return failed(err)
	}
	if result != nil {
		verdict.Status, verdict.Message = trace.VerdictFail, result.Message
		result.Trace = decision()
		return result, nil
	}

	// Prefetch image data through Get() one image at a time to avoid triggering
	// racy concurrent map writes in the SDK AddImages() implementation.
	for _, image := range imgList {
		if _, err := imgCtx.Get(ctx, image, c.authOpts, c.nameOpts); err != nil {
			return failed(err)
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

	validating = true
	ran := func(index int, status string) {
		if c.trace {
			validationTraces = append(validationTraces, trace.ValidationTrace{Index: index, Status: status, ExpressionTrace: verdict.ExpressionTrace})
		}
	}
	for i, v := range c.validations {
		boundRuntime.BeginValidation()
		out, _, err := v.Program.ContextEval(ctx, data)
		diagnostics := boundRuntime.VerificationDiagnostics()
		if c.trace {
			// diagnostics are already snapshotted, so the explain-only re-run cannot add to them,
			// and an expression that verifies images has no twin at all (see
			// withoutVerificationTwin), so it is never re-run
			details := engine.TraceDetails(ctx, v.Traced, data, err)
			verdict = trace.VerdictTrace{
				Status:          trace.VerdictPass,
				ExpressionTrace: buildExpressionTrace(v.AST, out, details, err),
			}
		}
		if err != nil {
			ran(i, trace.VerdictError)
			return failed(err)
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
			message += diagnostics
			ran(i, trace.VerdictFail)
			verdict.Status, verdict.Message = trace.VerdictFail, message
			auditAnnotations, err := c.evaluateAuditAnnotations(ctx, data)
			if err != nil {
				return failed(err)
			}
			return &EvaluationResult{
				Result:           outcome,
				Message:          message,
				AuditAnnotations: auditAnnotations,
				Index:            i,
				Error:            err,
				MatchedImages:    imgList,
				Trace:            decision(),
			}, nil
		} else if err != nil {
			ran(i, trace.VerdictError)
			verdict.Status, verdict.Message = trace.VerdictError, err.Error()
			return &EvaluationResult{Error: err, Trace: decision()}, nil
		}
		ran(i, trace.VerdictPass)
	}

	auditAnnotations, err := c.evaluateAuditAnnotations(ctx, data)
	if err != nil {
		return failed(err)
	}
	// required is enforced by the caller via EnforceRequired, not here
	return &EvaluationResult{Result: true, AuditAnnotations: auditAnnotations, MatchedImages: imgList, Trace: decision()}, nil
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
func (c *compiledPolicy) EnforceRequired(images []string, verifications *imageverify.ImageVerificationResults) error {
	if c.validationConfig.Required != nil && !*c.validationConfig.Required {
		return nil
	}
	for _, image := range images {
		verified, attempted := verifications.Status(image)
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
	imgCtx imagedataloader.ImageContext,
	cache imageverifycache.Client,
	results *imageverify.ImageVerificationResults,
	attr admission.Attributes,
	request interface{},
	namespace runtime.Object,
	resource unstructured.Unstructured,
	requestMapFn func() (map[string]any, error),
	cfg config.Configuration,
	libctx libs.Context,
) ([]jsonpatch.JsonPatchOperation, error) {
	data, err := prepareK8sData(attr, request, namespace, isK8s(request), requestMapFn)
	if err != nil {
		return nil, err
	}
	data[imageverify.RuntimeKey] = imageverify.NewRuntimeForPolicy(c.ivFuncs, imgCtx, cache, results)
	// override the compile-time http context so reused programs see this call's CLI HTTP mocks
	if libctx != nil {
		data["http"] = http.Context{ContextInterface: libs.NewMockAwareHTTPContext(engine.NewLazyCELHTTPContext(c.namespace), libctx.GetHTTPMocks())}
	}
	matched, err := c.match(ctx, data, c.matchConditions, nil)
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
		match, err := c.match(ctx, data, polex.MatchConditions, nil)
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
			data, err := imgCtx.Get(ctx, image, c.authOpts, c.nameOpts)
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

// match evaluates matchConditions against activation data assembled once by the
// caller (Evaluate or MutateDigest, via prepareK8sData) -- it does not build or
// convert anything itself, so it can be called once for the policy's own
// matchConditions and once per exception without repeating the request-map
// build or the object/oldObject conversion.
func (p *compiledPolicy) match(
	ctx context.Context,
	data map[string]any,
	matchConditions []cel.Program,
	record func(index int, out ref.Val, err error),
) (bool, error) {
	var errs []error
	for i, matchCondition := range matchConditions {
		// evaluate the condition
		out, _, err := matchCondition.ContextEval(ctx, data)
		if record != nil {
			record(i, out, err)
		}
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

// prepareK8sData assembles CEL activation data once per Evaluate/MutateDigest call.
// requestMapFn is a memoized request-map builder shared across every policy in the
// admission request; pass nil to build the map from request instead (JSON mode,
// ExtractionMode synthetic requests). The request map MUST be treated as immutable.
//
// TODO(#17586): this helper is a twin of vpol's prepareK8sData
// (pkg/cel/policies/vpol/compiler/eval.go). Both should move behind a shared
// choke point in pkg/cel/compiler so a fourth engine cannot reintroduce the
// per-policy rebuild this design eliminates here and in #17572.
func prepareK8sData(
	attr admission.Attributes,
	request interface{},
	namespace runtime.Object,
	isK8s bool,
	requestMapFn func() (map[string]any, error),
) (map[string]any, error) {
	data := map[string]any{}
	if !isK8s {
		data[engine.ObjectKey] = request
		return data, nil
	}
	namespaceVal, err := utils.ObjectToResolveVal(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
	}
	objectVal, err := utils.ObjectToResolveVal(attr.GetObject())
	if err != nil {
		return nil, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
	}
	oldObjectVal, err := utils.ObjectToResolveVal(attr.GetOldObject())
	if err != nil {
		return nil, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
	}
	var requestMap map[string]any
	if requestMapFn != nil {
		requestMap, err = requestMapFn()
	} else {
		admissionReq, _ := request.(*admissionv1.AdmissionRequest)
		requestMap, err = engine.BuildRawRequestMap(admissionReq)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
	}
	data[engine.NamespaceObjectKey] = namespaceVal
	data[engine.RequestKey] = requestMap
	data[engine.ObjectKey] = objectVal
	data[engine.OldObjectKey] = oldObjectVal
	return data, nil
}

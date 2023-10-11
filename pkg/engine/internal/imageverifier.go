package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/cosign"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/notary"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/kyverno/kyverno/pkg/utils/jsonpointer"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"go.uber.org/multierr"
	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ImageVerifier struct {
	logger                   logr.Logger
	rclient                  engineapi.RegistryClient
	ivCache                  imageverifycache.Client
	policyContext            engineapi.PolicyContext
	rule                     kyvernov1.Rule
	ivm                      *engineapi.ImageVerificationMetadata
	imageSignatureRepository string
}

func NewImageVerifier(
	logger logr.Logger,
	rclient engineapi.RegistryClient,
	ivCache imageverifycache.Client,
	policyContext engineapi.PolicyContext,
	rule kyvernov1.Rule,
	ivm *engineapi.ImageVerificationMetadata,
	imageSignatureRepository string,
) *ImageVerifier {
	return &ImageVerifier{
		logger:                   logger,
		rclient:                  rclient,
		ivCache:                  ivCache,
		policyContext:            policyContext,
		rule:                     rule,
		ivm:                      ivm,
		imageSignatureRepository: imageSignatureRepository,
	}
}

func HasImageVerifiedAnnotationChanged(ctx engineapi.PolicyContext, log logr.Logger) bool {
	newResource := ctx.NewResource()
	oldResource := ctx.OldResource()
	if newResource.Object == nil || oldResource.Object == nil {
		return false
	}
	newValue := newResource.GetAnnotations()[kyverno.AnnotationImageVerify]
	oldValue := oldResource.GetAnnotations()[kyverno.AnnotationImageVerify]
	if newValue == oldValue {
		return false
	}
	var newValueObj, oldValueObj map[string]bool
	err := json.Unmarshal([]byte(newValue), &newValueObj)
	if err != nil {
		log.Error(err, "failed to parse new resource annotation.")
		return true
	}
	err = json.Unmarshal([]byte(oldValue), &oldValueObj)
	if err != nil {
		log.Error(err, "failed to parse old resource annotation.")
		return true
	}
	for img := range oldValueObj {
		_, found := newValueObj[img]
		if found {
			result := newValueObj[img] != oldValueObj[img]
			if result {
				log.V(2).Info("annotation mismatch", "oldValue", oldValue, "newValue", newValue, "key", kyverno.AnnotationImageVerify)
				return result
			}
		}
	}
	return false
}

func matchImageReferences(imageReferences []string, image string) bool {
	for _, imageRef := range imageReferences {
		if wildcard.Match(imageRef, image) {
			return true
		}
	}
	return false
}

func isImageVerified(resource unstructured.Unstructured, image string, log logr.Logger) (bool, error) {
	if resource.Object == nil {
		return false, fmt.Errorf("nil resource")
	}
	annotations := resource.GetAnnotations()
	if len(annotations) == 0 {
		return false, nil
	}
	data, ok := annotations[kyverno.AnnotationImageVerify]
	if !ok {
		log.V(2).Info("missing image metadata in annotation", "key", kyverno.AnnotationImageVerify)
		return false, fmt.Errorf("image is not verified")
	}
	ivm, err := engineapi.ParseImageMetadata(data)
	if err != nil {
		log.Error(err, "failed to parse image verification metadata", "data", data)
		return false, fmt.Errorf("failed to parse image metadata: %w", err)
	}
	return ivm.IsVerified(image), nil
}

func ExpandStaticKeys(attestorSet kyvernov1.AttestorSet) kyvernov1.AttestorSet {
	var entries []kyvernov1.Attestor
	for _, e := range attestorSet.Entries {
		if e.Keys != nil {
			keys := splitPEM(e.Keys.PublicKeys)
			if len(keys) > 1 {
				moreEntries := createStaticKeyAttestors(keys)
				entries = append(entries, moreEntries...)
				continue
			}
		}
		entries = append(entries, e)
	}
	return kyvernov1.AttestorSet{
		Count:   attestorSet.Count,
		Entries: entries,
	}
}

func splitPEM(pem string) []string {
	keys := strings.SplitAfter(pem, "-----END PUBLIC KEY-----")
	if len(keys) < 1 {
		return keys
	}
	return keys[0 : len(keys)-1]
}

func createStaticKeyAttestors(keys []string) []kyvernov1.Attestor {
	var attestors []kyvernov1.Attestor
	for _, k := range keys {
		a := kyvernov1.Attestor{
			Keys: &kyvernov1.StaticKeyAttestor{
				PublicKeys: k,
			},
		}
		attestors = append(attestors, a)
	}
	return attestors
}

func buildStatementMap(statements []map[string]interface{}) (map[string][]map[string]interface{}, []string) {
	results := map[string][]map[string]interface{}{}
	var predicateTypes []string
	for _, s := range statements {
		predicateType := s["type"].(string)
		if results[predicateType] != nil {
			results[predicateType] = append(results[predicateType], s)
		} else {
			results[predicateType] = []map[string]interface{}{s}
		}
		predicateTypes = append(predicateTypes, predicateType)
	}
	return results, predicateTypes
}

func makeAddDigestPatch(imageInfo apiutils.ImageInfo, digest string) jsonpatch.JsonPatchOperation {
	return jsonpatch.JsonPatchOperation{
		Operation: "replace",
		Path:      imageInfo.Pointer,
		Value:     imageInfo.String() + "@" + digest,
	}
}

func EvaluateConditions(
	conditions []kyvernov1.AnyAllConditions,
	ctx enginecontext.Interface,
	s map[string]interface{},
	log logr.Logger,
) (bool, string, error) {
	predicate, ok := s["predicate"].(map[string]interface{})
	if !ok {
		return false, "", fmt.Errorf("failed to extract predicate from statement: %v", s)
	}
	if err := enginecontext.AddJSONObject(ctx, predicate); err != nil {
		return false, "", fmt.Errorf("failed to add Statement to the context %v: %w", s, err)
	}
	c, err := variables.SubstituteAllInConditions(log, ctx, conditions)
	if err != nil {
		return false, "", fmt.Errorf("failed to substitute variables in attestation conditions: %w", err)
	}
	return variables.EvaluateAnyAllConditions(log, ctx, c)
}

// verify applies policy rules to each matching image. The policy rule results and annotation patches are
// added to tme imageVerifier `resp` and `ivm` fields.
func (iv *ImageVerifier) Verify(
	ctx context.Context,
	imageVerify kyvernov1.ImageVerification,
	matchedImageInfos []apiutils.ImageInfo,
	cfg config.Configuration,
) ([]jsonpatch.JsonPatchOperation, []*engineapi.RuleResponse) {
	var responses []*engineapi.RuleResponse
	var patches []jsonpatch.JsonPatchOperation

	// for backward compatibility
	imageVerify = *imageVerify.Convert()

	for _, imageInfo := range matchedImageInfos {
		image := imageInfo.String()

		if HasImageVerifiedAnnotationChanged(iv.policyContext, iv.logger) {
			msg := kyverno.AnnotationImageVerify + " annotation cannot be changed"
			iv.logger.Info("image verification error", "reason", msg)
			responses = append(responses, engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, msg))
			continue
		}

		pointer := jsonpointer.ParsePath(imageInfo.Pointer).JMESPath()
		changed, err := iv.policyContext.JSONContext().HasChanged(pointer)
		if err == nil && !changed {
			iv.logger.V(4).Info("no change in image, skipping check", "image", image)
			iv.ivm.Add(image, true)
			continue
		}

		verified, err := isImageVerified(iv.policyContext.NewResource(), image, iv.logger)
		if err == nil && verified {
			iv.logger.Info("image was previously verified, skipping check", "image", image)
			iv.ivm.Add(image, true)
			continue
		}
		start := time.Now()
		isInCache := false
		if iv.ivCache != nil {
			found, err := iv.ivCache.Get(ctx, iv.policyContext.Policy(), iv.rule.Name, image)
			if err != nil {
				iv.logger.Error(err, "error occurred during cache get")
			} else {
				isInCache = found
			}
		}

		var ruleResp *engineapi.RuleResponse
		var digest string
		if isInCache {
			iv.logger.V(2).Info("cache entry found", "namespace", iv.policyContext.Policy().GetNamespace(), "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name, "imageRef", image)
			ruleResp = engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, "verified from cache")
			digest = imageInfo.Digest
		} else {
			iv.logger.V(2).Info("cache entry not found", "namespace", iv.policyContext.Policy().GetNamespace(), "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name, "imageRef", image)
			ruleResp, digest = iv.verifyImage(ctx, imageVerify, imageInfo, cfg)
			if ruleResp != nil && ruleResp.Status() == engineapi.RuleStatusPass {
				if iv.ivCache != nil {
					setted, err := iv.ivCache.Set(ctx, iv.policyContext.Policy(), iv.rule.Name, image)
					if err != nil {
						iv.logger.Error(err, "error occurred during cache set")
					} else {
						if setted {
							iv.logger.V(4).Info("successfully set cache", "namespace", iv.policyContext.Policy().GetNamespace(), "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name, "imageRef", image)
						}
					}
				}
			}
		}
		iv.logger.V(4).Info("time taken by the image verify operation", "duration", time.Since(start))

		if imageVerify.MutateDigest {
			patch, retrievedDigest, err := iv.handleMutateDigest(ctx, digest, imageInfo)
			if err != nil {
				responses = append(responses, engineapi.RuleError(iv.rule.Name, engineapi.ImageVerify, "failed to update digest", err))
			} else if patch != nil {
				if ruleResp == nil {
					ruleResp = engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, "mutated image digest")
				}
				patches = append(patches, *patch)
				imageInfo.Digest = retrievedDigest
				image = imageInfo.String()
			}
		}

		if ruleResp != nil {
			if len(imageVerify.Attestors) > 0 || len(imageVerify.Attestations) > 0 {
				iv.ivm.Add(image, ruleResp.Status() == engineapi.RuleStatusPass)
			}
			responses = append(responses, ruleResp)
		}
	}
	return patches, responses
}

func (iv *ImageVerifier) verifyImage(
	ctx context.Context,
	imageVerify kyvernov1.ImageVerification,
	imageInfo apiutils.ImageInfo,
	cfg config.Configuration,
) (*engineapi.RuleResponse, string) {
	if len(imageVerify.Attestors) <= 0 && len(imageVerify.Attestations) <= 0 {
		return nil, ""
	}
	image := imageInfo.String()
	for _, att := range imageVerify.Attestations {
		if att.Type == "" && att.PredicateType != "" {
			att.Type = att.PredicateType
		}
	}
	iv.logger.V(2).Info("verifying image signatures", "image", image, "attestors", len(imageVerify.Attestors), "attestations", len(imageVerify.Attestations))
	if err := iv.policyContext.JSONContext().AddImageInfo(imageInfo, cfg); err != nil {
		iv.logger.Error(err, "failed to add image to context")
		return engineapi.RuleError(iv.rule.Name, engineapi.ImageVerify, fmt.Sprintf("failed to add image to context %s", image), err), ""
	}
	if len(imageVerify.Attestors) > 0 {
		if !matchImageReferences(imageVerify.ImageReferences, image) {
			return nil, ""
		}
		ruleResp, cosignResp := iv.verifyAttestors(ctx, imageVerify.Attestors, imageVerify, imageInfo, "")
		if ruleResp.Status() != engineapi.RuleStatusPass {
			return ruleResp, ""
		}
		if imageInfo.Digest == "" {
			imageInfo.Digest = cosignResp.Digest
		}
		if len(imageVerify.Attestations) == 0 {
			return ruleResp, cosignResp.Digest
		}
	}

	return iv.verifyAttestations(ctx, imageVerify, imageInfo)
}

func (iv *ImageVerifier) verifyAttestors(
	ctx context.Context,
	attestors []kyvernov1.AttestorSet,
	imageVerify kyvernov1.ImageVerification,
	imageInfo apiutils.ImageInfo,
	predicateType string,
) (*engineapi.RuleResponse, *images.Response) {
	var cosignResponse *images.Response
	image := imageInfo.String()
	for i, attestorSet := range attestors {
		var err error
		path := fmt.Sprintf(".attestors[%d]", i)
		iv.logger.V(4).Info("verifying attestors", "path", path)
		cosignResponse, err = iv.verifyAttestorSet(ctx, attestorSet, imageVerify, imageInfo, path)
		if err != nil {
			iv.logger.Error(err, "failed to verify image")
			return iv.handleRegistryErrors(image, err), nil
		}
	}
	if cosignResponse == nil {
		return engineapi.RuleError(iv.rule.Name, engineapi.ImageVerify, "invalid response", fmt.Errorf("nil")), nil
	}
	msg := fmt.Sprintf("verified image signatures for %s", image)
	return engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, msg), cosignResponse
}

// handle registry network errors as a rule error (instead of a policy failure)
func (iv *ImageVerifier) handleRegistryErrors(image string, err error) *engineapi.RuleResponse {
	msg := fmt.Sprintf("failed to verify image %s: %s", image, err.Error())
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return engineapi.RuleError(iv.rule.Name, engineapi.ImageVerify, fmt.Sprintf("failed to verify image %s", image), err)
	}
	return engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, msg)
}

func (iv *ImageVerifier) verifyAttestations(
	ctx context.Context,
	imageVerify kyvernov1.ImageVerification,
	imageInfo apiutils.ImageInfo,
) (*engineapi.RuleResponse, string) {
	image := imageInfo.String()
	for i, attestation := range imageVerify.Attestations {
		var attestationError error
		path := fmt.Sprintf(".attestations[%d]", i)

		iv.logger.V(2).Info(fmt.Sprintf("attestation %+v", attestation))
		if attestation.Type == "" && attestation.PredicateType == "" {
			return engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, path+": missing type"), ""
		}

		if attestation.Type == "" && attestation.PredicateType != "" {
			attestation.Type = attestation.PredicateType
		}

		if len(attestation.Attestors) == 0 {
			// add an empty attestor to allow fetching and checking attestations
			attestation.Attestors = []kyvernov1.AttestorSet{{Entries: []kyvernov1.Attestor{{}}}}
		}

		for j, attestor := range attestation.Attestors {
			attestorPath := fmt.Sprintf("%s.attestors[%d]", path, j)
			requiredCount := attestor.RequiredCount()
			verifiedCount := 0

			for _, a := range attestor.Entries {
				entryPath := fmt.Sprintf("%s.entries[%d]", attestorPath, i)
				v, opts, subPath := iv.buildVerifier(a, imageVerify, image, &imageVerify.Attestations[i])
				cosignResp, err := v.FetchAttestations(ctx, *opts)
				if err != nil {
					iv.logger.Error(err, "failed to fetch attestations")
					return iv.handleRegistryErrors(image, err), ""
				}

				if imageInfo.Digest == "" {
					imageInfo.Digest = cosignResp.Digest
					image = imageInfo.String()
				}

				attestationError = iv.verifyAttestation(cosignResp.Statements, attestation, imageInfo)
				if attestationError != nil {
					attestationError = fmt.Errorf("%s: %w", entryPath+subPath, attestationError)
					return engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, attestationError.Error()), ""
				}

				verifiedCount++
				if verifiedCount >= requiredCount {
					iv.logger.V(2).Info("image attestations verification succeeded", "verifiedCount", verifiedCount, "requiredCount", requiredCount)
					break
				}
			}

			if verifiedCount < requiredCount {
				msg := fmt.Sprintf("image attestations verification failed, verifiedCount: %v, requiredCount: %v", verifiedCount, requiredCount)
				return engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, msg), ""
			}
		}

		iv.logger.V(4).Info("attestation checks passed", "path", path, "image", imageInfo.String(), "type", attestation.Type)
	}

	msg := fmt.Sprintf("verified image attestations for %s", image)
	iv.logger.V(2).Info(msg)
	return engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, msg), imageInfo.Digest
}

func (iv *ImageVerifier) verifyAttestorSet(
	ctx context.Context,
	attestorSet kyvernov1.AttestorSet,
	imageVerify kyvernov1.ImageVerification,
	imageInfo apiutils.ImageInfo,
	path string,
) (*images.Response, error) {
	var errorList []error
	verifiedCount := 0
	attestorSet = ExpandStaticKeys(attestorSet)
	requiredCount := attestorSet.RequiredCount()
	image := imageInfo.String()

	for i, a := range attestorSet.Entries {
		var entryError error
		var cosignResp *images.Response
		attestorPath := fmt.Sprintf("%s.entries[%d]", path, i)
		iv.logger.V(4).Info("verifying attestorSet", "path", attestorPath)

		if a.Attestor != nil {
			nestedAttestorSet, err := kyvernov1.AttestorSetUnmarshal(a.Attestor)
			if err != nil {
				entryError = fmt.Errorf("failed to unmarshal nested attestor %s: %w", attestorPath, err)
			} else {
				attestorPath += ".attestor"
				cosignResp, entryError = iv.verifyAttestorSet(ctx, *nestedAttestorSet, imageVerify, imageInfo, attestorPath)
			}
		} else {
			v, opts, subPath := iv.buildVerifier(a, imageVerify, image, nil)
			cosignResp, entryError = v.VerifySignature(ctx, *opts)
			if entryError != nil {
				entryError = fmt.Errorf("%s: %w", attestorPath+subPath, entryError)
			}
		}

		if entryError == nil {
			verifiedCount++
			if verifiedCount >= requiredCount {
				iv.logger.V(2).Info("image attestors verification succeeded", "verifiedCount", verifiedCount, "requiredCount", requiredCount)
				return cosignResp, nil
			}
		} else {
			errorList = append(errorList, entryError)
		}
	}

	err := multierr.Combine(errorList...)
	iv.logger.Info("image attestors verification failed", "verifiedCount", verifiedCount, "requiredCount", requiredCount, "errors", err.Error())
	return nil, err
}

func (iv *ImageVerifier) buildVerifier(
	attestor kyvernov1.Attestor,
	imageVerify kyvernov1.ImageVerification,
	image string,
	attestation *kyvernov1.Attestation,
) (images.ImageVerifier, *images.Options, string) {
	switch imageVerify.Type {
	case kyvernov1.Notary:
		return iv.buildNotaryVerifier(attestor, imageVerify, image, attestation)
	default:
		return iv.buildCosignVerifier(attestor, imageVerify, image, attestation)
	}
}

func (iv *ImageVerifier) buildCosignVerifier(
	attestor kyvernov1.Attestor,
	imageVerify kyvernov1.ImageVerification,
	image string,
	attestation *kyvernov1.Attestation,
) (images.ImageVerifier, *images.Options, string) {
	path := ""
	repository := iv.imageSignatureRepository
	if imageVerify.Repository != "" {
		repository = imageVerify.Repository
	}
	opts := &images.Options{
		ImageRef:    image,
		Repository:  repository,
		Annotations: imageVerify.Annotations,
		Client:      iv.rclient,
	}

	if imageVerify.Roots != "" {
		opts.Roots = imageVerify.Roots
	}

	if attestation != nil {
		opts.PredicateType = attestation.PredicateType
		opts.Type = attestation.Type
		opts.IgnoreSCT = true // TODO: Add option to allow SCT when attestors are not provided
		if attestation.PredicateType != "" && attestation.Type == "" {
			iv.logger.Info("predicate type has been deprecated, please use type instead")
			opts.Type = attestation.PredicateType
		}
		opts.FetchAttestations = true
	}

	if attestor.Keys != nil {
		path = path + ".keys"
		if attestor.Keys.PublicKeys != "" {
			opts.Key = attestor.Keys.PublicKeys
		} else if attestor.Keys.Secret != nil {
			opts.Key = fmt.Sprintf("k8s://%s/%s", attestor.Keys.Secret.Namespace, attestor.Keys.Secret.Name)
		} else if attestor.Keys.KMS != "" {
			opts.Key = attestor.Keys.KMS
		}
		if attestor.Keys.Rekor != nil {
			opts.RekorURL = attestor.Keys.Rekor.URL
			opts.RekorPubKey = attestor.Keys.Rekor.RekorPubKey
			opts.IgnoreTlog = attestor.Keys.Rekor.IgnoreTlog
		} else {
			opts.RekorURL = "https://rekor.sigstore.dev"
			opts.IgnoreTlog = false
		}

		if attestor.Keys.CTLog != nil {
			opts.IgnoreSCT = attestor.Keys.CTLog.IgnoreSCT
			opts.CTLogsPubKey = attestor.Keys.CTLog.CTLogPubKey
		} else {
			opts.IgnoreSCT = false
		}

		opts.SignatureAlgorithm = attestor.Keys.SignatureAlgorithm
	} else if attestor.Certificates != nil {
		path = path + ".certificates"
		opts.Cert = attestor.Certificates.Certificate
		opts.CertChain = attestor.Certificates.CertificateChain
		if attestor.Certificates.Rekor != nil {
			opts.RekorURL = attestor.Certificates.Rekor.URL
		}
	} else if attestor.Keyless != nil {
		path = path + ".keyless"
		if attestor.Keyless.Rekor != nil {
			opts.RekorURL = attestor.Keyless.Rekor.URL
			opts.RekorPubKey = attestor.Keyless.Rekor.RekorPubKey
			opts.IgnoreTlog = attestor.Keyless.Rekor.IgnoreTlog
		} else {
			opts.RekorURL = "https://rekor.sigstore.dev"
			opts.IgnoreTlog = false
		}

		if attestor.Keyless.CTLog != nil {
			opts.IgnoreSCT = attestor.Keyless.CTLog.IgnoreSCT
			opts.CTLogsPubKey = attestor.Keyless.CTLog.CTLogPubKey
		} else {
			opts.IgnoreSCT = false
		}

		opts.Roots = attestor.Keyless.Roots
		opts.Issuer = attestor.Keyless.Issuer
		opts.Subject = attestor.Keyless.Subject
		opts.AdditionalExtensions = attestor.Keyless.AdditionalExtensions
	}

	if attestor.Repository != "" {
		opts.Repository = attestor.Repository
	}

	if attestor.Annotations != nil {
		opts.Annotations = attestor.Annotations
	}

	return cosign.NewVerifier(), opts, path
}

func (iv *ImageVerifier) buildNotaryVerifier(
	attestor kyvernov1.Attestor,
	imageVerify kyvernov1.ImageVerification,
	image string,
	attestation *kyvernov1.Attestation,
) (images.ImageVerifier, *images.Options, string) {
	path := ""
	opts := &images.Options{
		ImageRef:  image,
		Cert:      attestor.Certificates.Certificate,
		CertChain: attestor.Certificates.CertificateChain,
		Client:    iv.rclient,
	}

	if attestation != nil {
		opts.Type = attestation.Type
		opts.PredicateType = attestation.PredicateType
		if attestation.PredicateType != "" && attestation.Type == "" {
			iv.logger.Info("predicate type has been deprecated, please use type instead")
			opts.Type = attestation.PredicateType
		}
		opts.FetchAttestations = true
	}

	if attestor.Repository != "" {
		opts.Repository = attestor.Repository
	}

	if attestor.Annotations != nil {
		opts.Annotations = attestor.Annotations
	}

	return notary.NewVerifier(), opts, path
}

func (iv *ImageVerifier) verifyAttestation(statements []map[string]interface{}, attestation kyvernov1.Attestation, imageInfo apiutils.ImageInfo) error {
	if attestation.Type == "" && attestation.PredicateType == "" {
		return fmt.Errorf("a type is required")
	}
	image := imageInfo.String()
	statementsByPredicate, types := buildStatementMap(statements)
	iv.logger.V(4).Info("checking attestations", "predicates", types, "image", image)
	statements = statementsByPredicate[attestation.Type]
	if statements == nil {
		iv.logger.Info("no attestations found for predicate", "type", attestation.Type, "predicates", types, "image", imageInfo.String())
		return fmt.Errorf("attestions not found for predicate type %s", attestation.Type)
	}
	for _, s := range statements {
		iv.logger.Info("checking attestation", "predicates", types, "image", imageInfo.String())
		val, msg, err := iv.checkAttestations(attestation, s)
		if err != nil {
			return fmt.Errorf("failed to check attestations: %w", err)
		}
		if !val {
			return fmt.Errorf("attestation checks failed for %s and predicate %s: %s", imageInfo.String(), attestation.Type, msg)
		}
	}
	return nil
}

func (iv *ImageVerifier) checkAttestations(a kyvernov1.Attestation, s map[string]interface{}) (bool, string, error) {
	if len(a.Conditions) == 0 {
		return true, "", nil
	}
	iv.policyContext.JSONContext().Checkpoint()
	defer iv.policyContext.JSONContext().Restore()
	return EvaluateConditions(a.Conditions, iv.policyContext.JSONContext(), s, iv.logger)
}

func (iv *ImageVerifier) handleMutateDigest(ctx context.Context, digest string, imageInfo apiutils.ImageInfo) (*jsonpatch.JsonPatchOperation, string, error) {
	if imageInfo.Digest != "" {
		return nil, "", nil
	}
	if digest == "" {
		desc, err := iv.rclient.FetchImageDescriptor(ctx, imageInfo.String())
		if err != nil {
			return nil, "", err
		}
		digest = desc.Digest.String()
	}
	patch := makeAddDigestPatch(imageInfo, digest)
	iv.logger.V(4).Info("adding digest patch", "image", imageInfo.String(), "patch", patch.Json())
	return &patch, digest, nil
}

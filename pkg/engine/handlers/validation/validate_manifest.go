package validation

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineresources "github.com/kyverno/kyverno/pkg/engine/resources"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DefaultAnnotationKeyDomain = "cosign.sigstore.dev"
	CosignEnvVariable          = "COSIGN_EXPERIMENTAL"
)

type validateManifestHandler struct {
	client engineapi.Client
}

func NewValidateManifestHandler(
	policyContext engineapi.PolicyContext,
	client engineapi.Client,
) (handlers.Handler, error) {
	if engineutils.IsDeleteRequest(policyContext) {
		return nil, nil
	}
	return validateManifestHandler{
		client: client,
	}, nil
}

func (h validateManifestHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	_ engineapi.EngineContextLoader,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	// verify manifest
	verified, reason, err := h.verifyManifest(ctx, logger, policyContext, *rule.Validation.Manifests)
	if err != nil {
		logger.V(3).Info("verifyManifest return err", "error", err.Error())
		return resource, handlers.WithError(rule, engineapi.Validation, "error occurred during manifest verification", err)
	}
	logger.V(3).Info("verifyManifest result", "verified", strconv.FormatBool(verified), "reason", reason)
	if !verified {
		return resource, handlers.WithFail(rule, engineapi.Validation, reason)
	}
	return resource, handlers.WithPass(rule, engineapi.Validation, reason)
}

func (h validateManifestHandler) verifyManifest(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	verifyRule kyvernov1.Manifests,
) (bool, string, error) {
	// load AdmissionRequest
	request, err := policyContext.JSONContext().Query("request")
	if err != nil {
		return false, "", fmt.Errorf("failed to get a request from policyContext: %w", err)
	}
	reqByte, _ := json.Marshal(request)
	var adreq *admissionv1.AdmissionRequest
	err = json.Unmarshal(reqByte, &adreq)
	if err != nil {
		return false, "", fmt.Errorf("failed to unmarshal a request from requestByte: %w", err)
	}
	// unmarshal admission request object
	var resource unstructured.Unstructured
	objectBytes := adreq.Object.Raw
	err = json.Unmarshal(objectBytes, &resource)
	if err != nil {
		return false, "", fmt.Errorf("failed to Unmarshal a requested object: %w", err)
	}

	logger.V(4).Info("verifying manifest", "namespace", adreq.Namespace, "kind", adreq.Kind.Kind,
		"name", adreq.Name, "username", adreq.UserInfo.Username)

	// allow dryrun request
	if adreq.DryRun != nil && *adreq.DryRun {
		return true, "allowed because of DryRun request", nil
	}

	// prepare verifyResource option
	vo := &k8smanifest.VerifyResourceOption{}
	// adding default ignoreFields from
	// github.com/sigstore/k8s-manifest-sigstore/blob/main/pkg/k8smanifest/resources/default-config.yaml
	vo = k8smanifest.AddDefaultConfig(vo)
	// adding default ignoreFields from pkg/engine/resources/default-config.yaml
	vo = addDefaultConfig(vo)
	// adding ignoreFields from Policy
	for _, i := range verifyRule.IgnoreFields {
		converted := k8smanifest.ObjectFieldBinding(i)
		vo.IgnoreFields = append(vo.IgnoreFields, converted)
	}

	// dryrun setting
	vo.DisableDryRun = !verifyRule.DryRunOption.Enable
	if verifyRule.DryRunOption.Namespace != "" {
		vo.DryRunNamespace = verifyRule.DryRunOption.Namespace
	} else {
		vo.DryRunNamespace = config.KyvernoDryRunNamespace()
	}
	if !vo.DisableDryRun {
		// check if kyverno can 'create' dryrun resource
		ok, err := h.checkDryRunPermission(ctx, adreq.Kind.Kind, vo.DryRunNamespace)
		if err != nil {
			logger.V(1).Info("failed to check permissions to 'create' resource. disabled DryRun option.", "dryrun namespace", vo.DryRunNamespace, "kind", adreq.Kind.Kind, "error", err.Error())
			vo.DisableDryRun = true
		}
		if !ok {
			logger.V(1).Info("kyverno does not have permissions to 'create' resource. disabled DryRun option.", "dryrun namespace", vo.DryRunNamespace, "kind", adreq.Kind.Kind)
			vo.DisableDryRun = true
		}
		// check if kyverno namespace is not used for dryrun
		ok = checkDryRunNamespace(vo.DryRunNamespace)
		if !ok {
			logger.V(1).Info("an inappropriate dryrun namespace is set; set a namespace other than kyverno.", "dryrun namespace", vo.DryRunNamespace)
			vo.DisableDryRun = true
		}
	}

	// can be overridden per Attestor
	if verifyRule.Repository != "" {
		vo.ResourceBundleRef = verifyRule.Repository
	}

	// signature annotation
	// set default annotation domain
	if verifyRule.AnnotationDomain != "" && verifyRule.AnnotationDomain != DefaultAnnotationKeyDomain {
		vo.AnnotationConfig.AnnotationKeyDomain = verifyRule.AnnotationDomain
	}

	// signature verification by each attestor
	verifiedMsgs := []string{}
	for i, attestorSet := range verifyRule.Attestors {
		path := fmt.Sprintf(".attestors[%d]", i)
		verified, reason, err := verifyManifestAttestorSet(resource, attestorSet, vo, path, string(adreq.UID), logger)
		if err != nil {
			return verified, reason, err
		}
		if !verified {
			return verified, reason, err
		} else {
			verifiedMsgs = append(verifiedMsgs, reason)
		}
	}
	msg := fmt.Sprintf("verified manifest signatures; %s", strings.Join(verifiedMsgs, ","))
	return true, msg, nil
}

func (h validateManifestHandler) checkDryRunPermission(ctx context.Context, kind, namespace string) (bool, error) {
	ok, _, err := h.client.CanI(ctx, kind, namespace, "create", "", config.KyvernoServiceAccountName())
	return ok, err
}

func verifyManifestAttestorSet(resource unstructured.Unstructured, attestorSet kyvernov1.AttestorSet, vo *k8smanifest.VerifyResourceOption, path string, uid string, logger logr.Logger) (bool, string, error) {
	verifiedCount := 0
	attestorSet = internal.ExpandStaticKeys(attestorSet)
	requiredCount := attestorSet.RequiredCount()
	errorList := []error{}
	verifiedMessageList := []string{}
	failedMessageList := []string{}

	for i, a := range attestorSet.Entries {
		var entryError error
		var verified bool
		var reason string
		attestorPath := fmt.Sprintf("%s.entries[%d]", path, i)
		if a.Attestor != nil {
			nestedAttestorSet, err := kyvernov1.AttestorSetUnmarshal(a.Attestor)
			if err != nil {
				entryError = fmt.Errorf("failed to unmarshal nested attestor %s: %w", attestorPath, err)
			} else {
				attestorPath += ".attestor"
				verified, reason, err = verifyManifestAttestorSet(resource, *nestedAttestorSet, vo, attestorPath, uid, logger)
				if err != nil {
					entryError = fmt.Errorf("failed to verify signature; %s: %w", attestorPath, err)
				}
			}
		} else {
			verified, reason, entryError = k8sVerifyResource(resource, a, vo, attestorPath, uid, i, logger)
		}

		if entryError != nil {
			errorList = append(errorList, entryError)
		} else if verified {
			// verification success.
			verifiedCount++
			verifiedMessageList = append(verifiedMessageList, reason)
		} else {
			failedMessageList = append(failedMessageList, reason)
		}

		if verifiedCount >= requiredCount {
			logger.V(2).Info("manifest verification succeeded", "verifiedCount", verifiedCount, "requiredCount", requiredCount)
			reason := fmt.Sprintf("manifest verification succeeded; verifiedCount %d; requiredCount %d; message %s",
				verifiedCount, requiredCount, strings.Join(verifiedMessageList, ","))
			return true, reason, nil
		}
	}

	if len(errorList) != 0 {
		err := multierr.Combine(errorList...)
		logger.V(2).Info("manifest verification failed", "verifiedCount", verifiedCount, "requiredCount",
			requiredCount, "errors", errorList)
		return false, "", err
	}
	reason := fmt.Sprintf("manifest verification failed; verifiedCount %d; requiredCount %d; message %s",
		verifiedCount, requiredCount, strings.Join(failedMessageList, ","))
	logger.V(2).Info("manifest verification failed", "verifiedCount", verifiedCount, "requiredCount",
		requiredCount, "reason", failedMessageList)
	return false, reason, nil
}

func k8sVerifyResource(resource unstructured.Unstructured, a kyvernov1.Attestor, vo *k8smanifest.VerifyResourceOption, attestorPath, uid string, i int, logger logr.Logger) (bool, string, error) {
	// check annotations
	if a.Annotations != nil {
		mnfstAnnotations := resource.GetAnnotations()
		err := checkManifestAnnotations(mnfstAnnotations, a.Annotations)
		if err != nil {
			return false, "", err
		}
	}

	// build verify option
	vo, subPath, envVariables, err := buildVerifyResourceOptionsAndPath(a, vo, uid, i)
	// unset env variables after verification
	defer cleanEnvVariables(envVariables)
	if err != nil {
		logger.V(4).Info("failed to build verify option", err.Error())
		return false, "", fmt.Errorf("%s: %w", attestorPath+subPath, err)
	}

	logger.V(4).Info("verifying resource by k8s-manifest-sigstore")
	result, err := k8smanifest.VerifyResource(resource, vo)
	if err != nil {
		logger.V(4).Info("verifyResoource return err", err.Error())
		if k8smanifest.IsSignatureNotFoundError(err) {
			// no signature found
			failReason := fmt.Sprintf("%s: %s", attestorPath+subPath, err.Error())
			return false, failReason, nil
		} else if k8smanifest.IsMessageNotFoundError(err) {
			// no signature and message found
			failReason := fmt.Sprintf("%s: %s", attestorPath+subPath, err.Error())
			return false, failReason, nil
		} else {
			return false, "", fmt.Errorf("%s: %w", attestorPath+subPath, err)
		}
	} else {
		resBytes, _ := json.Marshal(result)
		logger.V(4).Info("verify result", string(resBytes))
		if result.Verified {
			// verification success.
			reason := fmt.Sprintf("singed by a valid signer: %s", result.Signer)
			return true, reason, nil
		} else {
			failReason := fmt.Sprintf("%s: %s", attestorPath+subPath, "failed to verify signature.")
			if result.Diff != nil && result.Diff.Size() > 0 {
				failReason = fmt.Sprintf("%s: failed to verify signature. diff found; %s", attestorPath+subPath, result.Diff.String())
			} else if result.Signer != "" {
				failReason = fmt.Sprintf("%s: no signer matches with this resource. signed by %s", attestorPath+subPath, result.Signer)
			}
			return false, failReason, nil
		}
	}
}

func buildVerifyResourceOptionsAndPath(a kyvernov1.Attestor, vo *k8smanifest.VerifyResourceOption, uid string, i int) (*k8smanifest.VerifyResourceOption, string, []string, error) {
	subPath := ""
	var entryError error
	envVariables := []string{}
	if a.Keys != nil {
		subPath = subPath + ".keys"
		Key := a.Keys.PublicKeys
		if strings.HasPrefix(Key, "-----BEGIN PUBLIC KEY-----") || strings.HasPrefix(Key, "-----BEGIN PGP PUBLIC KEY BLOCK-----") {
			// prepare env variable for pubkey
			// it consists of admission request ID, key index and random num
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(math.MaxInt64)))
			pubkeyEnv := fmt.Sprintf("_PK_%s_%d_%d", uid, i, n)
			err := os.Setenv(pubkeyEnv, Key)
			envVariables = append(envVariables, pubkeyEnv)
			if err != nil {
				entryError = fmt.Errorf("failed to set env variable; %s: %w", pubkeyEnv, err)
			} else {
				keyPath := fmt.Sprintf("env://%s", pubkeyEnv)
				vo.KeyPath = keyPath
			}
		} else {
			// this supports Kubernetes secrets and kms
			vo.KeyPath = Key
		}
		if a.Keys.Rekor != nil {
			vo.RekorURL = a.Keys.Rekor.URL
		}
	} else if a.Certificates != nil {
		subPath = subPath + ".certificates"
		if a.Certificates.Certificate != "" {
			Cert := a.Certificates.Certificate
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(math.MaxInt64)))
			certEnv := fmt.Sprintf("_CERT_%s_%d_%d", uid, i, n)
			err := os.Setenv(certEnv, Cert)
			envVariables = append(envVariables, certEnv)
			if err != nil {
				entryError = fmt.Errorf("failed to set env variable; %s: %w", certEnv, err)
			} else {
				certPath := fmt.Sprintf("env://%s", certEnv)
				vo.Certificate = certPath
			}
		}
		if a.Certificates.CertificateChain != "" {
			CertChain := a.Certificates.CertificateChain
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(math.MaxInt64)))
			certChainEnv := fmt.Sprintf("_CC_%s_%d_%d", uid, i, n)
			err := os.Setenv(certChainEnv, CertChain)
			envVariables = append(envVariables, certChainEnv)
			if err != nil {
				entryError = fmt.Errorf("failed to set env variable; %s: %w", certChainEnv, err)
			} else {
				certChainPath := fmt.Sprintf("env://%s", certChainEnv)
				vo.CertificateChain = certChainPath
			}
		}
		if a.Certificates.Rekor != nil {
			vo.RekorURL = a.Keys.Rekor.URL
		}
	} else if a.Keyless != nil {
		subPath = subPath + ".keyless"
		_ = os.Setenv(CosignEnvVariable, "1")
		envVariables = append(envVariables, CosignEnvVariable)
		if a.Keyless.Rekor != nil {
			vo.RekorURL = a.Keyless.Rekor.URL
		}
		if a.Keyless.Roots != "" {
			Roots := a.Keyless.Roots
			cp, err := loadCertPool([]byte(Roots))
			if err != nil {
				entryError = fmt.Errorf("failed to load Root certificates: %w", err)
			} else {
				vo.RootCerts = cp
			}
		}
		Issuer := a.Keyless.Issuer
		vo.OIDCIssuer = Issuer
		Subject := a.Keyless.Subject
		vo.Signers = k8smanifest.SignerList{Subject}
	}
	if a.Repository != "" {
		vo.ResourceBundleRef = a.Repository
	}
	return vo, subPath, envVariables, entryError
}

func cleanEnvVariables(envVariables []string) {
	for _, ev := range envVariables {
		os.Unsetenv(ev)
	}
}

func addConfig(vo, defaultConfig *k8smanifest.VerifyResourceOption) *k8smanifest.VerifyResourceOption {
	if vo == nil {
		return nil
	}
	ignoreFields := []k8smanifest.ObjectFieldBinding(vo.IgnoreFields)
	ignoreFields = append(ignoreFields, []k8smanifest.ObjectFieldBinding(defaultConfig.IgnoreFields)...)
	vo.IgnoreFields = ignoreFields
	return vo
}

func loadDefaultConfig() *k8smanifest.VerifyResourceOption {
	var defaultConfig *k8smanifest.VerifyResourceOption
	err := yaml.Unmarshal(engineresources.DefaultConfigBytes, &defaultConfig)
	if err != nil {
		return nil
	}
	return defaultConfig
}

func addDefaultConfig(vo *k8smanifest.VerifyResourceOption) *k8smanifest.VerifyResourceOption {
	dvo := loadDefaultConfig()
	return addConfig(vo, dvo)
}

func loadCertPool(roots []byte) (*x509.CertPool, error) {
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(roots) {
		return nil, fmt.Errorf("error creating root cert pool")
	}

	return cp, nil
}

func checkManifestAnnotations(mnfstAnnotations map[string]string, annotations map[string]string) error {
	for key, val := range annotations {
		if val != mnfstAnnotations[key] {
			return fmt.Errorf("annotations mismatch: %s does not match expected value %s for key %s",
				mnfstAnnotations[key], val, key)
		}
	}
	return nil
}

func checkDryRunNamespace(namespace string) bool {
	// should not use kyverno namespace for dryrun
	return namespace != config.KyvernoNamespace()
}

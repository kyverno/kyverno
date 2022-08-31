package engine

import (
	"crypto/rand"
	"crypto/x509"
	_ "embed"
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
	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/pkg/errors"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DefaultAnnotationKeyDomain = "cosign.sigstore.dev"
	CosignEnvVariable          = "COSIGN_EXPERIMENTAL"
)

//go:embed resources/default-config.yaml
var defaultConfigBytes []byte

func processYAMLValidationRule(log logr.Logger, ctx *PolicyContext, rule *kyvernov1.Rule) *response.RuleResponse {
	if isDeleteRequest(ctx) {
		return nil
	}
	ruleResp := handleVerifyManifest(ctx, rule, log)
	return ruleResp
}

func handleVerifyManifest(ctx *PolicyContext, rule *kyvernov1.Rule, logger logr.Logger) *response.RuleResponse {
	verified, reason, err := verifyManifest(ctx, *rule.Validation.Manifests, logger)
	if err != nil {
		logger.V(3).Info("verifyManifest return err", "error", err.Error())
		return ruleError(rule, response.Validation, "error occurred during manifest verification", err)
	}
	logger.V(3).Info("verifyManifest result", "verified", strconv.FormatBool(verified), "reason", reason)
	if !verified {
		return ruleResponse(*rule, response.Validation, reason, response.RuleStatusFail, nil)
	}
	return ruleResponse(*rule, response.Validation, reason, response.RuleStatusPass, nil)
}

func verifyManifest(policyContext *PolicyContext, verifyRule kyvernov1.Manifests, logger logr.Logger) (bool, string, error) {
	// load AdmissionRequest
	request, err := policyContext.JSONContext.Query("request")
	if err != nil {
		return false, "", errors.Wrapf(err, "failed to get a request from policyContext")
	}
	reqByte, _ := json.Marshal(request)
	var adreq *admissionv1.AdmissionRequest
	err = json.Unmarshal(reqByte, &adreq)
	if err != nil {
		return false, "", errors.Wrapf(err, "failed to unmarshal a request from requestByte")
	}
	// unmarshal admission request object
	var resource unstructured.Unstructured
	objectBytes := adreq.Object.Raw
	err = json.Unmarshal(objectBytes, &resource)
	if err != nil {
		return false, "", errors.Wrapf(err, "failed to Unmarshal a requested object")
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
		vo.DryRunNamespace = config.KyvernoNamespace()
	}
	if !vo.DisableDryRun {
		// check if kyverno can 'create' dryrun resource
		ok, err := checkDryRunPermission(policyContext.Client, adreq.Kind.Kind, vo.DryRunNamespace)
		if err != nil {
			logger.V(1).Info("failed to check permissions to 'create' resource. disabled DryRun option.", "dryrun namespace", vo.DryRunNamespace, "kind", adreq.Kind.Kind, "error", err.Error())
			vo.DisableDryRun = true
		}
		if !ok {
			logger.V(1).Info("kyverno does not have permissions to 'create' resource. disabled DryRun option.", "dryrun namespace", vo.DryRunNamespace, "kind", adreq.Kind.Kind)
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

func verifyManifestAttestorSet(resource unstructured.Unstructured, attestorSet kyvernov1.AttestorSet, vo *k8smanifest.VerifyResourceOption, path string, uid string, logger logr.Logger) (bool, string, error) {
	verifiedCount := 0
	attestorSet = expandStaticKeys(attestorSet)
	requiredCount := getRequiredCount(attestorSet)
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
				entryError = errors.Wrapf(err, "failed to unmarshal nested attestor %s", attestorPath)
			} else {
				attestorPath += ".attestor"
				verified, reason, err = verifyManifestAttestorSet(resource, *nestedAttestorSet, vo, attestorPath, uid, logger)
				if err != nil {
					entryError = errors.Wrapf(err, "failed to verify signature; %s", attestorPath)
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
		return false, "", errors.Wrapf(err, attestorPath+subPath)
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
			return false, "", errors.Wrapf(err, attestorPath+subPath)
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
				entryError = errors.Wrapf(err, "failed to set env variable; %s", pubkeyEnv)
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
				entryError = errors.Wrapf(err, "failed to set env variable; %s", certEnv)
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
				entryError = errors.Wrapf(err, "failed to set env variable; %s", certChainEnv)
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
				entryError = errors.Wrap(err, "failed to load Root certificates")
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
	err := yaml.Unmarshal(defaultConfigBytes, &defaultConfig)
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

func checkDryRunPermission(dclient dclient.Interface, kind, namespace string) (bool, error) {
	canI := auth.NewCanI(dclient, kind, namespace, "create")
	ok, err := canI.RunAccessCheck()
	if err != nil {
		return false, err
	}
	return ok, nil
}

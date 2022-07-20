package engine

import (
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/pkg/errors"

	"github.com/ghodss/yaml"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	k8smnfutil "github.com/sigstore/k8s-manifest-sigstore/pkg/util"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DefaultAnnotationKeyDomain = "cosign.sigstore.dev"
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
		logger.V(2).Info(fmt.Sprintf("verifyManifest return err: %s", err.Error()))
		return ruleError(rule, response.Validation, "error occured during manifest verification", err)
	}
	logger.V(2).Info(fmt.Sprintf("verifyManifest result: verified %s; %s", strconv.FormatBool(verified), reason))
	if !verified {
		return ruleResponse(*rule, response.Validation, reason, response.RuleStatusFail, nil)
	}
	return ruleResponse(*rule, response.Validation, reason, response.RuleStatusPass, nil)
}

func verifyManifest(policyContext *PolicyContext, verifyRule kyvernov1.Manifests, logger logr.Logger) (bool, string, error) {
	// load AdmissionRequest
	request, err := policyContext.JSONContext.Query("request")
	if err != nil {
		return false, "", fmt.Errorf("failed to get a request from policyContext: %s", err.Error())
	}
	reqByte, _ := json.Marshal(request)
	var adreq *admissionv1.AdmissionRequest
	err = json.Unmarshal(reqByte, &adreq)
	if err != nil {
		return false, "", fmt.Errorf("failed to unmarshal a request from requestByte: %s", err.Error())
	}
	// unmarshal admission request object
	var resource unstructured.Unstructured
	objectBytes := adreq.Object.Raw
	err = json.Unmarshal(objectBytes, &resource)
	if err != nil {
		return false, "", fmt.Errorf("failed to Unmarshal a requested object: %s", err.Error())
	}

	logger.V(4).Info("verifying manifest...", adreq.Namespace, adreq.Kind.Kind, adreq.Name, adreq.UserInfo.Username)

	// allow dryrun request
	if *adreq.DryRun {
		return true, "allowed because of DryRun request", nil
	}

	// check skipping user
	if Match(verifyRule.SkipUsers, resource, adreq.UserInfo.Username) {
		return true, "allowed by skipObjects rule", nil
	}

	// prepare verifyResource option
	vo := &k8smanifest.VerifyResourceOption{}
	// adding default ignoreFields from
	// github.com/sigstore/k8s-manifest-sigstore/blob/main/pkg/k8smanifest/resources/default-config.yaml
	vo = k8smanifest.AddDefaultConfig(vo)
	// adding default ignoreFields from pkg/engine/resources/default-config.yaml
	vo = addDefaultConfig(vo)
	// adding ignoreFields from Policy
	vo.IgnoreFields = append(vo.IgnoreFields, verifyRule.IgnoreFields...)

	// dryrun setting
	vo.DisableDryRun = !verifyRule.DryRunOption.Enable
	if verifyRule.DryRunOption.Namespace != "" {
		vo.DryRunNamespace = verifyRule.DryRunOption.Namespace
	} else {
		vo.DryRunNamespace = config.KyvernoNamespace()
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
		verified, reason, err := verify(resource, attestorSet, vo, path, string(adreq.UID), logger)
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

func verify(resource unstructured.Unstructured, attestorSet kyvernov1.AttestorSet, vo *k8smanifest.VerifyResourceOption, path string, uid string, logger logr.Logger) (bool, string, error) {
	verifiedCount := 0
	attestorSet = expandStaticKeys(attestorSet)
	requiredCount := getRequiredCount(attestorSet)
	errorList := []error{}
	verifiedMessageList := []string{}
	failedMessageList := []string{}

	for i, a := range attestorSet.Entries {
		var entryError error
		attestorPath := fmt.Sprintf("%s.entries[%d]", path, i)
		if a.Attestor != nil {
			nestedAttestorSet, err := kyvernov1.AttestorSetUnmarshal(a.Attestor)
			if err != nil {
				entryError = errors.Wrapf(err, "failed to unmarshal nested attestor %s", attestorPath)
			} else {
				attestorPath += ".attestor"
				verified, reason, err := verify(resource, *nestedAttestorSet, vo, attestorPath, uid, logger)
				if err != nil {
					entryError = errors.Wrapf(err, "failed to verify signature; %s", attestorPath)
				}
				if verified {
					// verification success.
					verifiedCount++
					verifiedMessageList = append(verifiedMessageList, reason)
				} else {
					failedMessageList = append(failedMessageList, reason)
				}
			}
		} else {
			subPath := ""
			if a.Keys != nil {
				subPath = subPath + ".keys"
				Key := a.Keys.PublicKeys
				if strings.HasPrefix(Key, "-----BEGIN PUBLIC KEY-----") || strings.HasPrefix(Key, "-----BEGIN PGP PUBLIC KEY BLOCK-----") {
					// prepare env variable for pubkey
					pubkeyEnv := fmt.Sprintf("_PK_%s_%d", uid, i)
					err := os.Setenv(pubkeyEnv, Key)
					if err != nil {
						entryError = errors.Wrapf(err, "failed to set env variable; %s", pubkeyEnv)
					} else {
						keyPath := fmt.Sprintf("env://%s", pubkeyEnv)
						vo.KeyPath = keyPath
					}
					defer os.Unsetenv(pubkeyEnv)
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
					certEnv := fmt.Sprintf("_CERT_%s_%d", uid, i)
					err := os.Setenv(certEnv, Cert)
					if err != nil {
						entryError = errors.Wrapf(err, "failed to set env variable; %s", certEnv)
					} else {
						certPath := fmt.Sprintf("env://%s", certEnv)
						vo.Certificate = certPath
					}
					defer os.Unsetenv(certEnv)
				}
				if a.Certificates.CertificateChain != "" {
					CertChain := a.Certificates.CertificateChain
					certChainEnv := fmt.Sprintf("_CC_%s_%d", uid, i)
					err := os.Setenv(certChainEnv, CertChain)
					if err != nil {
						entryError = errors.Wrapf(err, "failed to set env variable; %s", certChainEnv)
					} else {
						certChainPath := fmt.Sprintf("env://%s", certChainEnv)
						vo.CertificateChain = certChainPath
					}
					defer os.Unsetenv(certChainEnv)
				}
				if a.Certificates.Rekor != nil {
					vo.RekorURL = a.Keys.Rekor.URL
				}
			} else if a.Keyless != nil {
				subPath = subPath + ".keyless"
				_ = os.Setenv("COSIGN_EXPERIMENTAL", "1")
				defer os.Unsetenv("COSIGN_EXPERIMENTAL")
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

			if a.Annotations != nil {
				// check annotations
				mnfstAnnotations := resource.GetAnnotations()
				err := checkManifestAnnotations(mnfstAnnotations, a.Annotations)
				if err != nil {
					entryError = err
				}
			}

			if entryError != nil {
				entryError = fmt.Errorf("%s: %s", attestorPath+subPath, entryError.Error())
				errorList = append(errorList, entryError)
				continue
			}

			logger.V(4).Info("verifying resource by k8s-manifest-sigstore...")
			result, err := k8smanifest.VerifyResource(resource, vo)
			if err != nil {
				logger.V(4).Info("verifyResoource return err;", err.Error())
				entryError = fmt.Errorf("%s: %s", attestorPath+subPath, err.Error())
			} else {
				resBytes, _ := json.Marshal(result)
				logger.V(4).Info("verify result:", string(resBytes))
				if result.Verified {
					// verification success.
					verifiedCount++
					reason := fmt.Sprintf("singed by a valid signer: %s", result.Signer)
					verifiedMessageList = append(verifiedMessageList, reason)
				} else {
					failReason := fmt.Sprintf("%s: %s", attestorPath+subPath, "failed to verify signature.")
					if result.Diff != nil && result.Diff.Size() > 0 {
						failReason = fmt.Sprintf("%s: failed to verify signature. diff found; %s", attestorPath+subPath, result.Diff.String())
					} else if result.Signer != "" {
						failReason = fmt.Sprintf("%s: no signer matches with this resource. signed by %s", attestorPath+subPath, result.Signer)
					}
					failedMessageList = append(failedMessageList, failReason)
				}
			}
		}

		if entryError != nil {
			errorList = append(errorList, entryError)
		}
		if verifiedCount >= requiredCount {
			logger.V(2).Info("manigest verification succeeded", "verifiedCount", verifiedCount, "requiredCount", requiredCount)
			reason := fmt.Sprintf("manigest verification succeeded; verifiedCount %d; requiredCount %d; message %s",
				verifiedCount, requiredCount, strings.Join(verifiedMessageList, ","))
			return true, reason, nil
		}
	}

	if len(errorList) != 0 {
		var mergedErr error
		for _, e := range errorList {
			if mergedErr != nil {
				mergedErr = fmt.Errorf("%s; %w", mergedErr.Error(), e)
			} else {
				mergedErr = e
			}
		}
		mergedErr = fmt.Errorf("manigest verification failed; verifiedCount %d; requiredCount %d; %w", verifiedCount, requiredCount, mergedErr)
		return false, "", mergedErr
	}
	reason := fmt.Sprintf("manigest verification failed; verifiedCount %d; requiredCount %d; message %s",
		verifiedCount, requiredCount, strings.Join(failedMessageList, ","))
	return false, reason, nil
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

func Match(skipList kyvernov1.ObjectUserBindingList, obj unstructured.Unstructured, username string) bool {
	if len(skipList) == 0 {
		return false
	}
	for _, u := range skipList {
		if u.Objects.Match(obj) {
			if k8smnfutil.MatchWithPatternArray(username, u.Users) {
				return true
			}
		}
	}
	return false
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

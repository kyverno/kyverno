package engine

import (
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
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const DefaultSignatureAnnotationKey = "cosign.sigstore.dev/signature"
const DefaultMessageAnnotationKey = "cosign.sigstore.dev/message"
const DefaultAnnotationKeyDomain = "cosign.sigstore.dev"
const DefaultSignatureAnnotationMessage = "signature"
const DefaultMessageAnnotationMessage = "message"
const DefaultDryRunNamespace = ""
const ValidateLogicMustAll = "mustAll"
const ValidateLogicAtLeastOne = "atLeastOne"

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
	verified, reason, err := verifyManifest(ctx, rule.Validation.Manifests, logger)
	if err != nil {
		logger.V(2).Info("verifyManifest return err: %s", err.Error())
		return ruleError(rule, response.Validation, "failed to verify manifest", err)
	}
	logger.V(2).Info("verifyManifest result: verified %s; %s", strconv.FormatBool(verified), reason)
	if !verified {
		return ruleResponse(*rule, response.Validation, reason, response.RuleStatusFail, nil)
	}

	return ruleResponse(*rule, response.Validation, reason, response.RuleStatusPass, nil)
}

func verifyManifest(policyContext *PolicyContext, verifyRule kyvernov1.Manifests, logger logr.Logger) (bool, string, error) {
	// load AdmissionRequest
	request, err := policyContext.JSONContext.Query("request")
	if err != nil {
		return false, fmt.Sprintf("failed to get a request from policyContext: %s", err.Error()), err
	}
	reqByte, _ := json.Marshal(request)
	var adreq *v1.AdmissionRequest
	err = json.Unmarshal(reqByte, &adreq)
	if err != nil {
		return false, fmt.Sprintf("failed to unmarshal a request from requestByte: %s", err.Error()), err
	}
	// unmarshal admission request object
	var resource unstructured.Unstructured
	objectBytes := adreq.Object.Raw
	err = json.Unmarshal(objectBytes, &resource)
	if err != nil {
		errMsg := "failed to Unmarshal a requested object: " + err.Error()
		return false, errMsg, err
	}

	logger.V(2).Info("verifying manifest...", adreq.Namespace, adreq.Kind.Kind, adreq.Name, adreq.UserInfo.Username)

	// allow dryrun request
	if *adreq.DryRun {
		return true, "allowed because of DryRun request", nil
	}

	// check skipping user
	if Match(verifyRule.SkipUsers, resource, adreq.UserInfo.Username) {
		return true, "allowed by skipObjects rule", nil
	}

	// signature verification
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

	// signature annotation
	// set default annotation domain
	if verifyRule.AnnotationDomain != "" && verifyRule.AnnotationDomain != DefaultAnnotationKeyDomain {
		vo.AnnotationConfig.AnnotationKeyDomain = verifyRule.AnnotationDomain
	}

	if verifyRule.ResourceBundleRef != "" {
		vo.ResourceBundleRef = verifyRule.ResourceBundleRef
	}

	// key setting
	// prepare tmpDir to save pubkey file
	// tmpDir, err := ioutil.TempDir("", string(adreq.UID))
	// if err != nil {
	// 	return false, "", errors.New(fmt.Sprintf("failed to make temp dir; %s; %s", tmpDir, err))
	// }
	// defer os.RemoveAll(tmpDir)
	// if ecdsaPub != "" { // keyed
	// 	keyPath, err := convertToLocalFilePath(tmpDir, ecdsaPub)
	// 	if err != nil {
	// 		return false, err.Error(), err
	// 	}
	// 	vo.KeyPath = keyPath
	// }

	// set key operation
	mustAll := false
	if verifyRule.KeyOperation != "" {
		if verifyRule.KeyOperation == ValidateLogicMustAll {
			logger.V(2).Info("keyOperation is set mustAll. All signature should be verified.")
			mustAll = true
		} else if verifyRule.KeyOperation == ValidateLogicAtLeastOne {
			mustAll = false
		} else {
			logger.V(2).Info("warning: unexpected value for key operation.", verifyRule.KeyOperation)
		}
	}

	// verify
	pubkeys := []string{}
	subjects := []string{}
	for _, key := range verifyRule.Keys {
		if key.Key != "" {
			pubkeys = append(pubkeys, key.Key)
		}
		if key.Subject != "" {
			subjects = append(subjects, key.Subject)
		}
	}

	if mustAll {
		vresults := VerifyResultMustAll{}
		// keyed
		if len(pubkeys) != 0 {
			for i, pk := range pubkeys {
				// prepare env variable for pubkey
				pubkeyEnv := fmt.Sprintf("_PK_%s_%d", string(adreq.UID), i)
				err = os.Setenv(pubkeyEnv, pk)
				if err != nil {
					return false, "", errors.New(fmt.Sprintf("failed to set env variable; %s; %s", pubkeyEnv, err))
				}
				defer os.Unsetenv(pubkeyEnv)
				keyPath := fmt.Sprintf("env://%s", pubkeyEnv)
				vo.KeyPath = keyPath
				logger.V(2).Info("verifying resource. key:", keyPath)
				result, err := k8smanifest.VerifyResource(resource, vo)
				if err != nil {
					logger.V(2).Info("verifyResoource return err;", err.Error())
					vresults = vresults.addErrResult(err)
					continue
				}
				resBytes, _ := json.Marshal(result)
				logger.V(2).Info("verify result:", string(resBytes))
				vresults = vresults.addResult(result)
			}
		}
		// keyless
		if len(subjects) != 0 {
			_ = os.Setenv("COSIGN_EXPERIMENTAL", "1")
			defer os.Unsetenv("COSIGN_EXPERIMENTAL")
			for _, sub := range subjects {
				vo.Signers = k8smanifest.SignerList{sub}
				result, err := k8smanifest.VerifyResource(resource, vo)
				if err != nil {
					logger.V(2).Info("verifyResoource return err;", err.Error())
					vresults = vresults.addErrResult(err)
					continue
				}
				resBytes, _ := json.Marshal(result)
				logger.V(2).Info("verify result:", string(resBytes))
				vresults = vresults.addResult(result)
			}
		}
		return vresults.makeFinalResult()

	} else { // atLeastOne
		verified := false
		failReasosn := []string{}
		// keyed
		if len(pubkeys) != 0 {
			keyPathList := []string{}
			for i, pk := range pubkeys {
				pubkeyEnv := fmt.Sprintf("_PK_%s_%d", string(adreq.UID), i)
				err = os.Setenv(pubkeyEnv, pk)
				if err != nil {
					return false, "", errors.New(fmt.Sprintf("failed to set env variable; %s; %s", pubkeyEnv, err))
				}
				defer os.Unsetenv(pubkeyEnv)
				keyPath := fmt.Sprintf("env://%s", pubkeyEnv)
				keyPathList = append(keyPathList, keyPath)
			}
			keyPathString := strings.Join(keyPathList, ",")
			if keyPathString != "" {
				vo.KeyPath = keyPathString
			}
			result, err := k8smanifest.VerifyResource(resource, vo)
			if err != nil {
				logger.V(2).Info("verifyResoource return err;", err.Error())
				failReasosn = append(failReasosn, err.Error())
			} else {
				resBytes, _ := json.Marshal(result)
				logger.V(2).Info("verify result:", string(resBytes))
				verified = result.Verified
				if verified {
					// verification success.
					reason := fmt.Sprintf("Singed by a valid signer: %s", result.Signer)
					return verified, reason, nil
				} else {
					reason := "failed to verify signature."
					if result.Diff != nil && result.Diff.Size() > 0 {
						reason = fmt.Sprintf("failed to verify signature. diff found: %s", result.Diff.String())
					} else if result.Signer != "" {
						reason = fmt.Sprintf(" no signer matches with this resource. signed by %s", result.Signer)
					}
					failReasosn = append(failReasosn, reason)
				}
			}
		}
		// keyless
		if len(subjects) != 0 {
			_ = os.Setenv("COSIGN_EXPERIMENTAL", "1")
			defer os.Unsetenv("COSIGN_EXPERIMENTAL")
			vo.Signers = append(vo.Signers, subjects...)
			result, err := k8smanifest.VerifyResource(resource, vo)
			if err != nil {
				logger.V(2).Info("verifyResoource return err;", err.Error())
				failReasosn = append(failReasosn, err.Error())
			} else {
				resBytes, _ := json.Marshal(result)
				logger.V(2).Info("verify result:", string(resBytes))
				verified = result.Verified
				if verified {
					// verification success.
					reason := fmt.Sprintf("Singed by a valid signer: %s", result.Signer)
					return verified, reason, nil
				} else {
					reason := "failed to verify signature."
					if result.Diff != nil && result.Diff.Size() > 0 {
						reason = fmt.Sprintf("failed to verify signature. diff found: %s", result.Diff.String())
					} else if result.Signer != "" {
						reason = fmt.Sprintf(" no signer matches with this resource. signed by %s", result.Signer)
					}
					failReasosn = append(failReasosn, reason)
				}
			}
		}
		finalReason := strings.Join(failReasosn, ";")
		return verified, finalReason, nil
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

type VerifyResultMustAll struct {
	Signers []string
	Results []bool
	Reasons []string
}

func (self VerifyResultMustAll) addResult(result *k8smanifest.VerifyResourceResult) VerifyResultMustAll {
	self.Results = append(self.Results, result.Verified)
	self.Signers = append(self.Signers, result.Signer)
	failMsg := "failed to verify signature; no signature found."
	if result.Diff != nil && result.Diff.Size() > 0 {
		failMsg = fmt.Sprintf("diff found: %s", result.Diff.String())
	} else if result.Signer != "" {
		failMsg = fmt.Sprintf("no signer matches with this resource. signed by %s", result.Signer)
	}
	self.Reasons = append(self.Reasons, failMsg)
	return self
}

func (self VerifyResultMustAll) addErrResult(err error) VerifyResultMustAll {
	self.Reasons = append(self.Reasons, err.Error())
	self.Results = append(self.Results, false)
	return self
}

func (self VerifyResultMustAll) makeFinalResult() (bool, string, error) {
	finalRes := true
	failCount := 0
	for _, r := range self.Results {
		if !r {
			finalRes = false
			failCount += 1
		}
	}
	if finalRes {
		// verification success.
		signersStr := strings.Join(self.Signers, ",")
		return true, fmt.Sprintf("singed by a valid signer: %s", signersStr), nil
	} else {
		// verification failed
		failReason := strings.Join(self.Reasons, ";")
		reason := fmt.Sprintf("%d out of %d failed verification; %s", failCount, len(self.Results), failReason)
		return false, reason, nil
	}
}

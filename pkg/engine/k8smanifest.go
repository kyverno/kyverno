package engine

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/pkg/errors"

	"github.com/ghodss/yaml"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	k8smnfutil "github.com/sigstore/k8s-manifest-sigstore/pkg/util"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const DefaultAnnotationKey = "cosign.sigstore.dev/signature"
const DefaultAnnotationKeyDomain = "cosign.sigstore.dev"
const DefaultAnnotationMessage = "signature"
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
	verified, reason, err := verifyManifest(ctx, rule.Validation.Manifest, logger)
	logger.V(4).Info("verifyManifest result:", verified, reason)
	if err != nil {
		return ruleError(rule, response.Validation, "failed to verify manifest", err)
	}

	if !verified {
		return ruleResponse(*rule, response.Validation, reason, response.RuleStatusFail, nil)
	}

	return ruleResponse(*rule, response.Validation, reason, response.RuleStatusPass, nil)
}

func verifyManifest(policyContext *PolicyContext, verifyRule kyvernov1.Manifest, logger logr.Logger) (bool, string, error) {
	// load AdmissionRequest
	request, err := policyContext.JSONContext.Query("request")
	if err != nil {
		return false, fmt.Sprintf("failed to get a request from policyContext: %s", err.Error()), err
	}
	reqByte, _ := json.Marshal(request)
	var adreq *v1beta1.AdmissionRequest
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
	vo.DisableDryRun = !verifyRule.VerifyConfig.EnableDryRun
	if verifyRule.VerifyConfig.DryRunNamespace != "" {
		vo.DryRunNamespace = verifyRule.VerifyConfig.DryRunNamespace
	} else {
		vo.DryRunNamespace = config.KyvernoNamespace()
	}

	// signature annotation
	annotations := resource.GetAnnotations()
	sigAnnotations := verifyRule.VerifyConfig.SignatureAnnotations
	if sigAnnotations != nil {
		for i, annotation := range sigAnnotations {
			if i < 1 {
				_, ok := annotations[annotation]
				if ok {
					domainMsg := strings.Split(annotation, "/")
					vo.AnnotationConfig.AnnotationKeyDomain = domainMsg[0]
					if domainMsg[1] != DefaultAnnotationMessage {
						vo.AnnotationConfig.MessageBaseName = domainMsg[1]
					}
				}
			} else {
				_, ok := annotations[annotation]
				if ok {
					vo.AnnotationConfig.AdditionalSignatureKeysForVerify = append(vo.AnnotationConfig.AdditionalSignatureKeysForVerify, annotation)
				}
			}
		}
	}

	if verifyRule.SignatureRef.ImageRef != "" {
		vo.ImageRef = verifyRule.SignatureRef.ImageRef
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

	// verify logic
	mustAll := false
	vl := verifyRule.VerifyConfig.ValidateLogic
	if vl != "" {
		if vl == ValidateLogicMustAll {
			mustAll = true
		} else if vl == ValidateLogicAtLeastOne {
			mustAll = false
		} else {
			logger.V(2).Info("warning: unexpected value for validateLogic", vl)
		}
	}

	// verify
	if verifyRule.Keys != nil { // keyed
		signers := []string{}
		vresults := []bool{}
		reasons := []string{}
		errMsgs := []string{}
		for i, key := range verifyRule.Keys {
			// prepare env variable for pubkey
			pubkeyEnv := fmt.Sprintf("_PK_%s_%d", string(adreq.UID), i)
			err = os.Setenv(pubkeyEnv, key)
			if err != nil {
				return false, "", errors.New(fmt.Sprintf("failed to set env variable; %s; %s", pubkeyEnv, err))
			}
			defer os.Unsetenv(pubkeyEnv)
			keyPath := fmt.Sprintf("env://%s", pubkeyEnv)
			vo.KeyPath = keyPath
			logger.V(2).Info("verifying resource. key:", keyPath)
			result, err := k8smanifest.VerifyResource(resource, vo)
			resBytes, _ := json.Marshal(result)
			logger.V(2).Info("verify result:", string(resBytes))
			if err != nil {
				logger.V(2).Info("verifyResoource return err;", err.Error())
				errMsgs = append(errMsgs, err.Error())
				vresults = append(vresults, false)
				continue
			}
			vresults = append(vresults, result.Verified)
			signers = append(signers, result.Signer)
			failMsg := "failed to verify signature; no signature found."
			if result.Diff != nil && result.Diff.Size() > 0 {
				failMsg = fmt.Sprintf("diff found: %s", result.Diff.String())
			} else if result.Signer != "" {
				failMsg = fmt.Sprintf("no signer matches with this resource. signed by %s", result.Signer)
			}
			reasons = append(reasons, failMsg)
		}
		if mustAll {
			finalRes := true
			for _, r := range vresults {
				if !r {
					finalRes = false
				}
			}
			if finalRes {
				// verification success.
				signersStr := strings.Join(signers, ",")
				return true, fmt.Sprintf("singed by a valid signer: %s", signersStr), nil
			} else if len(errMsgs) != 0 {
				// err occurred
				return false, "", errors.Errorf("error occurred during manifest validation;", strings.Join(errMsgs, ";"))
			} else {
				// verification failed
				reason := strings.Join(reasons, ";")
				return false, reason, nil
			}
		} else { // atLeastOne
			finalRes := false
			for _, r := range vresults {
				if r {
					finalRes = true
				}
			}
			if finalRes {
				// verification success.
				signersStr := strings.Join(signers, ",")
				return true, fmt.Sprintf("singed by a valid signer: %s", signersStr), nil
			} else if len(errMsgs) != 0 {
				// err occurred
				return false, "", errors.Errorf("error occurred during manifest validation;", strings.Join(errMsgs, ";"))
			} else {
				// verification failed
				reason := strings.Join(reasons, ";")
				return false, reason, nil
			}
		}
	}

	verified := false
	reason := "failed to verify signature; no signature found."
	if verifyRule.Subjects != nil { // keyless
		vo.Signers = append(vo.Signers, verifyRule.Subjects...)
		result, err := k8smanifest.VerifyResource(resource, vo)
		if err != nil {
			// handle the error
			return false, err.Error(), err
		}
		verified = result.Verified
		if result.Diff != nil && result.Diff.Size() > 0 {
			reason = fmt.Sprintf("failed to verify signature. diff found: %s", result.Diff.String())
		} else if result.Signer != "" {
			reason = fmt.Sprintf(" no signer matches with this resource. signed by %s", result.Signer)
		}
	}
	return verified, reason, nil
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

func convertToLocalFilePath(dir, pem string) (string, error) {
	fpath := filepath.Join(dir, "yaml-verify-key.pub")
	err := ioutil.WriteFile(fpath, []byte(pem), 0644)
	if err != nil {
		return "", errors.New(fmt.Sprintf("failed to save PEM public key as a file; %s; %s", fpath, err))
	}

	return fpath, nil
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

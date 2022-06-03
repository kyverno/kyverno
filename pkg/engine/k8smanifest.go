package engine

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

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

const DefaultAnnotationKeyDomain = "cosign.sigstore.dev/"
const DefaultDryRunNamespace = ""

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
	verified, reason, err := verifyManifest(ctx, rule.Validation.Key, rule.Validation.Subject, rule.Validation.IgnoreFields, rule.Validation.SkipUsers, rule.Validation.VerifyConfig, logger)
	logger.V(4).Info("verifyManifest result:", verified, reason)
	if err != nil {
		return ruleError(rule, response.Validation, "failed to verify manifest", err)
	}

	if !verified {
		return ruleResponse(*rule, response.Validation, reason, response.RuleStatusFail, nil)
	}

	return ruleResponse(*rule, response.Validation, reason, response.RuleStatusPass, nil)
}

func verifyManifest(policyContext *PolicyContext, ecdsaPub string, subject string, ignoreFields k8smanifest.ObjectFieldBindingList, skipUsers kyvernov1.ObjectUserBindingList, verifyConfig kyvernov1.YamlVerifyConfig, logger logr.Logger) (bool, string, error) {
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
		return true, "Allowed because of DryRun request", nil
	}
	// check skipping user
	if Match(skipUsers, resource, adreq.UserInfo.Username) {
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
	vo.IgnoreFields = append(vo.IgnoreFields, ignoreFields...)

	// dryrun setting
	vo.DisableDryRun = verifyConfig.DisableDryRun
	if verifyConfig.DryRunNamespace != "" {
		vo.DryRunNamespace = verifyConfig.DryRunNamespace
	} else {
		vo.DryRunNamespace = config.KyvernoNamespace()
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

	if ecdsaPub != "" { // keyed
		// prepare env variable for pubkey
		pubkeyEnv := fmt.Sprintf("SIGNATURE_ENV_KEY%s", string(adreq.UID))
		err = os.Setenv(pubkeyEnv, ecdsaPub)
		if err != nil {
			return false, "", errors.New(fmt.Sprintf("failed to set env variable; %s; %s", pubkeyEnv, err))
		}
		defer os.Unsetenv(pubkeyEnv)
		vo.KeyPath = fmt.Sprintf("env://%s", pubkeyEnv)
	}

	if subject != "" { // keyless
		vo.Signers = append(vo.Signers, subject)
	}

	result, err := k8smanifest.VerifyResource(resource, vo)
	if err != nil {
		// handle the error
		return false, err.Error(), err
	}
	if result.Verified {
		// verification success.
		return result.Verified, fmt.Sprintf("singed by a valid signer: %s", result.Signer), nil
	} else {
		// verification failure. you can check the detail of the verification by the `result` variable.
		reason := "failed to verify signature; no signature found."
		if result.Diff != nil && result.Diff.Size() > 0 {
			reason = fmt.Sprintf("failed to verify signature. diff found: %s", result.Diff.String())
		} else if result.Signer != "" {
			reason = fmt.Sprintf(" no signer matches with this resource. signed by %s", result.Signer)
		}
		return result.Verified, reason, nil
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

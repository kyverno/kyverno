package validation

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func matchResource(logger logr.Logger, resource unstructured.Unstructured, rule kyvernov1.Rule, namespaceLabels map[string]string, policyNamespace string, operation kyvernov1.AdmissionOperation, jsonContext enginecontext.Interface) bool {
	if rule.RawAnyAllConditions != nil {
		preconditionsPassed, _, err := internal.CheckPreconditions(logger, jsonContext, rule.RawAnyAllConditions)
		if !preconditionsPassed || err != nil {
			return false
		}
	}

	// cannot use admission info from the current request as the user can be different, if the rule matches on old request user info, it should skip
	admissionInfo := kyvernov2.RequestInfo{
		Roles:        []string{"kyverno:invalidrole"},
		ClusterRoles: []string{"kyverno:invalidrole"},
		AdmissionUserInfo: authenticationv1.UserInfo{
			Username: "kyverno:kyverno-invalid-controller",
			UID:      "kyverno:invaliduid",
			Groups:   []string{"kyverno:invalidgroup"},
		},
	}

	err := engineutils.MatchesResourceDescription(
		resource,
		rule,
		admissionInfo,
		namespaceLabels,
		policyNamespace,
		resource.GroupVersionKind(),
		"",
		operation,
	)
	return err == nil
}

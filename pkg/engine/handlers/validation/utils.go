package validation

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func matchResource(resource unstructured.Unstructured, rule kyvernov1.Rule, namespaceLabels map[string]string, policyNamespace string, operation kyvernov1.AdmissionOperation) bool {
	// cannot use admission info from the current request as the user can be different, if the rule matches on old request user info, it should skip
	admissionInfo := kyvernov1beta1.RequestInfo{
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

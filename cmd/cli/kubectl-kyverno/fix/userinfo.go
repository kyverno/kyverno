package fix

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
)

func FixUserInfo(info v1alpha1.UserInfo) (v1alpha1.UserInfo, []string, error) {
	var messages []string
	if info.APIVersion == "" {
		messages = append(messages, "api version is not set, setting `cli.kyverno.io/v1alpha1`")
		info.APIVersion = "cli.kyverno.io/v1alpha1"
	}
	if info.Kind == "" {
		messages = append(messages, "kind is not set, setting `UserInfo`")
		info.Kind = "UserInfo"
	}
	return info, messages, nil
}

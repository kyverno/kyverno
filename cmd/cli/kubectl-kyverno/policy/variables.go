package policy

import (
	"encoding/json"
	"errors"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
)

func ExtractVariables(policy kyvernov1.PolicyInterface) ([]string, error) {
	var variables []string
	raw, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}
	for _, match := range regex.RegexVariables.FindAllStringSubmatch(string(raw), -1) {
		if len(match) != 3 {
			err := errors.New("extract variables match has wrong elements number")
			log.Log.Error(err, err.Error())
		} else {
			variables = append(variables, match[2])
		}
	}
	return variables, nil
}

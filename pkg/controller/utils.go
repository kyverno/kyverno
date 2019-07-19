package controller

import (
	"bytes"

	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
)

const policyWorkQueueName = "policyworkqueue"

const policyWorkQueueRetryLimit = 3

const policyControllerWorkerCount = 2

func concatFailedRules(frules []v1alpha1.FailedRule) string {
	var buffer bytes.Buffer
	for _, frule := range frules {
		buffer.WriteString(frule.Name + ";")
	}
	return buffer.String()
}

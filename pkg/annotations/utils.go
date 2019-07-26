package annotations

const annotationQueueName = "annotation-queue"
const workerThreadCount = 1
const workQueueRetryLimit = 5

func getStatus(status bool) string {
	if status {
		return "Success"
	}
	return "Failure"
}

func BuildKey(policyName string) string {
	//JSON Pointers
	return "policies.kyverno.io~1" + policyName
}

func BuildKeyString(policyName string) string {
	return "policies.kyverno.io/" + policyName
}

package annotations

const annotationQueueName = "annotation-queue"
const workerThreadCount = 1
const workQueueRetryLimit = 3

func getStatus(status bool) string {
	if status {
		return "Success"
	}
	return "Failure"
}

func BuildKey(policyName string) string {
	return "policies.kyverno.io/" + policyName
}

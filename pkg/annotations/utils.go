package annotations

const annotationQueueName = "annotation-queue"
const workerThreadCount = 1

type info struct {
	RKind string
	RNs   string
	RName string
	Patch []byte
}

func newInfo(rkind, rns, rname string, patch []byte) info {
	return info{
		RKind: rkind,
		RNs:   rname,
		RName: rname,
		Patch: patch,
	}
}

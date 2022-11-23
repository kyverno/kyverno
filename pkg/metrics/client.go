package metrics

type Recorder interface {
	Record(clientQueryOperation ClientQueryOperation)
}

type clientQueryRecorder struct {
	manager MetricsConfigManager
	ns      string
	kind    string
	client  ClientType
}

func NamespacedClientQueryRecorder(m MetricsConfigManager, ns, kind string, client ClientType) Recorder {
	return &clientQueryRecorder{
		manager: m,
		ns:      ns,
		kind:    kind,
		client:  client,
	}
}

func ClusteredClientQueryRecorder(m MetricsConfigManager, kind string, client ClientType) Recorder {
	return &clientQueryRecorder{
		manager: m,
		kind:    kind,
		client:  client,
	}
}

func (r *clientQueryRecorder) Record(clientQueryOperation ClientQueryOperation) {
	r.manager.RecordClientQueries(clientQueryOperation, r.client, r.kind, r.ns)
}

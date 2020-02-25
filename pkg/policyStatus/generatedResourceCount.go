package policyStatus

import v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"

type generatedResourceCount struct {
	sync            *Sync
	generateRequest v1.GenerateRequest
}

func (s *Sync) UpdatePolicyStatusWithGeneratedResourceCount(generateRequest v1.GenerateRequest) {
	s.listener <- &generatedResourceCount{
		sync:            s,
		generateRequest: generateRequest,
	}
}

func (vc *generatedResourceCount) updateStatus() {
	vc.sync.cache.mutex.Lock()
	status, exist := vc.sync.cache.data[vc.generateRequest.Spec.Policy]
	if !exist {
		policy, _ := vc.sync.policyStore.Get(vc.generateRequest.Spec.Policy)
		if policy != nil {
			status = policy.Status
		}
	}

	status.ResourcesGeneratedCount += len(vc.generateRequest.Status.GeneratedResources)

	vc.sync.cache.data[vc.generateRequest.Spec.Policy] = status
	vc.sync.cache.mutex.Unlock()
}

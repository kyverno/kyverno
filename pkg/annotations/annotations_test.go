package annotations

import (
	"encoding/json"
	"testing"

	pinfo "github.com/nirmata/kyverno/pkg/info"
)

func TestAddPatch(t *testing.T) {
	// Create
	objRaw := []byte(`{"kind":"Deployment","apiVersion":"apps/v1","metadata":{"name":"nginx-deployment","namespace":"default","creationTimestamp":null,"labels":{"app":"nginx"}},"spec":{"replicas":1,"selector":{"matchLabels":{"app":"nginx"}},"template":{"metadata":{"creationTimestamp":null,"labels":{"app":"nginx"}},"spec":{"containers":[{"name":"nginx","image":"nginx:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{},"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"},{"name":"ghost","image":"ghost:latest","resources":{},"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","securityContext":{},"schedulerName":"default-scheduler"}},"strategy":{"type":"RollingUpdate","rollingUpdate":{"maxUnavailable":"25%","maxSurge":"25%"}},"revisionHistoryLimit":10,"progressDeadlineSeconds":600},"status":{}}`)
	piRaw := []byte(`{"Name":"set-image-pull-policy","RKind":"Deployment","RName":"nginx-deployment","RNamespace":"default","ValidationFailureAction":"","Rules":[{"Name":"nginx-deployment","Msgs":["Rule nginx-deployment: Overlay succesfully applied."],"RuleType":0}]}`)
	ann := ParseAnnotationsFromObject(objRaw)
	pi := pinfo.PolicyInfo{}
	err := json.Unmarshal(piRaw, &pi)
	if err != nil {
		panic(err)
	}
	ann, _, err = AddPolicyJSONPatch(ann, &pi, pinfo.Mutation)
	if err != nil {
		panic(err)
	}
	// Update
	piRaw = []byte(`{"Name":"set-image-pull-policy","RKind":"Deployment","RName":"nginx-deployment","RNamespace":"default","ValidationFailureAction":"","Rules":[{"Name":"nginx-deployment","Msgs":["Rule nginx-deployment1: Overlay succesfully applied."],"RuleType":0}]}`)
	// ann = ParseAnnotationsFromObject(objRaw)
	pi = pinfo.PolicyInfo{}
	err = json.Unmarshal(piRaw, &pi)
	if err != nil {
		panic(err)
	}
	ann, _, err = AddPolicyJSONPatch(ann, &pi, pinfo.Mutation)
	if err != nil {
		panic(err)
	}
}

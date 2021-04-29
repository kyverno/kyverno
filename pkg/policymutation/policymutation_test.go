package policymutation

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/utils"
	"gotest.tools/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func currentDir() (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}

	return filepath.Join(homedir, "github.com/kyverno/kyverno"), nil
}

func Test_Exclude(t *testing.T) {
	dir, err := os.Getwd()
	baseDir := filepath.Dir(filepath.Dir(dir))
	assert.NilError(t, err)
	file, err := ioutil.ReadFile(baseDir + "/test/best_practices/disallow_bind_mounts.yaml")
	if err != nil {
		t.Log(err)
	}
	policies, err := utils.GetPolicy(file)
	if err != nil {
		t.Log(err)
	}

	policy := policies[0]
	policy.Spec.Rules[0].ExcludeResources.Namespaces = []string{"fake-namespce"}

	rulePatches, errs := generateRulePatches(*policy, engine.PodControllers, log.Log)
	if len(errs) != 0 {
		t.Log(errs)
	}

	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-validate-hostPath","match":{"resources":{"kinds":["DaemonSet","Deployment","Job","StatefulSet"]}},"exclude":{"resources":{"namespaces":["fake-namespce"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}`),
		[]byte(`{"path":"/spec/rules/2","op":"add","value":{"name":"autogen-cronjob-validate-hostPath","match":{"resources":{"kinds":["CronJob"]}},"exclude":{"resources":{"namespaces":["fake-namespce"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}}}`),
	}

	assert.DeepEqual(t, rulePatches, expectedPatches)
}

func Test_CronJobOnly(t *testing.T) {

	controllers := engine.PodControllerCronJob
	dir, err := os.Getwd()
	baseDir := filepath.Dir(filepath.Dir(dir))
	assert.NilError(t, err)
	file, err := ioutil.ReadFile(baseDir + "/test/best_practices/disallow_bind_mounts.yaml")
	if err != nil {
		t.Log(err)
	}
	policies, err := utils.GetPolicy(file)
	if err != nil {
		t.Log(err)
	}

	policy := policies[0]
	policy.SetAnnotations(map[string]string{
		engine.PodControllersAnnotation: controllers,
	})

	rulePatches, errs := generateRulePatches(*policy, controllers, log.Log)
	if len(errs) != 0 {
		t.Log(errs)
	}

	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-cronjob-validate-hostPath","match":{"resources":{"kinds":["CronJob"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}}}`),
	}

	assert.DeepEqual(t, rulePatches, expectedPatches)
}

func Test_CronJob_hasExclude(t *testing.T) {

	controllers := engine.PodControllerCronJob
	dir, err := os.Getwd()
	baseDir := filepath.Dir(filepath.Dir(dir))
	assert.NilError(t, err)

	file, err := ioutil.ReadFile(baseDir + "/test/best_practices/disallow_bind_mounts.yaml")
	if err != nil {
		t.Log(err)
	}
	policies, err := utils.GetPolicy(file)
	if err != nil {
		t.Log(err)
	}

	policy := policies[0]
	policy.SetAnnotations(map[string]string{
		engine.PodControllersAnnotation: controllers,
	})

	rule := policy.Spec.Rules[0].DeepCopy()
	rule.ExcludeResources.Kinds = []string{"Pod"}
	rule.ExcludeResources.Namespaces = []string{"test"}
	policy.Spec.Rules[0] = *rule

	rulePatches, errs := generateRulePatches(*policy, controllers, log.Log)
	if len(errs) != 0 {
		t.Log(errs)
	}

	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-cronjob-validate-hostPath","match":{"resources":{"kinds":["CronJob"]}},"exclude":{"resources":{"kinds":["CronJob"],"namespaces":["test"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}}}`),
	}

	assert.DeepEqual(t, rulePatches, expectedPatches)
}

func Test_CronJobAndDeployment(t *testing.T) {
	controllers := strings.Join([]string{engine.PodControllerCronJob, "Deployment"}, ",")
	dir, err := os.Getwd()
	baseDir := filepath.Dir(filepath.Dir(dir))
	assert.NilError(t, err)
	file, err := ioutil.ReadFile(baseDir + "/test/best_practices/disallow_bind_mounts.yaml")
	if err != nil {
		t.Log(err)
	}
	policies, err := utils.GetPolicy(file)
	if err != nil {
		t.Log(err)
	}

	policy := policies[0]
	policy.SetAnnotations(map[string]string{
		engine.PodControllersAnnotation: controllers,
	})

	rulePatches, errs := generateRulePatches(*policy, controllers, log.Log)
	if len(errs) != 0 {
		t.Log(errs)
	}

	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-validate-hostPath","match":{"resources":{"kinds":["Deployment"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}`),
		[]byte(`{"path":"/spec/rules/2","op":"add","value":{"name":"autogen-cronjob-validate-hostPath","match":{"resources":{"kinds":["CronJob"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}}}`),
	}

	assert.DeepEqual(t, rulePatches, expectedPatches)
}

func Test_getControllers(t *testing.T) {
	testCases := []struct {
		name                string
		policy              []byte
		expectedControllers string
	}{
		{
			name:                "rule-with-match-name",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"test","match":{"resources":{"kinds":["Namespace"],"name":"*"}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-match-selector",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"],"selector":{"matchLabels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-exclude-name",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"test"}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-exclude-selector",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test-getcontrollers"},"spec":{"background":false,"rules":[{"name":"test-getcontrollers","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"selector":{"matchLabels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-deny",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","deny":{"conditions":[{"key":"{{request.object.metadata.labels.foo}}","operator":"Equals","value":"bar"}]}}}]}}`),
			expectedControllers: "none",
		},

		{
			name:                "rule-with-match-mixed-kinds-pod-podcontrollers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"preconditions":{"any":[{"key":"{{request.operation}}","operator":"Equals","value":"CREATE"}]},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-exclude-mixed-kinds-pod-podcontrollers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-match-kinds-pod-only",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"validate":{"message":"testpolicy","pattern":{"metadata":{"labels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: engine.PodControllers,
		},
		{
			name:                "rule-with-exclude-kinds-pod-only",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"require-network-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"kinds":["Pod"],"namespaces":["test"]}},"validate":{"message":"testpolicy","pattern":{"metadata":{"labels":{"foo":"bar"}}}}}]}}`),
			expectedControllers: engine.PodControllers,
		},
		{
			name:                "rule-with-mutate-patches",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"test"},"spec":{"rules":[{"name":"test","match":{"resources":{"kinds":["Pod"]}},"mutate":{"patchesJson6902":"-op:add\npath:/spec/containers/0/env/-1\nvalue:{\"name\":\"SERVICE\",\"value\":{{request.object.spec.template.metadata.labels.app}}}"}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-generate",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"add-networkpolicy"},"spec":{"rules":[{"name":"default-deny-ingress","match":{"resources":{"kinds":["Namespace"],"name":"*"}},"exclude":{"resources":{"namespaces":["kube-system","default","kube-public","kyverno"]}},"generate":{"kind":"NetworkPolicy","name":"default-deny-ingress","namespace":"{{request.object.metadata.name}}","synchronize":true,"data":{"spec":{"podSelector":{},"policyTypes":["Ingress"]}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-predefined-invalid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"DaemonSet,Deployment,StatefulSet","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: "none",
		},
		{
			name:                "rule-with-predefined-valid-controllers",
			policy:              []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"set-service-labels-env"},"annotations":null,"pod-policies.kyverno.io/autogen-controllers":"none","spec":{"background":false,"rules":[{"name":"set-service-label","match":{"resources":{"kinds":["Pod","Deployment"]}},"mutate":{"patchStrategicMerge":{"metadata":{"labels":{"+(service)":"{{request.object.spec.template.metadata.labels.app}}"}}}}}]}}`),
			expectedControllers: "none",
		},
	}

	for _, test := range testCases {
		var policy kyverno.ClusterPolicy
		err := json.Unmarshal(test.policy, &policy)
		assert.NilError(t, err)

		_, controllers := CanAutoGen(&policy, log.Log)
		assert.Equal(t, test.expectedControllers, controllers, fmt.Sprintf("test %s failed", test.name))
	}
}

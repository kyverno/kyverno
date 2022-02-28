package autogen

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
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

func Test_Any(t *testing.T) {
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
	policy.Spec.Rules[0].MatchResources.Any = kyverno.ResourceFilters{
		{
			ResourceDescription: kyverno.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
	}

	rulePatches, errs := GenerateRulePatches(&policy.Spec, engine.PodControllers, log.Log)
	fmt.Println("utils.JoinPatches(patches)erterter", string(utils.JoinPatches(rulePatches)))
	if len(errs) != 0 {
		t.Log(errs)
	}
	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-validate-hostPath","match":{"any":[{"resources":{"kinds":["DaemonSet","Deployment","Job","StatefulSet"]}}],"resources":{"kinds":["Pod"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}`),
		[]byte(`{"path":"/spec/rules/2","op":"add","value":{"name":"autogen-cronjob-validate-hostPath","match":{"any":[{"resources":{"kinds":["CronJob"]}}],"resources":{"kinds":["Pod"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}}}`),
	}

	for i, ep := range expectedPatches {
		assert.Equal(t, string(rulePatches[i]), string(ep),
			fmt.Sprintf("unexpected patch: %s\nexpected: %s", rulePatches[i], ep))
	}
}

func Test_All(t *testing.T) {
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
	policy.Spec.Rules[0].MatchResources.All = kyverno.ResourceFilters{
		{
			ResourceDescription: kyverno.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
	}

	rulePatches, errs := GenerateRulePatches(&policy.Spec, engine.PodControllers, log.Log)
	if len(errs) != 0 {
		t.Log(errs)
	}

	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-validate-hostPath","match":{"all":[{"resources":{"kinds":["DaemonSet","Deployment","Job","StatefulSet"]}}],"resources":{"kinds":["Pod"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}`),
		[]byte(`{"path":"/spec/rules/2","op":"add","value":{"name":"autogen-cronjob-validate-hostPath","match":{"all":[{"resources":{"kinds":["CronJob"]}}],"resources":{"kinds":["Pod"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}}}`),
	}

	for i, ep := range expectedPatches {
		assert.Equal(t, string(rulePatches[i]), string(ep),
			fmt.Sprintf("unexpected patch: %s\nexpected: %s", rulePatches[i], ep))
	}
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

	rulePatches, errs := GenerateRulePatches(&policy.Spec, engine.PodControllers, log.Log)
	if len(errs) != 0 {
		t.Log(errs)
	}

	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-validate-hostPath","match":{"resources":{"kinds":["DaemonSet","Deployment","Job","StatefulSet"]}},"exclude":{"resources":{"namespaces":["fake-namespce"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}`),
		[]byte(`{"path":"/spec/rules/2","op":"add","value":{"name":"autogen-cronjob-validate-hostPath","match":{"resources":{"kinds":["CronJob"]}},"exclude":{"resources":{"namespaces":["fake-namespce"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}}}`),
	}

	for i, ep := range expectedPatches {
		assert.Equal(t, string(rulePatches[i]), string(ep),
			fmt.Sprintf("unexpected patch: %s\nexpected: %s", rulePatches[i], ep))
	}
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

	rulePatches, errs := GenerateRulePatches(&policy.Spec, controllers, log.Log)
	if len(errs) != 0 {
		t.Log(errs)
	}

	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-cronjob-validate-hostPath","match":{"resources":{"kinds":["CronJob"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}}}`),
	}

	assert.DeepEqual(t, rulePatches, expectedPatches)
}

func Test_ForEachPod(t *testing.T) {
	dir, err := os.Getwd()
	baseDir := filepath.Dir(filepath.Dir(dir))
	assert.NilError(t, err)
	file, err := ioutil.ReadFile(baseDir + "/test/policy/mutate/policy_mutate_pod_foreach_with_context.yaml")
	if err != nil {
		t.Log(err)
	}
	policies, err := utils.GetPolicy(file)
	if err != nil {
		t.Log(err)
	}

	policy := policies[0]
	policy.Spec.Rules[0].ExcludeResources.Namespaces = []string{"fake-namespce"}

	rulePatches, errs := GenerateRulePatches(&policy.Spec, engine.PodControllers, log.Log)
	if len(errs) != 0 {
		t.Log(errs)
	}

	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-resolve-image-containers","match":{"resources":{"kinds":["DaemonSet","Deployment","Job","StatefulSet"]}},"exclude":{"resources":{"namespaces":["fake-namespce"]}},"preconditions":{"all":[{"key":"{{request.operation}}","operator":"In","value":["CREATE","UPDATE"]}]},"mutate":{"foreach":[{"list":"request.object.spec.template.spec.containers","context":[{"name":"dictionary","configMap":{"name":"some-config-map","namespace":"some-namespace"}}],"patchStrategicMerge":{"spec":{"template":{"spec":{"containers":[{"image":"{{ dictionary.data.image }}","name":"{{ element.name }}"}]}}}}}]}}}`),
		[]byte(`{"path":"/spec/rules/2","op":"add","value":{"name":"autogen-cronjob-resolve-image-containers","match":{"resources":{"kinds":["CronJob"]}},"exclude":{"resources":{"namespaces":["fake-namespce"]}},"preconditions":{"all":[{"key":"{{request.operation}}","operator":"In","value":["CREATE","UPDATE"]}]},"mutate":{"foreach":[{"list":"request.object.spec.jobTemplate.spec.template.spec.containers","context":[{"name":"dictionary","configMap":{"name":"some-config-map","namespace":"some-namespace"}}],"patchStrategicMerge":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"containers":[{"image":"{{ dictionary.data.image }}","name":"{{ element.name }}"}]}}}}}}}]}}}`),
	}

	for i, ep := range expectedPatches {
		assert.Equal(t, string(rulePatches[i]), string(ep),
			fmt.Sprintf("unexpected patch: %s\nexpected: %s", rulePatches[i], ep))
	}
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

	rulePatches, errs := GenerateRulePatches(&policy.Spec, controllers, log.Log)
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

	rulePatches, errs := GenerateRulePatches(&policy.Spec, controllers, log.Log)
	if len(errs) != 0 {
		t.Log(errs)
	}

	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-validate-hostPath","match":{"resources":{"kinds":["Deployment"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}`),
		[]byte(`{"path":"/spec/rules/2","op":"add","value":{"name":"autogen-cronjob-validate-hostPath","match":{"resources":{"kinds":["CronJob"]}},"validate":{"message":"Host path volumes are not allowed","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"=(volumes)":[{"X(hostPath)":"null"}]}}}}}}}}}`),
	}

	assert.DeepEqual(t, rulePatches, expectedPatches)
}

func Test_UpdateVariablePath(t *testing.T) {
	dir, err := os.Getwd()
	baseDir := filepath.Dir(filepath.Dir(dir))
	assert.NilError(t, err)
	file, err := ioutil.ReadFile(baseDir + "/test/best_practices/select-secrets.yaml")
	if err != nil {
		t.Log(err)
	}
	policies, err := utils.GetPolicy(file)
	if err != nil {
		t.Log(err)
	}

	policy := policies[0]

	rulePatches, errs := GenerateRulePatches(&policy.Spec, engine.PodControllers, log.Log)
	if len(errs) != 0 {
		t.Log(errs)
	}
	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-select-secrets-from-volumes","match":{"resources":{"kinds":["DaemonSet","Deployment","Job","StatefulSet"]}},"context":[{"name":"volsecret","apiCall":{"urlPath":"/api/v1/namespaces/{{request.object.spec.template.metadata.namespace}}/secrets/{{request.object.spec.template.spec.volumes[0].secret.secretName}}","jmesPath":"metadata.labels.foo"}}],"preconditions":[{"key":"{{ request.operation }}","operator":"Equals","value":"CREATE"}],"validate":{"message":"The Secret named {{request.object.spec.template.spec.volumes[0].secret.secretName}} is restricted and may not be used.","pattern":{"spec":{"template":{"spec":{"containers":[{"image":"registry.domain.com/*"}]}}}}}}}`),
		[]byte(`{"path":"/spec/rules/2","op":"add","value":{"name":"autogen-cronjob-select-secrets-from-volumes","match":{"resources":{"kinds":["CronJob"]}},"context":[{"name":"volsecret","apiCall":{"urlPath":"/api/v1/namespaces/{{request.object.spec.template.metadata.namespace}}/secrets/{{request.object.spec.jobTemplate.spec.template.spec.volumes[0].secret.secretName}}","jmesPath":"metadata.labels.foo"}}],"preconditions":[{"key":"{{ request.operation }}","operator":"Equals","value":"CREATE"}],"validate":{"message":"The Secret named {{request.object.spec.jobTemplate.spec.template.spec.volumes[0].secret.secretName}} is restricted and may not be used.","pattern":{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"containers":[{"image":"registry.domain.com/*"}]}}}}}}}}}`),
	}

	assert.DeepEqual(t, rulePatches, expectedPatches)
}

func Test_Deny(t *testing.T) {
	dir, err := os.Getwd()
	baseDir := filepath.Dir(filepath.Dir(dir))
	assert.NilError(t, err)
	file, err := ioutil.ReadFile(baseDir + "/test/policy/deny/policy.yaml")
	if err != nil {
		t.Log(err)
	}
	policies, err := utils.GetPolicy(file)
	if err != nil {
		t.Log(err)
	}

	policy := policies[0]
	policy.Spec.Rules[0].MatchResources.Any = kyverno.ResourceFilters{
		{
			ResourceDescription: kyverno.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
	}

	rulePatches, errs := GenerateRulePatches(&policy.Spec, engine.PodControllers, log.Log)
	fmt.Println("utils.JoinPatches(patches)erterter", string(utils.JoinPatches(rulePatches)))
	if len(errs) != 0 {
		t.Log(errs)
	}
	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/rules/1","op":"add","value":{"name":"autogen-disallow-mount-containerd-sock","match":{"any":[{"resources":{"kinds":["DaemonSet","Deployment","Job","StatefulSet"]}}],"resources":{"kinds":["Pod"]}},"validate":{"foreach":[{"list":"request.object.spec.template.spec.volumes[]","deny":{"conditions":{"any":[{"key":"{{ path_canonicalize(element.hostPath.path) }}","operator":"Equals","value":"/var/run/containerd/containerd.sock"},{"key":"{{ path_canonicalize(element.hostPath.path) }}","operator":"Equals","value":"/run/containerd/containerd.sock"},{"key":"{{ path_canonicalize(element.hostPath.path) }}","operator":"Equals","value":"\\var\\run\\containerd\\containerd.sock"}]}}}]}}}`),
		[]byte(`{"path":"/spec/rules/2","op":"add","value":{"name":"autogen-cronjob-disallow-mount-containerd-sock","match":{"any":[{"resources":{"kinds":["CronJob"]}}],"resources":{"kinds":["Pod"]}},"validate":{"foreach":[{"list":"request.object.spec.jobTemplate.spec.template.spec.volumes[]","deny":{"conditions":{"any":[{"key":"{{ path_canonicalize(element.hostPath.path) }}","operator":"Equals","value":"/var/run/containerd/containerd.sock"},{"key":"{{ path_canonicalize(element.hostPath.path) }}","operator":"Equals","value":"/run/containerd/containerd.sock"},{"key":"{{ path_canonicalize(element.hostPath.path) }}","operator":"Equals","value":"\\var\\run\\containerd\\containerd.sock"}]}}}]}}}`),
	}

	for i, ep := range expectedPatches {
		assert.Equal(t, string(rulePatches[i]), string(ep),
			fmt.Sprintf("unexpected patch: %s\nexpected: %s", rulePatches[i], ep))
	}
}

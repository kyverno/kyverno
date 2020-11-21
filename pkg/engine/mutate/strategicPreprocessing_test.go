package mutate

import (
	"regexp"
	"strings"
	"testing"

	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
)

func Test_preProcessStrategicMergePatch(t *testing.T) {
	rawPolicy := []byte(`{"metadata":{"annotations":{"+(annotation1)":"atest1", "+(annotation2)":"atest2"},"labels":{"+(label1)":"test1"}},"spec":{"(volumes)":[{"(hostPath)":{"path":"/var/run/docker.sock"}}],"containers":[{"(image)":"*:latest","command":["ls"],"imagePullPolicy":"Always"}]}}`)

	rawResource := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"annotations":{"annotation1":"atest2"},"labels":{"label1":"test2","label2":"test2"},"name":"check-root-user"},"spec":{"containers":[{"command":["ll"],"image":"nginx:latest","imagePullPolicy":"Never","name":"nginx"},{"image":"busybox:latest","imagePullPolicy":"Never","name":"busybox"}],"volumes":[{"hostPath":{"path":"/var/run/docker.sock"},"name":"test-volume"}]}}`)

	expected := `{"metadata": {"annotations": {"annotation2":"atest2"}, "labels": {}},"spec": {"containers": [{"command": ["ls", "ll"], "imagePullPolicy": "Always", "name": "nginx"},{"command": ["ls"], "imagePullPolicy": "Always", "name": "busybox"}]}}`

	preProcessedPolicy, err := preProcessStrategicMergePatch(string(rawPolicy), string(rawResource))
	assert.NilError(t, err)
	output, err := preProcessedPolicy.String()
	assert.NilError(t, err)
	re := regexp.MustCompile("\\n")
	if !assertnew.Equal(t, strings.ReplaceAll(expected, " ", ""), strings.ReplaceAll(re.ReplaceAllString(output, ""), " ", "")) {
		t.FailNow()
	}
}

func Test_preProcessStrategicMergePatch_Deployment(t *testing.T) {
	rawPolicy := []byte(`"spec": {
						  "template": {
							 "spec": {
								"containers": [
								   {
									  "(name)": "*",
									  "resources": {
										 "limits": {
											"+(memory)": "300Mi",
											"+(cpu)": "100"
										 }
									  }
								   }
								]
							 }
						  }
					   }`)

	rawResource := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "name": "qos-demo",
		   "labels": {
			  "test": "qos"
		   }
		},
		"spec": {
		   "replicas": 1,
		   "selector": {
			  "matchLabels": {
				 "app": "nginx"
			  }
		   },
		   "template": {
			  "metadata": {
				 "labels": {
					"app": "nginx"
				 }
			  },
			  "spec": {
				 "containers": [
					{
					   "name": "nginx",
					   "image": "nginx:latest",
					   "resources": {
						  "limits": {
							 "cpu": "50m"
						  }
					   }
					}
				 ]
			  }
		   }
		}
	 }`)

	expected := `"spec":{"template":{"spec":{"containers":[{"resources":{"limits":{"memory":"300Mi"}},"name":"nginx"}]}}}`

	preProcessedPolicy, err := preProcessStrategicMergePatch(string(rawPolicy), string(rawResource))
	assert.NilError(t, err)
	output, err := preProcessedPolicy.String()
	assert.NilError(t, err)
	re := regexp.MustCompile("\\n")
	if !assertnew.Equal(t, strings.ReplaceAll(expected, " ", ""), strings.ReplaceAll(re.ReplaceAllString(output, ""), " ", "")) {
		t.FailNow()
	}
}

func Test_preProcessStrategicMergePatch_AnnotationMap(t *testing.T) {
	rawPolicy := []byte(`{
		"metadata": {
			"annotations": {
				"+(alb.ingress.kubernetes.io/backend-protocol)": "HTTPS",
				"+(alb.ingress.kubernetes.io/healthcheck-protocol)": "HTTPS",
				"+(alb.ingress.kubernetes.io/scheme)": "internal",
				"+(alb.ingress.kubernetes.io/target-type)": "ip",
				"+(kubernetes.io/ingress.class)": "alb"
			}
		}
	}`)

	rawResource := []byte(`{"apiVersion": "extensions/v1beta1","kind": "Ingress","metadata": {"annotations": {"alb.ingress.kubernetes.io/backend-protocol": "HTTPS","alb.ingress.kubernetes.io/healthcheck-protocol": "HTTPS","alb.ingress.kubernetes.io/scheme": "internal","alb.ingress.kubernetes.io/target-type": "ip","external-dns.alpha.kubernetes.io/hostname": "argo","kubernetes.io/ingress.class": "test"},"labels": {"app": "argocd-server","app.kubernetes.io/name": "argocd-server"},"name": "argocd","namespace": "default"}}`)

	expected := `{"metadata":{"annotations":{}}}`

	preProcessedPolicy, err := preProcessStrategicMergePatch(string(rawPolicy), string(rawResource))
	assert.NilError(t, err)
	output, err := preProcessedPolicy.String()
	assert.NilError(t, err)

	re := regexp.MustCompile("\\n")
	if !assertnew.Equal(t, strings.ReplaceAll(expected, " ", ""), strings.ReplaceAll(re.ReplaceAllString(output, ""), " ", "")) {
		t.FailNow()
	}
}

func Test_preProcessStrategicMergePatch_Annotation(t *testing.T) {
	rawPolicy := []byte(`{"metadata":{"annotations":{"+(cluster-autoscaler.kubernetes.io/safe-to-evict)":true}},"spec":{"volumes":[{"(hostPath)":{"path":"*"}}]}}`)

	rawResource := []byte(`{"kind":"Pod","apiVersion":"v1","metadata":{"name":"nginx","annotations":{"cluster-autoscaler.kubernetes.io/safe-to-evict":"false"}},"spec":{"containers":[{"name":"nginx","image":"nginx:latest","imagePullPolicy":"Never","volumeMounts":[{"mountPath":"/cache","name":"cache-volume"}]}],"volumes":[{"name":"cache-volume","hostPath":{"path":"/data","type":"Directory"}}]}}`)

	expected := `{"metadata":{"annotations":{}},"spec":{"volumes":[{"name":"cache-volume"}]}}`

	preProcessedPolicy, err := preProcessStrategicMergePatch(string(rawPolicy), string(rawResource))
	assert.NilError(t, err)
	output, err := preProcessedPolicy.String()
	assert.NilError(t, err)
	re := regexp.MustCompile("\\n")
	if !assertnew.Equal(t, strings.ReplaceAll(expected, " ", ""), strings.ReplaceAll(re.ReplaceAllString(output, ""), " ", "")) {
		t.FailNow()
	}
}

func Test_preProcessStrategicMergePatch_BlankAnnotation(t *testing.T) {
	rawPolicy := []byte(`{"metadata":{"annotations":{"+(cluster-autoscaler.kubernetes.io/safe-to-evict)":true},"labels":{"+(add-labels)":"add"}},"spec":{"volumes":[{"(hostPath)":{"path":"*"}}]}}`)

	rawResource := []byte(`{"kind":"Pod","apiVersion":"v1","metadata":{"name":"nginx"},"spec":{"containers":[{"name":"nginx","image":"nginx:latest","imagePullPolicy":"Never","volumeMounts":[{"mountPath":"/cache","name":"cache-volume"}]}],"volumes":[{"name":"cache-volume","hostPath":{"path":"/data","type":"Directory"}}]}}`)

	expected := `{"metadata":{"annotations":{"cluster-autoscaler.kubernetes.io/safe-to-evict":true},"labels":{"add-labels":"add"}},"spec":{"volumes":[{"name":"cache-volume"}]}}`

	preProcessedPolicy, err := preProcessStrategicMergePatch(string(rawPolicy), string(rawResource))
	assert.NilError(t, err)
	output, err := preProcessedPolicy.String()
	assert.NilError(t, err)
	re := regexp.MustCompile("\\n")
	if !assertnew.Equal(t, strings.ReplaceAll(expected, " ", ""), strings.ReplaceAll(re.ReplaceAllString(output, ""), " ", "")) {
		t.FailNow()
	}
}

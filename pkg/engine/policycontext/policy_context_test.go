package policycontext

import (
	"encoding/json"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/assert"
)

var (
	cfg = config.NewDefaultConfiguration(false)
	jp  = jmespath.New(cfg)
)

func TestPolicyContextRefresh(t *testing.T) {
	// If image tag is latest then imagepull policy needs to be checked
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-image"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-tag",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "An image tag is required",
					"pattern": {
					   "spec": {
						  "containers": [
							 {
								"image": "*:*"
							 }
						  ]
					   }
					}
				 }
			  },
			  {
				 "name": "validate-latest",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "imagePullPolicy 'Always' required with tag 'latest'",
					"pattern": {
					   "spec": {
						  "containers": [
							 {
								"(image)": "*latest",
								"imagePullPolicy": "NotPresent"
							 }
						  ]
					   }
					}
				 }
			  }
		   ]
		}
	 }
	`)

	rawNewResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		   "name": "test-pod",
		   "labels": {
			  "version": "new"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "name": "nginx",
				 "image": "nginx:latest",
				 "imagePullPolicy": "Always"
			  }
		   ]
		}
	 }
	`)

	rawOldResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		   "name": "test-pod",
		   "labels": {
			  "version": "old"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "name": "nginx",
				 "image": "nginx:latest",
				 "imagePullPolicy": "Always"
			  }
		   ]
		}
	 }
	`)

	var policy kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	newResourceUnstructured, err := kubeutils.BytesToUnstructured(rawNewResource)
	assert.NilError(t, err)
	oldResourceUnstructured, err := kubeutils.BytesToUnstructured(rawOldResource)
	assert.NilError(t, err)

	pc, err := NewPolicyContext(jp, *newResourceUnstructured, kyvernov1.Update, nil, cfg)
	assert.NilError(t, err)
	pc = pc.WithOldResource(*oldResourceUnstructured)

	policyContext, err := pc.OldPolicyContext()
	assert.NilError(t, err)
	newResourceVersionLabel := policyContext.NewResource().Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["version"].(string)
	assert.Equal(t, newResourceVersionLabel, "old")
	assert.Equal(t, len(policyContext.OldResource().Object), 0)

	policyContext, err = policyContext.RefreshPolicyContext()
	newResourceVersionLabel = policyContext.NewResource().Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["version"].(string)
	assert.Equal(t, newResourceVersionLabel, "new")
	oldResourceVersionLabel := policyContext.OldResource().Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["version"].(string)
	assert.Equal(t, oldResourceVersionLabel, "old")
	assert.NilError(t, err)
}

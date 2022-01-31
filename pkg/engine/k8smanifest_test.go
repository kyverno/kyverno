package engine

import (
	"testing"

	mapnode "github.com/sigstore/k8s-manifest-sigstore/pkg/util/mapnode"
	"gotest.tools/assert"
)

var test_policy = `{}`

var signed_resource = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
  		"annotations": {
    		"cosign.sigstore.dev/message": "H4sIAAAAAAAA/wD5AAb/H4sIAAAAAAAA/+zQwUrEMBAG4J7zFPMC3c7EbSt5Ck/eh3aoYTeTkmQLq/ju0mUVT4p7EbHf5SfJkJC/KWFuhhjmJDl7nerCqZ6eiTrbdT3RfZMkx1MaZHfmcKxugYjYt+2a1Lf4OS+sbSvaU0+2R3tnK7S076gCvOm1HzrlwqlCVA6sX8x9d379y0f+ETz7R0nZR3WwkDl4HR08xNEEKTxyYWcAWDUWLj5qdvDyagCUgzgokks9x9HkWYZ1cIha2KukvK5q8IEncTA9DWnnY3Pp8GgRqTmcF0ka3UIG4P2+66b57VI2m83mH3gLAAD//wTXx/kACAAAAQAA//+pDnnm+QAAAA==",
    		"cosign.sigstore.dev/signature": "MEUCIBOoHsOltuGTwefyXro4lWhI7IAxysMvP/AIcDM9Ge6kAiEAt9Z/kNp8VYP8iAh6LiMvsM+Q3ju9pdG5KNW9YZakTmE="
		},
		"name": "test-pod"
	},
	"spec": {
		"containers": [
			{
			  "name": "kyverno",
			  "image": "ghcr.io/kyverno/kyverno"
			}
		]
	}
}`

const ecdsaPub = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEtS69SctDWseJdFcQnpJGh3Yq3WfM
EpLCSBfCcCyTotqkFCsGjhAFiSblvmX8vn51dbnZ7cdtiahat/eehymyJg==
-----END PUBLIC KEY-----`

func Test_VerifyManifest(t *testing.T) {
	policyContext := buildContext(t, test_policy, signed_resource)
	var diffVar *mapnode.DiffResult
	ignoreFields := []string{"spec.containers.0.image"}

	verified, diff, err := VerifyManifest(policyContext, ecdsaPub, ignoreFields)
	assert.NilError(t, err)
	assert.Equal(t, verified, true)
	assert.Equal(t, diff, diffVar)

}

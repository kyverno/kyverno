package engine

import (
	"testing"

	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	mapnode "github.com/sigstore/k8s-manifest-sigstore/pkg/util/mapnode"
	"gotest.tools/assert"
)

var test_policy = `{}`

var signed_resource = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
  		"annotations": {
    		"cosign.sigstore.dev/message": "H4sIAAAAAAAA/wDiAB3/H4sIAAAAAAAA/+zPTUrGMBAG4KxzirlA20l/vtCcwpX7oQ0laCYhGcUfvLtUVHSj8m1E7LN5CXmTYTqJuVtSzMXXGnhrhEqzPQwzop3Gee67nNb2nuK1Ohvi/tm0p7ETfswXxlplxh6tHU4nOyjscZwGBXj+yJ+7qUJFITJF4i96392/7vKefwTlcOlLDYkd3Bp9FXh1cJFWHb3QSkJOAxBzEpKQuDp4fNIATNE74C3wna7ZL3trSSwU2Je6nxoIkba3kjOtGdteA3x++9vrHw6Hw7/1HAAA//830Kk7AAgAAAEAAP//Byxp7uIAAAA=",
    		"cosign.sigstore.dev/signature": "MEUCIA2LILw5kUfcig/l3bFk8JnW3xWCk3AKSMaiWl6Q2DBqAiEAzOAkJjpLk5w86PD+vy/Wr+hRm6R8/dE37YuDzqIbT9c="
		},
		"name": "nginx"
	},
	"spec": {
		"containers": [
			{
				"image": "nginx:1.14.2",
				"name": "nginx"
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
	ignoreFields := k8smanifest.ObjectFieldBindingList{}

	verified, diff, err := VerifyManifest(policyContext, ecdsaPub, ignoreFields)
	assert.NilError(t, err)
	assert.Equal(t, verified, true)
	assert.Equal(t, diff, diffVar)
}

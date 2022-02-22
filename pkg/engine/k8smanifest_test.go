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
	"kind": "Secret",
	"metadata": {
  		"annotations": {
    		"cosign.sigstore.dev/message": "H4sIAAAAAAAA/wD/AAD/H4sIAAAAAAAA/+zSTU7DMBAF4KxzirlAmrHTYMlrboDEftpYxSoeW54JqCDujmgr1BWIbhAo3+YtbD/LP72m0m9zKjWIRN51SrXbvQzD2jmDxrpewrYGXR0oPTbXQUR04/iRxo14mUd2fdOYtcXBWTcOY4PWGOsawCv3+5FZlGqDyJSIv5j33fj5LJ/5R1CJ96FKzOzhybT7yJOHu+OTtykoTaTkWwBizkoaM4uH17cWgCkFD6ff0W1I4rajWR9a0Rp5d3teV0jkOdfJg2LpzsUAs4R6KqApRW71UIKH/bwJlYMGWcXcX3T+9i0tFovF//MeAAD//1D95RUACAAAAQAA//+x8WKU/wAAAA==",
    		"cosign.sigstore.dev/signature": "MEYCIQDucAI+AguMuKbrKvHf9oQK2Kl36qndbuOGg895Wt9E5gIhANzHdauuP0FMfhm0rzOFCECdBFvh32Luc5FXvBXBJ2i3"
		},
		"name": "secret"
	},
	"stringData": {
		"password": "naman",
		"username": "admin"
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
	assert.Equal(t, verified, false)
	assert.Equal(t, diff, diffVar)
}

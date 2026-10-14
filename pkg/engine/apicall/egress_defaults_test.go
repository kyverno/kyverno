package apicall

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kyverno/kyverno/pkg/toggle"
	"gotest.tools/v3/assert"
)

// The unspecified address connects to a loopback listener but is not inside
// 127.0.0.0/8, so the loopback entry alone did not stop a policy reaching a
// sidecar in the Kyverno pod.
func Test_ExecuteServiceCall_BlocksUnspecifiedAddressByDefault(t *testing.T) {
	toggle.HTTPBlocklist.Reset()
	resetSharedServiceHTTP()

	var gotHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	assert.NilError(t, err)

	for _, host := range []string{"0.0.0.0", "[::]"} {
		t.Run(host, func(t *testing.T) {
			gotHit = false
			_, err := testExecutor().Execute(context.Background(), getServiceCall("http://"+host+":"+port+"/"))
			assert.ErrorContains(t, err, "blocked")
			assert.Assert(t, !gotHit)
		})
	}
}

// The blocklist must cover the whole link-local range, not only the metadata
// addresses an advisory happened to name. 169.254.170.2 is the AWS ECS task
// metadata endpoint, which serves credentials without authentication.
func Test_ExecuteServiceCall_BlocksLinkLocalRangeByDefault(t *testing.T) {
	toggle.HTTPBlocklist.Reset()
	resetSharedServiceHTTP()

	for _, url := range []string{
		"http://169.254.170.2/v2/credentials/",
		"http://169.254.169.254/latest/meta-data/",
		"http://169.254.169.253/computeMetadata/v1/",
		"http://100.100.100.200/latest/meta-data/",
		"http://[fe80::1]/",
	} {
		t.Run(url, func(t *testing.T) {
			_, err := testExecutor().Execute(context.Background(), getServiceCall(url))
			assert.ErrorContains(t, err, "blocked")
		})
	}
}

// Oracle Cloud's metadata hostname is blocked by name, since its address is not
// in any blocked range.
func Test_ExecuteServiceCall_BlocksOracleMetadataHostByDefault(t *testing.T) {
	toggle.HTTPBlocklist.Reset()
	resetSharedServiceHTTP()

	_, err := testExecutor().Execute(context.Background(), getServiceCall("http://metadata.internal/opc/v1/instance/"))
	assert.ErrorContains(t, err, "blocked")
}

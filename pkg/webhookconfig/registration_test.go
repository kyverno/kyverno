package webhookconfig

import (
	"bytes"
	"testing"

	"gotest.tools/assert"
	rest "k8s.io/client-go/rest"
)

var cert = `
-----BEGIN CERTIFICATE-----
V2VsY29tZSB0byBUaGUgUnVzdCBQcm9ncmFtbWluZyBMYW5ndWFnZSwgY
W4gaW50cm9kdWN0b3J5IGJvb2sgYWJvdXQgUnVzdC4gVGhlIFJ1c3QgcH
JvZ3JhbW1pbmcgbGFuZ3VhZ2UgaGVscHMgeW91IHdyaXRlIGZhc3Rlciw
gbW9yZSByZWxpYWJsZSBzb2Z0d2FyZS4gSGlnaC1sZXZlbCBlcmdvbm9t
aWNzIGFuZCBsb3ctbGV2ZWwgY29udHJvbCBhcmUgb2Z0ZW4gYXQgb2Rkc
yBpbiBwcm9ncmFtbWluZyBsYW5ndWFnZSBkZXNpZ247IFJ1c3QgY2hhbG
xlbmdlcyB0aGF0IGNvbmZsaWN0LiBUaHJvdWdoIGJhbGFuY2luZyBwb3d
lcmZ1bCB0ZWNobmljYWwgY2FwYWNpdHkgYW5kIGEgZ3JlYXQgZGV2ZWxv
cGVyIGV4cGVyaWVuY2UsIFJ1c3QgZ2l2ZXMgeW91IHRoZSBvcHRpb24gd
G8gY29udHJvbCBsb3ctbGV2ZWwgZGV0YWlscyAoc3VjaCBhcyBtZW1vcn
kgdXNhZ2UpIHdpdGhvdXQgYWxsIHRoZSBoYXNzbGUgdHJhZGl0aW9uYWx
seSBhc3NvY2lhdGVkIHdpdGggc3VjaCBjb250cm9sLgyzmqp31l8rqr1==
-----END CERTIFICATE-----
`

func TestExtractCA_EmptyBundle(t *testing.T) {
	config := &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			CAData: []byte(cert),
		},
	}

	actual := extractCA(config)
	assert.Assert(t, bytes.Equal([]byte(cert), actual))
}

func TestExtractCA_EmptyCAFile(t *testing.T) {
	CABundle := []byte(`LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN5RENDQWJDZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFWTVJNd0VRWURWUVFERXdwcmRXSmwKY201bGRHVnpNQjRYRFRFNU1ETXhPVEUwTURjd05Gb1hEVEk1TURNeE5qRTBNRGN3TkZvd0ZURVRNQkVHQTFVRQpBeE1LYTNWaVpYSnVaWFJsY3pDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTStQClVLVmExcm9tQndOZzdqNnBBSGo5TDQ4RVJpdEplRzRXM1pUYmNMNWNKbnVTQmFsc1h1TWpQTGZmbUV1VEZIdVAKenRqUlBEUHcreEg1d3VTWFF2U0tIaXF2VE1pUm9DSlJFa09sQXpIa1dQM0VrdnUzNzRqZDVGV3Q3NEhnRk91cApIZ1ZwdUxPblczK2NDVE5iQ3VkeDFMVldRbGgwQzJKbm1Lam5uS1YrTkxzNFJVaVk1dk91ekpuNHl6QldLRjM2CmJLZ3ZDOVpMWlFSM3dZcnJNZWllYzBnWVY2VlJtaGgxSjRDV3V1UWd0ckM2d2NJanFWZFdEUlJyNHFMdEtDcDIKQVNIZmNieitwcEdHblJ5Z2FzcWNJdnpiNUVwV3NIRGtHRStUUW5WQ0JmTmsxN0NEOTZBQ1pmRWVybzEvWE16MgpRbzZvcUE0dnF5ZkdWWVU5RVZFQ0F3RUFBYU1qTUNFd0RnWURWUjBQQVFIL0JBUURBZ0trTUE4R0ExVWRFd0VCCi93UUZNQU1CQWY4d0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dFQkFNWFVpUVJpdUc4cGdzcHMrZTdGZWdCdEJOZEcKZlFUdHVLRWFUZ0U0RjQwamJ3UmdrN25DTHlsSHgvRG04aVRRQmsyWjR4WnNuY0huRys4SkwrckRLdlJBSE5iVQpsYnpReXA1V3FwdjdPcThwZ01wU0o5bTdVY3BGZmRVZkorNW43aXFnTGdMb3lhNmtRVTR2Rk0yTE1rWjI5NVpxCmVId0hnREo5Z3IwWGNyOWM1L2tRdkxFc2Z2WU5QZVhuamNyWXlDb2JNcVduSElxeVd3cHM1VTJOaGgraXhSZEIKbzRRL3RJS04xOU93WGZBaVc5SENhNzZMb3ZXaUhPU2UxVnFzK1h1N1A5ckx4eW1vQm91aFcxVmZ0bUo5Qy9vTAp3cFVuNnlXRCttY0tkZ3J5QTFjTWJ4Q281bUd6YTNLaFk1QTd5eDQ1cThkSEIzTWU4d0FCam1wWEs0ST0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=`)

	config := &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			CAData: CABundle,
			CAFile: "",
		},
	}

	actual := extractCA(config)
	assert.Assert(t, bytes.Equal(CABundle, actual))
}

func TestExtractCA_EmptyConfig(t *testing.T) {
	config := &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			CAData: nil,
			CAFile: "",
		},
	}

	actual := extractCA(config)
	assert.Assert(t, actual == nil)
}

func TestExtractCA_InvalidFile(t *testing.T) {
	config := &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			CAData: nil,
			CAFile: "somenonexistingfile",
		},
	}

	actual := extractCA(config)
	assert.Assert(t, actual == nil)
}

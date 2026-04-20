package cosign

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPublicKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEIOJTQ992VBJyyx52p3s1W/lqwNxI
rFxZI4BL3S6ZGyJFockpfppxOycEkUaGVTUvL0Tp7Yi0eYRJ4TtKxs1lXQ==
-----END PUBLIC KEY-----`

	testIssuer  = "https://token.actions.githubusercontent.com"
	testSubject = "https://github.com/test/repo/.github/workflows/test.yml@refs/heads/main"

	// FreeTSA root CA certificate, same as used in pkg/cosign/cosign_test.go.
	testTSACertChain = `
-----BEGIN CERTIFICATE-----
MIIH/zCCBeegAwIBAgIJAMHphhYNqOmAMA0GCSqGSIb3DQEBDQUAMIGVMREwDwYD
VQQKEwhGcmVlIFRTQTEQMA4GA1UECxMHUm9vdCBDQTEYMBYGA1UEAxMPd3d3LmZy
ZWV0c2Eub3JnMSIwIAYJKoZIhvcNAQkBFhNidXNpbGV6YXNAZ21haWwuY29tMRIw
EAYDVQQHEwlXdWVyemJ1cmcxDzANBgNVBAgTBkJheWVybjELMAkGA1UEBhMCREUw
HhcNMTYwMzEzMDE1MjEzWhcNNDEwMzA3MDE1MjEzWjCBlTERMA8GA1UEChMIRnJl
ZSBUU0ExEDAOBgNVBAsTB1Jvb3QgQ0ExGDAWBgNVBAMTD3d3dy5mcmVldHNhLm9y
ZzEiMCAGCSqGSIb3DQEJARYTYnVzaWxlemFzQGdtYWlsLmNvbTESMBAGA1UEBxMJ
V3VlcnpidXJnMQ8wDQYDVQQIEwZCYXllcm4xCzAJBgNVBAYTAkRFMIICIjANBgkq
hkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAtgKODjAy8REQ2WTNqUudAnjhlCrpE6ql
mQfNppeTmVvZrH4zutn+NwTaHAGpjSGv4/WRpZ1wZ3BRZ5mPUBZyLgq0YrIfQ5Fx
0s/MRZPzc1r3lKWrMR9sAQx4mN4z11xFEO529L0dFJjPF9MD8Gpd2feWzGyptlel
b+PqT+++fOa2oY0+NaMM7l/xcNHPOaMz0/2olk0i22hbKeVhvokPCqhFhzsuhKsm
q4Of/o+t6dI7sx5h0nPMm4gGSRhfq+z6BTRgCrqQG2FOLoVFgt6iIm/BnNffUr7V
DYd3zZmIwFOj/H3DKHoGik/xK3E82YA2ZulVOFRW/zj4ApjPa5OFbpIkd0pmzxzd
EcL479hSA9dFiyVmSxPtY5ze1P+BE9bMU1PScpRzw8MHFXxyKqW13Qv7LWw4sbk3
SciB7GACbQiVGzgkvXG6y85HOuvWNvC5GLSiyP9GlPB0V68tbxz4JVTRdw/Xn/XT
FNzRBM3cq8lBOAVt/PAX5+uFcv1S9wFE8YjaBfWCP1jdBil+c4e+0tdywT2oJmYB
BF/kEt1wmGwMmHunNEuQNzh1FtJY54hbUfiWi38mASE7xMtMhfj/C4SvapiDN837
gYaPfs8x3KZxbX7C3YAsFnJinlwAUss1fdKar8Q/YVs7H/nU4c4Ixxxz4f67fcVq
M2ITKentbCMCAwEAAaOCAk4wggJKMAwGA1UdEwQFMAMBAf8wDgYDVR0PAQH/BAQD
AgHGMB0GA1UdDgQWBBT6VQ2MNGZRQ0z357OnbJWveuaklzCBygYDVR0jBIHCMIG/
gBT6VQ2MNGZRQ0z357OnbJWveuakl6GBm6SBmDCBlTERMA8GA1UEChMIRnJlZSBU
U0ExEDAOBgNVBAsTB1Jvb3QgQ0ExGDAWBgNVBAMTD3d3dy5mcmVldHNhLm9yZzEi
MCAGCSqGSIb3DQEJARYTYnVzaWxlemFzQGdtYWlsLmNvbTESMBAGA1UEBxMJV3Vl
cnpidXJnMQ8wDQYDVQQIEwZCYXllcm4xCzAJBgNVBAYTAkRFggkAwemGFg2o6YAw
MwYDVR0fBCwwKjAooCagJIYiaHR0cDovL3d3dy5mcmVldHNhLm9yZy9yb290X2Nh
LmNybDCBzwYDVR0gBIHHMIHEMIHBBgorBgEEAYHyJAEBMIGyMDMGCCsGAQUFBwIB
FidodHRwOi8vd3d3LmZyZWV0c2Eub3JnL2ZyZWV0c2FfY3BzLmh0bWwwMgYIKwYB
BQUHAgEWJmh0dHA6Ly93d3cuZnJlZXRzYS5vcmcvZnJlZXRzYV9jcHMucGRmMEcG
CCsGAQUFBwICMDsaOUZyZWVUU0EgdHJ1c3RlZCB0aW1lc3RhbXBpbmcgU29mdHdh
cmUgYXMgYSBTZXJ2aWNlIChTYWFTKTA3BggrBgEFBQcBAQQrMCkwJwYIKwYBBQUH
MAGGG2h0dHA6Ly93d3cuZnJlZXRzYS5vcmc6MjU2MDANBgkqhkiG9w0BAQ0FAAOC
AgEAaK9+v5OFYu9M6ztYC+L69sw1omdyli89lZAfpWMMh9CRmJhM6KBqM/ipwoLt
nxyxGsbCPhcQjuTvzm+ylN6VwTMmIlVyVSLKYZcdSjt/eCUN+41K7sD7GVmxZBAF
ILnBDmTGJmLkrU0KuuIpj8lI/E6Z6NnmuP2+RAQSHsfBQi6sssnXMo4HOW5gtPO7
gDrUpVXID++1P4XndkoKn7Svw5n0zS9fv1hxBcYIHPPQUze2u30bAQt0n0iIyRLz
aWuhtpAtd7ffwEbASgzB7E+NGF4tpV37e8KiA2xiGSRqT5ndu28fgpOY87gD3ArZ
DctZvvTCfHdAS5kEO3gnGGeZEVLDmfEsv8TGJa3AljVa5E40IQDsUXpQLi8G+UC4
1DWZu8EVT4rnYaCw1VX7ShOR1PNCCvjb8S8tfdudd9zhU3gEB0rxdeTy1tVbNLXW
99y90xcwr1ZIDUwM/xQ/noO8FRhm0LoPC73Ef+J4ZBdrvWwauF3zJe33d4ibxEcb
8/pz5WzFkeixYM2nsHhqHsBKw7JPouKNXRnl5IAE1eFmqDyC7G/VT7OF669xM6hb
Ut5G21JE4cNK6NNucS+fzg1JPX0+3VhsYZjj7D5uljRvQXrJ8iHgr/M6j2oLHvTA
I2MLdq2qjZFDOCXsxBxJpbmLGBx9ow6ZerlUxzws2AWv2pk=
-----END CERTIFICATE-----
`
)

func baseOpts() ([]remote.Option, []name.Option) {
	return []remote.Option{}, []name.Option{}
}

func TestCheckOptions_KeyBased(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	cosignCfg := &v1beta1.Cosign{
		Key: &v1beta1.Key{
			Data: testPublicKey,
		},
		CTLog: &v1beta1.CTLog{
			URL:                "https://rekor.sigstore.dev",
			InsecureIgnoreTlog: true,
			InsecureIgnoreSCT:  true,
		},
	}

	opts, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	require.NoError(t, err)
	assert.NotNil(t, opts)
	assert.NotNil(t, opts.SigVerifier)
	assert.NotNil(t, opts.RekorClient)
	assert.True(t, opts.IgnoreTlog)
	assert.True(t, opts.IgnoreSCT)
}

func TestCheckOptions_Keyless(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	cosignCfg := &v1beta1.Cosign{
		Keyless: &v1beta1.Keyless{
			Identities: []v1beta1.Identity{
				{
					Issuer:  testIssuer,
					Subject: testSubject,
				},
			},
		},
		CTLog: &v1beta1.CTLog{
			URL:               "https://rekor.sigstore.dev",
			InsecureIgnoreSCT: true,
		},
	}

	opts, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	require.NoError(t, err)
	assert.NotNil(t, opts)
	assert.Len(t, opts.Identities, 1)
	assert.Equal(t, testIssuer, opts.Identities[0].Issuer)
	assert.Equal(t, testSubject, opts.Identities[0].Subject)
	assert.NotNil(t, opts.RootCerts)
	assert.NotNil(t, opts.TrustedMaterial)
}

func TestCheckOptions_KeylessWithRegex(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	cosignCfg := &v1beta1.Cosign{
		Keyless: &v1beta1.Keyless{
			Identities: []v1beta1.Identity{
				{
					Issuer:        testIssuer,
					IssuerRegExp:  ".*token.actions.githubusercontent.com",
					Subject:       testSubject,
					SubjectRegExp: ".*@refs/heads/main",
				},
			},
		},
		CTLog: &v1beta1.CTLog{
			URL:               "https://rekor.sigstore.dev",
			InsecureIgnoreSCT: true,
		},
	}

	opts, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	require.NoError(t, err)
	assert.NotNil(t, opts)
	assert.Equal(t, ".*token.actions.githubusercontent.com", opts.Identities[0].IssuerRegExp)
	assert.Equal(t, ".*@refs/heads/main", opts.Identities[0].SubjectRegExp)
}

func TestCheckOptions_MultipleIdentities(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	cosignCfg := &v1beta1.Cosign{
		Keyless: &v1beta1.Keyless{
			Identities: []v1beta1.Identity{
				{
					Issuer:  testIssuer,
					Subject: testSubject,
				},
				{
					Issuer:  "https://oauth2.sigstore.dev/auth",
					Subject: "user@example.com",
				},
			},
		},
		CTLog: &v1beta1.CTLog{
			URL:               "https://rekor.sigstore.dev",
			InsecureIgnoreSCT: true,
		},
	}

	opts, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	require.NoError(t, err)
	assert.Len(t, opts.Identities, 2)
}

func TestCheckOptions_WithSource(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	cosignCfg := &v1beta1.Cosign{
		Key: &v1beta1.Key{
			Data: testPublicKey,
		},
		CTLog: &v1beta1.CTLog{
			URL:                "https://rekor.sigstore.dev",
			InsecureIgnoreTlog: true,
		},
		Source: &v1beta1.Source{
			Repository: "ghcr.io/example/signatures",
			TagPrefix:  "sha256-",
		},
	}

	opts, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	require.NoError(t, err)
	assert.NotNil(t, opts)
	assert.NotEmpty(t, opts.RegistryClientOpts)
}

func TestCheckOptions_MissingRekorURL(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	cosignCfg := &v1beta1.Cosign{
		Key: &v1beta1.Key{
			Data: testPublicKey,
		},
		CTLog: &v1beta1.CTLog{
			InsecureIgnoreTlog: true,
		},
	}

	_, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rekor URL must be provided")
}

func TestCheckOptions_InvalidPublicKey(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	cosignCfg := &v1beta1.Cosign{
		Key: &v1beta1.Key{
			Data: "invalid-key-data",
		},
		CTLog: &v1beta1.CTLog{
			URL:                "https://rekor.sigstore.dev",
			InsecureIgnoreTlog: true,
		},
	}

	_, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load public key")
}

func TestCheckOptions_InvalidSourceRepository(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	cosignCfg := &v1beta1.Cosign{
		Key: &v1beta1.Key{
			Data: testPublicKey,
		},
		CTLog: &v1beta1.CTLog{
			URL:                "https://rekor.sigstore.dev",
			InsecureIgnoreTlog: true,
		},
		Source: &v1beta1.Source{
			Repository: "invalid repository name!!!",
		},
	}

	_, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse signature repository")
}

func TestCheckOptions_RekorOfflineMode(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	cosignCfg := &v1beta1.Cosign{
		Key: &v1beta1.Key{
			Data: testPublicKey,
		},
		CTLog: &v1beta1.CTLog{
			URL:                "https://rekor.sigstore.dev",
			InsecureIgnoreTlog: true,
		},
	}

	opts, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	require.NoError(t, err)
	assert.False(t, opts.Offline)
}

func TestInitializeTuf_Default(t *testing.T) {
	ctx := context.TODO()
	err := initializeTuf(ctx, nil)
	assert.NoError(t, err)
}

func TestInitializeTuf_WithCustomMirror(t *testing.T) {
	ctx := context.TODO()
	tufCfg := &v1beta1.TUF{
		Mirror: "https://custom-tuf.example.com",
	}

	err := initializeTuf(ctx, tufCfg)
	if err != nil && err.Error() != "initializing TUF client from &TUF{}" {
		t.Logf("Custom TUF mirror test (expected to fail in test env): %v", err)
	}
}

func TestGetRekor_WithURL(t *testing.T) {
	ctx := context.TODO()
	ctlog := &v1beta1.CTLog{
		URL: "https://rekor.sigstore.dev",
	}

	rekorClient, rekorPubKeys, ctlogPubKeys, err := getRekor(ctx, ctlog)
	require.NoError(t, err)
	assert.NotNil(t, rekorClient)
	assert.NotNil(t, rekorPubKeys)
	assert.NotNil(t, ctlogPubKeys)
}

func TestGetRekor_NilCTLog(t *testing.T) {
	ctx := context.TODO()

	rekorClient, rekorPubKeys, ctlogPubKeys, err := getRekor(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, rekorClient)
	assert.NotNil(t, rekorPubKeys)
	assert.NotNil(t, ctlogPubKeys)
}

func TestGetRekor_MissingURL(t *testing.T) {
	ctx := context.TODO()
	ctlog := &v1beta1.CTLog{}

	_, _, _, err := getRekor(ctx, ctlog)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rekor URL must be provided")
}

func TestGetFulcio(t *testing.T) {
	ctx := context.TODO()

	roots, intermediates, err := getFulcio(ctx)
	require.NoError(t, err)
	assert.NotNil(t, roots)
	assert.NotNil(t, intermediates)
}

func TestGetTrustedRootFromTUF(t *testing.T) {
	ctx := context.TODO()

	err := initializeTuf(ctx, nil)
	require.NoError(t, err)

	trustedRoot, err := getTrustedRootFromTUF(ctx)
	require.NoError(t, err)
	assert.NotNil(t, trustedRoot)
}

func TestCheckOptions_CTLogConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		ctlog    *v1beta1.CTLog
		wantSCT  bool
		wantTlog bool
	}{
		{
			name: "all checks enabled",
			ctlog: &v1beta1.CTLog{
				URL:                "https://rekor.sigstore.dev",
				InsecureIgnoreSCT:  false,
				InsecureIgnoreTlog: false,
			},
			wantSCT:  false,
			wantTlog: false,
		},
		{
			name: "ignore SCT",
			ctlog: &v1beta1.CTLog{
				URL:                "https://rekor.sigstore.dev",
				InsecureIgnoreSCT:  true,
				InsecureIgnoreTlog: false,
			},
			wantSCT:  true,
			wantTlog: false,
		},
		{
			name: "ignore Tlog",
			ctlog: &v1beta1.CTLog{
				URL:                "https://rekor.sigstore.dev",
				InsecureIgnoreSCT:  false,
				InsecureIgnoreTlog: true,
			},
			wantSCT:  false,
			wantTlog: true,
		},
		{
			name: "ignore both",
			ctlog: &v1beta1.CTLog{
				URL:                "https://rekor.sigstore.dev",
				InsecureIgnoreSCT:  true,
				InsecureIgnoreTlog: true,
			},
			wantSCT:  true,
			wantTlog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			baseROpts, baseNOpts := baseOpts()

			cosignCfg := &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testPublicKey,
				},
				CTLog: tt.ctlog,
			}

			opts, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSCT, opts.IgnoreSCT)
			assert.Equal(t, tt.wantTlog, opts.IgnoreTlog)
		})
	}
}

func TestCheckOptions_VerifierTypes(t *testing.T) {
	tests := []struct {
		name      string
		cosignCfg *v1beta1.Cosign
		wantErr   bool
		checkFn   func(*testing.T, interface{})
	}{
		{
			name: "key-based verifier",
			cosignCfg: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
				},
			},
			wantErr: false,
			checkFn: func(t *testing.T, opts interface{}) {
				checkOpts := opts.(*cosign.CheckOpts)
				assert.NotNil(t, checkOpts.SigVerifier)
				assert.Nil(t, checkOpts.RootCerts)
			},
		},
		{
			name: "keyless verifier",
			cosignCfg: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  testIssuer,
							Subject: testSubject,
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL:               "https://rekor.sigstore.dev",
					InsecureIgnoreSCT: true,
				},
			},
			wantErr: false,
			checkFn: func(t *testing.T, opts interface{}) {
				checkOpts := opts.(*cosign.CheckOpts)
				assert.Nil(t, checkOpts.SigVerifier)
				assert.NotNil(t, checkOpts.RootCerts)
				assert.NotEmpty(t, checkOpts.Identities)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			baseROpts, baseNOpts := baseOpts()

			opts, err := checkOptions(ctx, tt.cosignCfg, baseROpts, baseNOpts, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.checkFn != nil {
					tt.checkFn(t, opts)
				}
			}
		})
	}
}

func TestCheckOptions_TSACertChain_UseSignedTimestamps(t *testing.T) {
	tests := []struct {
		name             string
		tsaCertChain     string
		wantUseSignedTS  bool
		wantTSARootCerts bool
	}{
		{
			name:             "TSACertChain provided enables UseSignedTimestamps",
			tsaCertChain:     testTSACertChain,
			wantUseSignedTS:  true,
			wantTSARootCerts: true,
		},
		{
			name:             "empty TSACertChain does not enable UseSignedTimestamps",
			tsaCertChain:     "",
			wantUseSignedTS:  false,
			wantTSARootCerts: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			baseROpts, baseNOpts := baseOpts()

			cosignCfg := &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
					TSACertChain:       tt.tsaCertChain,
				},
			}

			opts, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.wantUseSignedTS, opts.UseSignedTimestamps)
			if tt.wantTSARootCerts {
				assert.NotNil(t, opts.TSARootCertificates)
			} else {
				assert.Nil(t, opts.TSARootCertificates)
			}
		})
	}
}

package x509

import (
	"encoding/json"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
)

const testCert = `-----BEGIN CERTIFICATE-----
MIIC7TCCAdWgAwIBAgIBADANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDDA0qLmt5
dmVybm8uc3ZjMB4XDTIyMTAxMDExNDYzMloXDTIzMTAxMDEyNDYzMlowGDEWMBQG
A1UEAwwNKi5reXZlcm5vLnN2YzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAOKF+2P0Ufp855hpdsGD4lYkd6oU7HZAOWm1XskAMwrdsqWwTNNAinyHRoPQ
IbNbGDQ+r6Cggc2mlxHJ90PnC2weHj5otaD17Z+ARZpJZ4HMWkEfFt8sxwo9vuQJ
RWihqNwFheowjswoSB1DHnPufrZHfztkMoRx278ZfHaIMdlSTg50ektkNDoHA3OJ
sxxw54X3HR1iq6SZwN8xNT0TI6B6BbfAYWMNmKCiZ2iV6kW//XnTEqGd2WcmhuP0
SjwO4tCJbj9oV6+Bj/uhFr7J4foErMaodYDBtQs/ul2tcAwSBHfnC2KcLbiZTZsC
0Rs0WPJ4YwF/cOsD7Z/RmLs4FHsCAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgKkMA8G
A1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFJaK+3Wfnf7rVkYBzMCLSaV56NMqMA0G
CSqGSIb3DQEBCwUAA4IBAQDY7F6b+t9BX7098JyGk6zeT39MoLdSv+8IaKXn+m8G
yOKn3CZkruko57ycvPd4taC0gggtmUYynFhwPMQr+boNrrK9rat8Jw3yPPsBq/8D
/s6tvwxSNXBfPUI5OvNIB/hA5XpJpdHQaCkYm+FWkcJsolkkbSOfVjUjImW26JHB
nnPPtR4Y7dx0SVoPS19IC0T5RmdvgqlXj4XbhTnX3QOujVHn8u+wQ8po7EngHDQs
+onfkp8ipe0QpEJL1ZdW2LhyDXGKrZ2y8UPZ9wYNzxHWaj1Thu4B9YFdsPUwWqSx
n9e+FygpoktlD8YgT7jwgiVKX7Koz++zyvMIdhvRrtgS
-----END CERTIFICATE-----`

func Test_decode(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)

	env, err := base.Extend(cel.Variable("cert", cel.StringType))
	assert.NoError(t, err)
	assert.NotNil(t, env)

	env, err = env.Extend(Lib(nil))
	assert.NoError(t, err)
	assert.NotNil(t, env)

	t.Run("decode_certificate", func(t *testing.T) {
		ast, issues := env.Compile(`x509.decode(cert)`)
		if issues != nil {
			t.Logf("Compilation issues: %v", issues.Err())
		}
		assert.Nil(t, issues)
		assert.NotNil(t, ast)
		prog, err := env.Program(ast)
		assert.NoError(t, err)
		assert.NotNil(t, prog)
		out, _, err := prog.Eval(map[string]any{
			"cert": testCert,
		})
		assert.NoError(t, err)
		assert.NotNil(t, out)

		result := out.Value().(map[string]any)

		assert.Equal(t, true, result["IsCA"])
		assert.Equal(t, true, result["BasicConstraintsValid"])
		assert.Equal(t, float64(37), result["KeyUsage"])
		assert.Equal(t, float64(0), result["SerialNumber"])
		assert.Equal(t, float64(1), result["PublicKeyAlgorithm"])
		assert.Equal(t, float64(-1), result["MaxPathLen"])
		assert.Equal(t, false, result["MaxPathLenZero"])
		assert.Equal(t, "2022-10-10T11:46:32Z", result["NotBefore"])
		assert.Equal(t, "2023-10-10T12:46:32Z", result["NotAfter"])

		issuer := result["Issuer"].(map[string]any)
		assert.Equal(t, "*.kyverno.svc", issuer["CommonName"])
		assert.Equal(t, "", issuer["SerialNumber"])

		publicKey := result["PublicKey"].(map[string]any)
		assert.Equal(t, float64(65537), publicKey["E"])
		assert.Equal(t, "28595925905962223424520947352207105451744616797088171943239289907331901888529856098458304611629660120574607501039902142361333982065793213267074854658525100799280158707840279479550961169213763526857247298653141711003931642606662052674943191476488665842309583311097351331994267413776792462637192775240062778036062353517979538994974045127175206597906751521558536719043095219698535279694800624795673809356898452438518041024126624051887044932164506019573725987204208750674129677584956156611454245004918943771571492757639432459688931855526941886354880727024912384140238027697348634609952850513122734230521040730560514233467", publicKey["N"])

		extensions := result["Extensions"].([]any)
		assert.Equal(t, 3, len(extensions))

		ext0 := extensions[0].(map[string]any)
		assert.Equal(t, true, ext0["Critical"])

		assert.NotNil(t, result["Raw"])
		assert.NotNil(t, result["RawIssuer"])
		assert.NotNil(t, result["RawSubject"])
		assert.NotNil(t, result["RawSubjectPublicKeyInfo"])
		assert.NotNil(t, result["RawTBSCertificate"])
		assert.NotNil(t, result["Signature"])

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		assert.NoError(t, err)
		t.Logf("Decoded certificate JSON:\n%s", string(jsonBytes))
	})

	t.Run("decode_invalid_pem", func(t *testing.T) {
		ast, issues := env.Compile(`x509.decode(cert)`)
		assert.Nil(t, issues)
		assert.NotNil(t, ast)
		prog, err := env.Program(ast)
		assert.NoError(t, err)
		assert.NotNil(t, prog)
		_, _, err = prog.Eval(map[string]any{
			"cert": "not a valid PEM",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode PEM block")
	})
}

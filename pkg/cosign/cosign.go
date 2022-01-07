package cosign

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sigstore/cosign/pkg/oci"
	"github.com/sigstore/cosign/pkg/oci/remote"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/kyverno/kyverno/pkg/engine/common"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/cosign/attestation"
	sigs "github.com/sigstore/cosign/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature"
	"k8s.io/client-go/kubernetes"
)

var (
	// ImageSignatureRepository is an alternate signature repository
	ImageSignatureRepository string
	Secrets                  []string

	kubeClient            kubernetes.Interface
	kyvernoNamespace      string
	kyvernoServiceAccount string
)

// Initialize loads the image pull secrets and initializes the default auth method for container registry API calls
func Initialize(client kubernetes.Interface, namespace, serviceAccount string, imagePullSecrets []string) error {
	kubeClient = client
	kyvernoNamespace = namespace
	kyvernoServiceAccount = serviceAccount
	Secrets = imagePullSecrets

	var kc authn.Keychain
	kcOpts := &k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccount,
		ImagePullSecrets:   imagePullSecrets,
	}

	kc, err := k8schain.New(context.Background(), client, *kcOpts)
	if err != nil {
		return errors.Wrap(err, "failed to initialize registry keychain")
	}

	authn.DefaultKeychain = kc
	return nil
}

// UpdateKeychain reinitializes the image pull secrets and default auth method for container registry API calls
func UpdateKeychain() error {
	var err = Initialize(kubeClient, kyvernoNamespace, kyvernoServiceAccount, Secrets)
	if err != nil {
		return err
	}
	return nil
}

func VerifySignature(imageRef string, key []byte, repository string, log logr.Logger) (digest string, err error) {
	ctx := context.Background()
	var remoteOpts []remote.Option
	ro := options.RegistryOptions{}
	remoteOpts, err = ro.ClientOpts(ctx)
	if err != nil {
		return "", errors.Wrap(err, "constructing client options")
	}

	cosignOpts := &cosign.CheckOpts{
		Annotations:        map[string]interface{}{},
		RegistryClientOpts: remoteOpts,
	}

	if key != nil {
		cosignOpts.SigVerifier, err = sigs.PublicKeyFromKeyRef(ctx, string(key))
	}

	if err != nil {
		return "", errors.Wrap(err, "loading credentials")
	}

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse image")
	}

	if repository != "" {
		signatureRepo, err := name.NewRepository(repository)
		if err != nil {
			return "", errors.Wrapf(err, "failed to parse signature repository %s", repository)
		}

		cosignOpts.RegistryClientOpts = append(cosignOpts.RegistryClientOpts, remote.WithTargetRepository(signatureRepo))
	}

	verified, _, err := client.Verify(ctx, ref, cosign.SignaturesAccessor, cosignOpts)
	if err != nil {
		msg := err.Error()
		log.Info("image verification failed", "error", msg)
		if strings.Contains(msg, "NAME_UNKNOWN: repository name not known to registry") {
			return "", fmt.Errorf("signature not found")
		} else if strings.Contains(msg, "no matching signatures") {
			return "", fmt.Errorf("invalid signature")
		}

		return "", errors.Wrap(err, "failed to verify image")
	}

	digest, err = extractDigest(imageRef, verified, log)
	if err != nil {
		return "", errors.Wrap(err, "failed to get digest")
	}

	return digest, nil
}

// FetchAttestations retrieves signed attestations and decodes them into in-toto statements
// https://github.com/in-toto/attestation/blob/main/spec/README.md#statement
func FetchAttestations(imageRef string, key []byte, repository string) ([]map[string]interface{}, error) {
	ctx := context.Background()
	var pubKey signature.Verifier
	var err error

	pubKey, err = decodePEM([]byte(key))
	if err != nil {
		return nil, errors.Wrap(err, "decode pem")
	}

	var opts []remote.Option
	ro := options.RegistryOptions{}

	opts, err = ro.ClientOpts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "constructing client options")
	}

	if repository != "" {
		signatureRepo, err := name.NewRepository(repository)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse signature repository %s", repository)
		}
		opts = append(opts, remote.WithTargetRepository(signatureRepo))
	}

	cosignOpts := &cosign.CheckOpts{
		ClaimVerifier:      cosign.IntotoSubjectClaimVerifier,
		SigVerifier:        pubKey,
		RegistryClientOpts: opts,
	}

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse image")
	}

	verified, _, err := client.Verify(context.Background(), ref, cosign.AttestationsAccessor, cosignOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to verify image attestations")
	}

	inTotoStatements, err := decodeStatements(verified)
	if err != nil {
		return nil, err
	}

	return inTotoStatements, nil
}

func decodeStatements(sigs []oci.Signature) ([]map[string]interface{}, error) {
	if len(sigs) == 0 {
		return []map[string]interface{}{}, nil
	}

	decodedStatements := make([]map[string]interface{}, len(sigs))
	for i, sig := range sigs {
		payload, err := sig.Payload()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get payload")
		}

		data := make(map[string]interface{})
		if err := json.Unmarshal(payload, &data); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal JSON payload: %v", sig)
		}

		payloadBase64 := data["payload"].(string)
		decodedStatement, err := decodeStatement(payloadBase64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode statement %s", payloadBase64)
		}

		decodedStatements[i] = decodedStatement
	}

	return decodedStatements, nil
}

func decodeStatement(payloadBase64 string) (map[string]interface{}, error) {
	statementRaw, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to base64 decode payload for %v", statementRaw)
	}

	var statement in_toto.Statement
	if err := json.Unmarshal(statementRaw, &statement); err != nil {
		return nil, err
	}

	if statement.PredicateType != attestation.CosignCustomProvenanceV01 {
		// This assumes that the following statements are JSON objects:
		// - in_toto.PredicateSLSAProvenanceV01
		// - in_toto.PredicateLinkV1
		// - in_toto.PredicateSPDX
		// any other custom predicate
		return common.ToMap(statement)
	}

	return decodeCosignCustomProvenanceV01(statement)
}

func decodeCosignCustomProvenanceV01(statement in_toto.Statement) (map[string]interface{}, error) {
	if statement.PredicateType != attestation.CosignCustomProvenanceV01 {
		return nil, fmt.Errorf("invalid statement type %s", attestation.CosignCustomProvenanceV01)
	}

	predicate, ok := statement.Predicate.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to decode CosignCustomProvenanceV01")
	}

	cosignPredicateData := predicate["Data"]
	if cosignPredicateData == nil {
		return nil, fmt.Errorf("missing predicate in CosignCustomProvenanceV01")
	}

	// attempt to parse as a JSON object type
	data, err := stringToJSONMap(cosignPredicateData)
	if err == nil {
		predicate["Data"] = data
		statement.Predicate = predicate
	}

	return common.ToMap(statement)
}

func stringToJSONMap(i interface{}) (map[string]interface{}, error) {
	s, ok := i.(string)
	if !ok {
		return nil, fmt.Errorf("expected string type")
	}

	var data = map[string]interface{}{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %s", err.Error())
	}

	return data, nil
}

func decodePEM(raw []byte) (signature.Verifier, error) {
	// PEM encoded file.
	ed, err := cosign.PemToECDSAKey(raw)
	if err != nil {
		return nil, errors.Wrap(err, "pem to ecdsa")
	}

	return signature.LoadECDSAVerifier(ed, crypto.SHA256)
}

func extractDigest(imgRef string, verified []oci.Signature, log logr.Logger) (string, error) {
	var jsonMap map[string]interface{}
	for _, vp := range verified {
		payload, err := vp.Payload()
		if err != nil {
			return "", errors.Wrap(err, "failed to get payload")
		}

		if err := json.Unmarshal(payload, &jsonMap); err != nil {
			return "", err
		}

		log.V(4).Info("image verification response", "image", imgRef, "payload", jsonMap)

		// The cosign response is in the JSON format:
		// {
		//   "critical": {
		// 	   "identity": {
		// 	     "docker-reference": "registry-v2.nirmata.io/pause"
		// 	    },
		//   	"image": {
		// 	     "docker-manifest-digest": "sha256:4a1c4b21597c1b4415bdbecb28a3296c6b5e23ca4f9feeb599860a1dac6a0108"
		// 	    },
		// 	    "type": "cosign container image signature"
		//    },
		//    "optional": null
		// }
		critical := jsonMap["critical"].(map[string]interface{})
		if critical != nil {
			typeStr := critical["type"].(string)
			if typeStr == "cosign container image signature" {
				identity := critical["identity"].(map[string]interface{})
				if identity != nil {
					image := critical["image"].(map[string]interface{})
					if image != nil {
						return image["docker-manifest-digest"].(string), nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("digest not found for " + imgRef)
}

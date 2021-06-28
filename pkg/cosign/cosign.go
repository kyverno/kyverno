package cosign

import (
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/sigstore/pkg/signature"
	"k8s.io/client-go/kubernetes"
)

func Initialize(client kubernetes.Interface, namespace, serviceAccount string, imagePullSecrets []string) error {
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

func Verify(imageRef string, key []byte, log logr.Logger) (digest string, err error) {
	pubKey, err := decodePEM(key)
	if err != nil {
		return "", errors.Wrapf(err, "failed to decode PEM %v", string(key))
	}

	cosignOpts := &cosign.CheckOpts{
		Annotations: map[string]interface{}{},
		Claims:      false,
		Tlog:        false,
		Roots:       nil,
		PubKey:      pubKey,
	}

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse image")
	}

	verified, err := cosign.Verify(context.Background(), ref, cosignOpts, "https://rekor.sigstore.dev")
	if err != nil {
		return "", errors.Wrap(err, "failed to verify image")
	}

	digest, err = extractDigest(imageRef, verified, log)
	if err != nil {
		return "", errors.Wrap(err, "failed to get digest")
	}

	return digest, nil
}

func decodePEM(raw []byte) (pub cosign.PublicKey, err error) {
	// PEM encoded file.
	ed, err := cosign.PemToECDSAKey(raw)
	if err != nil {
		return nil, errors.Wrap(err, "pem to ecdsa")
	}

	return signature.ECDSAVerifier{Key: ed, HashAlg: crypto.SHA256}, nil
}

func extractDigest(imgRef string, verified []cosign.SignedPayload, log logr.Logger) (string, error) {
	var jsonMap map[string]interface{}
	for _, vp := range verified {
		if err := json.Unmarshal(vp.Payload, &jsonMap); err != nil {
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

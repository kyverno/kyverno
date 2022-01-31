package engine

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/ghodss/yaml"
	k8smnfutil "github.com/sigstore/k8s-manifest-sigstore/pkg/util"
	mapnode "github.com/sigstore/k8s-manifest-sigstore/pkg/util/mapnode"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const DefaultAnnotationKeyDomain = "cosign.sigstore.dev/"

func VerifyManifest(policyContext *PolicyContext, ecdsaPub string, ignoreFields []string) (bool, *mapnode.DiffResult, error) {
	objManifest, err := yaml.Marshal(policyContext.NewResource.Object)
	if err != nil {
		return false, nil, fmt.Errorf("err: %v\n", err)
	}
	annotation := policyContext.NewResource.GetAnnotations()
	signatureAnnotationKey := DefaultAnnotationKeyDomain + "signature"
	messageAnnotationKey := DefaultAnnotationKeyDomain + "message"

	sig, _ := base64.StdEncoding.DecodeString(annotation[signatureAnnotationKey])

	gzipMsg, _ := base64.StdEncoding.DecodeString(annotation[messageAnnotationKey])
	// `gzipMsg` is a gzip compressed .tar.gz file, so getting a tar ball by decompressing it.
	message := k8smnfutil.GzipDecompress(gzipMsg)
	yamls, err := k8smnfutil.GetYAMLsInArtifact(message)
	if err != nil {
		return false, nil, fmt.Errorf("failed to read YAMLs in the gzipped message: %v", err)
	}
	concatYAMLbytes := k8smnfutil.ConcatenateYAMLs(yamls)
	found, resourceManifests := k8smnfutil.FindManifestYAML(concatYAMLbytes, objManifest, nil, ignoreFields)
	if !found {
		return false, nil, fmt.Errorf("failed to find a YAML manifest in the gzipped message: %v", err)
	}

	// fields which needs to be ignored while comparing manifests.
	// ignoreFields := []string{}

	var mnfMatched bool
	var diff *mapnode.DiffResult
	var diffsForAllCandidates []*mapnode.DiffResult
	for _, candidate := range resourceManifests {
		// log.Debugf("try matching with the candidate %v out of %v", i+1, len(resourceManifests))
		cndMatched, tmpDiff, err := matchManifest(objManifest, candidate, ignoreFields)
		if err != nil {
			return false, nil, fmt.Errorf("error occurred during matching manifest: %v", err)
		}
		diffsForAllCandidates = append(diffsForAllCandidates, tmpDiff)
		if cndMatched {
			mnfMatched = true
			break
		}
	}
	if !mnfMatched && len(diffsForAllCandidates) > 0 {
		diff = diffsForAllCandidates[0]
	}

	publicKey, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(ecdsaPub))
	if err != nil {
		return false, nil, fmt.Errorf("unexpected error unmarshalling public key: %v", err)
	}

	digest := sha256.Sum256(message)
	// verifying message and signature for the supplied key.
	sigVerified := ecdsa.VerifyASN1(publicKey.(*ecdsa.PublicKey), digest[:], sig)

	verified := mnfMatched && sigVerified
	return verified, diff, nil
}

func matchManifest(inputManifestBytes, foundManifestBytes []byte, ignoreFields []string) (bool, *mapnode.DiffResult, error) {
	log.Debug("manifest:", string(inputManifestBytes))
	log.Debug("manifest in reference:", string(foundManifestBytes))
	inputFileNode, err := mapnode.NewFromYamlBytes(inputManifestBytes)
	if err != nil {
		return false, nil, err
	}
	mask := "metadata.annotations." + DefaultAnnotationKeyDomain
	annotationMask := []string{
		mask + "message",
		mask + "signature",
		mask + "certificate",
		mask + "message",
		mask + "bundle",
	}
	maskedInputNode := inputFileNode.Mask(annotationMask)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(inputManifestBytes, &obj)
	if err != nil {
		return false, nil, err
	}

	manifestNode, err := mapnode.NewFromYamlBytes(foundManifestBytes)
	if err != nil {
		return false, nil, err
	}
	maskedManifestNode := manifestNode.Mask(annotationMask)
	var matched bool
	diff := maskedInputNode.Diff(maskedManifestNode)

	// filter out ignoreFields
	if diff != nil && len(ignoreFields) > 0 {
		_, diff, _ = diff.Filter(ignoreFields)
	}
	if diff == nil || diff.Size() == 0 {
		matched = true
		diff = nil
	}
	return matched, diff, nil
}

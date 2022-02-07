package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ghodss/yaml"
	k8smnfutil "github.com/sigstore/k8s-manifest-sigstore/pkg/util"
	mapnode "github.com/sigstore/k8s-manifest-sigstore/pkg/util/mapnode"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const DefaultAnnotationKeyDomain = "cosign.sigstore.dev/"

type VerifyResourceOption struct {
	verifyOption `json:""`
}

// common options for verify functions
// this verifyOption should not be used directly by those functions
type verifyOption struct {
	IgnoreFields ObjectFieldBindingList `json:"ignoreFields,omitempty"`
}

type ObjectReferenceList []ObjectReference

type ObjectReference struct {
	Group     string `json:"group,omitempty"`
	Version   string `json:"version,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type ObjectFieldBinding struct {
	Fields  []string            `json:"fields,omitempty"`
	Objects ObjectReferenceList `json:"objects,omitempty"`
}

type ObjectFieldBindingList []ObjectFieldBinding

// This is common ignore fields for changes by k8s system
//go:embed resources/default-config.yaml
var defaultConfigBytes []byte

func VerifyManifest(policyContext *PolicyContext, ecdsaPub string, ignoreFields []string) (bool, *mapnode.DiffResult, error) {
	vo := &VerifyResourceOption{}
	vo = AddDefaultConfig(vo)
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
	byteStream := bytes.NewBuffer(message)
	uncompressedStream, err := gzip.NewReader(byteStream)
	defer uncompressedStream.Close()

	byteSlice, _ := ioutil.ReadAll(uncompressedStream)
	i := strings.Index(string(byteSlice), "api")
	byteSlice = byteSlice[i:]
	var foundManifest []byte
	for _, ch := range byteSlice {
		if ch != 0 {
			foundManifest = append(foundManifest, ch)
		} else {
			break
		}
	}

	// fields which needs to be ignored while comparing manifests.
	var obj unstructured.Unstructured
	_ = yaml.Unmarshal(objManifest, &obj)
	if vo != nil {
		if ok, fields := vo.IgnoreFields.Match(obj); ok {
			ignoreFields = append(ignoreFields, fields...)
		}
	}

	var mnfMatched bool
	var diff *mapnode.DiffResult
	var diffsForAllCandidates []*mapnode.DiffResult
	cndMatched, tmpDiff, err := matchManifest(objManifest, foundManifest, ignoreFields)
	if err != nil {
		return false, nil, fmt.Errorf("error occurred during matching manifest: %v", err)
	}
	diffsForAllCandidates = append(diffsForAllCandidates, tmpDiff)
	if cndMatched {
		mnfMatched = true
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

func (vo *VerifyResourceOption) AddDefaultConfig(defaultConfig *VerifyResourceOption) *VerifyResourceOption {
	if vo == nil {
		return nil
	}
	ignoreFields := []ObjectFieldBinding(vo.verifyOption.IgnoreFields)
	ignoreFields = append(ignoreFields, []ObjectFieldBinding(defaultConfig.verifyOption.IgnoreFields)...)
	vo.verifyOption.IgnoreFields = ignoreFields
	return vo
}

func LoadDefaultConfig() *VerifyResourceOption {
	var defaultConfig *VerifyResourceOption
	err := yaml.Unmarshal(defaultConfigBytes, &defaultConfig)
	if err != nil {
		return nil
	}
	return defaultConfig
}

func AddDefaultConfig(vo *VerifyResourceOption) *VerifyResourceOption {
	dvo := LoadDefaultConfig()
	return vo.AddDefaultConfig(dvo)
}

func (l ObjectFieldBindingList) Match(obj unstructured.Unstructured) (bool, []string) {
	if len(l) == 0 {
		return false, nil
	}
	matched := false
	matchedFields := []string{}
	for _, f := range l {
		if tmpMatched, tmpFields := f.Match(obj); tmpMatched {
			matched = tmpMatched
			matchedFields = append(matchedFields, tmpFields...)
		}
	}
	return matched, matchedFields
}

func (l ObjectReferenceList) Match(obj unstructured.Unstructured) bool {
	if len(l) == 0 {
		return true
	}
	for _, r := range l {
		if r.Match(obj) {
			return true
		}
	}
	return false
}

func (r ObjectReference) Match(obj unstructured.Unstructured) bool {
	return r.Equal(ObjectToReference(obj))
}

func (f ObjectFieldBinding) Match(obj unstructured.Unstructured) (bool, []string) {
	if f.Objects.Match(obj) {
		return true, f.Fields
	}
	return false, nil
}

func ObjectToReference(obj unstructured.Unstructured) ObjectReference {
	return ObjectReference{
		Group:     obj.GroupVersionKind().Group,
		Version:   obj.GroupVersionKind().Version,
		Kind:      obj.GroupVersionKind().Kind,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}

func (r ObjectReference) Equal(r2 ObjectReference) bool {
	return k8smnfutil.MatchPattern(r.Group, r2.Group) &&
		k8smnfutil.MatchPattern(r.Version, r2.Version) &&
		k8smnfutil.MatchPattern(r.Kind, r2.Kind) &&
		k8smnfutil.MatchPattern(r.Name, r2.Name) &&
		k8smnfutil.MatchPattern(r.Namespace, r2.Namespace)
}

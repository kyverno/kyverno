// Package manifest preserves legacy manifest matching while routing policy-selected
// registry and signature-verification requests through Kyverno's egress guard.
package manifest

import (
	"context"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	manifestutil "github.com/sigstore/k8s-manifest-sigstore/pkg/util"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/util/kubeutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// VerifyResource uses the existing manifest formats and matching rules. The
// upstream VerifyResource entry point constructs its own unguarded clients, so
// only its local parsing/matching helpers and operator Kubernetes client are used.
func VerifyResource(ctx context.Context, obj unstructured.Unstructured, options *k8smanifest.VerifyResourceOption) (*k8smanifest.VerifyResourceResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if options == nil {
		options = k8smanifest.LoadDefaultConfig()
	}
	vo := *options
	if len(vo.SkipObjects) > 0 && vo.SkipObjects.Match(obj) {
		return &k8smanifest.VerifyResourceResult{InScope: false}, nil
	}
	vo.SetAnnotationIgnoreFields()
	_, ignoreFields := vo.IgnoreFields.Match(obj)
	objectYAML, err := yaml.Marshal(obj.Object)
	if err != nil {
		return nil, err
	}
	if ref, ok := obj.GetAnnotations()[vo.AnnotationConfig.ResourceBundleRefAnnotationKey()]; ok {
		vo.ResourceBundleRef = ref
	}
	candidates, signatureRef, err := fetchManifests(ctx, objectYAML, &vo, ignoreFields)
	if err != nil {
		return nil, fmt.Errorf("YAML manifest not found for this resource: %w", err)
	}
	result := &k8smanifest.VerifyResourceResult{InScope: true, SigRef: signatureRef}
	matched := false
	for _, candidate := range candidates {
		ok, diff, err := matchResourceWithManifest(obj, candidate, ignoreFields, vo.DryRunNamespace, vo.DisableDryRun, vo.CheckDryRunForApply, vo.CheckMutatingResource)
		if err != nil {
			return nil, fmt.Errorf("matching manifest: %w", err)
		}
		if ok {
			matched = true
			result.Diff = nil
			break
		}
		if result.Diff == nil {
			result.Diff = diff
		}
	}
	verified, signer, err := verifySignatures(ctx, obj, signatureRef, &vo)
	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}
	result.Verified = matched && verified && vo.Signers.Match(signer)
	result.Signer = signer
	result.ContainerImages, _ = kubeutil.GetAllImagesFromObject(&obj)
	return result, nil
}

func fetchManifests(ctx context.Context, objectYAML []byte, vo *k8smanifest.VerifyResourceOption, ignoreFields []string) ([][]byte, string, error) {
	if vo.ResourceBundleRef == "" {
		// This branch only reads embedded annotations or the operator's Kubernetes
		// API. It must never select upstream's unguarded image fetcher.
		return k8smanifest.NewManifestFetcher("", vo.SignatureResourceRef, vo.AnnotationConfig, ignoreFields, vo.MaxResourceManifestNum, vo.AllowInsecure).Fetch(objectYAML)
	}
	opts, nameOpts := registryclient.GlobalOptsOrDefault(ctx)
	if vo.AllowInsecure {
		nameOpts = append(nameOpts, name.Insecure)
	}
	var maximum *int
	if vo.MaxResourceManifestNum > 0 {
		maximum = &vo.MaxResourceManifestNum
	}
	for _, value := range manifestutil.SplitCommaSeparatedString(vo.ResourceBundleRef) {
		ref, err := name.ParseReference(value, nameOpts...)
		if err != nil {
			return nil, "", err
		}
		img, err := remote.Image(ref, opts...)
		if err != nil {
			return nil, "", fmt.Errorf("fetching manifest image: %w", err)
		}
		data, err := manifestutil.GenerateConcatYAMLsFromImage(img)
		if err != nil {
			return nil, "", err
		}
		found, manifests := manifestutil.FindManifestYAML(data, objectYAML, maximum, ignoreFields)
		if found {
			digest, err := img.Digest()
			if err != nil {
				return nil, "", err
			}
			// Verify the exact image whose manifest was examined, even if its tag
			// changes between the manifest pull and the signature lookup.
			return manifests, ref.Context().Digest(digest.String()).Name(), nil
		}
	}
	return nil, "", k8smanifest.NewMessageNotFoundError(fmt.Errorf("no matching YAML manifest in image"))
}

func signatureSets(obj unstructured.Unstructured, signatureRef string, vo *k8smanifest.VerifyResourceOption) ([]map[string]string, error) {
	if !strings.HasPrefix(signatureRef, kubeutil.InClusterObjectPrefix) {
		return vo.AnnotationConfig.GetAllSignatureSets(obj.GetAnnotations()), nil
	}
	cm, err := k8smanifest.GetConfigMapFromK8sObjectRef(signatureRef)
	if err != nil {
		return nil, err
	}
	if cm.Data["message"] == "" || cm.Data["signature"] == "" {
		return nil, k8smanifest.NewSignatureNotFoundError(fmt.Errorf("signature configmap must contain message and signature"))
	}
	return []map[string]string{cm.Data}, nil
}

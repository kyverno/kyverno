package pull

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

// legacyKinds lists kyverno.io/v1 kinds that are no longer accepted in OCI bundles.
var legacyKinds = map[string]bool{
	"Policy":               true,
	"ClusterPolicy":        true,
	"CleanupPolicy":        true,
	"ClusterCleanupPolicy": true,
}

// celGroups lists the API groups for CEL policy kinds that are accepted in OCI bundles.
var celGroups = map[string]bool{
	"policies.kyverno.io":          true,
	"admissionregistration.k8s.io": true,
}

type options struct {
	imageRef string
}

func (o options) validate(dir string) error {
	if o.imageRef == "" {
		return errors.New("image is required")
	}
	if dir == "" {
		return errors.New("dir is required")
	}
	return nil
}

func (o options) execute(ctx context.Context, dir string, keychain authn.Keychain) error {
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir, err = securejoin.SecureJoin(cwd, dir)
		if err != nil {
			return err
		}
	}
	fi, err := os.Lstat(dir)
	// Dir does not need to exist, as it can later be created.
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("unable to create directory %s: %w", dir, err)
		}
	}
	if err == nil && !fi.IsDir() {
		return fmt.Errorf("dir '%s' must be a directory", dir)
	}
	ref, err := name.ParseReference(o.imageRef)
	if err != nil {
		return fmt.Errorf("parsing image reference: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Downloading policies from an image [%s]...\n", ref.Name())
	rmt, err := remote.Get(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return fmt.Errorf("getting image: %v", err)
	}
	img, err := rmt.Image()
	if err != nil {
		return fmt.Errorf("getting image: %v", err)
	}
	l, err := img.Layers()
	if err != nil {
		return fmt.Errorf("getting image layers: %v", err)
	}
	for _, layer := range l {
		lmt, err := layer.MediaType()
		if err != nil {
			return fmt.Errorf("getting layer media type: %v", err)
		}
		if lmt == internal.PolicyLayerMediaType {
			if err := extractAndSavePolicies(layer, dir); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(os.Stderr, "Done.")
	return nil
}

// extractAndSavePolicies reads CEL policy documents from a single layer blob and
// writes each accepted document to disk. Legacy kyverno.io/v1 policy kinds are
// rejected. Unknown Kubernetes objects are skipped with a warning.
//
// The layer's ReadCloser is closed at the end of the call regardless of outcome.
func extractAndSavePolicies(layer v1.Layer, dir string) error {
	blob, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("getting layer blob: %v", err)
	}
	defer blob.Close()

	layerBytes, err := io.ReadAll(blob)
	if err != nil {
		return fmt.Errorf("reading layer blob: %v", err)
	}

	documents, err := extyaml.SplitDocuments(layerBytes)
	if err != nil {
		return fmt.Errorf("splitting YAML documents: %v", err)
	}

	for _, doc := range documents {
		jsonBytes, err := k8syaml.ToJSON(doc)
		if err != nil {
			return fmt.Errorf("converting document to JSON: %v", err)
		}
		var us unstructured.Unstructured
		if err := us.UnmarshalJSON(jsonBytes); err != nil {
			return fmt.Errorf("unmarshaling document: %v", err)
		}

		kind := us.GetKind()
		apiVersion := us.GetAPIVersion()
		objName := us.GetName()

		if len(strings.TrimSpace(string(doc))) == 0 || (kind == "" && objName == "") {
			continue
		}

		if legacyKinds[kind] {
			return fmt.Errorf(
				"legacy policy kind %q (apiVersion: %s) is no longer supported in OCI bundles; "+
					"migrate to policies.kyverno.io/v1beta1 CEL policy kinds",
				kind, apiVersion,
			)
		}

		group := strings.SplitN(apiVersion, "/", 2)[0]
		if apiVersion != "" && !strings.Contains(apiVersion, "/") {
			// core API group resources have no group prefix
			group = ""
		}
		if !celGroups[group] {
			return fmt.Errorf(
				"unsupported resource %s/%s %q; only CEL policy kinds are supported in OCI bundles",
				apiVersion, kind, objName,
			)
		}

		pp := filepath.Join(dir, objName+".yaml")
		fmt.Fprintf(os.Stderr, "Saving %s [%s] to disk [%s]...\n", kind, objName, pp)
		if err := os.WriteFile(pp, doc, 0o600); err != nil {
			return fmt.Errorf("creating file %s: %v", pp, err)
		}
	}
	return nil
}

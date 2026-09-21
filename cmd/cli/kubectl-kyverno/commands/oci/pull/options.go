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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
)

// legacyKinds lists kyverno.io/v1 kinds that are no longer accepted in OCI bundles.
var legacyKinds = map[string]bool{
	"Policy":               true,
	"ClusterPolicy":        true,
	"CleanupPolicy":        true,
	"ClusterCleanupPolicy": true,
}

const supportedAPIVersion = "policies.kyverno.io/v1beta1"

// supportedCELKinds lists the exact policies.kyverno.io/v1beta1 CEL policy kinds
// and PolicyException accepted in OCI bundles.
var supportedCELKinds = map[string]bool{
	"ValidatingPolicy":                true,
	"NamespacedValidatingPolicy":      true,
	"MutatingPolicy":                  true,
	"NamespacedMutatingPolicy":        true,
	"GeneratingPolicy":                true,
	"NamespacedGeneratingPolicy":      true,
	"DeletingPolicy":                  true,
	"NamespacedDeletingPolicy":        true,
	"ImageValidatingPolicy":           true,
	"NamespacedImageValidatingPolicy": true,
	"PolicyException":                 true,
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
	seen := make(map[string]bool)
	for _, layer := range l {
		lmt, err := layer.MediaType()
		if err != nil {
			return fmt.Errorf("getting layer media type: %v", err)
		}
		if lmt == internal.PolicyLayerMediaType {
			if err := extractAndSavePolicies(layer, dir, seen); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(os.Stderr, "Done.")
	return nil
}

// extractAndSavePolicies reads CEL policy documents from a single layer blob and
// writes each accepted document to disk. Legacy kyverno.io/v1 policy kinds and
// unknown or non-CEL Kubernetes resources are rejected with an error.
//
// The layer's ReadCloser is closed at the end of the call regardless of outcome.
func extractAndSavePolicies(layer v1.Layer, dir string, seen ...map[string]bool) error {
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

	var seenTracker map[string]bool
	if len(seen) > 0 && seen[0] != nil {
		seenTracker = seen[0]
	} else {
		seenTracker = make(map[string]bool)
	}

	for _, doc := range documents {
		if len(strings.TrimSpace(string(doc))) == 0 {
			continue
		}

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
		ns := us.GetNamespace()

		if strings.TrimSpace(kind) == "" || strings.TrimSpace(objName) == "" {
			return fmt.Errorf("resource missing kind or metadata.name")
		}

		if legacyKinds[kind] {
			return fmt.Errorf(
				"legacy policy kind %q (apiVersion: %s) is no longer supported in OCI bundles; "+
					"migrate to policies.kyverno.io/v1beta1 CEL policy kinds",
				kind, apiVersion,
			)
		}

		if apiVersion != supportedAPIVersion || !supportedCELKinds[kind] {
			return fmt.Errorf(
				"unsupported resource %s/%s %q; only policies.kyverno.io/v1beta1 CEL policy kinds are supported in OCI bundles",
				apiVersion, kind, objName,
			)
		}

		identity := fmt.Sprintf("%s/%s/%s", kind, ns, objName)
		if seenTracker[identity] {
			return fmt.Errorf("duplicate resource identity %s", identity)
		}
		seenTracker[identity] = true

		// Include kind and namespace (if present) in filename to prevent collisions
		// when multiple resources share the same name across kinds or namespaces.
		// Use securejoin to prevent path traversal attacks if objName contains "../".
		filename := strings.ToLower(kind) + "-" + objName + ".yaml"
		if ns != "" {
			filename = strings.ToLower(kind) + "-" + ns + "-" + objName + ".yaml"
		}
		pp, err := securejoin.SecureJoin(dir, filename)
		if err != nil {
			return fmt.Errorf("constructing output path for %s %q: %v", kind, objName, err)
		}
		fmt.Fprintf(os.Stderr, "Saving %s [%s] to disk [%s]...\n", kind, objName, pp)
		if err := os.WriteFile(pp, doc, 0o600); err != nil {
			return fmt.Errorf("creating file %s: %v", pp, err)
		}
	}
	return nil
}

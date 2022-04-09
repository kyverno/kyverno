package context

import (
	"fmt"

	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
)

// MutateResourceWithImageInfo will set images to their canonical form so that they can be compared
// in a predictable manner. This sets the default registry as `docker.io` and the tag as `latest` if
// these are missing.
func MutateResourceWithImageInfo(raw []byte, ctx Interface) error {
	images := ctx.ImageInfo()
	if images == nil {
		return nil
	}
	buildJSONPatch := func(op, path, value string) []byte {
		p := fmt.Sprintf(`{ "op": "%s", "path": "%s", "value":"%s" }`, op, path, value)
		return []byte(p)
	}
	var patches [][]byte
	for path, info := range images {
		patches = append(patches, buildJSONPatch("replace", path, info.String()))
	}
	patchedResource, err := engineutils.ApplyPatches(raw, patches)
	if err != nil {
		return err
	}
	return AddResource(ctx, patchedResource)
}

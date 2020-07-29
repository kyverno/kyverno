package mutate

import (
	"bytes"
	"fmt"

	"sigs.k8s.io/kustomize/api/filters/patchstrategicmerge"
	filtersutil "sigs.k8s.io/kustomize/kyaml/filtersutil"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

func strategicMergePatchfilter(base, overlay string) ([]byte, error) {
	patch := yaml.MustParse(overlay)

	f := patchstrategicmerge.Filter{
		Patch: patch,
	}

	baseObj := buffer{Buffer: bytes.NewBufferString(base)}
	err := filtersutil.ApplyToJSON(f, baseObj)

	fmt.Println(baseObj.String())

	return baseObj.Bytes(), err
}

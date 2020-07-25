package mutate

import (
	"bytes"

	"sigs.k8s.io/kustomize/api/filters/patchstrategicmerge"
	"sigs.k8s.io/kustomize/kyaml/kio"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

func strategicMergePatchfilter() (string, error) {
	patch := yaml.MustParse(overlay)

	f := patchstrategicmerge.Filter{
		Patch: patch,
	}

	var out bytes.Buffer
	rw := kio.ByteReadWriter{
		Reader: bytes.NewBufferString(base),
		Writer: &out,
	}

	err := kio.Pipeline{
		Inputs:  []kio.Reader{&rw},
		Filters: []kio.Filter{f},
		Outputs: []kio.Writer{&rw},
	}.Execute()

	return out.String(), err
}

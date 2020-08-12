package mutate

import (
	"fmt"

	"github.com/mattbaird/jsonpatch"
)

func generatePatches(src, dst []byte) ([][]byte, error) {
	var patchesBytes [][]byte
	pp, err := jsonpatch.CreatePatch(src, dst)
	for _, p := range pp {
		// TODO: handle remove nil value, i.e.,
		// {remove /spec/securityContext <nil>}
		// {remove /status/conditions/0/lastProbeTime <nil>}

		pbytes, err := p.MarshalJSON()
		if err != nil {
			return patchesBytes, err
		}

		patchesBytes = append(patchesBytes, pbytes)
		fmt.Printf("generated patch %s\n", p)
	}

	return patchesBytes, err
}

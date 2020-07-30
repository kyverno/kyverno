package mutate

import (
	"fmt"
	"time"

	"github.com/mattbaird/jsonpatch"
)

func generatePatches(src, dst []byte) ([][]byte, error) {
	t := time.Now()
	defer fmt.Printf("finished in %v\n", time.Since(t).String())

	var patchesBytes [][]byte
	pp, err := jsonpatch.CreatePatch(src, dst)
	for _, p := range pp {
		pbytes, err := p.MarshalJSON()
		if err != nil {
			return patchesBytes, err
		}

		patchesBytes = append(patchesBytes, pbytes)
		fmt.Println(p)
	}

	return patchesBytes, err
}

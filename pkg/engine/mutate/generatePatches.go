package mutate

import (
	"fmt"
	"time"

	"github.com/mattbaird/jsonpatch"
)

func generatePatches(src, dst []byte) ([]jsonpatch.JsonPatchOperation, error) {
	t := time.Now()
	defer fmt.Printf("finished in %v\n", time.Since(t).String())

	// pp, err := jsonpatch.CreatePatch(baseBytes, expectBytes)
	pp, err := jsonpatch.CreatePatch(src, dst)
	for _, p := range pp {
		fmt.Println(p)
	}

	return pp, err
}

package mutate

import (
	"fmt"
	"time"

	"github.com/mattbaird/jsonpatch"
)

func generatePatches(src string, dst string) ([]jsonpatch.JsonPatchOperation, error) {
	t := time.Now()
	defer fmt.Printf("finished in %v\n", time.Since(t).String())

	pp, err := jsonpatch.CreatePatch(baseBytes, expectBytes)
	for _, p := range pp {
		fmt.Println(p)
	}
	return pp, err
}

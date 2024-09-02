// run with:
// (cd hack/chainsaw-matrix && go run . > ../../test/conformance/chainsaw/e2e-matrix.json)

package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kyverno/chainsaw/pkg/discovery"
)

const chunkSize = 16

func main() {
	tests, err := discovery.DiscoverTests("chainsaw-test.yaml", nil, false, "../../test/conformance/chainsaw")
	if err != nil {
		panic(err)
	}
	var paths []string
	for _, test := range tests {
		path, err := filepath.Rel("../../test/conformance/chainsaw", test.BasePath)
		if err != nil {
			panic(err)
		}
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			panic("not enough folder parts: " + path)
		}
		if strings.HasSuffix(parts[0], "-cel") {
			continue
		}
		paths = append(paths, strings.Join(parts, "/"))
	}
	suites := map[string][]string{}
	for _, path := range paths {
		parts := strings.Split(path, "/")
		root := strings.Join(parts[:len(parts)-1], "/")
		suites[root] = append(suites[root], parts[len(parts)-1])
	}
	ts := map[string][]string{}
	for _, key := range slices.Sorted(maps.Keys(suites)) {
		root := ""
		for _, part := range strings.Split(key, "/") {
			root += "^" + part + "$" + "/"
		}
		slices.Sort(suites[key])
		for i := 0; i < len(suites[key]); i += chunkSize {
			end := i + chunkSize
			if end > len(suites[key]) {
				end = len(suites[key])
			}
			pattern := root + "^" + "(" + strings.Join(suites[key][i:end], "|") + ")\\[.*\\]$"
			key := strings.Split(key, "/")[0]
			ts[key] = append(ts[key], pattern)
		}
	}
	data, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
}

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

const chunkSize = 20

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
	suites := map[string]map[string][]string{}
	for _, path := range paths {
		parts := strings.Split(path, "/")
		root := parts[0]
		folder := strings.Join(parts[:len(parts)-1], "/")
		if suites[root] == nil {
			suites[root] = map[string][]string{}
		}
		suites[root][folder] = append(suites[root][folder], parts[len(parts)-1])
	}
	ts := map[string][]string{}
	for _, root := range slices.Sorted(maps.Keys(suites)) {
		count := 0
		for _, tests := range suites[root] {
			count += len(tests)
		}
		if count <= chunkSize {
			ts[root] = []string{
				"^" + root + "$",
			}
		} else {
			for _, folder := range slices.Sorted(maps.Keys(suites[root])) {
				tests := suites[root][folder]
				pattern := ""
				for _, part := range strings.Split(folder, "/") {
					pattern += "^" + part + "$" + "/"
				}
				for i := 0; i < len(tests); i += chunkSize {
					end := i + chunkSize
					if end > len(tests) {
						end = len(tests)
					}
					pattern := pattern + "^" + "(" + strings.Join(tests[i:end], "|") + ")\\[.*\\]$"
					ts[root] = append(ts[root], pattern)
				}
			}
		}
	}
	data, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
}

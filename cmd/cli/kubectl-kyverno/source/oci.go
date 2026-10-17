package source

import "strings"

const ociPrefix = "oci://"

func IsOCI(in string) bool {
	return strings.HasPrefix(in, ociPrefix)
}

func StripOCIPrefix(in string) string {
	return strings.TrimPrefix(in, ociPrefix)
}

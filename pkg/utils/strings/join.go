package strings

import "strings"

func JoinNonEmpty(elems []string, sep string) string {
	var bldr strings.Builder
	var idx int = 0
	for _, s := range elems {
		if s != "" {
			if idx > 0 {
				bldr.WriteString(sep)
			}

			bldr.WriteString(s)
			idx++
		}
	}

	return bldr.String()
}

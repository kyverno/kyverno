package utils

func Contains(list []string, element string) bool {
	for _, e := range list {
		if e == element {
			return true
		}
	}
	return false
}

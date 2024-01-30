package fuzz

import (
	"strings"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

func GetK8sString(ff *fuzz.ConsumeFuzzer) (string, error) {
	allowedChars := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.")
	stringLength, err := ff.GetInt()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for i := 0; i < stringLength%63; i++ {
		charIndex, err := ff.GetInt()
		if err != nil {
			return "", err
		}
		sb.WriteString(string(allowedChars[charIndex%len(allowedChars)]))
	}
	return sb.String(), nil
}

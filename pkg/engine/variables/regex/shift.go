package regex

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
)

func FindAndShiftReferences(log logr.Logger, value, shift, pivot string) string {
	for _, reference := range RegexReferences.FindAllString(value, -1) {
		initial := reference[:2] == `$(`
		oldReference := reference

		if !initial {
			reference = reference[1:]
		}

		index := strings.Index(reference, pivot)
		if index == -1 {
			log.Error(fmt.Errorf(`failed to shift reference: pivot value "%s" was not found`, pivot), "pivot search failed")
		}

		// try to get rule index from the reference
		if pivot == "anyPattern" {
			ruleIndex := strings.Split(reference[index+len(pivot)+1:], "/")[0]
			pivot = pivot + "/" + ruleIndex
		}

		shiftedReference := strings.Replace(reference, pivot, pivot+"/"+shift, -1)
		replacement := ""

		if !initial {
			replacement = string(oldReference[0])
		}

		replacement += shiftedReference

		value = strings.Replace(value, oldReference, replacement, 1)
	}

	return value
}

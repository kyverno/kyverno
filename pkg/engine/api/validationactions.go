package api

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// GetValidationActionsFromStrings converts a slice of string actions (e.g., ["Warn", "Audit", "Deny"])
// to a set of ValidationAction types. It supports parsing common action strings used in PolicyExceptions.
func GetValidationActionsFromStrings(actions []string) sets.Set[admissionregistrationv1.ValidationAction] {
	result := sets.New[admissionregistrationv1.ValidationAction]()
	for _, action := range actions {
		switch action {
		case "Warn", "warn":
			result.Insert(admissionregistrationv1.Warn)
		case "Audit", "audit":
			result.Insert(admissionregistrationv1.Audit)
		case "Deny", "deny":
			result.Insert(admissionregistrationv1.Deny)
		}
	}
	return result
}

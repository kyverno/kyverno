package v1

import (
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// UserInfo contains information about the user performing the operation.
type UserInfo struct {
	// Roles is the list of namespaced role names for the user.
	// +optional
	Roles []string `json:"roles,omitempty" yaml:"roles,omitempty"`

	// ClusterRoles is the list of cluster-wide role names for the user.
	// +optional
	ClusterRoles []string `json:"clusterRoles,omitempty" yaml:"clusterRoles,omitempty"`

	// Subjects is the list of subject names like users, user groups, and service accounts.
	// +optional
	Subjects []rbacv1.Subject `json:"subjects,omitempty" yaml:"subjects,omitempty"`
}

// ValidateSubjects implements programmatic validation of Subjects
func (u *UserInfo) ValidateSubjects(path *field.Path) (errs field.ErrorList) {
	for index, subject := range u.Subjects {
		entry := path.Index(index)
		if subject.Kind == "" {
			errs = append(errs, field.Required(entry.Child("kind"), ""))
		}
		if subject.Name == "" {
			errs = append(errs, field.Required(entry.Child("name"), ""))
		}
		if subject.Kind == rbacv1.ServiceAccountKind && subject.Namespace == "" {
			errs = append(errs, field.Required(entry.Child("namespace"), fmt.Sprintf("namespace is required when Kind is %s", rbacv1.ServiceAccountKind)))
		}
	}
	return errs
}

// ValidateRoles implements programmatic validation of Roles
func (u *UserInfo) ValidateRoles(path *field.Path) (errs field.ErrorList) {
	for i, r := range u.Roles {
		role := strings.Split(r, ":")
		if len(role) != 2 {
			errs = append(errs, field.Invalid(path.Index(i), r, "Role is expected to be in namespace:name format"))
		}
	}
	return errs
}

// Validate implements programmatic validation
func (u *UserInfo) Validate(path *field.Path) (errs field.ErrorList) {
	errs = append(errs, u.ValidateSubjects(path.Child("subjects"))...)
	errs = append(errs, u.ValidateRoles(path.Child("roles"))...)
	return errs
}

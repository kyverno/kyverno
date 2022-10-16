package report

import (
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	eventsv1beta1 "k8s.io/api/events/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// bannedOwners are GVKs that are not allowed to be owners of other resources
var bannedOwners = map[schema.GroupVersionKind]struct{}{
	corev1.SchemeGroupVersion.WithKind("Event"):        {},
	eventsv1.SchemeGroupVersion.WithKind("Event"):      {},
	eventsv1beta1.SchemeGroupVersion.WithKind("Event"): {},
}

func IsGvkSupported(gvk schema.GroupVersionKind) bool {
	_, exists := bannedOwners[gvk]
	return !exists
}

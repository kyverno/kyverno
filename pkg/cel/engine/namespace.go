package engine

import (
	corev1 "k8s.io/api/core/v1"
)

type NamespaceResolver = func(string) *corev1.Namespace

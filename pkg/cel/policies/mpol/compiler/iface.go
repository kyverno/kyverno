package compiler

import (
	"context"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
)

type Patcher interface {
	Patch(context.Context, *admissionv1.AdmissionRequest, patch.Request, int64) (runtime.Object, error)
}

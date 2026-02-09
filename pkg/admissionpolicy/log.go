package admissionpolicy

import "github.com/kyverno/kyverno/pkg/logging"

var (
	vapLogger = logging.WithName("validatingadmissionpolicy")
	mapLogger = logging.WithName("mutatingadmissionpolicy")
)

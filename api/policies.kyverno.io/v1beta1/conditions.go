package v1beta1

import (
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
)

type (
	PolicyConditionType = v1alpha1.PolicyConditionType
	ConditionStatus     = v1alpha1.ConditionStatus
)

const (
	PolicyConditionTypeWebhookConfigured      PolicyConditionType = "WebhookConfigured"
	PolicyConditionTypePolicyCached           PolicyConditionType = "PolicyCached"
	PolicyConditionTypeRBACPermissionsGranted PolicyConditionType = "RBACPermissionsGranted"
)

package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// AuthorizingFailurePolicyType controls behavior if policy evaluation fails.
// +kubebuilder:validation:Enum=NoOpinion;Deny
type AuthorizingFailurePolicyType string

const (
	AuthorizingFailurePolicyNoOpinion AuthorizingFailurePolicyType = "NoOpinion"
	AuthorizingFailurePolicyDeny      AuthorizingFailurePolicyType = "Deny"
)

// AuthorizingRuleEffect is the outcome of a matching authorizing rule.
// +kubebuilder:validation:Enum=Allow;Deny;NoOpinion;Conditional
type AuthorizingRuleEffect string

const (
	AuthorizingRuleEffectAllow       AuthorizingRuleEffect = "Allow"
	AuthorizingRuleEffectDeny        AuthorizingRuleEffect = "Deny"
	AuthorizingRuleEffectNoOpinion   AuthorizingRuleEffect = "NoOpinion"
	AuthorizingRuleEffectConditional AuthorizingRuleEffect = "Conditional"
)

// AuthorizingConditionEffect is the outcome of a matching conditional branch.
// +kubebuilder:validation:Enum=Allow;Deny;NoOpinion
type AuthorizingConditionEffect string

const (
	AuthorizingConditionEffectAllow     AuthorizingConditionEffect = "Allow"
	AuthorizingConditionEffectDeny      AuthorizingConditionEffect = "Deny"
	AuthorizingConditionEffectNoOpinion AuthorizingConditionEffect = "NoOpinion"
)

// PolicyConditionType identifies readiness conditions for policy lifecycle state.
type PolicyConditionType string

const (
	PolicyConditionTypePolicyCached      PolicyConditionType = "PolicyCached"
	PolicyConditionTypeWebhookConfigured PolicyConditionType = "WebhookConfigured"
)

const (
	policyConditionReasonSucceeded = "Succeeded"
	policyConditionReasonFailed    = "Failed"
)

// AuthorizingMatchCondition is a named CEL expression evaluated before a policy or rule runs.
type AuthorizingMatchCondition struct {
	Name       string `json:"name,omitempty"`
	Expression string `json:"expression,omitempty"`
}

// AuthorizingVariable is a reusable named CEL expression.
type AuthorizingVariable struct {
	Name       string `json:"name,omitempty"`
	Expression string `json:"expression,omitempty"`
}

// AuthorizingCondition is one conditional authorization branch evaluated after a conditional rule matches.
type AuthorizingCondition struct {
	// +kubebuilder:validation:MaxLength=63
	ID          string                     `json:"id"`
	Expression  string                     `json:"expression"`
	Effect      AuthorizingConditionEffect `json:"effect"`
	Description string                     `json:"description,omitempty"`
}

// AuthorizingRule defines one authorization rule.
type AuthorizingRule struct {
	Name            string                      `json:"name,omitempty"`
	Expression      string                      `json:"expression,omitempty"`
	Effect          AuthorizingRuleEffect       `json:"effect,omitempty"`
	MatchConditions []AuthorizingMatchCondition `json:"matchConditions,omitempty"`
	Conditions      []AuthorizingCondition      `json:"conditions,omitempty"`
}

// AuthorizingResourceRule declares which resource operations an authorizing policy applies to.
type AuthorizingResourceRule struct {
	APIGroups []string `json:"apiGroups,omitempty"`
	Resources []string `json:"resources,omitempty"`
	Verbs     []string `json:"verbs,omitempty"`
}

// AuthorizingMatchConstraints narrows policy applicability by request metadata.
type AuthorizingMatchConstraints struct {
	ResourceRules []AuthorizingResourceRule `json:"resourceRules,omitempty"`
}

// AuthorizingPolicySpec defines the desired authorizing policy behavior.
type AuthorizingPolicySpec struct {
	FailurePolicy    AuthorizingFailurePolicyType `json:"failurePolicy,omitempty"`
	MatchConditions  []AuthorizingMatchCondition  `json:"matchConditions,omitempty"`
	MatchConstraints *AuthorizingMatchConstraints `json:"matchConstraints,omitempty"`
	Rules            []AuthorizingRule            `json:"rules,omitempty"`
	Subjects         []rbacv1.Subject             `json:"subjects,omitempty"`
	Variables        []AuthorizingVariable        `json:"variables,omitempty"`
}

// ConditionStatus is the shared policy readiness status.
type ConditionStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	Message    string             `json:"message,omitempty"`
	Ready      *bool              `json:"ready,omitempty"`
}

// SetReadyByCondition updates a single condition and recomputes aggregate readiness.
func (status *ConditionStatus) SetReadyByCondition(conditionType PolicyConditionType, conditionStatus metav1.ConditionStatus, message string) {
	condition := metav1.Condition{
		Type:    string(conditionType),
		Status:  conditionStatus,
		Message: message,
	}
	if conditionStatus == metav1.ConditionTrue {
		condition.Reason = policyConditionReasonSucceeded
	} else {
		condition.Reason = policyConditionReasonFailed
	}
	meta.SetStatusCondition(&status.Conditions, condition)
	ready := true
	for _, existing := range status.Conditions {
		if existing.Status != metav1.ConditionTrue {
			ready = false
			break
		}
	}
	status.Ready = &ready
	status.Message = message
}

// IsReady reports the aggregate readiness flag.
func (status *ConditionStatus) IsReady() bool {
	return status.Ready != nil && *status.Ready
}

// AuthorizingPolicyStatus captures readiness information for an authorizing policy.
type AuthorizingPolicyStatus struct {
	ConditionStatus ConditionStatus `json:"conditionStatus,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=apol,categories=kyverno
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=".status.conditionStatus.ready"
type AuthorizingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthorizingPolicySpec   `json:"spec,omitempty"`
	Status AuthorizingPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type AuthorizingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthorizingPolicy `json:"items"`
}

func (in *AuthorizingPolicy) DeepCopyInto(out *AuthorizingPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *AuthorizingPolicy) DeepCopy() *AuthorizingPolicy {
	if in == nil {
		return nil
	}
	out := new(AuthorizingPolicy)
	in.DeepCopyInto(out)
	return out
}

func (in *AuthorizingPolicy) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *AuthorizingPolicyList) DeepCopyInto(out *AuthorizingPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]AuthorizingPolicy, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *AuthorizingPolicyList) DeepCopy() *AuthorizingPolicyList {
	if in == nil {
		return nil
	}
	out := new(AuthorizingPolicyList)
	in.DeepCopyInto(out)
	return out
}

func (in *AuthorizingPolicyList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *AuthorizingPolicySpec) DeepCopyInto(out *AuthorizingPolicySpec) {
	*out = *in
	if in.MatchConditions != nil {
		out.MatchConditions = append([]AuthorizingMatchCondition(nil), in.MatchConditions...)
	}
	if in.MatchConstraints != nil {
		out.MatchConstraints = new(AuthorizingMatchConstraints)
		in.MatchConstraints.DeepCopyInto(out.MatchConstraints)
	}
	if in.Rules != nil {
		out.Rules = make([]AuthorizingRule, len(in.Rules))
		for i := range in.Rules {
			in.Rules[i].DeepCopyInto(&out.Rules[i])
		}
	}
	if in.Subjects != nil {
		out.Subjects = append([]rbacv1.Subject(nil), in.Subjects...)
	}
	if in.Variables != nil {
		out.Variables = append([]AuthorizingVariable(nil), in.Variables...)
	}
}

func (in *AuthorizingRule) DeepCopyInto(out *AuthorizingRule) {
	*out = *in
	if in.MatchConditions != nil {
		out.MatchConditions = append([]AuthorizingMatchCondition(nil), in.MatchConditions...)
	}
	if in.Conditions != nil {
		out.Conditions = append([]AuthorizingCondition(nil), in.Conditions...)
	}
}

func (in *AuthorizingMatchConstraints) DeepCopyInto(out *AuthorizingMatchConstraints) {
	*out = *in
	if in.ResourceRules != nil {
		out.ResourceRules = append([]AuthorizingResourceRule(nil), in.ResourceRules...)
	}
}

func (in *AuthorizingPolicyStatus) DeepCopyInto(out *AuthorizingPolicyStatus) {
	*out = *in
	in.ConditionStatus.DeepCopyInto(&out.ConditionStatus)
}

func (in *ConditionStatus) DeepCopyInto(out *ConditionStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
	if in.Ready != nil {
		out.Ready = new(bool)
		*out.Ready = *in.Ready
	}
}

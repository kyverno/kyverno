package webhook

import (
	"slices"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	objectmeta "k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

// webhook is the instance that aggregates the GVK of existing policies
// based on group, kind, scopeType, failurePolicy and webhookTimeout
// a fine-grained webhook is created per policy with a unique path
type webhook struct {
	// policyMeta is set for fine-grained webhooks
	policyMeta        objectmeta.ObjectName
	maxWebhookTimeout int32
	failurePolicy     admissionregistrationv1.FailurePolicyType
	rules             sets.Set[ruleEntry]
	matchConditions   []admissionregistrationv1.MatchCondition
}

type ruleEntry struct {
	group       string
	version     string
	resource    string
	subresource string
	scope       admissionregistrationv1.ScopeType
	operation   kyvernov1.AdmissionOperation
}

type groupVersionScope struct {
	group   string
	version string
	scope   admissionregistrationv1.ScopeType
}

type resourceOperations struct {
	create  bool
	update  bool
	delete  bool
	connect bool
}

func (r resourceOperations) operations() []admissionregistrationv1.OperationType {
	// if r.create && r.update && r.delete && r.connect {
	// 	return []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll}
	// }
	var ops []admissionregistrationv1.OperationType
	if r.create {
		ops = append(ops, admissionregistrationv1.Create)
	}
	if r.update {
		ops = append(ops, admissionregistrationv1.Update)
	}
	if r.delete {
		ops = append(ops, admissionregistrationv1.Delete)
	}
	if r.connect {
		ops = append(ops, admissionregistrationv1.Connect)
	}
	return ops
}

func newWebhook(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType, matchConditions []admissionregistrationv1.MatchCondition) *webhook {
	return &webhook{
		maxWebhookTimeout: timeout,
		failurePolicy:     failurePolicy,
		rules:             sets.New[ruleEntry](),
		matchConditions:   matchConditions,
	}
}

func newWebhookPerPolicy(timeout int32, failurePolicy admissionregistrationv1.FailurePolicyType, matchConditions []admissionregistrationv1.MatchCondition, policy kyvernov1.PolicyInterface) *webhook {
	webhook := newWebhook(timeout, failurePolicy, matchConditions)
	webhook.policyMeta = objectmeta.ObjectName{
		Namespace: policy.GetNamespace(),
		Name:      policy.GetName(),
	}
	if policy.GetSpec().CustomWebhookMatchConditions() {
		webhook.matchConditions = policy.GetSpec().GetMatchConditions()
	}
	return webhook
}

func (wh *webhook) hasRule(
	group, version, resource, subresource string,
	scope admissionregistrationv1.ScopeType,
	operation kyvernov1.AdmissionOperation,
) bool {
	var scopes []admissionregistrationv1.ScopeType
	if scope == admissionregistrationv1.AllScopes {
		scopes = []admissionregistrationv1.ScopeType{scope}
	} else {
		scopes = []admissionregistrationv1.ScopeType{scope, admissionregistrationv1.AllScopes}
	}
	var groups, versions []string
	if group == "*" {
		groups = []string{group}
	} else {
		groups = []string{group, "*"}
	}
	if version == "*" {
		versions = []string{version}
	} else {
		versions = []string{version, "*"}
	}
	type resourceAndSub struct {
		resource, sub string
	}
	var resources []resourceAndSub
	// */* -> */*
	// pods/* -> pods/*, */*
	// */scale -> */scale, */*
	// pods/scale -> pods/scale, pods/*, */scale, */*
	// * -> *, */*
	// pods -> pods, *, */* (but not pods/*)
	if subresource == "" {
		if resource == "*" {
			resources = []resourceAndSub{{"*", ""}, {"*", "*"}}
		} else {
			resources = []resourceAndSub{{resource, ""}, {"*", ""}, {"*", "*"}}
		}
	} else if subresource == "*" {
		if resource == "*" {
			resources = []resourceAndSub{{"*", "*"}}
		} else {
			resources = []resourceAndSub{{resource, "*"}, {"*", "*"}}
		}
	} else {
		if resource == "*" {
			resources = []resourceAndSub{{"*", subresource}, {"*", "*"}}
		} else {
			resources = []resourceAndSub{{resource, subresource}, {resource, "*"}, {"*", subresource}, {"*", "*"}}
		}
	}
	for _, _scope := range scopes {
		for _, _group := range groups {
			for _, _version := range versions {
				for _, _resource := range resources {
					if _scope != scope || _group != group || _version != version || _resource.resource != resource || _resource.sub != subresource {
						test := ruleEntry{
							group:       _group,
							version:     _version,
							resource:    _resource.resource,
							subresource: _resource.sub,
							scope:       _scope,
							operation:   operation,
						}
						if wh.rules.Has(test) {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func (wh *webhook) buildRulesWithOperations() []admissionregistrationv1.RuleWithOperations {
	rules := map[groupVersionScope]map[string]resourceOperations{}
	// keep only the relevant rules and map operations by [group, version, scope] first, then by [resource]
	for rule := range wh.rules {
		if !wh.hasRule(rule.group, rule.version, rule.resource, rule.subresource, rule.scope, rule.operation) {
			key := groupVersionScope{rule.group, rule.version, rule.scope}
			gvs := rules[key]
			if gvs == nil {
				gvs = map[string]resourceOperations{}
				rules[key] = gvs
			}
			resource := rule.resource
			if rule.subresource != "" {
				resource = rule.resource + "/" + rule.subresource
			}
			ops := gvs[resource]
			switch rule.operation {
			case kyvernov1.Create:
				ops.create = true
			case kyvernov1.Update:
				ops.update = true
			case kyvernov1.Delete:
				ops.delete = true
			case kyvernov1.Connect:
				ops.connect = true
			}
			gvs[resource] = ops
		}
	}
	// build rules
	out := make([]admissionregistrationv1.RuleWithOperations, 0, len(rules))
	for gvs, resources := range rules {
		// invert the resources map
		opsResources := map[resourceOperations]sets.Set[string]{}
		for resource, ops := range resources {
			r := opsResources[ops]
			if r == nil {
				r = sets.New[string]()
			}
			opsResources[ops] = r.Insert(resource)
		}
		for ops, resources := range opsResources {
			// if we have pods, we add pods/ephemeralcontainers by default
			if (gvs.group == "" || gvs.group == "*") && (gvs.version == "v1" || gvs.version == "*") && (resources.Has("pods") || resources.Has("*")) {
				resources = resources.Insert("pods/ephemeralcontainers")
			}
			out = append(out, admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{gvs.group},
					APIVersions: []string{gvs.version},
					Resources:   resources.UnsortedList(),
					Scope:       ptr.To(gvs.scope),
				},
				Operations: ops.operations(),
			})
		}
	}
	// sort rules
	for _, rule := range out {
		slices.Sort(rule.APIGroups)
		slices.Sort(rule.APIVersions)
		slices.Sort(rule.Resources)
		slices.Sort(rule.Operations)
	}
	slices.SortFunc(out, func(a admissionregistrationv1.RuleWithOperations, b admissionregistrationv1.RuleWithOperations) int {
		if x := less(a.APIGroups, b.APIGroups); x != 0 {
			return x
		}
		if x := less(a.APIVersions, b.APIVersions); x != 0 {
			return x
		}
		if x := less(a.Resources, b.Resources); x != 0 {
			return x
		}
		if x := less(a.Operations, b.Operations); x != 0 {
			return x
		}
		if x := strings.Compare(string(*a.Scope), string(*b.Scope)); x != 0 {
			return x
		}
		return 0
	})
	return out
}

func (wh *webhook) set(
	group string,
	version string,
	resource string,
	subresource string,
	scope admissionregistrationv1.ScopeType,
	operations ...kyvernov1.AdmissionOperation,
) {
	for _, operation := range operations {
		wh.rules.Insert(ruleEntry{
			group:       group,
			version:     version,
			resource:    resource,
			subresource: subresource,
			scope:       scope,
			operation:   operation,
		})
	}
}

func (wh *webhook) isEmpty() bool {
	return len(wh.rules) == 0
}

func (wh *webhook) key(separator string) string {
	p := wh.policyMeta
	if p.Namespace != "" {
		return p.Namespace + separator + p.Name
	}
	return p.Name
}

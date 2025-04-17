package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/distribution/reference"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/jmoiron/jsonq"
	"github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/engine/variables/operator"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"github.com/kyverno/kyverno/pkg/logging"
	celutils "github.com/kyverno/kyverno/pkg/utils/cel"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	vaputils "github.com/kyverno/kyverno/pkg/validatingadmissionpolicy"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/cel/openapi/resolver"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
)

var (
	allowedVariables                   = enginecontext.ReservedKeys
	allowedVariablesBackground         = regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|([a-z_0-9]+\()[^{}]`)
	allowedVariablesInTarget           = regexp.MustCompile(`request\.|serviceAccountName|serviceAccountNamespace|element|elementIndex|@|images|images\.|image\.|target\.|([a-z_0-9]+\()[^{}]`)
	allowedVariablesBackgroundInTarget = regexp.MustCompile(`request\.|element|elementIndex|@|images|images\.|image\.|target\.|([a-z_0-9]+\()[^{}]`)
	regexVariables                     = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	bindingIdentifier                  = regexp.MustCompile(`^\w+$`)
	// wildCardAllowedVariables represents regex for the allowed fields in wildcards
	wildCardAllowedVariables = regexp.MustCompile(`\{\{\s*(request\.|serviceAccountName|serviceAccountNamespace)[^{}]*\}\}`)
	errOperationForbidden    = errors.New("variables are forbidden in the path of a JSONPatch")
)

var allowedJsonPatch = regexp.MustCompile("^/")

// validateJSONPatchPathForForwardSlash checks for forward slash
func validateJSONPatchPathForForwardSlash(patch string) error {
	// Replace all variables in PatchesJSON6902, all variable checks should have happened already.
	// This prevents further checks from failing unexpectedly.
	patch = variables.ReplaceAllVars(patch, func(s string) string { return "kyvernojsonpatchvariable" })

	jsonPatch, err := yaml.ToJSON([]byte(patch))
	if err != nil {
		return err
	}

	decodedPatch, err := jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return err
	}

	for _, operation := range decodedPatch {
		path, err := operation.Path()
		if err != nil {
			return err
		}

		val := allowedJsonPatch.MatchString(path)

		if !val {
			return fmt.Errorf("%s", path)
		}
	}
	return nil
}

func validateJSONPatch(patch string, ruleIdx int) error {
	patch = variables.ReplaceAllVars(patch, func(s string) string { return "kyvernojsonpatchvariable" })
	jsonPatch, err := yaml.ToJSON([]byte(patch))
	if err != nil {
		return err
	}
	decodedPatch, err := jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return err
	}
	for _, operation := range decodedPatch {
		op := operation.Kind()
		requiresValue := op != "remove" && op != "move" && op != "copy"
		validOperation := op == "add" || op == "remove" || op == "replace" || op == "move" || op == "copy" || op == "test"
		if !validOperation {
			return fmt.Errorf("unexpected kind: spec.rules[%d]: %s", ruleIdx, op)
		}
		if requiresValue {
			if _, err := operation.ValueInterface(); err != nil {
				return fmt.Errorf("invalid value: spec.rules[%d]: %s", ruleIdx, err)
			}
		}
	}

	return nil
}

func checkValidationFailureAction(validationFailureAction kyvernov1.ValidationFailureAction, validationFailureActionOverrides []kyvernov1.ValidationFailureActionOverride) []string {
	msg := "Validation failure actions enforce/audit are deprecated, use Enforce/Audit instead."
	if validationFailureAction == "enforce" || validationFailureAction == "audit" {
		return []string{msg}
	}
	for _, override := range validationFailureActionOverrides {
		if override.Action == "enforce" || override.Action == "audit" {
			return []string{msg}
		}
	}
	return nil
}

// Validate checks the policy and rules declarations for required configurations
func Validate(policy, oldPolicy kyvernov1.PolicyInterface, client dclient.Interface, kyvernoClient versioned.Interface, mock bool, backgroundSA, reportsSA string) ([]string, error) {
	var warnings []string
	spec := policy.GetSpec()
	background := spec.BackgroundProcessingEnabled()
	if policy.GetSpec().CustomWebhookMatchConditions() &&
		!kubeutils.HigherThanKubernetesVersion(client.GetKubeClient().Discovery(), logging.GlobalLogger(), 1, 27, 0) {
		return warnings, fmt.Errorf("custom webhook configurations are only supported in kubernetes version 1.27.0 and above")
	}

	warnings = append(warnings, checkValidationFailureAction(spec.ValidationFailureAction, spec.ValidationFailureActionOverrides)...)
	for _, rule := range spec.Rules {
		if rule.HasValidate() {
			if rule.Validation.FailureAction != nil {
				warnings = append(warnings, checkValidationFailureAction(*rule.Validation.FailureAction, rule.Validation.FailureActionOverrides)...)
			}
		}
	}
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	mc := spec.GetMatchConditions()
	if mc != nil {
		if err := ValidateCustomWebhookMatchConditions(spec.GetMatchConditions()); err != nil {
			return warnings, err
		}
	}

	err := ValidateVariables(policy, background)
	if err != nil {
		return warnings, err
	}

	getClusteredResources := func(invalidate bool) (sets.Set[string], error) {
		clusterResources := sets.New[string]()
		// Get all the cluster type kind supported by cluster
		d := client.Discovery().CachedDiscoveryInterface()
		if invalidate {
			d.Invalidate()
		}
		res, err := discovery.ServerPreferredResources(d)
		if err != nil {
			if discovery.IsGroupDiscoveryFailedError(err) {
				err := err.(*discovery.ErrGroupDiscoveryFailed)
				for gv, err := range err.Groups {
					logging.Error(err, "failed to list api resources", "group", gv)
				}
			} else {
				return nil, err
			}
		}
		for _, resList := range res {
			for _, r := range resList.APIResources {
				if !r.Namespaced {
					clusterResources.Insert(resList.GroupVersion + "/" + r.Kind)
					clusterResources.Insert(r.Kind)
				}
			}
		}
		return clusterResources, nil
	}
	clusterResources := sets.New[string]()

	// if not using a mock, we first try to validate and if it fails we retry with cache invalidation in between
	if !mock {
		clusterResources, err = getClusteredResources(false)
		if err != nil {
			return warnings, err
		}
		if errs := policy.Validate(clusterResources); len(errs) != 0 {
			clusterResources, err = getClusteredResources(true)
			if err != nil {
				return warnings, err
			}
			if errs := policy.Validate(clusterResources); len(errs) != 0 {
				return warnings, errs.ToAggregate()
			}
		}
	} else {
		if errs := policy.Validate(clusterResources); len(errs) != 0 {
			return warnings, errs.ToAggregate()
		}
	}

	if !policy.IsNamespaced() {
		for i, r := range spec.Rules {
			if r.HasValidate() {
				err := validateNamespaces(r.Validation.FailureActionOverrides, specPath.Child("rules").Index(i).Child("validate").Child("validationFailureActionOverrides"))
				if err != nil {
					return warnings, err
				}
			}
		}
		err := validateNamespaces(spec.ValidationFailureActionOverrides, specPath.Child("validationFailureActionOverrides"))
		if err != nil {
			return warnings, err
		}
	}
	if !policy.AdmissionProcessingEnabled() && !policy.BackgroundProcessingEnabled() {
		return warnings, fmt.Errorf("disabling both admission and background processing is not allowed")
	}
	if !policy.AdmissionProcessingEnabled() {
		if spec.HasMutate() || spec.HasGenerate() || spec.HasVerifyImages() {
			return warnings, fmt.Errorf("disabling admission processing is only allowed with validation policies")
		}
	}

	if warning, err := immutableGenerateFields(policy, oldPolicy); warning != "" || err != nil {
		warnings = append(warnings, fmt.Sprintf("no synchronization will be performed to the old target resource upon policy updates: %s", warning))
		return warnings, err
	}

	rules := autogen.ComputeRules(policy, "")
	rulesPath := specPath.Child("rules")

	for i, rule := range rules {
		match := rule.MatchResources
		for j, value := range match.Any {
			if err := validateKinds(value.ResourceDescription.Kinds, rule, mock, background, client); err != nil {
				return warnings, fmt.Errorf("path: spec.rules[%d].match.any[%d].kinds: %v", i, j, err)
			}
		}
		for j, value := range match.All {
			if err := validateKinds(value.ResourceDescription.Kinds, rule, mock, background, client); err != nil {
				return warnings, fmt.Errorf("path: spec.rules[%d].match.all[%d].kinds: %v", i, j, err)
			}
		}
		if err := validateKinds(rule.MatchResources.Kinds, rule, mock, background, client); err != nil {
			return warnings, fmt.Errorf("path: spec.rules[%d].match.kinds: %v", i, err)
		}
		if exclude := rule.ExcludeResources; exclude != nil {
			for j, value := range exclude.Any {
				if err := validateKinds(value.ResourceDescription.Kinds, rule, mock, background, client); err != nil {
					return warnings, fmt.Errorf("path: spec.rules[%d].exclude.any[%d].kinds: %v", i, j, err)
				}
			}
			for j, value := range exclude.All {
				if err := validateKinds(value.ResourceDescription.Kinds, rule, mock, background, client); err != nil {
					return warnings, fmt.Errorf("path: spec.rules[%d].exclude.all[%d].kinds: %v", i, j, err)
				}
			}
			if err := validateKinds(exclude.Kinds, rule, mock, background, client); err != nil {
				return warnings, fmt.Errorf("path: spec.rules[%d].exclude.kinds: %v", i, err)
			}
		}
	}

	for i, rule := range rules {
		rulePath := rulesPath.Index(i)
		if rule.Mutation != nil {
			// check for forward slash
			if err := validateJSONPatchPathForForwardSlash(rule.Mutation.PatchesJSON6902); err != nil {
				return warnings, fmt.Errorf("path must begin with a forward slash: spec.rules[%d]: %s", i, err)
			}
			if err := validateJSONPatch(rule.Mutation.PatchesJSON6902, i); err != nil {
				return warnings, fmt.Errorf("%s", err)
			}
		}

		if jsonPatchOnPod(rule) {
			msg := "Pods managed by workload controllers should not be directly mutated using policies. " +
				"Use the autogen feature or write policies that match Pod controllers."
			logging.V(1).Info(msg)
			warnings = append(warnings, msg)
		}

		// validate resource description
		if path, err := validateResources(rulePath, rule); err != nil {
			return warnings, fmt.Errorf("path: spec.rules[%d].%s: %v", i, path, err)
		}

		err := validateElementInForEach(rule)
		if err != nil {
			return warnings, err
		}

		if err := validateRuleContext(rule); err != nil {
			return warnings, fmt.Errorf("path: spec.rules[%d]: %v", i, err)
		}

		if err := validateRuleImageExtractorsJMESPath(rule); err != nil {
			return warnings, fmt.Errorf("path: spec.rules[%d]: %v", i, err)
		}

		// If a rule's match block does not match any kind,
		// we should only allow it to have metadata in its overlay
		if len(rule.MatchResources.Any) > 0 {
			for _, rmr := range rule.MatchResources.Any {
				if len(rmr.Kinds) == 0 {
					return warnings, validateMatchKindHelper(rule)
				}
			}
		} else if len(rule.MatchResources.All) > 0 {
			for _, rmr := range rule.MatchResources.All {
				if len(rmr.Kinds) == 0 {
					return warnings, validateMatchKindHelper(rule)
				}
			}
		} else {
			if len(rule.MatchResources.Kinds) == 0 {
				return warnings, validateMatchKindHelper(rule)
			}
		}

		w, err := validateActions(i, &rules[i], client, mock, backgroundSA, reportsSA)
		if err != nil {
			return warnings, err
		} else if len(w) > 0 {
			warnings = append(warnings, w...)
		}

		if rule.HasVerifyImages() {
			isAuditFailureAction := false
			if spec.ValidationFailureAction.Audit() {
				isAuditFailureAction = true
			}

			verifyImagePath := rulePath.Child("verifyImages")
			for index, i := range rule.VerifyImages {
				action := i.FailureAction
				if action != nil {
					if action.Audit() {
						isAuditFailureAction = true
					} else {
						isAuditFailureAction = false
					}
				}
				errs = append(errs, i.Validate(isAuditFailureAction, verifyImagePath.Index(index))...)
			}
			if len(errs) != 0 {
				return warnings, errs.ToAggregate()
			}
		}

		kindsFromRule := rule.MatchResources.GetKinds()
		resourceTypesMap := make(map[string]bool)
		for _, kind := range kindsFromRule {
			_, k := kubeutils.GetKindFromGVK(kind)
			k, _ = kubeutils.SplitSubresource(k)
			resourceTypesMap[k] = true
		}
		if len(resourceTypesMap) == 1 {
			for k := range resourceTypesMap {
				if k == "Pod" && podControllerAutoGenExclusion(policy) {
					msg := "Policies that match Pods apply to all Pods including those created and managed by controllers " +
						"excluded from autogen. Use preconditions to exclude the Pods managed by controllers which are " +
						"excluded from autogen. Refer to https://kyverno.io/docs/writing-policies/autogen/ for details."

					warnings = append(warnings, msg)
				}
			}
		}

		// Validate string values in labels
		if !isLabelAndAnnotationsString(rule) {
			return warnings, fmt.Errorf("labels and annotations supports only string values, \"use double quotes around the non string values\"")
		}

		match := rule.MatchResources
		matchKinds := match.GetKinds()
		var allKinds []string
		allKinds = append(allKinds, matchKinds...)
		if exclude := rule.ExcludeResources; exclude != nil {
			excludeKinds := exclude.GetKinds()
			allKinds = append(allKinds, excludeKinds...)
		}
		if rule.HasValidate() {
			validationElem := rule.Validation.DeepCopy()
			if validationElem.Deny != nil {
				validationElem.Deny.RawAnyAllConditions = nil
			}
			validationJson, err := json.Marshal(validationElem)
			if err != nil {
				return nil, err
			}
			checkForScaleSubresource(validationJson, allKinds, &warnings)
			checkForStatusSubresource(validationJson, allKinds, &warnings)
		}

		if rule.HasMutate() {
			mutationJson, err := json.Marshal(rule.Mutation)
			targets := rule.Mutation.Targets
			for _, target := range targets {
				allKinds = append(allKinds, target.GetKind())
			}
			if err != nil {
				return nil, err
			}
			checkForScaleSubresource(mutationJson, allKinds, &warnings)
			checkForStatusSubresource(mutationJson, allKinds, &warnings)

			mutateExisting := rule.Mutation.MutateExistingOnPolicyUpdate
			if mutateExisting != nil {
				if *mutateExisting {
					if err := ValidateOnPolicyUpdate(policy, true); err != nil {
						return warnings, err
					}
				}
			} else if spec.MutateExistingOnPolicyUpdate {
				if err := ValidateOnPolicyUpdate(policy, true); err != nil {
					return warnings, err
				}
			}
		}

		if rule.HasVerifyImages() {
			checkForDeprecatedFieldsInVerifyImages(rule, &warnings)

			if rule.HasValidateImageVerification() {
				for _, verifyImage := range rule.VerifyImages {
					validationElem := verifyImage.Validation.DeepCopy()
					if validationElem.Deny != nil {
						validationElem.Deny.RawAnyAllConditions = nil
					}
					validationJson, err := json.Marshal(validationElem)
					if err != nil {
						return nil, err
					}
					checkForScaleSubresource(validationJson, allKinds, &warnings)
					checkForStatusSubresource(validationJson, allKinds, &warnings)
				}
			}
		}

		checkForDeprecatedOperatorsInRule(rule, &warnings)
	}

	// global context entry validation
	if kyvernoClient != nil {
		gctxentries, err := kyvernoClient.KyvernoV2alpha1().GlobalContextEntries().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, rule := range rules {
			if rule.Context == nil {
				continue
			}
			for _, ctxEntry := range rule.Context {
				if ctxEntry.GlobalReference != nil {
					if ctxEntry.GlobalReference.Name == "" {
						warnings = append(warnings, "Global context entry name is not provided")
						return warnings, nil
					}
					if !isGlobalContextEntryReady(ctxEntry.GlobalReference.Name, gctxentries) {
						warnings = append(warnings, fmt.Sprintf("Global context entry %s does not exist or is not ready", ctxEntry.GlobalReference.Name))
						return warnings, nil
					}
				}
			}
		}
	}

	// check for CEL expression warnings in case of CEL subrules
	if ok, _ := vaputils.CanGenerateVAP(spec, nil); ok && client != nil {
		resolver := &resolver.ClientDiscoveryResolver{
			Discovery: client.GetKubeClient().Discovery(),
		}
		groupResources, err := restmapper.GetAPIGroupResources(client.GetKubeClient().Discovery())
		if err != nil {
			return nil, err
		}
		mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
		checker := &validating.TypeChecker{
			SchemaResolver: resolver,
			RestMapper:     mapper,
		}

		// build Kubernetes ValidatingAdmissionPolicy
		vap := &admissionregistrationv1beta1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: policy.GetName(),
			},
		}
		err = vaputils.BuildValidatingAdmissionPolicy(client.Discovery(), vap, policy, nil)
		if err != nil {
			return nil, err
		}
		v1vap := vaputils.ConvertValidatingAdmissionPolicy(*vap)

		// check cel expression warnings
		ctx := checker.CreateContext(&v1vap)
		fieldRef := field.NewPath("spec", "rules[0]", "validate", "cel", "expressions")
		for i, e := range spec.Rules[0].Validation.CEL.Expressions {
			results := checker.CheckExpression(ctx, e.Expression)
			if len(results) != 0 {
				msg := fmt.Sprintf("%s:%s", fieldRef.Index(i).Child("expression").String(), strings.ReplaceAll(results.String(), "\n", ";"))
				warnings = append(warnings, msg)
			}

			if e.MessageExpression == "" {
				continue
			}
			results = checker.CheckExpression(ctx, e.MessageExpression)
			if len(results) != 0 {
				msg := fmt.Sprintf("%s:%s", fieldRef.Index(i).Child("messageExpression").String(), strings.ReplaceAll(results.String(), "\n", ";"))
				warnings = append(warnings, msg)
			}
		}
	}
	return warnings, nil
}

func isGlobalContextEntryReady(name string, gctxentries *kyvernov2alpha1.GlobalContextEntryList) bool {
	for _, gctxentry := range gctxentries.Items {
		if gctxentry.Name == name {
			return gctxentry.Status.IsReady()
		}
	}
	return false
}

func ValidateCustomWebhookMatchConditions(wc []admissionregistrationv1.MatchCondition) error {
	c, err := celutils.NewCompiler(nil, nil, wc, nil)
	if err != nil {
		return err
	}
	f := c.CompileMatchExpressions(cel.OptionalVariableDeclarations{})
	if len(f.CompilationErrors()) > 0 {
		return fmt.Errorf("match conditions compilation errors: %v", f.CompilationErrors())
	}
	return nil
}

func ValidateVariables(p kyvernov1.PolicyInterface, backgroundMode bool) error {
	vars, err := hasVariables(p)
	if err != nil {
		return err
	}
	if backgroundMode {
		if err := containsUserVariables(p, vars); err != nil {
			return fmt.Errorf("only select variables are allowed in background mode. Set spec.background=false to disable background mode for this policy rule: %s ", err)
		}
	}
	if err := hasInvalidVariables(p, backgroundMode); err != nil {
		return fmt.Errorf("policy contains invalid variables: %s", err.Error())
	}
	return nil
}

// hasInvalidVariables - checks for unexpected variables in the policy
func hasInvalidVariables(policy kyvernov1.PolicyInterface, background bool) error {
	for _, r := range autogen.ComputeRules(policy, "") {
		ruleCopy := r.DeepCopy()

		if err := ruleForbiddenSectionsHaveVariables(ruleCopy); err != nil {
			return err
		}

		// skip variable checks on verifyImages.attestations, as variables in attestations are dynamic
		for i, vi := range ruleCopy.VerifyImages {
			for j := range vi.Attestations {
				ruleCopy.VerifyImages[i].Attestations[j].Conditions = nil
			}
		}

		mutateTarget := false
		if ruleCopy.Mutation != nil && ruleCopy.Mutation.Targets != nil {
			mutateTarget = true
			withTargetOnly := ruleWithoutPattern(ruleCopy)
			for i := range ruleCopy.Mutation.Targets {
				withTargetOnly.Mutation.Targets[i].ResourceSpec = ruleCopy.Mutation.Targets[i].ResourceSpec
				ctx := buildContext(withTargetOnly, background, false)
				if _, err := variables.SubstituteAllInRule(logging.GlobalLogger(), ctx, *withTargetOnly); !variables.CheckNotFoundErr(err) {
					return fmt.Errorf("invalid variables defined at mutate.targets[%d]: %s", i, err.Error())
				}
			}
		}

		ctx := buildContext(ruleCopy, background, mutateTarget)
		if _, err := variables.SubstituteAllInRule(logging.GlobalLogger(), ctx, *ruleCopy); !variables.CheckNotFoundErr(err) {
			return fmt.Errorf("variable substitution failed for rule %s: %s", ruleCopy.Name, err.Error())
		}
	}

	return nil
}

func ValidateOnPolicyUpdate(p kyvernov1.PolicyInterface, onPolicyUpdate bool) error {
	vars, err := hasVariables(p)
	if err != nil {
		return err
	}
	if len(vars) == 0 {
		return nil
	}

	if err := hasInvalidVariables(p, onPolicyUpdate); err != nil {
		return fmt.Errorf("update event, policy contains invalid variables: %s", err.Error())
	}

	if err := containsUserVariables(p, vars); err != nil {
		return fmt.Errorf("only select variables are allowed in on policy update. Set spec.mutateExistingOnPolicyUpdate=false to disable update policy mode for this policy rule: %s ", err)
	}

	return nil
}

// for now forbidden sections are match, exclude and
func ruleForbiddenSectionsHaveVariables(rule *kyvernov1.Rule) error {
	var err error
	if rule.Mutation != nil {
		err = jsonPatchPathHasVariables(rule.Mutation.PatchesJSON6902)
		if err != nil && errors.Is(errOperationForbidden, err) {
			return fmt.Errorf("rule \"%s\" should not have variables in patchesJSON6902 path section", rule.Name)
		}
	}

	err = objectHasVariables(rule.ExcludeResources)
	if err != nil {
		return fmt.Errorf("rule \"%s\" should not have variables in exclude section", rule.Name)
	}

	err = objectHasVariables(rule.MatchResources)
	if err != nil {
		return fmt.Errorf("rule \"%s\" should not have variables in match section", rule.Name)
	}

	err = imageRefHasVariables(rule.VerifyImages)
	if err != nil {
		return fmt.Errorf("rule \"%s\" should not have variables in image reference section", rule.Name)
	}

	return nil
}

// hasVariables - check for variables in the policy
func hasVariables(policy kyvernov1.PolicyInterface) ([][]string, error) {
	polCopy := cleanup(policy.CreateDeepCopy())
	policyRaw, err := json.Marshal(polCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize the policy: %v", err)
	}
	matches := regex.RegexVariables.FindAllStringSubmatch(string(policyRaw), -1)
	return matches, nil
}

func cleanup(policy kyvernov1.PolicyInterface) kyvernov1.PolicyInterface {
	ann := policy.GetAnnotations()
	if ann != nil {
		ann["kubectl.kubernetes.io/last-applied-configuration"] = ""
		policy.SetAnnotations(ann)
	}
	if policy.GetNamespace() == "" {
		var pol *kyvernov1.ClusterPolicy
		var ok bool
		if pol, ok = policy.(*kyvernov1.ClusterPolicy); !ok {
			return policy
		}
		pol.Status.Autogen.Rules = nil
		return pol
	} else {
		var pol *kyvernov1.Policy
		var ok bool
		if pol, ok = policy.(*kyvernov1.Policy); !ok {
			return policy
		}
		pol.Status.Autogen.Rules = nil
		return pol
	}
}

func jsonPatchPathHasVariables(patch string) error {
	jsonPatch, err := yaml.ToJSON([]byte(patch))
	if err != nil {
		return err
	}

	decodedPatch, err := jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return err
	}

	for _, operation := range decodedPatch {
		path, err := operation.Path()
		if err != nil {
			return err
		}

		vars := regex.RegexVariables.FindAllString(path, -1)
		if len(vars) > 0 {
			return errOperationForbidden
		}
	}

	return nil
}

func objectHasVariables(object any) error {
	if object != nil {
		objectJSON, err := json.Marshal(object)
		if err != nil {
			return err
		}
		if len(regexVariables.FindAllStringSubmatch(string(objectJSON), -1)) > 0 {
			return fmt.Errorf("invalid variables")
		}
	}
	return nil
}

func imageRefHasVariables(verifyImages []kyvernov1.ImageVerification) error {
	for _, verifyImage := range verifyImages {
		verifyImage = *verifyImage.Convert()
		for _, imageRef := range verifyImage.ImageReferences {
			matches := regex.RegexVariables.FindAllString(imageRef, -1)
			if len(matches) > 0 {
				return fmt.Errorf("variables are not allowed in image reference")
			}
		}
	}
	return nil
}

func ruleWithoutPattern(ruleCopy *kyvernov1.Rule) *kyvernov1.Rule {
	withTargetOnly := &kyvernov1.Rule{
		Mutation: &kyvernov1.Mutation{},
	}
	withTargetOnly.Mutation.Targets = make([]kyvernov1.TargetResourceSpec, len(ruleCopy.Mutation.Targets))
	withTargetOnly.Context = ruleCopy.Context
	withTargetOnly.RawAnyAllConditions = ruleCopy.RawAnyAllConditions
	return withTargetOnly
}

func buildContext(rule *kyvernov1.Rule, background bool, target bool) *enginecontext.MockContext {
	re := getAllowedVariables(background, target)

	ctx := enginecontext.NewMockContext(re)

	addContextVariables(rule.Context, ctx)
	addImageVerifyVariables(rule, ctx)

	if rule.Validation != nil {
		for _, fe := range rule.Validation.ForEachValidation {
			addContextVariables(fe.Context, ctx)
		}
	}
	if rule.Mutation != nil {
		for _, fe := range rule.Mutation.ForEachMutation {
			addContextVariables(fe.Context, ctx)
		}
		for _, fe := range rule.Mutation.Targets {
			addContextVariables(fe.Context, ctx)
		}
	}
	if rule.HasGenerate() {
		for _, fe := range rule.Generation.ForEachGeneration {
			addContextVariables(fe.Context, ctx)
		}
	}
	return ctx
}

func getAllowedVariables(background bool, target bool) *regexp.Regexp {
	if target {
		if background {
			return allowedVariablesBackgroundInTarget
		}
		return allowedVariablesInTarget
	} else {
		if background {
			return allowedVariablesBackground
		}
		return allowedVariables
	}
}

func addContextVariables(entries []kyvernov1.ContextEntry, ctx *enginecontext.MockContext) {
	for _, contextEntry := range entries {
		if contextEntry.APICall != nil || contextEntry.GlobalReference != nil || contextEntry.ImageRegistry != nil || contextEntry.Variable != nil {
			ctx.AddVariable(contextEntry.Name + "*")
		}

		if contextEntry.ConfigMap != nil {
			ctx.AddVariable(contextEntry.Name + ".data")
			ctx.AddVariable(contextEntry.Name + ".metadata")
			ctx.AddVariable(contextEntry.Name + ".data.*")
			ctx.AddVariable(contextEntry.Name + ".metadata.*")
		}
	}
}

func addImageVerifyVariables(rule *kyvernov1.Rule, ctx *enginecontext.MockContext) {
	if rule.HasValidateImageVerification() {
		for _, verifyImage := range rule.VerifyImages {
			for _, attestation := range verifyImage.Attestations {
				ctx.AddVariable(attestation.Name + "*")
			}
		}
	}
}

func validateElementInForEach(document apiextensions.JSON) error {
	jsonByte, err := json.Marshal(document)
	if err != nil {
		return err
	}

	var jsonInterface interface{}
	err = json.Unmarshal(jsonByte, &jsonInterface)
	if err != nil {
		return err
	}
	_, err = variables.ValidateElementInForEach(logging.GlobalLogger(), jsonInterface)
	return err
}

func validateMatchKindHelper(rule kyvernov1.Rule) error {
	if !ruleOnlyDealsWithResourceMetaData(rule) {
		return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
			" the rule does not match any kind")
	}

	return fmt.Errorf("at least one element must be specified in a kind block, the kind attribute is mandatory when working with the resources element")
}

// isMapStringString goes through a map to verify values are string
func isMapStringString(m map[string]interface{}) bool {
	// range over labels
	for _, val := range m {
		if val == nil || reflect.TypeOf(val).String() != "string" {
			return false
		}
	}
	return true
}

// isLabelAndAnnotationsString :- Validate if labels and annotations contains only string values
func isLabelAndAnnotationsString(rule kyvernov1.Rule) bool {
	checkLabelAnnotation := func(metaKey map[string]interface{}) bool {
		for mk := range metaKey {
			if mk == "labels" {
				labelKey, ok := metaKey[mk].(map[string]interface{})
				if ok {
					if !isMapStringString(labelKey) {
						return false
					}
				}
			} else if mk == "annotations" {
				annotationKey, ok := metaKey[mk].(map[string]interface{})
				if ok {
					if !isMapStringString(annotationKey) {
						return false
					}
				}
			}
		}
		return true
	}

	// checkMetadata - Verify if the labels and annotations contains string value inside metadata
	checkMetadata := func(patternMap map[string]interface{}) bool {
		for k := range patternMap {
			if k == "metadata" {
				metaKey, ok := patternMap[k].(map[string]interface{})
				if ok {
					// range over metadata
					return checkLabelAnnotation(metaKey)
				}
			}
			if k == "spec" {
				metadata, _ := jsonq.NewQuery(patternMap).Object("spec", "template", "metadata")
				return checkLabelAnnotation(metadata)
			}
		}
		return true
	}

	if rule.HasValidate() {
		if rule.Validation.ForEachValidation != nil {
			for _, foreach := range rule.Validation.ForEachValidation {
				patternMap, ok := foreach.GetPattern().(map[string]interface{})
				if ok {
					return checkMetadata(patternMap)
				}
			}
		} else {
			patternMap, ok := rule.Validation.GetPattern().(map[string]interface{})
			if ok {
				return checkMetadata(patternMap)
			} else if rule.Validation.GetAnyPattern() != nil {
				anyPatterns, err := rule.Validation.DeserializeAnyPattern()
				if err != nil {
					logging.Error(err, "failed to deserialize anyPattern, expect type array")
					return false
				}

				for _, pattern := range anyPatterns {
					patternMap, ok := pattern.(map[string]interface{})
					if ok {
						ret := checkMetadata(patternMap)
						if !ret {
							return ret
						}
					}
				}
			}
		}
	}

	if rule.HasMutate() {
		if rule.Mutation.ForEachMutation != nil {
			for _, foreach := range rule.Mutation.ForEachMutation {
				forEachStrategicMergeMap, ok := foreach.GetPatchStrategicMerge().(map[string]interface{})
				if ok {
					return checkMetadata(forEachStrategicMergeMap)
				}
			}
		} else {
			strategicMergeMap, ok := rule.Mutation.GetPatchStrategicMerge().(map[string]interface{})
			if ok {
				return checkMetadata(strategicMergeMap)
			}
		}
	}

	return true
}

func ruleOnlyDealsWithResourceMetaData(rule kyvernov1.Rule) bool {
	if rule.Mutation != nil {
		patches, _ := rule.Mutation.GetPatchStrategicMerge().(map[string]interface{})
		for k := range patches {
			if k != "metadata" {
				return false
			}
		}
	}

	if rule.Mutation != nil && rule.Mutation.PatchesJSON6902 != "" {
		bytes := []byte(rule.Mutation.PatchesJSON6902)
		jp, _ := jsonpatch.DecodePatch(bytes)
		for _, o := range jp {
			path, _ := o.Path()
			if !strings.HasPrefix(path, "/metadata") {
				return false
			}
		}
	}
	if rule.Validation != nil {
		patternMap, _ := rule.Validation.GetPattern().(map[string]interface{})
		for k := range patternMap {
			if k != "metadata" {
				return false
			}
		}
		anyPatterns, err := rule.Validation.DeserializeAnyPattern()
		if err != nil {
			logging.Error(err, "failed to deserialize anyPattern, expect type array")
			return false
		}
		for _, pattern := range anyPatterns {
			patternMap, _ := pattern.(map[string]interface{})
			for k := range patternMap {
				if k != "metadata" {
					return false
				}
			}
		}
	}
	return true
}

func validateResources(path *field.Path, rule kyvernov1.Rule) (string, error) {
	// validate userInfo in match and exclude
	if exclude := rule.ExcludeResources; exclude != nil {
		if errs := exclude.UserInfo.Validate(path.Child("exclude")); len(errs) != 0 {
			return "exclude", errs.ToAggregate()
		}
		if (len(exclude.Any) > 0 || len(exclude.All) > 0) && !datautils.DeepEqual(exclude.ResourceDescription, kyvernov1.ResourceDescription{}) {
			return "exclude.", fmt.Errorf("can't specify any/all together with exclude resources")
		}
		if len(exclude.Any) > 0 && len(exclude.All) > 0 {
			return "match.", fmt.Errorf("can't specify any and all together")
		}
	}

	if (len(rule.MatchResources.Any) > 0 || len(rule.MatchResources.All) > 0) && !datautils.DeepEqual(rule.MatchResources.ResourceDescription, kyvernov1.ResourceDescription{}) {
		return "match.", fmt.Errorf("can't specify any/all together with match resources")
	}

	if len(rule.MatchResources.Any) > 0 {
		for _, rmr := range rule.MatchResources.Any {
			// matched resources
			if path, err := validateMatchedResourceDescription(rmr.ResourceDescription); err != nil {
				return fmt.Sprintf("match.resources.%s", path), err
			}
		}
	} else if len(rule.MatchResources.All) > 0 {
		for _, rmr := range rule.MatchResources.All {
			// matched resources
			if path, err := validateMatchedResourceDescription(rmr.ResourceDescription); err != nil {
				return fmt.Sprintf("match.resources.%s", path), err
			}
		}
	} else {
		// matched resources
		if path, err := validateMatchedResourceDescription(rule.MatchResources.ResourceDescription); err != nil {
			return fmt.Sprintf("match.resources.%s", path), err
		}
	}

	// validating the values present under validate.preconditions, if they exist
	if target := rule.GetAnyAllConditions(); target != nil {
		if path, err := validateConditions(target, "preconditions"); err != nil {
			return fmt.Sprintf("validate.%s", path), err
		}
		if path, err := validateRawJSONConditionOperator(target, "preconditions"); err != nil {
			return fmt.Sprintf("validate.%s", path), err
		}
	}
	// validating the values present under validate.conditions, if they exist
	if rule.Validation != nil {
		if rule.Validation.Deny != nil {
			if target := rule.Validation.Deny.GetAnyAllConditions(); target != nil {
				if path, err := validateConditions(target, "conditions"); err != nil {
					return fmt.Sprintf("validate.deny.%s", path), err
				}
				if path, err := validateRawJSONConditionOperator(target, "conditions"); err != nil {
					return fmt.Sprintf("validate.deny.%s", path), err
				}
			}
		}
		if len(rule.Validation.ForEachValidation) != 0 {
			if path, err := validateValidationForEach(rule.Validation.ForEachValidation, "validate.foreach"); err != nil {
				return path, err
			}
		}
	}

	if rule.Mutation != nil && len(rule.Mutation.ForEachMutation) != 0 {
		if path, err := validateMutationForEach(rule.Mutation.ForEachMutation, "mutate.foreach"); err != nil {
			return path, err
		}
	}

	if len(rule.VerifyImages) != 0 {
		for _, vi := range rule.VerifyImages {
			for _, att := range vi.Attestations {
				for _, c := range att.Conditions {
					if path, err := validateAnyAllConditionOperator(c, "conditions"); err != nil {
						return fmt.Sprintf("verifyImages.attestations.%s", path), err
					}
				}
			}
			if rule.HasValidateImageVerification() {
				if target := vi.Validation.Deny.GetAnyAllConditions(); target != nil {
					if path, err := validateConditions(target, "conditions"); err != nil {
						return fmt.Sprintf("imageVerify.validate.deny.%s", path), err
					}
					if path, err := validateRawJSONConditionOperator(target, "conditions"); err != nil {
						return fmt.Sprintf("imageVerify.validate.deny.%s", path), err
					}
				}
			}
		}
	}

	return "", nil
}

func validateValidationForEach(foreach []kyvernov1.ForEachValidation, schemaKey string) (string, error) {
	for _, fe := range foreach {
		if fe.AnyAllConditions != nil {
			if path, err := validateAnyAllConditionOperator(*fe.AnyAllConditions, "conditions"); err != nil {
				return fmt.Sprintf("%s.%s", schemaKey, path), err
			}
		}
		if fe.Deny != nil {
			if target := fe.Deny.GetAnyAllConditions(); target != nil {
				if path, err := validateRawJSONConditionOperator(target, "conditions"); err != nil {
					return fmt.Sprintf("%s.deny.%s", schemaKey, path), err
				}
			}
		}
		fev := fe.GetForEachValidation()
		if len(fev) > 0 {
			if path, err := validateValidationForEach(fev, schemaKey); err != nil {
				return fmt.Sprintf("%s.%s", schemaKey, path), err
			}
		}
	}
	return "", nil
}

func validateMutationForEach(foreach []kyvernov1.ForEachMutation, schemaKey string) (string, error) {
	for _, fe := range foreach {
		if fe.AnyAllConditions != nil {
			if path, err := validateAnyAllConditionOperator(*fe.AnyAllConditions, "conditions"); err != nil {
				return fmt.Sprintf("%s.%s", schemaKey, path), err
			}
		}
		fem := fe.GetForEachMutation()
		if len(fem) > 0 {
			if path, err := validateMutationForEach(fem, schemaKey); err != nil {
				return fmt.Sprintf("%s.%s", schemaKey, path), err
			}
		}
	}
	return "", nil
}

// validateConditions validates all the 'conditions' or 'preconditions' of a rule depending on the corresponding 'condition.key'.
// As of now, it is validating the 'value' field whether it contains the only allowed set of values or not when 'condition.key' is {{request.operation}}
// this is backwards compatible i.e. conditions can be provided in the old manner as well i.e. without 'any' or 'all'
func validateConditions(conditions any, schemaKey string) (string, error) {
	// Conditions can only exist under some specific keys of the policy schema
	allowedSchemaKeys := map[string]bool{
		"preconditions": true,
		"conditions":    true,
	}
	if !allowedSchemaKeys[schemaKey] {
		return schemaKey, fmt.Errorf("wrong schema key found for validating the conditions. Conditions can only occur under one of ['preconditions', 'conditions'] keys in the policy schema")
	}

	switch typedConditions := conditions.(type) {
	case kyvernov1.AnyAllConditions:
		// validating the conditions under 'any', if there are any
		if !datautils.DeepEqual(typedConditions, kyvernov1.AnyAllConditions{}) && typedConditions.AnyConditions != nil {
			for i, condition := range typedConditions.AnyConditions {
				if path, err := validateConditionValues(condition); err != nil {
					return fmt.Sprintf("%s.any[%d].%s", schemaKey, i, path), err
				}
			}
		}
		// validating the conditions under 'all', if there are any
		if !datautils.DeepEqual(typedConditions, kyvernov1.AnyAllConditions{}) && typedConditions.AllConditions != nil {
			for i, condition := range typedConditions.AllConditions {
				if path, err := validateConditionValues(condition); err != nil {
					return fmt.Sprintf("%s.all[%d].%s", schemaKey, i, path), err
				}
			}
		}

	case []kyvernov1.Condition: // backwards compatibility
		for i, condition := range typedConditions {
			if path, err := validateConditionValues(condition); err != nil {
				return fmt.Sprintf("%s[%d].%s", schemaKey, i, path), err
			}
		}
	}
	return "", nil
}

// validateConditionValues validates whether all the values under the 'value' field of a 'conditions' field
// are apt with respect to the provided 'condition.key'
func validateConditionValues(c kyvernov1.Condition) (string, error) {
	k := c.GetKey()
	v := c.GetValue()
	if k == nil || v == nil || c.Operator == "" {
		return "", fmt.Errorf("entered value of `key`, `value` or `operator` is missing or misspelled")
	}
	switch reflect.TypeOf(k).Kind() {
	case reflect.String:
		value, err := validateValuesKeyRequest(c)
		return value, err
	default:
		return "", nil
	}
}

func validateOperator(c kyvernov1.ConditionOperator) (string, error) {
	if !operator.IsOperatorValid(c) {
		return "", fmt.Errorf("entered value of `operator` is invalid. valid values: %+q", operator.GetAllConditionOperators())
	}
	return "", nil
}

func validateConditionOperator(c []kyvernov1.Condition, schemaKey string) (string, error) {
	allowedSchemaKeys := map[string]bool{
		"preconditions": true,
		"conditions":    true,
	}
	if !allowedSchemaKeys[schemaKey] {
		return schemaKey, fmt.Errorf("wrong schema key found for validating the conditions. Conditions can only occur under one of ['preconditions', 'conditions'] keys in the policy schema")
	}
	for i, condition := range c {
		if path, err := validateOperator(condition.Operator); err != nil {
			return fmt.Sprintf("%s[%d].%s", schemaKey, i, path), err
		}
	}
	return "", nil
}

func validateAnyAllConditionOperator(c kyvernov1.AnyAllConditions, schemaKey string) (string, error) {
	allowedSchemaKeys := map[string]bool{
		"preconditions": true,
		"conditions":    true,
	}
	if !allowedSchemaKeys[schemaKey] {
		return schemaKey, fmt.Errorf("wrong schema key found for validating the conditions. Conditions can only occur under one of ['preconditions', 'conditions'] keys in the policy schema")
	}
	if !datautils.DeepEqual(c, kyvernov1.AnyAllConditions{}) && c.AnyConditions != nil {
		for i, condition := range c.AnyConditions {
			if path, err := validateOperator(condition.Operator); err != nil {
				return fmt.Sprintf("%s.any[%d].%s", schemaKey, i, path), err
			}
		}
	}
	if !datautils.DeepEqual(c, kyvernov1.AnyAllConditions{}) && c.AllConditions != nil {
		for i, condition := range c.AllConditions {
			if path, err := validateOperator(condition.Operator); err != nil {
				return fmt.Sprintf("%s.any[%d].%s", schemaKey, i, path), err
			}
		}
	}
	return "", nil
}

func validateRawJSONConditionOperator(c any, schemaKey string) (string, error) {
	allowedSchemaKeys := map[string]bool{
		"preconditions": true,
		"conditions":    true,
	}
	if !allowedSchemaKeys[schemaKey] {
		return schemaKey, fmt.Errorf("wrong schema key found for validating the conditions. Conditions can only occur under one of ['preconditions', 'conditions'] keys in the policy schema")
	}

	switch typedConditions := c.(type) {
	case kyvernov1.AnyAllConditions:
		if path, err := validateAnyAllConditionOperator(typedConditions, schemaKey); err != nil {
			return path, err
		}
	case []kyvernov1.Condition: // backwards compatibility
		if path, err := validateConditionOperator(typedConditions, schemaKey); err != nil {
			return path, err
		}
	}
	return "", nil
}

func validateValuesKeyRequest(c kyvernov1.Condition) (string, error) {
	k := c.GetKey()
	switch strings.ReplaceAll(k.(string), " ", "") {
	case "{{request.operation}}":
		return validateConditionValuesKeyRequestOperation(c)
	default:
		return "", nil
	}
}

// validateConditionValuesKeyRequestOperation validates whether all the values under the 'value' field of a 'conditions' field
// are one of ["CREATE", "UPDATE", "DELETE", "CONNECT"] when 'condition.key' is {{request.operation}}
func validateConditionValuesKeyRequestOperation(c kyvernov1.Condition) (string, error) {
	valuesAllowed := map[string]bool{
		"CREATE":  true,
		"UPDATE":  true,
		"DELETE":  true,
		"CONNECT": true,
	}
	v := c.GetValue()
	switch reflect.TypeOf(v).Kind() {
	case reflect.String:
		valueStr := v.(string)
		// allow templatized values like {{ config-map.data.sample-key }}
		// because they might be actually pointing to a rightful value in the provided config-map
		if len(valueStr) >= 4 && valueStr[:2] == "{{" && valueStr[len(valueStr)-2:] == "}}" {
			return "", nil
		}
		if !valuesAllowed[valueStr] {
			return fmt.Sprintf("value: %s", v.(string)), fmt.Errorf("unknown value '%s' found under the 'value' field. Only the following values are allowed: [CREATE, UPDATE, DELETE, CONNECT]", v.(string))
		}
	case reflect.Slice:
		values := reflect.ValueOf(v)
		for i := 0; i < values.Len(); i++ {
			value := values.Index(i).Interface().(string)
			if !valuesAllowed[value] {
				return fmt.Sprintf("value[%d]", i), fmt.Errorf("unknown value '%s' found under the 'value' field. Only the following values are allowed: [CREATE, UPDATE, DELETE, CONNECT]", value)
			}
		}
	default:
		return "value", fmt.Errorf("'value' field found to be of the type %v. The provided value/values are expected to be either in the form of a string or list", reflect.TypeOf(v).Kind())
	}
	return "", nil
}

func validateRuleContext(rule kyvernov1.Rule) error {
	if len(rule.Context) == 0 {
		return nil
	}

	for _, entry := range rule.Context {
		if entry.Name == "" {
			return fmt.Errorf("a name is required for context entries")
		}
		// if it the rule uses kyverno-json we add some constraints on the name of context entries to make
		// sure we can create the corresponding bindings
		if rule.Validation != nil && rule.Validation.Assert.Value != nil {
			if !bindingIdentifier.MatchString(entry.Name) {
				return fmt.Errorf("context entry name %s is invalid, it must be a single word when the validation rule uses `assert`", entry.Name)
			}
		}
		for _, v := range []string{"images", "request", "serviceAccountName", "serviceAccountNamespace", "element", "elementIndex"} {
			if entry.Name == v || strings.HasPrefix(entry.Name, v+".") {
				return fmt.Errorf("entry name %s is invalid as it conflicts with a pre-defined variable %s", entry.Name, v)
			}
		}

		var err error
		if entry.ConfigMap != nil && entry.APICall == nil && entry.GlobalReference == nil && entry.ImageRegistry == nil && entry.Variable == nil {
			err = validateConfigMap(entry)
		} else if entry.ConfigMap == nil && entry.APICall != nil && entry.GlobalReference == nil && entry.ImageRegistry == nil && entry.Variable == nil {
			err = validateAPICall(entry)
		} else if entry.ConfigMap == nil && entry.APICall == nil && entry.GlobalReference != nil && entry.ImageRegistry == nil && entry.Variable == nil {
			err = validateGlobalReference(entry)
		} else if entry.ConfigMap == nil && entry.APICall == nil && entry.GlobalReference == nil && entry.ImageRegistry != nil && entry.Variable == nil {
			err = validateImageRegistry(entry)
		} else if entry.ConfigMap == nil && entry.APICall == nil && entry.GlobalReference == nil && entry.ImageRegistry == nil && entry.Variable != nil {
			err = validateVariable(entry)
		} else {
			return fmt.Errorf("exactly one of configMap or apiCall or imageRegistry or variable is required for context entries")
		}

		if err != nil {
			return err
		}
	}
	return nil
}

// validateRuleImageExtractorsJMESPath ensures that the rule does not
// mutate image digests if it has an image extractor that uses a JMESPath.
func validateRuleImageExtractorsJMESPath(rule kyvernov1.Rule) error {
	imageExtractorConfigs := rule.ImageExtractors
	imageVerifications := rule.VerifyImages
	if imageExtractorConfigs == nil || imageVerifications == nil {
		return nil
	}

	anyMutateDigest := false
	for _, imageVerification := range imageVerifications {
		if imageVerification.MutateDigest {
			anyMutateDigest = true
			break
		}
	}

	if !anyMutateDigest {
		return nil
	}

	anyJMESPath := false
	for _, imageExtractors := range imageExtractorConfigs {
		for _, imageExtractor := range imageExtractors {
			if imageExtractor.JMESPath != "" {
				anyJMESPath = true
				break
			}
		}
	}

	if anyJMESPath {
		return fmt.Errorf("jmespath may not be used in an image extractor when mutating digests with verify images")
	}

	return nil
}

func validateVariable(entry kyvernov1.ContextEntry) error {
	// If JMESPath contains variables, the validation will fail because it's not possible to infer which value
	// will be inserted by the variable
	// Skip validation if a variable is detected
	jmesPath := variables.ReplaceAllVars(entry.Variable.JMESPath, func(s string) string { return "kyvernojmespathvariable" })
	if !strings.Contains(jmesPath, "kyvernojmespathvariable") && entry.Variable.JMESPath != "" {
		if _, err := jmespath.NewParser().Parse(entry.Variable.JMESPath); err != nil {
			return fmt.Errorf("failed to parse JMESPath %s: %v", entry.Variable.JMESPath, err)
		}
	}
	if entry.Variable.GetValue() == nil && jmesPath == "" {
		return fmt.Errorf("a variable must define a value or a jmesPath expression")
	}
	if entry.Variable.GetDefault() != nil && jmesPath == "" {
		return fmt.Errorf("a variable must define a default value only when a jmesPath expression is defined")
	}
	return nil
}

func validateConfigMap(entry kyvernov1.ContextEntry) error {
	if entry.ConfigMap.Name == "" {
		return fmt.Errorf("a name is required for configMap context entry")
	}

	if entry.ConfigMap.Namespace == "" {
		return fmt.Errorf("a namespace is required for configMap context entry")
	}

	return nil
}

func validateAPICall(entry kyvernov1.ContextEntry) error {
	if entry.APICall == nil {
		return nil
	}

	if entry.APICall.URLPath != "" {
		if entry.APICall.Service != nil {
			return fmt.Errorf("a URLPath cannot be used for service API calls")
		}
	}

	// If JMESPath contains variables, the validation will fail because it's not
	// possible to infer which value will be inserted by the variable
	// Skip validation if a variable is detected

	jmesPath := variables.ReplaceAllVars(entry.APICall.JMESPath, func(s string) string { return "kyvernojmespathvariable" })

	if !strings.Contains(jmesPath, "kyvernojmespathvariable") && entry.APICall.JMESPath != "" {
		if _, err := jmespath.NewParser().Parse(entry.APICall.JMESPath); err != nil {
			return fmt.Errorf("failed to parse JMESPath %s: %v", entry.APICall.JMESPath, err)
		}
	}

	return nil
}

func validateGlobalReference(entry kyvernov1.ContextEntry) error {
	if entry.GlobalReference == nil {
		return nil
	}

	// If JMESPath contains variables, the validation will fail because it's not
	// possible to infer which value will be inserted by the variable
	// Skip validation if a variable is detected

	jmesPath := variables.ReplaceAllVars(entry.GlobalReference.JMESPath, func(s string) string { return "kyvernojmespathvariable" })

	if !strings.Contains(jmesPath, "kyvernojmespathvariable") && entry.GlobalReference.JMESPath != "" {
		if _, err := jmespath.NewParser().Parse(entry.GlobalReference.JMESPath); err != nil {
			return fmt.Errorf("failed to parse JMESPath %s: %v", entry.GlobalReference.JMESPath, err)
		}
	}

	return nil
}

func validateImageRegistry(entry kyvernov1.ContextEntry) error {
	if entry.ImageRegistry.Reference == "" {
		return fmt.Errorf("a ref is required for imageRegistry context entry")
	}
	// Replace all variables to prevent validation failing on variable keys.
	ref := variables.ReplaceAllVars(entry.ImageRegistry.Reference, func(s string) string { return "kyvernoimageref" })

	// it's no use validating a reference that contains a variable
	if !strings.Contains(ref, "kyvernoimageref") {
		_, err := reference.Parse(ref)
		if err != nil {
			return fmt.Errorf("bad image: %s: %w", ref, err)
		}
	}

	// If JMESPath contains variables, the validation will fail because it's not possible to infer which value
	// will be inserted by the variable
	// Skip validation if a variable is detected
	jmesPath := variables.ReplaceAllVars(entry.ImageRegistry.JMESPath, func(s string) string { return "kyvernojmespathvariable" })

	if !strings.Contains(jmesPath, "kyvernojmespathvariable") && entry.ImageRegistry.JMESPath != "" {
		if _, err := jmespath.NewParser().Parse(entry.ImageRegistry.JMESPath); err != nil {
			return fmt.Errorf("failed to parse JMESPath %s: %v", entry.ImageRegistry.JMESPath, err)
		}
	}

	return nil
}

// validateResourceDescription checks if all necessary fields are present and have values. Also checks a Selector.
// field type is checked through openapi
// Returns error if
// - kinds is empty array in matched resource block, i.e. kinds: []
// - selector is invalid
func validateMatchedResourceDescription(rd kyvernov1.ResourceDescription) (string, error) {
	if datautils.DeepEqual(rd, kyvernov1.ResourceDescription{}) {
		return "", fmt.Errorf("match resources not specified")
	}

	return "", nil
}

// jsonPatchOnPod checks if a rule applies JSON patches to Pod
func jsonPatchOnPod(rule kyvernov1.Rule) bool {
	if !rule.HasMutate() {
		return false
	}

	if slices.Contains(rule.MatchResources.Kinds, "Pod") && rule.Mutation != nil && rule.Mutation.PatchesJSON6902 != "" {
		return true
	}

	return false
}

func podControllerAutoGenExclusion(policy kyvernov1.PolicyInterface) bool {
	annotations := policy.GetAnnotations()
	val, ok := annotations[kyverno.AnnotationAutogenControllers]
	if !ok || val == "none" {
		return false
	}

	reorderVal := strings.Split(strings.ToLower(val), ",")
	sort.Slice(reorderVal, func(i, j int) bool { return reorderVal[i] < reorderVal[j] })
	if ok && !datautils.DeepEqual(reorderVal, []string{"cronjob", "daemonset", "deployment", "job", "statefulset"}) {
		return true
	}
	return false
}

func validateKinds(kinds []string, rule kyvernov1.Rule, mock, background bool, client dclient.Interface) error {
	if err := validateWildcard(kinds, background, rule); err != nil {
		return err
	}

	if slices.Contains(kinds, "*") {
		return nil
	}

	if err := validKinds(kinds, mock, background, rule.HasValidate(), client); err != nil {
		return fmt.Errorf("the kind defined in the all match resource is invalid: %w", err)
	}
	return nil
}

// validateWildcard check for an Match/Exclude block contains "*"
func validateWildcard(kinds []string, background bool, rule kyvernov1.Rule) error {
	if slices.Contains(kinds, "*") && background {
		return fmt.Errorf("wildcard policy not allowed in background mode. Set spec.background=false to disable background mode for this policy rule ")
	}
	if slices.Contains(kinds, "*") && len(kinds) > 1 {
		return fmt.Errorf("wildcard policy can not deal with more than one kind")
	}
	if slices.Contains(kinds, "*") {
		if rule.HasGenerate() || rule.HasVerifyImages() || (rule.Validation != nil && rule.Validation.ForEachValidation != nil) {
			return fmt.Errorf("wildcard policy does not support rule type")
		}

		if rule.HasValidate() {
			if rule.Validation.GetPattern() != nil || rule.Validation.GetAnyPattern() != nil {
				if !ruleOnlyDealsWithResourceMetaData(rule) {
					return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
						" the rule does not match any kind")
				}
			}

			if rule.Validation.Deny != nil {
				switch typedConditions := rule.Validation.Deny.GetAnyAllConditions().(type) {
				case []kyvernov1.Condition: // backwards compatibility
					for _, condition := range typedConditions {
						key := condition.GetKey()
						if !strings.Contains(key.(string), "request.object.metadata.") && (!wildCardAllowedVariables.MatchString(key.(string)) || strings.Contains(key.(string), "request.object.spec")) {
							return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
								" the rule does not match any kind")
						}
					}
				}
			}
		}

		if rule.HasMutate() {
			if !ruleOnlyDealsWithResourceMetaData(rule) {
				return fmt.Errorf("policy can only deal with the metadata field of the resource if" +
					" the rule does not match any kind")
			}
		}
	}
	return nil
}

// validKinds verifies if an API resource that matches 'kind' is valid kind
// and found in the cache, returns error if not found. It also returns an error if background scanning
// is enabled for a subresource.
func validKinds(kinds []string, mock, backgroundScanningEnabled, isValidationPolicy bool, client dclient.Interface) error {
	if !mock {
		for _, k := range kinds {
			group, version, kind, subresource := kubeutils.ParseKindSelector(k)
			gvrss, err := client.Discovery().FindResources(group, version, kind, subresource)
			if err != nil {
				return fmt.Errorf("unable to convert GVK to GVR for kinds %s, err: %s", k, err)
			}
			if len(gvrss) == 0 {
				return fmt.Errorf("unable to convert GVK to GVR for kinds %s", k)
			}
			if isValidationPolicy && backgroundScanningEnabled {
				for gvrs := range gvrss {
					if gvrs.SubResource != "" {
						return fmt.Errorf("background scan enabled with subresource %s", k)
					}
				}
			}
		}
	}
	return nil
}

func validateWildcardsWithNamespaces(enforce, audit, enforceW, auditW []string) error {
	pat, ns, notOk := wildcard.MatchPatterns(auditW, enforce...)
	if notOk {
		return fmt.Errorf("wildcard pattern '%s' matches with namespace '%s'", pat, ns)
	}
	pat, ns, notOk = wildcard.MatchPatterns(enforceW, audit...)
	if notOk {
		return fmt.Errorf("wildcard pattern '%s' matches with namespace '%s'", pat, ns)
	}
	pat1, pat2, notOk := wildcard.MatchPatterns(auditW, enforceW...)
	if notOk {
		return fmt.Errorf("wildcard pattern '%s' conflicts with the pattern '%s'", pat1, pat2)
	}
	pat1, pat2, notOk = wildcard.MatchPatterns(enforceW, auditW...)
	if notOk {
		return fmt.Errorf("wildcard pattern '%s' conflicts with the pattern '%s'", pat1, pat2)
	}
	return nil
}

func validateNamespaces(validationFailureActionOverrides []kyvernov1.ValidationFailureActionOverride, path *field.Path) error {
	action := map[string]sets.Set[string]{
		"enforce":  sets.New[string](),
		"audit":    sets.New[string](),
		"enforceW": sets.New[string](),
		"auditW":   sets.New[string](),
	}

	for i, vfa := range validationFailureActionOverrides {
		if !vfa.Action.IsValid() {
			return fmt.Errorf("invalid action")
		}
		patternList, nsList := wildcard.SeperateWildcards(vfa.Namespaces)

		if vfa.Action.Audit() {
			if action["enforce"].HasAny(nsList...) {
				return fmt.Errorf("conflicting namespaces found in path: %s: %s", path.Index(i).Child("namespaces").String(),
					strings.Join(sets.List(action["enforce"].Intersection(sets.New(nsList...))), ", "))
			}
			action["auditW"].Insert(patternList...)
		} else if vfa.Action.Enforce() {
			if action["audit"].HasAny(nsList...) {
				return fmt.Errorf("conflicting namespaces found in path: %s: %s", path.Index(i).Child("namespaces").String(),
					strings.Join(sets.List(action["audit"].Intersection(sets.New(nsList...))), ", "))
			}
			action["enforceW"].Insert(patternList...)
		}
		action[strings.ToLower(string(vfa.Action))].Insert(nsList...)

		err := validateWildcardsWithNamespaces(
			sets.List(action["enforce"]),
			sets.List(action["audit"]),
			sets.List(action["enforceW"]),
			sets.List(action["auditW"]),
		)
		if err != nil {
			return fmt.Errorf("path: %s: %s", path.Index(i).Child("namespaces").String(), err.Error())
		}
	}

	return nil
}

func checkForScaleSubresource(ruleTypeJson []byte, allKinds []string, warnings *[]string) {
	if strings.Contains(string(ruleTypeJson), "replicas") {
		for _, kind := range allKinds {
			if strings.Contains(strings.ToLower(kind), "scale") {
				return
			}
		}
		msg := "You are matching on replicas but not including the scale subresource in the policy."
		*warnings = append(*warnings, msg)
	}
}

func checkForStatusSubresource(ruleTypeJson []byte, allKinds []string, warnings *[]string) {
	rule := string(ruleTypeJson)
	if strings.Contains(rule, ".status") || strings.Contains(rule, "\"status\":") {
		for _, kind := range allKinds {
			if strings.Contains(strings.ToLower(kind), "status") {
				return
			}
		}
		msg := "You are matching on status but not including the status subresource in the policy."
		*warnings = append(*warnings, msg)
	}
}

func checkForDeprecatedFieldsInVerifyImages(rule kyvernov1.Rule, warnings *[]string) {
	for _, imageVerify := range rule.VerifyImages {
		for _, attestation := range imageVerify.Attestations {
			if attestation.PredicateType != "" {
				msg := fmt.Sprintf("predicateType has been deprecated use 'type: %s' instead of 'predicateType: %s'", attestation.PredicateType, attestation.PredicateType)
				*warnings = append(*warnings, msg)
			}
		}
	}
}

func checkDeprecatedOperator(c kyvernov1.ConditionOperator) string {
	if operator.IsOperatorDeprecated(c) {
		return fmt.Sprintf("Operator %s has been deprecated and will be removed soon. Use these instead: %+q", string(c), operator.GetDeprecatedOperatorAlternative(string(c)))
	}
	return ""
}

func checkDeprecatedConditionOperator(c []kyvernov1.Condition, warnings *[]string) {
	for _, condition := range c {
		if warn := checkDeprecatedOperator(condition.Operator); len(warn) > 0 {
			*warnings = append(*warnings, warn)
		}
	}
}

func checkDeprecatedAnyAllConditionOperator(c kyvernov1.AnyAllConditions, warnings *[]string) {
	if !datautils.DeepEqual(c, kyvernov1.AnyAllConditions{}) && c.AnyConditions != nil {
		for _, condition := range c.AnyConditions {
			if warn := checkDeprecatedOperator(condition.Operator); len(warn) > 0 {
				*warnings = append(*warnings, warn)
			}
		}
	}
	if !datautils.DeepEqual(c, kyvernov1.AnyAllConditions{}) && c.AllConditions != nil {
		for _, condition := range c.AllConditions {
			if warn := checkDeprecatedOperator(condition.Operator); len(warn) > 0 {
				*warnings = append(*warnings, warn)
			}
		}
	}
}

func checkDeprecatedRawJSONConditionOperator(c apiextensions.JSON, warnings *[]string) {
	switch typedConditions := c.(type) {
	case kyvernov1.AnyAllConditions:
		checkDeprecatedAnyAllConditionOperator(typedConditions, warnings)
	case []kyvernov1.Condition: // backwards compatibility
		checkDeprecatedConditionOperator(typedConditions, warnings)
	}
}

func checkForDeprecatedOperatorsInRule(rule kyvernov1.Rule, warnings *[]string) {
	if rule.Validation != nil {
		if rule.Validation.Deny != nil {
			if target := rule.Validation.Deny.GetAnyAllConditions(); target != nil {
				checkDeprecatedRawJSONConditionOperator(target, warnings)
			}
		}
		if len(rule.Validation.ForEachValidation) != 0 {
			for _, fe := range rule.Validation.ForEachValidation {
				if fe.AnyAllConditions != nil {
					checkDeprecatedAnyAllConditionOperator(*fe.AnyAllConditions, warnings)
				}
				if fe.Deny != nil {
					if target := fe.Deny.GetAnyAllConditions(); target != nil {
						checkDeprecatedRawJSONConditionOperator(target, warnings)
					}
				}
			}
		}
	}
	if len(rule.VerifyImages) != 0 {
		for _, vi := range rule.VerifyImages {
			for _, att := range vi.Attestations {
				for _, c := range att.Conditions {
					checkDeprecatedAnyAllConditionOperator(c, warnings)
				}
			}
		}
	}
}

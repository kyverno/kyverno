package audit

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type object interface {
	kyvernov1alpha2.ReportChangeRequest | kyvernov1alpha2.ClusterReportChangeRequest
}

type pointer[T any] interface {
	controllerutils.Object[T]
	GetResults() []policyreportv1alpha2.PolicyReportResult
	SetResults([]policyreportv1alpha2.PolicyReportResult)
	SetSummary(policyreportv1alpha2.PolicyReportSummary)
}

func reconcileReport[T object, R pointer[T], G controllerutils.Getter[R], S controllerutils.Setter[R]](c *controller, name string, getter G, setter S) error {
	// fetch report, if not found is not an error
	report, err := getter.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}
	namespace := report.GetNamespace()
	// load all policies
	policies, err := c.fetchClusterPolicies(logger)
	if err != nil {
		return err
	}
	if namespace != "" {
		pols, err := c.fetchPolicies(logger, namespace)
		if err != nil {
			return err
		}
		policies = append(policies, pols...)
	}
	// load background policies
	backgroundPolicies := removeNonBackgroundPolicies(logger, policies...)
	// build label/policy maps
	labelPolicyMap := map[string]kyvernov1.PolicyInterface{}
	for _, policy := range policies {
		labelPolicyMap[policyLabel(policy)] = policy
	}
	labelBackgroundPolicyMap := map[string]kyvernov1.PolicyInterface{}
	for _, policy := range backgroundPolicies {
		labelBackgroundPolicyMap[policyLabel(policy)] = policy
	}
	// update report
	_, err = controllerutils.Update(setter, report,
		func(report R) error {
			labels := controllerutils.SetLabel(report, kyvernov1.ManagedByLabel, kyvernov1.KyvernoAppValue)
			// check report policies versions against policies version
			toDelete := map[string]string{}
			var toCreate []kyvernov1.PolicyInterface
			for label := range labels {
				if isPolicyLabel(label) {
					// if the policy doesn't exist anymore
					if labelPolicyMap[label] == nil {
						if name, err := policyNameFromLabel(namespace, label); err != nil {
							return err
						} else {
							toDelete[name] = label
						}
					}
				}
			}
			for label, policy := range labelBackgroundPolicyMap {
				// if the background policy changed, we need to recreate entries
				if labels[label] != policy.GetResourceVersion() {
					if name, err := policyNameFromLabel(namespace, label); err != nil {
						return err
					} else {
						toDelete[name] = label
					}
					toCreate = append(toCreate, policy)
				}
			}
			// deletions
			for _, label := range toDelete {
				delete(labels, label)
			}
			var ruleResults []policyreportv1alpha2.PolicyReportResult
			for _, result := range report.GetResults() {
				if _, ok := toDelete[result.Policy]; !ok {
					ruleResults = append(ruleResults, result)
				}
			}
			// creations
			if len(toCreate) > 0 {
				scanner := NewScanner(logger, c.client)
				owner := report.GetOwnerReferences()[0]
				resource, err := c.client.GetResource(owner.APIVersion, owner.Kind, report.GetNamespace(), owner.Name)
				if err != nil {
					return err
				}
				var nsLabels map[string]string
				if namespace != "" {
					ns, err := c.nsLister.Get(namespace)
					if err != nil {
						return err
					}
					nsLabels = ns.GetLabels()
				}
				for _, result := range scanner.Scan(*resource, nsLabels, toCreate...) {
					if result.Error != nil {
						return err
					} else {
						controllerutils.SetLabel(report, policyLabel(result.EngineResponse.Policy), result.EngineResponse.Policy.GetResourceVersion())
						ruleResults = append(ruleResults, toReportResults(result)...)
					}
				}
			}
			// update results and summary
			report.SetResults(ruleResults)
			report.SetSummary(CalculateSummary(ruleResults))
			return nil
		},
	)
	if err != nil {
		logger.Error(err, "failed to create or update rcr")
	}
	return nil
}

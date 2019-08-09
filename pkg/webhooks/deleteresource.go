package webhooks

import (
	v1beta1 "k8s.io/api/admission/v1beta1"
)

func (ws *WebhookServer) removePolicyViolation(request *v1beta1.AdmissionRequest) error {
	//TODO: ClenUp will be handled by the policycontroller

	// // Get the list of policies that apply on the resource
	// policies, err := ws.policyLister.List(labels.NewSelector())
	// if err != nil {
	// 	// Unable to connect to policy Lister to access policies
	// 	return errors.New("Unable to connect to policy controller to access policies. Clean Up of Policy Violations is not being done")
	// }
	// for _, policy := range policies {
	// 	// check if policy has a rule for the admission request kind
	// 	if !StringInSlice(request.Kind.Kind, getApplicableKindsForPolicy(policy)) {
	// 		continue
	// 	}
	// 	// get the details from the request
	// 	rname := request.Name
	// 	rns := request.Namespace
	// 	rkind := request.Kind.Kind
	// 	// check if the resource meets the policy Resource description
	// 	for _, rule := range policy.Spec.Rules {
	// 		ok := engine.ResourceMeetsDescription(request.Object.Raw, rule.MatchResources.ResourceDescription, rule.ExcludeResources.ResourceDescription, request.Kind)
	// 		if ok {
	// 			// Check if the policy has a violation for this resource
	// 			err := ws.violationBuilder.ResourceRemoval(policy.Name, rkind, rns, rname)
	// 			if err != nil {
	// 				return err
	// 			}
	// 		}
	// 	}
	// }
	return nil
}

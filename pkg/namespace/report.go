package namespace

//func (nsc *NamespaceController) report(engineResponses []response.EngineResponse) {
//	// generate events
//	eventInfos := generateEvents(engineResponses)
//	nsc.eventGen.Add(eventInfos...)
//	// generate policy violations
//	pvInfos := policyviolation.GeneratePVsFromEngineResponse(engineResponses)
//	nsc.pvGenerator.Add(pvInfos...)
//}
//
//func generateEvents(ers []response.EngineResponse) []event.Info {
//	var eventInfos []event.Info
//	for _, er := range ers {
//		if er.IsSuccesful() {
//			continue
//		}
//		eventInfos = append(eventInfos, generateEventsPerEr(er)...)
//	}
//	return eventInfos
//}
//
//func generateEventsPerEr(er response.EngineResponse) []event.Info {
//	var eventInfos []event.Info
//	glog.V(4).Infof("reporting results for policy '%s' application on resource '%s/%s/%s'", er.PolicyResponse.Policy, er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
//	for _, rule := range er.PolicyResponse.Rules {
//		if rule.Success {
//			continue
//		}
//		// generate event on resource for each failed rule
//		glog.V(4).Infof("generation event on resource '%s/%s' for policy '%s'", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Name, er.PolicyResponse.Policy)
//		e := event.Info{}
//		e.Kind = er.PolicyResponse.Resource.Kind
//		e.Namespace = "" // event generate on namespace resource
//		e.Name = er.PolicyResponse.Resource.Name
//		e.Reason = "Failure"
//		e.Source = event.GeneratePolicyController
//		e.Message = fmt.Sprintf("policy '%s' (%s) rule '%s' not satisfied. %v", er.PolicyResponse.Policy, rule.Type, rule.Name, rule.Message)
//		eventInfos = append(eventInfos, e)
//	}
//	if er.IsSuccesful() {
//		return eventInfos
//	}
//	// generate a event on policy for all failed rules
//	glog.V(4).Infof("generation event on policy '%s'", er.PolicyResponse.Policy)
//	e := event.Info{}
//	e.Kind = "ClusterPolicy"
//	e.Namespace = ""
//	e.Name = er.PolicyResponse.Policy
//	e.Reason = "Failure"
//	e.Source = event.GeneratePolicyController
//	e.Message = fmt.Sprintf("policy '%s' rules '%v' on resource '%s/%s/%s' not stasified", er.PolicyResponse.Policy, er.GetFailedRules(), er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
//	return eventInfos
//}

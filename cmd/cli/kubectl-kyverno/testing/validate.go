package testing

import (
	"encoding/json"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

func validation(tests *kyvernov1.Test_manifest, isGit bool, policyResourcePath string, policyBytes string) error {
	d := json.NewDecoder(strings.NewReader(policyBytes))
	d.DisallowUnknownFields()
	if err := d.Decode(&kyvernov1.Test_manifest{}); err != nil {
		return fmt.Errorf("error : %v", err)
	}
	// var fs billy.Filesystem
	// var dClient dclient.Interface
	// var rf bool
	// var pf bool
	// var cf bool
	// var gf bool
	// var n bool
	// var pof bool
	// var rof bool
	// var rpf bool
	// var repf bool
	// var matchname bool

	// resourceFullPath := make(map[string][]string)

	// nm := regexp.MustCompile(`^[a-z]([a-z0-9\-]*[a-z])?$`)

	if tests.TypeMeta.APIVersion == "" {
		return fmt.Errorf("test execution failed because apiversion is empty")
	}
	// if tests.TypeMeta.APIVersion != "cli.kyverno.io/v1beta1" {
	// 	return fmt.Errorf("test execution failed because apiversion value is not correct. Correct format `apiVersion: cli.kyverno.io/v1beta1`")
	// }
	// if tests.TypeMeta.Kind == "" {
	// 	return fmt.Errorf("test execution failed because kind is empty")
	// } else if tests.TypeMeta.Kind != "KyvernoTest" {
	// 	return fmt.Errorf("test execution failed because the value of kind is not correct, it should be `KyvernoTest`")
	// }
	// if tests.Metadata.Name == "" {
	// 	return fmt.Errorf("test execution failed because metadata.name is empty")
	// }
	// matchname = nm.MatchString(tests.Metadata.Name)
	// if !matchname {
	// 	return fmt.Errorf("test execution failed because Metadata.Name is not valid")
	// }
	// if len(tests.Metadata.Labels) > 0 {
	// 	for k := range tests.Metadata.Labels {
	// 		if tests.Metadata.Labels[k] == "" {
	// 			return fmt.Errorf("test execution failed because metadata.labels.%v does not have a value", k)
	// 		}
	// 	}
	// }

	// if len(tests.Metadata.Annotations) > 0 {
	// 	for k := range tests.Metadata.Annotations {
	// 		if tests.Metadata.Annotations[k] == "" {
	// 			return fmt.Errorf("test execution failed because metadata.annotations.%v does not have a value", k)
	// 		}
	// 	}
	// }

	// r := regexp.MustCompile(`(((\.\.)(/))?)*(/)?(([a-zA-Z0-9]+)(/))*([a-zA-Z0-9])(\.yaml)$`)
	// if len(tests.Spec.Policies) < 1 {
	// 	return fmt.Errorf("test execution failed because spec.policies is empty")
	// } else {
	// 	for kp, p := range tests.Spec.Policies {

	// 		match := r.MatchString(p)
	// 		if !match {
	// 			return fmt.Errorf("test execution failed because spec.policies[%v] is not a correct path to yaml file", kp)
	// 		}
	// 	}
	// }

	// if len(tests.Spec.Resources) < 1 {
	// 	return fmt.Errorf("test execution failed because spec.resources is empty")
	// } else {
	// 	for kr := range tests.Spec.Resources {
	// 		if len(tests.Spec.Resources[kr]) < 1 {
	// 			return fmt.Errorf("test execution failed because spec.resources.%v is empty", kr)
	// 		}
	// 		for k, p := range tests.Spec.Resources[kr] {
	// 			match := r.MatchString(p)
	// 			if !match {
	// 				return fmt.Errorf("test execution failed because spec.resources.%v.[%v] is not a correct path to yaml file", kr, k)
	// 			}
	// 		}
	// 	}
	// }

	// if len(tests.Spec.Results) < 1 {
	// 	return fmt.Errorf("test execution failed because spec.results is empty")
	// }
	// for k := range tests.Spec.Resources {
	// 	resourceFullPath[k] = getFullPath(tests.Spec.Resources[k], policyResourcePath, isGit, "resource")
	// }
	// policyFullPath := getFullPath(tests.Spec.Policies, policyResourcePath, isGit, "policy")
	// policies, err := common.GetPoliciesFromPaths(fs, policyFullPath, isGit, policyResourcePath)
	// if err != nil {
	// 	fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
	// 	os.Exit(1)
	// }
	// filteredPolicies := []kyvernov1.PolicyInterface{}
	// for _, p := range policies {
	// 	for _, res := range tests.Spec.Results {
	// 		if p.GetName() == res.Policy {
	// 			filteredPolicies = append(filteredPolicies, p)
	// 			break
	// 		}
	// 	}
	// }
	// policies = filteredPolicies

	// mutatedPolicies, err := common.MutatePolicies(policies)
	// if err != nil {
	// 	return fmt.Errorf("failed to mutate policy, error : %v", err)
	// }
	// allresources := make(map[string][]*unstructured.Unstructured)
	// resourcesMap := make(map[string][]*unstructured.Unstructured)
	// for k := range tests.Spec.Resources {
	// 	allresources[k], err = common.GetResourceAccordingToResourcePath(fs, resourceFullPath[k], false, mutatedPolicies, dClient, "", false, isGit, policyResourcePath)
	// 	if err != nil {
	// 		return fmt.Errorf("error: failed to load resources\nCause: %s", err)
	// 	}
	// 	resourcesMap[k] = allresources[k]
	// }

	// for k, r := range tests.Spec.Results {
	// 	if r.Policy == "" {
	// 		return fmt.Errorf("test execution failed because spec.results[%v].policy is empty", k)
	// 	}
	// 	matchname = nm.MatchString(r.Policy)
	// 	if !matchname {
	// 		return fmt.Errorf("test execution failed because spec.results[%v].policy is not valid", k)
	// 	}
	// 	rpf = false
	// 	for _, p := range filteredPolicies {
	// 		if p.GetName() == r.Policy {
	// 			rpf = true
	// 			if r.Rule == "" {
	// 				return fmt.Errorf("test execution failed because spec.results[%v].rule is empty", k)
	// 			}
	// 			matchname = nm.MatchString(r.Rule)
	// 			if !matchname {
	// 				return fmt.Errorf("test execution failed because sspec.results[%v].rule is not valid", k)
	// 			}
	// 			repf = false
	// 			for _, rn := range p.GetSpec().Rules {
	// 				if rn.Name == r.Rule {
	// 					repf = true
	// 				}
	// 			}
	// 			if !repf {
	// 				return fmt.Errorf("test execution failed because spec.results[%v].rule: %v not found in the policy: %v", k, r.Rule, r.Policy)
	// 			}
	// 		}
	// 	}
	// 	if !rpf {
	// 		return fmt.Errorf("test execution failed because spec.results[%v].policy not found in spec.policies", k)
	// 	}
	// 	for re, res := range tests.Spec.Results {
	// 		for resk, testr := range res.Resources {
	// 			n = false
	// 			name := strings.FieldsFunc(testr.Object, Split)
	// 			if testr.Object == "" {
	// 				return fmt.Errorf("results[%v].resources[%v].object field is mandatory", re, resk)
	// 			}
	// 			// if len(name) < 5 || len(name) > 6 {
	// 			// 	return fmt.Errorf("results[%v].resources[%v].object field is not defined properly. ---> Correct format - object: my_resource_pool:apiversion/kind/group/namespace/name", re, resk)
	// 			// }
	// 			for k := range resourcesMap {
	// 				if k == name[0] {
	// 					n = true
	// 				}
	// 			}
	// 			if !n {
	// 				return fmt.Errorf("[%v] is not found defined under spec.resources", name[0])
	// 			}
	// 		}
	// 	}

	// 	for re, res := range tests.Spec.Results {
	// 		if len(res.Resources) < 1 {
	// 			return fmt.Errorf("results[%v].resources is found empty", re)
	// 		}
	// 		for resk, testr := range res.Resources {
	// 			rf = false
	// 			pf = false
	// 			cf = false
	// 			gf = false
	// 			name := strings.FieldsFunc(testr.Object, Split)
	// 			patched := strings.FieldsFunc(testr.Patched, Split)
	// 			clone := strings.FieldsFunc(testr.CloneSource, Split)
	// 			generated := strings.FieldsFunc(testr.Generated, Split)

	// 			// for _, r := range resourcesMap[name[0]] {
	// 			// 	if name[1] == r.GroupVersionKind().Version && name[2] == r.GetKind() && name[len(name)-2] == r.GetNamespace() && name[len(name)-1] == r.GetName() {
	// 			// 		if len(name) == 6 {
	// 			// 			if r.GroupVersionKind().Group == name[3] {
	// 			// 				rf = true
	// 			// 			}
	// 			// 		} else if r.GroupVersionKind().Group != "" {
	// 			// 			return fmt.Errorf("result[%v].resources[%v].object is not defined properly. ---> Correct format - object: my_resource_pool:apiversion/kind/group/namespace/name", re, resk)
	// 			// 		} else if len(name) == 5 {
	// 			// 			rf = true
	// 			// 		} else {
	// 			// 			return fmt.Errorf("result[%v].resources[%v].object is not defined properly. ---> Correct format - object: my_resource_pool:apiversion/kind/namespace/name", re, resk)
	// 			// 		}
	// 			// 	}
	// 			// }
	// 			if !rf {
	// 				if len(name) == 6 {
	// 					return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].object : %v:%v/%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4], name[5])
	// 				} else {
	// 					return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].object : %v:%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4])
	// 				}
	// 			}

	// 			for _, k := range filteredPolicies {
	// 				if k.GetName() == res.Policy {
	// 					if k.GetSpec().HasMutate() {
	// 						if len(patched) == 0 {
	// 							return fmt.Errorf("mutate rule detected but result[%v].resources[%v].patched is empty", re, resk)
	// 						}
	// 						if len(patched) < 5 || len(patched) > 6 {
	// 							return fmt.Errorf("results[%v].resources[%v].patched field is not defined properly. ---> Correct format - patched: patchedResource_pool:apiversion/kind/group/namespace/name", re, resk)
	// 						}
	// 						for _, r := range resourcesMap["patchedResource_pool"] {
	// 							if patched[1] == r.GroupVersionKind().Version && patched[2] == r.GetKind() && patched[len(patched)-2] == r.GetNamespace() && patched[len(patched)-1] == r.GetName() {
	// 								if len(patched) == 6 {
	// 									if r.GroupVersionKind().Group == patched[3] {
	// 										pf = true
	// 									}
	// 								} else if r.GroupVersionKind().Group != "" {
	// 									return fmt.Errorf("result[%v].resources[%v].patched is not defined properly. ---> Correct format - patched: patchedResource_pool:apiversion/kind/group/namespace/name", re, resk)
	// 								} else if len(patched) == 5 {
	// 									pf = true
	// 								} else {
	// 									return fmt.Errorf("result[%v].resources[%v].patched is not defined properly. ---> Correct format - patched: patchedResource_pool:apiversion/kind/namespace/name", re, resk)
	// 								}
	// 							}
	// 						}
	// 						if !pf {
	// 							if len(patched) == 6 {
	// 								return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].patched : %v:%v/%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4], name[5])
	// 							} else {
	// 								return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].patched : %v:%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4])
	// 							}
	// 						}
	// 					}
	// 				}
	// 			}

	// 		for _, k := range filteredPolicies {
	// 			if k.GetName() == res.Policy {
	// 				if k.GetSpec().HasGenerate() {
	// 					if len(generated) == 0 {
	// 						return fmt.Errorf("generate rule policy detected but result[%v].resources[%v].generated is empty", re, resk)
	// 					}
	// 					if len(generated) < 5 || len(generated) > 6 {
	// 						return fmt.Errorf("results[%v].resources[%v].generated field is not defined properly. ---> Correct format - generated: generatedResource_pool:apiversion/kind/group/namespace/name", re, resk)
	// 					}
	// 					if clone != nil {
	// 						if len(clone) < 5 || len(clone) > 6 {
	// 							return fmt.Errorf("results[%v].resources[%v].cloneSource field is not defined properly. ---> Correct format - cloneSource: cloneSourceResource:apiversion/kind/group/namespace/name", re, resk)
	// 						}
	// 						// for _, r := range resourcesMap["cloneSourceResource_pool"] {
	// 						// 	if clone[1] == r.GroupVersionKind().Version && clone[2] == r.GetKind() && clone[len(clone)-2] == r.GetNamespace() && clone[len(clone)-1] == r.GetName() {
	// 						// 		if len(clone) == 6 {
	// 						// 			if r.GroupVersionKind().Group == clone[3] {
	// 						// 				cf = true
	// 						// 			}
	// 						// 		} else if r.GroupVersionKind().Group != "" {
	// 						// 			return fmt.Errorf("result[%v].resources[%v].cloneSource is not defined properly. ---> Correct format - cloneSource: cloneSourceResource_pool:apiversion/kind/group/namespace/name", re, resk)
	// 						// 		} else if len(clone) == 5 {
	// 						// 			cf = true
	// 						// 		} else {
	// 						// 			return fmt.Errorf("result[%v].resources[%v].cloneSource is not defined properly. ---> Correct format - cloneSource: cloneSourceResource_pool:apiversion/kind/namespace/name", re, resk)
	// 						// 		}
	// 						// 	}
	// 						// }
	// 						if !cf {
	// 							if len(clone) == 6 {
	// 								return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].cloneSource : %v:%v/%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4], name[5])
	// 							} else if len(clone) != 5 && len(clone) != 0 {
	// 								return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].cloneSource : %v:%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4])
	// 							}
	// 						}
	// 					}

	// 					for _, r := range resourcesMap["generatedResource_pool"] {
	// 						if generated[1] == r.GroupVersionKind().Version && generated[2] == r.GetKind() && generated[len(generated)-2] == r.GetNamespace() && generated[len(generated)-1] == r.GetName() {
	// 							if len(generated) == 6 {
	// 								if r.GroupVersionKind().Group == generated[3] {
	// 									gf = true
	// 								}
	// 							} else if r.GroupVersionKind().Group != "" {
	// 								return fmt.Errorf("result[%v].resources[%v].generated is not defined properly. ---> Correct format - generated: generatedResource_pool:apiversion/kind/group/namespace/name", re, resk)
	// 							} else if len(generated) == 5 {
	// 								gf = true
	// 							} else {
	// 								return fmt.Errorf("result[%v].resources[%v].generated is not defined properly. ---> Correct format - generated: generatedResource_pool:apiversion/kind/namespace/name", re, resk)
	// 							}
	// 						}
	// 					}
	// 					if !gf {
	// 						if len(generated) == 6 {
	// 							return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].generated : %v:%v/%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4], name[5])
	// 						} else {
	// 							return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].generated : %v:%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4])
	// 						}
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}
	// }
	// if r.Result == "" {
	// 	return fmt.Errorf("test execution failed because spec.results[%v].result is empty", k)
	// } else if r.Result != "fail" && r.Result != "pass" && r.Result != "skip" {
	// 	return fmt.Errorf("test execution failed because spec.results[%v].result is not correct. only pass, fail or skip value can be used", k)
	// }

	// if len(tests.Spec.Variables.Policies) > 0 {
	// 	for vp, v := range tests.Spec.Variables.Policies {
	// 		if v.Name == "" {
	// 			return fmt.Errorf("test execution failed because spec.variables.policies[%v].name is empty", vp)
	// 		}
	// 		match := nm.MatchString(v.Name)
	// 		if !match {
	// 			return fmt.Errorf("test execution failed because spec.variables.policies[%v].name is not a valid name", vp)
	// 		}
	// 		pof = false
	// 		for _, k := range filteredPolicies {
	// 			if v.Name == k.GetName() {
	// 				pof = true
	// 				for vr, vor := range v.Rules {
	// 					if vor.Name == "" {
	// 						return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].name is empty", vp, vr)
	// 					}
	// 					match := nm.MatchString(vor.Name)
	// 					if !match {
	// 						return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].name is not a valid name", vp, vr)
	// 					}
	// 					rof = false
	// 					for _, k := range filteredPolicies {
	// 						for _, r := range k.GetSpec().Rules {
	// 							if r.Name == vor.Name {
	// 								rof = true
	// 							}
	// 						}
	// 					}
	// 					if !rof {
	// 						return fmt.Errorf("test execution failed because spec.variables.policies[%v].rule[%v].name does not match with the rule name in policy:%v", vp, vr, v.Name)
	// 					}
	// 				}
	// 			}
	// 		}
	// 		if !pof {
	// 			return fmt.Errorf("test execution failed because spec.variables.policies[%v].name does not match with any policy name mentioned in spec.policies", vp)
	// 		}
	// 		for vr, vor := range v.Rules {
	// 			if len(vor.Values) < 1 && len(vor.ForeachValues) < 1 && len(vor.NamespaceSelector) < 1 {
	// 				return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v] is empty", vp, vr)
	// 			}
	// 			for ka, voa := range vor.Attestations {
	// 				if voa.PredicateType == "" {
	// 					return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].attestations[%v].predicateType is empty", vp, vr, ka)
	// 				}
	// 				if voa.PredicateResource == "" {
	// 					return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].attestations[%v].predicateResource is empty", vp, vr, ka)
	// 				}
	// 			}
	// 			if len(vor.NamespaceSelector) > 0 {
	// 				for vn, von := range vor.NamespaceSelector {
	// 					if von.Name == "" {
	// 						return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].namespaceSelector[%v].name is empty", vp, vr, vn)
	// 					}
	// 					match := nm.MatchString(von.Name)
	// 					if !match {
	// 						return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].namespaceSelector[%v].name is not a valid name", vp, vr, vn)
	// 					}
	// 					if len(von.Labels) < 1 {
	// 						return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].namespaceSelector[%v].labels is empty", vp, vr, vn)
	// 					}
	// 				}
	// 			}
	// 		}
	// 		if len(v.Resources) > 0 {
	// 			for re, vre := range v.Resources {
	// 				if vre.Name == "" {
	// 					return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].name is empty", vp, re)
	// 				}
	// 				match := nm.MatchString(vre.Name)
	// 				if !match {
	// 					return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].name is not a valid name", vp, re)
	// 				}
	// 				if len(vre.Values) < 1 && len(vre.UserInfo.ClusterRoles) < 1 && len(vre.UserInfo.Roles) < 1 && len(vre.UserInfo.Subjects) < 1 {
	// 					return fmt.Errorf("test execution failed because spec.variables.policies[%v].recources[%v] is empty", vp, re)
	// 				}
	// 				for _, r := range resourcesMap {
	// 					rov := false
	// 					for _, re := range r {
	// 						if re.GetName() == vre.Name {
	// 							rov = true
	// 						}
	// 					}
	// 					if !rov {
	// 						return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].name does not match with any resource name mentioned in spec.recources", vp, re)
	// 					}
	// 				}
	// 				for s, vrs := range vre.UserInfo.Subjects {
	// 					if vrs.Name == "" {
	// 						return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].userInfo.subjects[%v].name is empty", vp, re, s)
	// 					}
	// 					match := nm.MatchString(vrs.Name)
	// 					if !match {
	// 						return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].userInfo.subjects[%v].name is not a valid name", vp, re, s)
	// 					}
	// 					if vrs.Kind == "" {
	// 						return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].userInfo.subjects[%v].kind is empty", vp, re, s)
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}
	// }
	//}

	return nil

}

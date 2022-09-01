package testing

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	billy "github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/pkg/dclient"
)

func validation(tests *kyvernov1.Test_manifest, isGit bool, policyResourcePath string) error {

	var fs billy.Filesystem
	var dClient dclient.Interface
	var rf bool
	var pf bool
	var cf bool
	var gf bool
	var n bool
	var pof bool
	var rof bool
	var rpf bool
	var repf bool
	var matchname bool

	resourceFullPath := make(map[string][]string)

	nm := regexp.MustCompile(`^[a-z]([a-z0-9\-]*[a-z])?$`)

	if tests.TypeMeta.APIVersion == "" {
		return fmt.Errorf("test execution failed because apiversion is empty")
	}
	apiv := strings.FieldsFunc(tests.TypeMeta.APIVersion, Split)
	if len(apiv) < 2 {
		return fmt.Errorf("test execution failed because apiversion value is not correct. Correct format `apiVersion: cli.kyverno.io/v1beta1`")
	}
	if len(apiv) > 1 {
		if apiv[0] != "cli.kyverno.io" || apiv[1] != "v1beta1" {
			return fmt.Errorf("test execution failed because apiversion value is not correct. Correct format `apiVersion: cli.kyverno.io/v1beta1`")
		}
	}
	if tests.TypeMeta.Kind == "" {
		return fmt.Errorf("test execution failed because kind is empty")
	} else if tests.TypeMeta.Kind != "KyvernoTest" {
		return fmt.Errorf("test execution failed because the value of kind is not correct, it should be `KyvernoTest`")
	}
	if tests.Metadata.Name == "" {
		return fmt.Errorf("test execution failed because metadata.name is empty")
	}
	matchname = nm.MatchString(tests.Metadata.Name)
	if !matchname {
		return fmt.Errorf("test execution failed because Metadata.Name is not valid")
	}
	if len(tests.Metadata.Labels) > 0 {
		for k := range tests.Metadata.Labels {
			if tests.Metadata.Labels[k] == "" {
				return fmt.Errorf("test execution failed because metadata.labels.%v does not have a value", k)
			}
		}
	}

	if len(tests.Metadata.Annotations) > 0 {
		for k := range tests.Metadata.Annotations {
			if tests.Metadata.Annotations[k] == "" {
				return fmt.Errorf("test execution failed because metadata.annotations.%v does not have a value", k)
			}
		}
	}

	r := regexp.MustCompile(`(((\.\.)(/))?)*(/)?(([a-zA-Z0-9]+)(/))*([a-zA-Z0-9])(\.yaml)$`)
	if len(tests.Spec.Policies) < 1 {
		return fmt.Errorf("test execution failed because spec.policies is empty")
	} else {
		for kp, p := range tests.Spec.Policies {

			match := r.MatchString(p)
			if !match {
				return fmt.Errorf("test execution failed because spec.policies[%v] is not a correct path to yaml file", kp)
			}
		}
	}

	if len(tests.Spec.Resources) < 1 {
		return fmt.Errorf("test execution failed because spec.resources is empty")
	} else {
		for kr := range tests.Spec.Resources {
			for k, p := range tests.Spec.Resources[kr] {
				match := r.MatchString(p)
				if !match {
					return fmt.Errorf("test execution failed because spec.resources.%v.[%v] is not a correct path to yaml file", kr, k)
				}
			}
		}
	}

	if len(tests.Spec.Results) < 1 {
		return fmt.Errorf("test execution failed because spec.results is empty")
	}

	for k := range tests.Spec.Resources {
		resourceFullPath[k] = getFullPath(tests.Spec.Resources[k], policyResourcePath, isGit, "resource")
	}
	policyFullPath := getFullPath(tests.Spec.Policies, policyResourcePath, isGit, "policy")
	policies, err := common.GetPoliciesFromPaths(fs, policyFullPath, isGit, policyResourcePath)
	if err != nil {
		fmt.Printf("Error: failed to load policies\nCause: %s\n", err)
		os.Exit(1)
	}
	filteredPolicies := []kyvernov1.PolicyInterface{}
	for _, p := range policies {
		for _, res := range tests.Spec.Results {
			if p.GetName() == res.Policy {
				filteredPolicies = append(filteredPolicies, p)
				break
			}
		}
	}
	policies = filteredPolicies

	mutatedPolicies, err := common.MutatePolicies(policies)
	if err != nil {
		return fmt.Errorf("failed to mutate policy, error : %v", err)
	}
	allresources := make(map[string][]*unstructured.Unstructured)
	resourcesMap := make(map[string][]*unstructured.Unstructured)
	for k := range tests.Spec.Resources {
		allresources[k], err = common.GetResourceAccordingToResourcePath(fs, resourceFullPath[k], false, mutatedPolicies, dClient, "", false, isGit, policyResourcePath)
		if err != nil {
			return fmt.Errorf("error: failed to load resources\nCause: %s", err)
		}
		resourcesMap[k] = allresources[k]
	}

	for k, r := range tests.Spec.Results {

		if r.Policy == "" {
			return fmt.Errorf("test execution failed because spec.results[%v].policy is empty", k)
		}
		matchname = nm.MatchString(r.Policy)
		if !matchname {
			return fmt.Errorf("test execution failed because spec.results[%v].policy is not valid", k)
		}
		rpf = false
		for _, p := range filteredPolicies {
			if p.GetName() == r.Policy {
				rpf = true
				if r.Rule == "" {
					return fmt.Errorf("test execution failed because spec.results[%v].rule is empty", k)
				}
				matchname = nm.MatchString(r.Rule)
				if !matchname {
					return fmt.Errorf("test execution failed because sspec.results[%v].rule is not valid", k)
				}
				repf = false
				for _, rn := range p.GetSpec().Rules {
					if rn.Name == r.Rule {
						repf = true
					}
				}
				if !repf {
					return fmt.Errorf("test execution failed because spec.results[%v].rule: %v not found in the policy: %v", k, r.Rule, r.Policy)
				}
			}
		}
		if !rpf {
			return fmt.Errorf("test execution failed because spec.results[%v].policy not found in spec.policies", k)
		}

		for re, res := range tests.Spec.Results {
			for resk, testr := range res.Resources {
				n = false
				name := strings.FieldsFunc(testr.Old, Split)
				if testr.Old == "" {
					return fmt.Errorf("results[%v].resources[%v].old field is mandaotry", re, resk)
				}
				for k := range resourcesMap {
					if k == name[0] {
						n = true
					}
				}
				if !n {
					return fmt.Errorf("[%v] is not found defined under spec.resources", name[0])
				}
			}
		}

		for re, res := range tests.Spec.Results {
			if len(res.Resources) < 1 {
				return fmt.Errorf("results[%v].resources is found empty", re)
			}
			for resk, testr := range res.Resources {
				rf = false
				pf = false
				cf = false
				gf = false
				name := strings.FieldsFunc(testr.Old, Split)
				patched := strings.FieldsFunc(testr.Patched, Split)
				clone := strings.FieldsFunc(testr.CloneSource, Split)
				generated := strings.FieldsFunc(testr.Generated, Split)

				for _, r := range resourcesMap[name[0]] {
					if name[1] == r.GroupVersionKind().Version && name[len(name)-2] == r.GetNamespace() && name[len(name)-1] == r.GetName() {
						if len(name) == 5 {
							if r.GroupVersionKind().Group == name[2] {
								rf = true
							}
						} else if r.GroupVersionKind().Group != "" {
							return fmt.Errorf("result[%v].resources[%v].old is not defined properly. ---> Correct format - old: my_resource_pool:apiversion/group/namespace/name", re, resk)
						} else if len(name) == 4 {
							rf = true
						} else {
							return fmt.Errorf("result[%v].resources[%v].old is not defined properly. ---> Correct format - old: my_resource_pool:apiversion/namespace/name", re, resk)
						}
					}
				}
				if !rf {
					if len(name) == 5 {
						return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].old : %v:%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4])
					} else {
						return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].old : %v:%v/%v/%v", re, resk, name[0], name[1], name[2], name[3])
					}
				}

				for _, k := range filteredPolicies {
					if k.GetName() == res.Policy {
						if k.GetSpec().HasMutate() {
							if len(patched) == 0 {
								return fmt.Errorf("mutate rule detected but result[%v].resources[%v].patched is empty", re, resk)
							}
							for _, r := range resourcesMap["patchedResource_pool"] {
								if patched[1] == r.GroupVersionKind().Version && patched[len(patched)-2] == r.GetNamespace() && patched[len(patched)-1] == r.GetName() {
									if len(patched) == 5 {
										if r.GroupVersionKind().Group == patched[2] {
											pf = true
										}
									} else if r.GroupVersionKind().Group != "" {
										return fmt.Errorf("result[%v].resources[%v].patched is not defined properly. ---> Correct format - patched: patchedResource_pool:apiversion/group/namespace/name", re, resk)
									} else if len(patched) == 4 {
										pf = true
									} else {
										return fmt.Errorf("result[%v].resources[%v].old is not defined properly. ---> Correct format - old: my_resource_pool:apiversion/namespace/name", re, resk)
									}
								}
							}
							if !pf {
								if len(patched) == 5 {
									return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].patched : %v:%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4])
								} else {
									return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].patched : %v:%v/%v/%v", re, resk, name[0], name[1], name[2], name[3])
								}
							}
						}
					}
				}

				for _, k := range filteredPolicies {
					if k.GetName() == res.Policy {
						if k.GetSpec().HasGenerate() {
							if len(generated) == 0 {
								return fmt.Errorf("generate rule policy detected but result[%v].resources[%v].generated is empty", re, resk)
							}
							if clone != nil {
								for _, r := range resourcesMap["cloneSourceResource_pool"] {
									if clone[1] == r.GroupVersionKind().Version && clone[len(clone)-2] == r.GetNamespace() && clone[len(clone)-1] == r.GetName() {
										if len(clone) == 5 {
											if r.GroupVersionKind().Group == clone[2] {
												cf = true
											}
										} else if r.GroupVersionKind().Group != "" {
											return fmt.Errorf("result[%v].resources[%v].clone is not defined properly. ---> Correct format - cloneSource: cloneSourceResource_pool:apiversion/group/namespace/name", re, resk)
										} else if len(clone) == 4 {
											cf = true
										} else {
											return fmt.Errorf("result[%v].resources[%v].old is not defined properly. ---> Correct format - cloneSource: cloneSourceResource_pool:apiversion/namespace/name", re, resk)
										}
									}
								}
								if !cf {
									if len(clone) == 5 {
										return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].cloneSource : %v:%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4])
									} else if len(clone) != 5 && len(clone) != 0 {
										return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].cloneSource : %v:%v/%v/%v", re, resk, name[0], name[1], name[2], name[3])
									}
								}
							}

							for _, r := range resourcesMap["generatedResource_pool"] {
								if generated[1] == r.GroupVersionKind().Version && generated[len(generated)-2] == r.GetNamespace() && generated[len(generated)-1] == r.GetName() {
									if len(generated) == 5 {
										if r.GroupVersionKind().Group == generated[2] {
											gf = true
										}
									} else if r.GroupVersionKind().Group != "" {
										return fmt.Errorf("result[%v].resources[%v].generated is not defined properly. ---> Correct format - generated: generatedResource_pool:apiversion/group/namespace/name", re, resk)
									} else if len(generated) == 4 {
										gf = true
									} else {
										return fmt.Errorf("result[%v].resources[%v].generated is not defined properly. ---> Correct format - generated: generatedResource_pool:apiversion/namespace/name", re, resk)
									}
								}
							}
							if !gf {
								if len(generated) == 5 {
									return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].generated : %v:%v/%v/%v/%v", re, resk, name[0], name[1], name[2], name[3], name[4])
								} else {
									return fmt.Errorf("resources given in the pool didn't match with the results[%v].resources[%v].generated : %v:%v/%v/%v", re, resk, name[0], name[1], name[2], name[3])
								}
							}
						}
					}
				}
			}
		}
		if r.Kind == "" {
			return fmt.Errorf("test execution failed because spec.results[%v].kind is empty", k)
		}
		if r.Result == "" {
			return fmt.Errorf("test execution failed because spec.results[%v].result is empty", k)
		} else if r.Result != "fail" && r.Result != "pass" && r.Result != "skip" {
			return fmt.Errorf("test execution failed because spec.results[%v].result is not correct. only pass, fail or skip value can be used", k)
		}

		if len(tests.Spec.Variables.Policies) > 0 {
			for vp, v := range tests.Spec.Variables.Policies {
				if v.Name == "" {
					return fmt.Errorf("test execution failed because spec.variables.policies[%v].name is empty", vp)
				}
				match := nm.MatchString(v.Name)
				if !match {
					return fmt.Errorf("test execution failed because spec.variables.policies[%v].name is not a valid name", vp)
				}
				pof = false
				for _, k := range filteredPolicies {
					if v.Name == k.GetName() {
						pof = true
						for vr, vor := range v.Rules {
							rof = false
							for _, k := range filteredPolicies {
								for _, r := range k.GetSpec().Rules {
									if r.Name == vor.Name {
										rof = true
									}
								}
							}
							if !rof {
								return fmt.Errorf("test execution failed because spec.variables.policies[%v].rule[%v].name does not match with the rule name in policy:%v", vp, vr, v.Name)
							}
						}
					}
				}
				if !pof {
					return fmt.Errorf("test execution failed because spec.variables.policies[%v].name does not match with any policy name mentioned in spec.policies", vp)
				}
				for vr, vor := range v.Rules {
					if vor.Name == "" {
						return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].name is empty", vp, vr)
					}
					match := nm.MatchString(vor.Name)
					if !match {
						return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].name is not a valid name", vp, vr)
					}
					for ka, voa := range vor.Attestations {
						if voa.PredicateType == "" {
							return fmt.Errorf("stest execution failed because spec.variables.policies[%v].rules[%v].attestations[%v].predicateType is empty", vp, vr, ka)
						}
						if voa.PredicateResource == "" {
							return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].attestations[%v].predicateResource is empty", vp, vr, ka)
						}
					}
					if len(vor.NamespaceSelector) > 0 {
						for vn, von := range vor.NamespaceSelector {
							if von.Name == "" {
								return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].namespaceSelector[%v].name is empty", vp, vr, vn)
							}
							match := nm.MatchString(von.Name)
							if !match {
								return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].namespaceSelector[%v].name is not a valid name", vp, vr, vn)
							}
							if len(von.Labels) < 1 {
								return fmt.Errorf("test execution failed because spec.variables.policies[%v].rules[%v].namespaceSelector[%v].labels is empty", vp, vr, vn)
							}
						}
					}
				}
				if len(v.Resources) > 0 {
					for re, vre := range v.Resources {
						if vre.Name == "" {
							return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].name is empty", vp, re)
						}
						match := nm.MatchString(vre.Name)
						if !match {
							return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].name is not a valid name", vp, re)
						}
						for _, r := range resourcesMap {
							rov := false
							for _, re := range r {
								if re.GetName() == vre.Name {
									rov = true
								}
							}
							if !rov {
								return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].name does not match with any resource name mentioned in spec.recources", vp, re)
							}
						}
						for s, vrs := range vre.UserInfo.Subjects {
							if vrs.Name == "" {
								return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].userInfo.subjects[%v].name is empty", vp, re, s)
							}
							match := nm.MatchString(vrs.Name)
							if !match {
								return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].userInfo.subjects[%v].name is not a valid name", vp, re, s)
							}
							if vrs.Kind == "" {
								return fmt.Errorf("test execution failed because spec.variables.policies[%v].resources[%v].userInfo.subjects[%v].kind is empty", vp, re, s)
							}
						}
					}
				}
			}
		}
	}

	return nil

}

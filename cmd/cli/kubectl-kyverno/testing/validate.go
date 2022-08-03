package testing

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func validation() error {

	test := "cmd/cli/kubectl-kyverno/testyaml/kyverno-test.yaml"
	yamlFile, err := ioutil.ReadFile(test)
	if err != nil {
		return err
	}
	policyBytes, err1 := yaml.ToJSON(yamlFile)
	if err1 != nil {
		return err1
	}
	tests := &kyvernov1.Test_manifest{}
	if err := json.Unmarshal(policyBytes, tests); err != nil {
		return err
	}
	if tests.TypeMeta.APIVersion == "" {
		return fmt.Errorf("skipping test as tests.TypeMeta.APIVersion not found")
	}
	if tests.TypeMeta.Kind == "" {
		return fmt.Errorf("skipping test as tests.TypeMeta.Kind not found")
	} else if tests.TypeMeta.Kind != "KyvernoTest" {
		return fmt.Errorf("skipping test as tests.TypeMeta.Kind value is not `KyvernoTest`")
	}
	if tests.Metadata.Name == "" {
		return fmt.Errorf("skipping test as tests.Metadata.Name not found")
	}

	// for k, v := range tests.Metadata.Labels {
	// 	fmt.Printf("%q and %q", reflect.TypeOf(v), reflect.TypeOf(k))
	// }

	if len(tests.Spec.Policies) < 1 {
		return fmt.Errorf("skipping test as tests.Spec.Policies not found")
	} else {
		for kp, p := range tests.Spec.Policies {
			r := regexp.MustCompile(`(((\.\.)(/))?)*(/)?(([a-zA-Z]+)(/))*([a-zA-Z])(\.yaml)$`)
			match := r.MatchString(p)
			if !match {
				return fmt.Errorf("skipping test as tests.Spec.Policies[%v] is not a yaml file", kp+1)
			}
		}
	}

	// if len(tests.Spec.Resources.My_resource_pool) < 1 {
	// 	return fmt.Errorf("skipping test as tests.Spec.Resources.My_resource_pool not found")
	// } else {
	// 	for kr, p := range tests.Spec.Resources.My_resource_pool {
	// 		r, _ := regexp.Compile(`(((\.\.)(/))?)*(/)?(([a-zA-Z]+)(/))*([a-zA-Z])(\.yaml)$`)
	// 		if p == "." {
	// 			continue
	// 		} else {
	// 			match := r.MatchString(p)
	// 			if !match {
	// 				return fmt.Errorf("skipping test as tests.Spec.Resources.My_resource_pool[%v] is not a yaml file", kr+1)
	// 			}
	// 		}

	// 	}
	// }

	if len(tests.Spec.Results) < 1 {
		return fmt.Errorf("skipping test as tests.Spec.Results not found")
	}

	for k, r := range tests.Spec.Results {
		if r.Policy == "" {
			return fmt.Errorf("skipping test as tests.Spec.Results[%v].Policy not found", k+1)
		}
		if r.Rule == "" {
			return fmt.Errorf("skipping test as tests.Spec.Results[%v].Rule not found", k+1)
		}
		// if r.Resources.Object == "" || r.Resources.Old == "" {
		// 	return fmt.Errorf("skipping test as tests.Spec.Results[%v].Resources requires either object or old to be defined", k)
		// }
		if r.Kind == "" {
			return fmt.Errorf("skipping test as tests.Spec.Results[%v].Kind not found", k+1)
		}
		if r.Result == "" {
			return fmt.Errorf("skipping test as tests.Spec.Results[%v].Result not found", k+1)
		}
		// if r.Result != "pass" {
		// 	return fmt.Errorf("skipping test as tests.Spec.Results[%v].Result is not pass or fail or skip", k+1)
		// }

		if len(tests.Spec.Variables.Policies) > 0 {
			for vp, v := range tests.Spec.Variables.Policies {
				if v.Name == "" {
					return fmt.Errorf("skipping test as tests.Spec.Variables.Policies[%v].Name not found", vp+1)
				}
				for vr, vor := range v.Rules {
					if vor.Name == "" {
						return fmt.Errorf("skipping test as tests.Spec.Variables.Policies[%v].Rules[%v].Name not found", vp+1, vr+1)
					}
					// if len(vor.Values) < 1 {
					// 	return fmt.Errorf("skipping test as tests.Spec.Variables.Policies[%v].Rules[%v].Values not found", vp+1, vr+1)
					// }
					for ka, voa := range vor.Attestations {
						if voa.PredicateType == "" {
							return fmt.Errorf("skipping test as tests.Spec.Variables.Policies[%v].Rules[%v].Attestations[%v].PredicateType not found", vp+1, vr+1, ka+1)
						}
						if voa.PredicateResource == "" {
							return fmt.Errorf("skipping test as tests.Spec.Variables.Policies[%v].Rules[%v].Attestations[%v].PredicateResource not found", vp+1, vr+1, ka+1)
						}
					}
					if len(vor.NamespaceSelector) > 0 {
						for vn, von := range vor.NamespaceSelector {
							if von.Name == "" {
								return fmt.Errorf("skipping test as tests.Spec.Variables.Policies[%v].Rules[%v].NamespaceSelector[%v].Name not found", vp+1, vr+1, vn+1)
							}
							if len(von.Labels) < 1 {
								return fmt.Errorf("skipping test as tests.Spec.Variables.Policies[%v].Rules[%v].NamespaceSelector[%v].Labels not found", vp+1, vr+1, vn+1)
							}
						}
					}
				}
				if len(v.Resources) > 0 {
					for re, vre := range v.Resources {
						if vre.Name == "" {
							return fmt.Errorf("skipping test as tests.Spec.Variables.Policies[%v].Resources[%v].Name not found", vp+1, re+1)
						}
						for s, vrs := range vre.UserInfo.Subjects {
							if vrs.Name == "" {
								return fmt.Errorf("skipping test as tests.Spec.Variables.Policies[%v].Resources[%v].UserInfo.Subjects[%v].Name not found", vp+1, re+1, s+1)
							}
							if vrs.Kind == "" {
								return fmt.Errorf("skipping test as tests.Spec.Variables.Policies[%v].Resources[%v].UserInfo.Subjects[%v].Kind not found", vp+1, re+1, s+1)
							}
						}
					}
				}
			}
		}
	}

	return nil

}

package validate

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nirmata/kyverno/pkg/utils"
	"os"

	"github.com/nirmata/kyverno/pkg/kyverno/common"
	"github.com/nirmata/kyverno/pkg/kyverno/sanitizedError"

	policy2 "github.com/nirmata/kyverno/pkg/policy"
	"github.com/spf13/cobra"

	_ "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"

	yamlv2 "gopkg.in/yaml.v2"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

func Command() *cobra.Command {
	var outputType string
	var crdPaths []string
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "Validates kyverno policies",
		Example: "kyverno validate /path/to/policy.yaml /path/to/folderOfPolicies",
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {
			log := log.Log

			defer func() {
				if err != nil {
					if !sanitizedError.IsErrorSanitized(err) {
						log.Error(err, "failed to sanitize")
						err = fmt.Errorf("internal error")
					}
				}
			}()

			if outputType != "" {
				if outputType != "yaml" && outputType != "json" {
					return sanitizedError.NewWithError(fmt.Sprintf("%s format is not supported", outputType), errors.New("yaml and json are supported"))
				}
			}

			policies, openAPIController, err := common.GetPoliciesValidation(policyPaths)
			if err != nil {
				return err
			}

			// if CRD's are passed, add these to OpenAPIController
			if len(crdPaths) > 0 {
				//err := common.ValidatePolicyAgainstCrd(policy, crdPath)
				//if err != nil {
				//	log.Log.Error(err, "policy "+policy.Name+" is invalid")
				//	//os.Exit(1)
				//	return err
				//}
				for _, path := range crdPaths {
					crd, err := common.ConvertFileToUnstructed(path)
					if err != nil {
						log.Error(err, "crd is invalid", "file", path)
					}
					openAPIController.ParseCRD(*crd)
				}
			}

			invalidPolicyFound := false
			for _, policy := range policies {
				err := policy2.Validate(utils.MarshalPolicy(*policy), nil, true, openAPIController)
				if err != nil {
					fmt.Printf("Policy %s is invalid.\n", policy.Name)
					log.Error(err, "policy "+policy.Name+" is invalid")
					invalidPolicyFound = true
				} else {
					fmt.Printf("Policy %s is valid.\n\n", policy.Name)
					if outputType != "" {
						logger := log.WithName("validate")
						p, err := common.MutatePolicy(policy, logger)
						if err != nil {
							if !sanitizedError.IsErrorSanitized(err) {
								return sanitizedError.NewWithError("failed to mutate policy.", err)
							}
							return err
						}
						if outputType == "yaml" {
							yamlPolicy, _ := yamlv2.Marshal(p)
							fmt.Println(string(yamlPolicy))
						} else {
							jsonPolicy, _ := json.MarshalIndent(p, "", "  ")
							fmt.Println(string(jsonPolicy))
						}
					}
				}
			}

			if invalidPolicyFound == true {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputType, "output", "o", "", "Prints the mutated policy")
	cmd.Flags().StringArrayVarP(&crdPaths, "crd", "c", []string{}, "Path to CRD files")
	return cmd
}

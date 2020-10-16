package validate

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/utils"

	"github.com/kyverno/kyverno/pkg/kyverno/common"
	"github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"

	policy2 "github.com/kyverno/kyverno/pkg/policy"
	"github.com/spf13/cobra"

	_ "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"

	log "sigs.k8s.io/controller-runtime/pkg/log"
	yaml "sigs.k8s.io/yaml"
	client "github.com/kyverno/kyverno/pkg/dclient"
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

			if len(policyPaths) == 0 {
				return sanitizedError.NewWithError(fmt.Sprintf("policy file(s) required"), err)
			}

			var policies []*v1.ClusterPolicy
			if policyPaths[0] == "-" {
				if common.IsInputFromPipe() {
					policyStr := ""
					scanner := bufio.NewScanner(os.Stdin)
					for scanner.Scan() {
						policyStr = policyStr + scanner.Text() + "\n"
					}

					yamlBytes := []byte(policyStr)
					var getErrors []error
					policies, getErrors = utils.GetPolicy(yamlBytes)
					var errString string

					for _, err := range getErrors {
						if err != nil {
							errString += err.Error() + "\n"
						}
					}
					if errString != "" {
						return sanitizedError.NewWithError("failed to extract the resources", errors.New(errString))
					}
				}
			} else {
				cluster := false
				var dClient *client.Client
				policies, err = common.ValidateAndGetPolicies(policyPaths, cluster, dClient)
				if err != nil {
					if !sanitizedError.IsErrorSanitized(err) {
						return sanitizedError.NewWithError("failed to mutate policies.", err)
					}
					return err
				}
			}

			openAPIController, err := openapi.NewOpenAPIController()
			if err != nil {
				return sanitizedError.NewWithError("failed to initialize openAPIController", err)
			}

			// if CRD's are passed, add these to OpenAPIController
			if len(crdPaths) > 0 {
				crds, err := common.GetCRDs(crdPaths)
				if err != nil {
					fmt.Printf("\nError: crd is invalid. \nFile: %s \nCause: %s\n", crdPaths, err)
					os.Exit(1)
				}
				for _, crd := range crds {
					openAPIController.ParseCRD(*crd)
				}
			}

			invalidPolicyFound := false
			for _, policy := range policies {
				err := policy2.Validate(utils.MarshalPolicy(*policy), nil, true, openAPIController)
				if err != nil {
					fmt.Printf("Policy %s is invalid.\n", policy.Name)
					fmt.Printf("Error: invalid policy.\nCause: %s\n\n", err)
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
							yamlPolicy, _ := yaml.Marshal(p)
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

package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernov1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/config"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/utils"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"os"
	"reflect"

	"github.com/nirmata/kyverno/pkg/engine/context"

	"strings"
	"time"

	client "github.com/nirmata/kyverno/pkg/dclient"

	"github.com/nirmata/kyverno/pkg/kyverno/common"
	"github.com/nirmata/kyverno/pkg/kyverno/sanitizedError"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/nirmata/kyverno/pkg/engine"

	engineutils "github.com/nirmata/kyverno/pkg/engine/utils"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/spf13/cobra"
	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type resultCounts struct {
	pass  int
	fail  int
	warn  int
	error int
	skip  int
}

func Command() *cobra.Command {
	var cmd *cobra.Command
	var namespace, kubeconfig string
	var cluster bool
	type Resource struct {
		Name   string            `json:"name"`
		Values map[string]string `json:"values"`
	}

	type Policy struct {
		Name      string     `json:"name"`
		Resources []Resource `json:"resources"`
	}

	type Values struct {
		Policies []Policy `json:"policies"`
	}

	kubernetesConfig := genericclioptions.NewConfigFlags(true)

	cmd = &cobra.Command{
		Use:     "report",
		Short:   "generate report",
		Example: fmt.Sprintf("To apply on a resource:\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2\n\nTo apply on a cluster\nkyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster"),
		RunE: func(cmd *cobra.Command, policyPaths []string) (err error) {

			var dClient *client.Client
			var kclient *kyvernoclient.Clientset

			restConfig, err := kubernetesConfig.ToRESTConfig()
			if err != nil {
				os.Exit(1)
			}

			dClient, err = client.NewClient(restConfig, 5*time.Minute, make(chan struct{}), log.Log)
			if err != nil {
				os.Exit(1)
			}

			kclient, err = kyvernoclient.NewForConfig(restConfig)
			if err != nil {
				os.Exit(1)
			}
			kubeClient, err := utils.NewKubeClient(restConfig)
			if err != nil {
				log.Log.Error(err, "Failed to create kubernetes client")
				os.Exit(1)
			}
			const resyncPeriod = 15 * time.Minute

			kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)

			configData := config.NewConfigData(
				kubeClient,
				kubeInformer.Core().V1().ConfigMaps(),
				"",
				"",
				"",
				log.Log.WithName("ConfigData"),
			)

			ns, err := dClient.ListResource("", "Namespace", "", &kyvernov1.LabelSelector{})
			if err != nil {
				os.Exit(1)
			}
			var engineResponses []response.EngineResponse
			for _, n := range ns.Items {
				policies, err := kclient.KyvernoV1().Policies(n.GetName()).List(kyvernov1.ListOption{})
				if err != nil {
					os.Exit(1)
				}
				// key uid
				resourceMap := map[string]unstructured.Unstructured{}
				for _, p := range policies.Items {
					for _, rule := range p.Spec.Rules {
						for _, k := range rule.MatchResources.Kinds {
							resourceSchema, _, err := dClient.DiscoveryClient.FindResource("", k)
							if err != nil {
								log.Log.Error(err, "failed to find resource", "kind", k)
								continue
							}

							if !resourceSchema.Namespaced && os.Getenv("SCOPE") == "CLUSTER" {
								rMap := policy.GetResourcesPerNamespace(k, dClient, "", rule, configData, log.Log)
								policy.MergeResources(resourceMap, rMap)
							} else if resourceSchema.Namespaced && os.Getenv("SCOPE") == "NAMESPACE" {
								namespaces := policy.GetNamespacesForRule(&rule, kubeInformer.Core().V1().Namespaces().Lister(), log.Log)
								for _, ns := range namespaces {
									rMap := policy.GetResourcesPerNamespace(k, dClient, ns, rule, configData, log.Log)
									policy.MergeResources(resourceMap, rMap)
								}
							}
						}
					}
					if p.HasAutoGenAnnotation() {
						resourceMap = policy.ExcludePod(resourceMap, log.Log)
					}
					pol := policy.ConvertPolicyToClusterPolicy(&p)
					for _, resource := range resourceMap {
						policyContext := engine.PolicyContext{
							NewResource:      resource,
							OldResource:      nil,
							Context:          context.Background(),
							Policy:           *pol,
							ExcludeGroupRole: configData.GetExcludeGroupRole(),
						}
						engineResponse := engine.Validate(policyContext)
						if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
							// we get an empty response if old and new resources created the same response
							// allow updates if resource update doesnt change the policy evaluation
							continue
						}
						if len(engineResponse.PolicyResponse.Rules) > 0 {
							engineResponses = append(engineResponses, engineResponse)
						}

						engineResponse = engine.Mutate(policyContext)
						if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
							// we get an empty response if old and new resources created the same response
							// allow updates if resource update doesnt change the policy evaluation
							continue
						}
						if len(engineResponse.PolicyResponse.Rules) > 0 {
							engineResponses = append(engineResponses, engineResponse)
						}

						engineResponse = engine.Generate(policyContext)
						if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
							// we get an empty response if old and new resources created the same response
							// allow updates if resource update doesnt change the policy evaluation
							continue
						}
						if len(engineResponse.PolicyResponse.Rules) > 0 {
							engineResponses = append(engineResponses, engineResponse)
						}
					}

				}

			}

			cmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "kubeconfig")
			cmd.Flags().BoolVarP(&cluster, "cluster", "c", false, "Checks if policies should be applied to cluster in the current context")
			cmd.Flags().BoolVarP(&helm, "helm", "h", false, "Checks if policies should be applied to cluster in the current context")
			cmd.Flags().BoolVarP(&namespace, "namespace", "n", false, "Checks if policies should be applied to cluster in the current context")
			return err
		},
	}
}


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
	"sync"

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
	var scope, kubeconfig string
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
			cmd.AddCommand(HelmCommand())
			cmd.AddCommand(NamespaceCommand())
			cmd.AddCommand(ClusterCommand())
			cmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "kubeconfig")
			return err
		},
	}
}

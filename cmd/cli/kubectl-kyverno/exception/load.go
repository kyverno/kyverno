package exception

import (
	"fmt"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource/convert"
	resourceloader "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource/loader"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

var (
	factory, _  = resourceloader.New(openapiclient.NewComposite(openapiclient.NewLocalCRDFiles(data.Crds(), data.CrdsFolder)))
	exceptionV2 = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("PolicyException")
)

func Load(content []byte) ([]*kyvernov2beta1.PolicyException, error) {
	documents, err := yamlutils.SplitDocuments(content)
	if err != nil {
		return nil, err
	}
	var exceptions []*kyvernov2beta1.PolicyException
	for _, document := range documents {
		gvk, untyped, err := factory.Load(document)
		if err != nil {
			return nil, err
		}
		switch gvk {
		case exceptionV2:
			exception, err := convert.To[kyvernov2beta1.PolicyException](untyped)
			if err != nil {
				return nil, err
			}
			exceptions = append(exceptions, exception)
		default:
			return nil, fmt.Errorf("policy exception type not supported %s", gvk)
		}
	}
	return exceptions, nil
}

package exception

import (
	"fmt"
	"os"
	"path/filepath"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/ext/resource/convert"
	resourceloader "github.com/kyverno/kyverno/ext/resource/loader"
	yamlutils "github.com/kyverno/kyverno/ext/yaml"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

var (
	factory, _        = resourceloader.New(openapiclient.NewComposite(openapiclient.NewLocalCRDFiles(data.Crds(), data.CrdsFolder)))
	exceptionV2alpha1 = schema.GroupVersion(kyvernov2alpha1.GroupVersion).WithKind("PolicyException")
	exceptionV2beta1  = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("PolicyException")
	exceptionV2       = schema.GroupVersion(kyvernov2.GroupVersion).WithKind("PolicyException")
)

func Load(paths ...string) ([]*kyvernov2beta1.PolicyException, error) {
	var out []*kyvernov2beta1.PolicyException
	for _, path := range paths {
		bytes, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, fmt.Errorf("unable to read yaml (%w)", err)
		}
		exceptions, err := load(bytes)
		if err != nil {
			return nil, fmt.Errorf("unable to load exceptions (%w)", err)
		}
		out = append(out, exceptions...)
	}
	return out, nil
}

func load(content []byte) ([]*kyvernov2beta1.PolicyException, error) {
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
		case exceptionV2alpha1, exceptionV2beta1, exceptionV2:
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

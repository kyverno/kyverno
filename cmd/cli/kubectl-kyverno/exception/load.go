package exception

import (
	"fmt"
	"io"
	"os"

	"github.com/go-git/go-billy/v5"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/ext/resource/convert"
	resourceloader "github.com/kyverno/kyverno/ext/resource/loader"
	yamlutils "github.com/kyverno/kyverno/ext/yaml"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

var (
	factory, _       = resourceloader.New(openapiclient.NewComposite(openapiclient.NewLocalCRDFiles(data.Crds(), data.CrdsFolder)))
	exceptionV2beta1 = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("PolicyException")
	exceptionV2      = schema.GroupVersion(kyvernov2.GroupVersion).WithKind("PolicyException")
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
		case exceptionV2beta1, exceptionV2:
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

func Loader(fs billy.Filesystem, resoucepath string, paths ...string) ([]*kyvernov2beta1.PolicyException, error) {
	var exceptions []*kyvernov2beta1.PolicyException
	for _, path := range paths {
		var content []byte
		var err error

		if fs != nil {
			// If a filesystem is provided, use it to read the file
			f, err := fs.Open(path)
			if err != nil {
				return nil, fmt.Errorf("failed to open file: %s, error: %v", path, err)
			}
			defer f.Close()

			content, err = io.ReadAll(f)
			if err != nil {
				return nil, fmt.Errorf("failed to read file: %s, error: %v", path, err)
			}
		} else {
			// Otherwise, read the file from the local filesystem
			content, err = os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read file: %s, error: %v", path, err)
			}
		}
		fmt.Printf("loading policy exception from file: %s\n", path)
		// Load the policy exceptions from the content
		excs, err := Load(content)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy exceptions from file: %s, error: %v", path, err)
		}

		exceptions = append(exceptions, excs...)
	}

	return exceptions, nil
}

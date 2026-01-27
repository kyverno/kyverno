package processor

import (
	"encoding/json"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	clicontext "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/context"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"k8s.io/apimachinery/pkg/api/meta"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

func policyHasValidateOrVerifyImageChecks(policy kyvernov1.PolicyInterface) bool {
	for _, rule := range policy.GetSpec().Rules {
		//  engine.validate handles both validate and verifyImageChecks atm
		if rule.HasValidate() || rule.HasVerifyImageChecks() {
			return true
		}
	}
	return false
}

func NewContextProvider(dclient dclient.Interface, restMapper meta.RESTMapper, f billy.Filesystem, contextPath string, registryAccess bool, isFake bool) (libs.Context, error) {
	if dclient != nil && !isFake {
		return libs.NewContextProvider(
			dclient,
			[]imagedataloader.Option{imagedataloader.WithLocalCredentials(registryAccess)},
			gctxstore.New(),
			restMapper,
			true,
		)
	}

	fakeContextProvider := libs.NewFakeContextProvider()
	fakeContextProvider.SetRESTMapper(restMapper)
	if contextPath != "" {
		ctx, err := clicontext.Load(f, contextPath)
		if err != nil {
			return nil, err
		}
		baseDir := filepath.Dir(contextPath)

		for _, resource := range ctx.ContextSpec.Resources {
			gvk := resource.GroupVersionKind()
			mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return nil, err
			}
			if err := fakeContextProvider.AddResource(mapping.Resource, &resource); err != nil {
				return nil, err
			}
		}
		for _, imgData := range ctx.ContextSpec.Images {
			raw, err := json.Marshal(imgData)
			if err != nil {
				return nil, err
			}
			var asMap map[string]any
			if err := json.Unmarshal(raw, &asMap); err != nil {
				return nil, err
			}
			fakeContextProvider.AddImageData(imgData.Image, asMap)
		}

		for _, entry := range ctx.ContextSpec.GlobalContext {
			var value any
			if entry.ValueFile != "" {
				raw, err := common.ReadFile(f, filepath.Join(baseDir, entry.ValueFile))
				if err != nil {
					return nil, err
				}
				jsonBytes, err := k8syaml.ToJSON(raw)
				if err != nil {
					return nil, err
				}
				if err := json.Unmarshal(jsonBytes, &value); err != nil {
					return nil, err
				}
			} else if entry.Value != nil {
				raw, err := json.Marshal(entry.Value)
				if err != nil {
					return nil, err
				}
				if err := json.Unmarshal(raw, &value); err != nil {
					return nil, err
				}
			}
			fakeContextProvider.AddGlobalReference(entry.Name, entry.Projection, value)
		}

		for _, stub := range ctx.ContextSpec.HTTP {
			var body []byte
			if stub.BodyFile != "" {
				raw, err := common.ReadFile(f, filepath.Join(baseDir, stub.BodyFile))
				if err != nil {
					return nil, err
				}
				jsonBytes, err := k8syaml.ToJSON(raw)
				if err != nil {
					return nil, err
				}
				body = jsonBytes
			} else if stub.Body != nil {
				raw, err := json.Marshal(stub.Body)
				if err != nil {
					return nil, err
				}
				body = raw
			}
			fakeContextProvider.AddHTTPStub(stub.Method, stub.URL, stub.Status, stub.Headers, body)
		}
	}
	return fakeContextProvider, nil
}

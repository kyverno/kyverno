module github.com/nirmata/kyverno

go 1.13

require (
	github.com/ahmetb/gen-crd-api-reference-docs v0.2.0 // indirect
	github.com/cameront/go-jsonpatch v0.0.0-20180223123257-a8710867776e
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/evanphx/json-patch/v5 v5.0.0 // indirect
	github.com/gardener/controller-manager-library v0.2.0
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.5
	github.com/go-openapi/strfmt v0.19.5
	github.com/go-openapi/validate v0.19.8
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20191227052852-215e87163ea7 // indirect
	github.com/googleapis/gnostic v0.3.1
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/jmespath/go-jmespath v0.0.0-20180206201540-c2b33e8439af
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a
	github.com/minio/minio v0.0.0-20200114012931-30922148fbb5
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/ory/go-acc v0.2.6 // indirect
	github.com/ory/x v0.0.85 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.4.1
	github.com/rogpeppe/godef v1.1.2 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.5.1
	github.com/tevino/abool v0.0.0-20170917061928-9b9efcf221b5
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200121175148-a6ecf24a6d71
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.17.4
	k8s.io/apiextensions-apiserver v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/cli-runtime v0.17.4
	k8s.io/client-go v0.17.4
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/kustomize/api v0.5.1
	sigs.k8s.io/kustomize/kyaml v0.6.1
	sigs.k8s.io/yaml v1.2.0
)

// Added for go1.13 migration https://github.com/golang/go/issues/32805
replace (
	github.com/gorilla/rpc v1.2.0+incompatible => github.com/gorilla/rpc v1.2.0
	k8s.io/client-go v0.17.4 => github.com/nirmata/client-go v0.17.5-0.20200625181911-7e81180b291e
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20200306081859-6a048a382944
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190612130303-4062e14deebe
)

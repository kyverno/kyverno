module github.com/kyverno/kyverno

go 1.13

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cornelk/hashmap v1.0.1
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/fatih/color v1.9.0 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/gardener/controller-manager-library v0.2.0
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.1.0
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/googleapis/gnostic v0.3.1
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/jmespath/go-jmespath v0.0.0-20180206201540-c2b33e8439af
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kr/pretty v0.2.0 // indirect
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/minio/minio v0.0.0-20200114012931-30922148fbb5
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/ory/go-acc v0.2.6 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.5.1
	github.com/tevino/abool v0.0.0-20170917061928-9b9efcf221b5
	golang.org/x/net v0.0.0-20201021035429-f5854403a974 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	gopkg.in/yaml.v2 v2.3.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.12
	k8s.io/apiextensions-apiserver v0.18.12
	k8s.io/apimachinery v0.18.12
	k8s.io/cli-runtime v0.18.12
	k8s.io/client-go v0.18.12
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
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20200306081859-6a048a382944
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190612130303-4062e14deebe
)

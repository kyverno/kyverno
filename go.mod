module github.com/kyverno/kyverno

go 1.14

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cornelk/hashmap v1.0.1
	github.com/dchest/siphash v1.2.1 // indirect
	github.com/distribution/distribution v2.7.1+incompatible
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.2.0
	github.com/fatih/color v1.12.0
	github.com/gardener/controller-manager-library v0.2.0
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-git/go-billy/v5 v5.0.0
	github.com/go-git/go-git/v5 v5.2.0
	github.com/go-logr/logr v0.4.0
	github.com/googleapis/gnostic v0.5.4
	github.com/jmespath/go-jmespath v0.4.0
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kataras/tablewriter v0.0.0-20180708051242-e063d29b7c23
	github.com/lensesio/tableprinter v0.0.0-20201125135848-89e81fc956e7
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a
	github.com/mattn/go-runewidth v0.0.7 // indirect
	github.com/minio/pkg v1.0.4
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/orcaman/concurrent-map v0.0.0-20190826125027-8c72a8bb44f6
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.8.0
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	golang.org/x/net v0.0.0-20210421230115-4e50805a0758 // indirect
	golang.org/x/sys v0.0.0-20210510120138-977fb7262007 // indirect
	golang.org/x/term v0.0.0-20210406210042-72f3dc4e9b72 // indirect
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.21.0
	k8s.io/apiextensions-apiserver v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/cli-runtime v0.20.2
	k8s.io/client-go v0.21.0
	k8s.io/klog/v2 v2.8.0
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	sigs.k8s.io/controller-runtime v0.8.1
	sigs.k8s.io/kustomize/api v0.7.0
	sigs.k8s.io/kustomize/kyaml v0.10.3
	sigs.k8s.io/yaml v1.2.0
)

// Added for go1.13 migration https://github.com/golang/go/issues/32805
replace (
	github.com/gorilla/rpc v1.2.0+incompatible => github.com/gorilla/rpc v1.2.0
	github.com/jmespath/go-jmespath => github.com/kyverno/go-jmespath v0.4.1-0.20210302163943-f30eab0a3ed6
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20200306081859-6a048a382944
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190612130303-4062e14deebe
)

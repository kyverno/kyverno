module github.com/kyverno/kyverno

go 1.16

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cornelk/hashmap v1.0.1
	github.com/dchest/siphash v1.2.1 // indirect
	github.com/distribution/distribution v2.7.1+incompatible
	github.com/docker/cli v20.10.10+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.5.0
	github.com/fatih/color v1.12.0
	github.com/gardener/controller-manager-library v0.2.0
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-git/go-billy/v5 v5.0.0
	github.com/go-git/go-git/v5 v5.2.0
	github.com/go-logr/logr v1.0.0
	github.com/google/go-containerregistry v0.6.0
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20210216200643-d81088d9983e
	github.com/googleapis/gnostic v0.5.5
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/in-toto/in-toto-golang v0.3.3
	github.com/jmespath/go-jmespath v0.4.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kataras/tablewriter v0.0.0-20180708051242-e063d29b7c23
	github.com/lensesio/tableprinter v0.0.0-20201125135848-89e81fc956e7
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a
	github.com/minio/pkg v1.1.3
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/orcaman/concurrent-map v0.0.0-20190826125027-8c72a8bb44f6
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/sigstore/cosign v1.2.1
	github.com/sigstore/sigstore v0.0.0-20210729211320-56a91f560f44
	github.com/spf13/cobra v1.2.1
	github.com/stretchr/testify v1.7.0
	github.com/xanzy/ssh-agent v0.3.0 // indirect
	golang.org/x/sys v0.0.0-20211103235746-7861aae1554b // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.22.4
	k8s.io/apiextensions-apiserver v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/cli-runtime v0.22.4
	k8s.io/client-go v0.22.4
	k8s.io/klog/v2 v2.30.0
	k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65
	sigs.k8s.io/controller-runtime v0.10.3
	sigs.k8s.io/kustomize/api v0.10.1
	sigs.k8s.io/kustomize/kyaml v0.13.0
	sigs.k8s.io/yaml v1.3.0
	github.com/aquilax/truncate v1.0.0
	github.com/blang/semver/v4 v4.0.0
	github.com/opencontainers/image-spec v1.0.2 // indirect
	gopkg.in/inf.v0 v0.9.1
)

replace (
	github.com/apache/thrift => github.com/apache/thrift v0.14.0
	github.com/buger/jsonparser => github.com/buger/jsonparser v1.1.1
	github.com/containerd/containerd => github.com/containerd/containerd v1.5.7
	github.com/docker/cli => github.com/docker/cli v20.10.9+incompatible
	github.com/evanphx/json-patch/v5 => github.com/kyverno/json-patch/v5 v5.5.1-0.20210915204938-7578f4ee9c77
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
	github.com/gorilla/rpc v1.2.0+incompatible => github.com/gorilla/rpc v1.2.0
	github.com/jmespath/go-jmespath => github.com/kyverno/go-jmespath v0.4.1-0.20210511164400-a1d46efa2ed6
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.10.0
	k8s.io/kubernetes => k8s.io/kubernetes v1.18.4
)

module github.com/kyverno/kyverno

go 1.16

require (
	github.com/GoogleContainerTools/skaffold v1.25.0 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cornelk/hashmap v1.0.1
	github.com/dchest/siphash v1.2.1 // indirect
	github.com/distribution/distribution v2.7.1+incompatible
	github.com/evanphx/json-patch/v5 v5.2.0
	github.com/fatih/color v1.12.0
	github.com/gardener/controller-manager-library v0.2.0
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-git/go-billy/v5 v5.0.0
	github.com/go-git/go-git/v5 v5.2.0
	github.com/go-logr/logr v0.4.0
	github.com/golangci/go-tools v0.0.0-20190318055746-e32c54105b7c // indirect
	github.com/golangci/golangci-lint v1.40.1 // indirect
	github.com/golangci/gosec v0.0.0-20190211064107-66fb7fc33547 // indirect
	github.com/google/go-containerregistry v0.5.1
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20210216200643-d81088d9983e
	github.com/google/monologue v0.0.0-20190606152607-4b11a32b5934 // indirect
	github.com/google/trillian-examples v0.0.0-20190603134952-4e75ba15216c // indirect
	github.com/googleapis/gnostic v0.5.4
	github.com/jmespath/go-jmespath v0.4.0
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kataras/tablewriter v0.0.0-20180708051242-e063d29b7c23
	github.com/lensesio/tableprinter v0.0.0-20201125135848-89e81fc956e7
	github.com/letsencrypt/pkcs11key v2.0.1-0.20170608213348-396559074696+incompatible // indirect
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a
	github.com/mattn/go-sqlite3 v1.10.0 // indirect
	github.com/minio/minio v0.0.0-20200114012931-30922148fbb5
	github.com/minio/pkg v1.0.7
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.11.0
	github.com/orcaman/concurrent-map v0.0.0-20190826125027-8c72a8bb44f6
	github.com/ory/go-acc v0.2.6 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/sassoftware/relic v7.2.1+incompatible // indirect
	github.com/sigstore/cosign v1.0.1
	github.com/sigstore/sigstore v0.0.0-20210726180807-7e34e36ecda1
	github.com/spf13/cobra v1.2.1
	github.com/stretchr/testify v1.7.0
	go.etcd.io/etcd v3.3.13+incompatible // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.1
	k8s.io/apimachinery v0.21.3
	k8s.io/cli-runtime v0.21.1
	k8s.io/client-go v0.21.3
	k8s.io/klog/v2 v2.9.0
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	sigs.k8s.io/controller-runtime v0.8.1
	sigs.k8s.io/kustomize/api v0.8.8
	sigs.k8s.io/kustomize/kyaml v0.10.17
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/evanphx/json-patch/v5 => github.com/kacejot/json-patch/v5 v5.3.1-0.20210513152033-7395b4a9e87f
	github.com/gorilla/rpc v1.2.0+incompatible => github.com/gorilla/rpc v1.2.0
	github.com/jmespath/go-jmespath => github.com/kyverno/go-jmespath v0.4.1-0.20210511164400-a1d46efa2ed6
)

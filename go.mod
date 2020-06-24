module github.com/nirmata/kyverno

go 1.13

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/googleapis/gnostic v0.4.1
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/jmespath/go-jmespath v0.0.0-20180206201540-c2b33e8439af
	github.com/minio/minio v0.0.0-20200114012931-30922148fbb5
	github.com/spf13/cobra v0.0.5
	github.com/tevino/abool v0.0.0-20170917061928-9b9efcf221b5
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	gopkg.in/yaml.v2 v2.2.8
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0-20200624085548-6f8e0fa87f2f
	k8s.io/apimachinery v0.0.0-20200624084815-eed6a5257d68
	k8s.io/cli-runtime v0.0.0-20191004110135-b9eb767d2e1a
	k8s.io/client-go v11.0.1-0.20190516230509-ae8359b20417+incompatible
	k8s.io/kube-openapi v0.0.0-20200427153329-656914f816f9
	sigs.k8s.io/kustomize v2.0.3+incompatible // indirect
)

// Added for go1.13 migration https://github.com/golang/go/issues/32805
replace github.com/gorilla/rpc v1.2.0+incompatible => github.com/gorilla/rpc v1.2.0

replace k8s.io/client-go => github.com/nirmata/client-go v0.0.0-20200624195028-e04fdc85f7b8

module github.com/kyverno/kyverno

go 1.22.2

require (
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20230811130428-ced1acdcaa24
	github.com/AdamKorcz/go-fuzz-headers-1 v0.0.0-20230919221257-8b5d3ce2d11d
	github.com/IGLOU-EU/go-wildcard v1.0.3
	github.com/Masterminds/sprig/v3 v3.2.3
	github.com/alitto/pond v1.8.3
	github.com/aquilax/truncate v1.0.0
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240525144225-0fe7eafab216
	github.com/blang/semver/v4 v4.0.0
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cyphar/filepath-securejoin v0.2.5
	github.com/dgraph-io/ristretto v0.1.1
	github.com/distribution/reference v0.6.0
	github.com/evanphx/json-patch/v5 v5.9.0
	github.com/fatih/color v1.17.0
	github.com/fluxcd/pkg/oci v0.36.0
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-git/go-billy/v5 v5.5.0
	github.com/go-git/go-git/v5 v5.12.0
	github.com/go-logr/logr v1.4.2
	github.com/go-logr/zapr v1.3.0
	github.com/google/gnostic-models v0.6.9-0.20230804172637-c7be7c783f49
	github.com/google/go-containerregistry v0.19.1
	github.com/google/go-containerregistry/pkg/authn/kubernetes v0.0.0-20240530172801-3764db238e3e
	github.com/in-toto/in-toto-golang v0.9.0
	github.com/jmoiron/jsonq v0.0.0-20150511023944-e874b168d07e
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kataras/tablewriter v0.0.0-20180708051242-e063d29b7c23
	github.com/kyverno/go-jmespath v0.4.1-0.20231124160150-95e59c162877
	github.com/kyverno/kyverno-json v0.0.3-alpha.2.0.20240530232232-5c38b4fee0b1
	github.com/lensesio/tableprinter v0.0.0-20201125135848-89e81fc956e7
	github.com/notaryproject/notation-core-go v1.0.3
	github.com/notaryproject/notation-go v1.1.1
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.33.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.19.0
	github.com/robfig/cron v1.2.0
	github.com/sigstore/cosign/v2 v2.2.4
	github.com/sigstore/k8s-manifest-sigstore v0.5.4
	github.com/sigstore/rekor v1.3.6
	github.com/sigstore/sigstore v1.8.4
	github.com/sigstore/sigstore/pkg/signature/kms/aws v1.8.4
	github.com/sigstore/sigstore/pkg/signature/kms/azure v1.8.4
	github.com/sigstore/sigstore/pkg/signature/kms/gcp v1.8.4
	github.com/sigstore/sigstore/pkg/signature/kms/hashivault v1.8.4
	github.com/spf13/cobra v1.8.0
	github.com/stretchr/testify v1.9.0
	github.com/zach-klippenstein/goregen v0.0.0-20160303162051-795b5e3961ea
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.51.0
	go.opentelemetry.io/otel v1.26.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.26.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.26.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.26.0
	go.opentelemetry.io/otel/exporters/prometheus v0.46.0
	go.opentelemetry.io/otel/metric v1.26.0
	go.opentelemetry.io/otel/sdk v1.26.0
	go.opentelemetry.io/otel/sdk/metric v1.26.0
	go.opentelemetry.io/otel/trace v1.26.0
	go.uber.org/automaxprocs v1.5.3
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.23.0
	golang.org/x/text v0.15.0
	gomodules.xyz/jsonpatch/v2 v2.4.0
	google.golang.org/grpc v1.64.0
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/yaml.v2 v2.4.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.30.1
	k8s.io/apiextensions-apiserver v0.30.1
	k8s.io/apimachinery v0.30.1
	k8s.io/apiserver v0.30.1
	k8s.io/cli-runtime v0.30.1
	k8s.io/client-go v0.30.1
	k8s.io/klog/v2 v2.120.1
	k8s.io/kube-aggregator v0.30.1
	k8s.io/pod-security-admission v0.30.1
	k8s.io/utils v0.0.0-20240502163921-fe8a2dddb1d0
	sigs.k8s.io/controller-runtime v0.17.3
	sigs.k8s.io/kubectl-validate v0.0.4
	sigs.k8s.io/kustomize/api v0.17.2
	sigs.k8s.io/kustomize/kyaml v0.17.1
	sigs.k8s.io/release-utils v0.8.2
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1
	sigs.k8s.io/yaml v1.4.0
)

require github.com/notaryproject/notation-plugin-framework-go v1.0.0 // indirect

require (
	cloud.google.com/go v0.113.0 // indirect
	cloud.google.com/go/auth v0.4.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.2 // indirect
	cloud.google.com/go/longrunning v0.5.7 // indirect
)

require (
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	cloud.google.com/go/iam v1.1.8 // indirect
	cloud.google.com/go/kms v1.17.1 // indirect
	cuelabs.dev/go/oci/ociregistry v0.0.0-20240314152124-224736b49f2e // indirect
	cuelang.org/go v0.8.1 // indirect
	dario.cat/mergo v1.0.0 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/AliyunContainerService/ack-ram-tool/pkg/credentials/alibabacloudsdkgo/helper v0.2.0 // indirect
	github.com/Azure/azure-sdk-for-go v68.0.0+incompatible // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.11.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.5.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.5.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys v1.1.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/internal v1.0.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.29 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.23 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.12 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.6 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/Azure/go-ntlmssp v0.0.0-20221128193559-754e69321358 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/ProtonMail/go-crypto v1.0.0 // indirect
	github.com/ThalesIgnite/crypto11 v1.2.5 // indirect
	github.com/agnivade/levenshtein v1.1.1 // indirect
	github.com/alibabacloud-go/alibabacloud-gateway-spi v0.0.4 // indirect
	github.com/alibabacloud-go/cr-20160607 v1.0.1 // indirect
	github.com/alibabacloud-go/cr-20181201 v1.0.10 // indirect
	github.com/alibabacloud-go/darabonba-openapi v0.2.1 // indirect
	github.com/alibabacloud-go/debug v1.0.0 // indirect
	github.com/alibabacloud-go/endpoint-util v1.1.1 // indirect
	github.com/alibabacloud-go/openapi-util v0.1.0 // indirect
	github.com/alibabacloud-go/tea v1.2.2 // indirect
	github.com/alibabacloud-go/tea-utils v1.4.5 // indirect
	github.com/alibabacloud-go/tea-xml v1.1.3 // indirect
	github.com/aliyun/credentials-go v1.3.2 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230305170008-8188dc5388df // indirect
	github.com/aptible/supercronic v0.2.29
	github.com/aws/aws-sdk-go-v2 v1.27.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.16 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.16 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.28.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.23.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/kms v1.32.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.20.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.24.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.28.10 // indirect
	github.com/aws/smithy-go v1.20.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/buildkite/agent/v3 v3.62.0 // indirect
	github.com/buildkite/go-pipeline v0.3.2 // indirect
	github.com/buildkite/interpolate v0.0.0-20200526001904-07f35b4ae251 // indirect
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chrismellard/docker-credential-acr-env v0.0.0-20230304212654-82a0ddb27589 // indirect
	github.com/clbanning/mxj/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/cockroachdb/apd/v3 v3.2.1 // indirect
	github.com/common-nighthawk/go-figure v0.0.0-20210622060536-734e95fb86be // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.15.1 // indirect
	github.com/coreos/go-oidc/v3 v3.10.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.3 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20231217050601-ba74d44ecf5f // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/digitorus/pkcs7 v0.0.0-20230818184609-3a137a874352 // indirect
	github.com/digitorus/timestamp v0.0.0-20231217203849-220c5c2851b7 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/djherbis/times v1.6.0 // indirect
	github.com/docker/cli v25.0.1+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v25.0.5+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.2 // indirect
	github.com/emicklei/proto v1.13.2 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/evanphx/json-patch v5.9.0+incompatible // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.6.0 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.5 // indirect
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-jose/go-jose/v3 v3.0.3 // indirect
	github.com/go-jose/go-jose/v4 v4.0.1 // indirect
	github.com/go-ldap/ldap/v3 v3.4.8 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v0.23.0 // indirect
	github.com/go-openapi/errors v0.22.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/loads v0.22.0 // indirect
	github.com/go-openapi/runtime v0.28.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/strfmt v0.23.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-openapi/validate v0.24.0 // indirect
	github.com/go-piv/piv-go v1.11.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/golang/glog v1.2.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/cel-go v0.20.1 // indirect
	github.com/google/certificate-transparency-go v1.1.7 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-github/v55 v55.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.4 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.1-0.20210315223345-82c243799c99 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.6 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.1.8 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.6 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-5 // indirect
	github.com/hashicorp/vault/api v1.14.0 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jedisct1/go-minisign v0.0.0-20230811132847-661be99b8267 // indirect
	github.com/jellydator/ttlcache/v3 v3.2.0 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/jmespath-community/go-jmespath v1.1.2-0.20240117150817-e430401a2172 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.17.8 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/kyverno/pkg/ext v0.0.0-20240418121121-df8add26c55c // indirect
	github.com/letsencrypt/boulder v0.0.0-20240127020530-97a19b18d21e // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/mozillazg/docker-credential-acr-helper v0.3.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nozzle/throttler v0.0.0-20180817012639-2ea982251481 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/oleiade/reflections v1.0.1 // indirect
	github.com/oliveagle/jsonpath v0.0.0-20180606110733-2e52cf6e6852 // indirect
	github.com/open-policy-agent/gatekeeper/v3 v3.14.0 // indirect
	github.com/open-policy-agent/opa v0.61.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pelletier/go-toml/v2 v2.1.1 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.0 // indirect
	github.com/prometheus/common v0.48.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/protocolbuffers/txtpbfmt v0.0.0-20240116145035-ef3ab179eed6 // indirect
	github.com/r3labs/diff v1.1.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/rivo/uniseg v0.4.6 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sassoftware/relic v7.2.1+incompatible // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.8.0 // indirect
	github.com/segmentio/ksuid v1.0.4 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sigstore/fulcio v1.4.3 // indirect
	github.com/sigstore/timestamp-authority v1.2.2 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/skeema/knownhosts v1.2.2 // indirect
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.18.2 // indirect
	github.com/spiffe/go-spiffe/v2 v2.2.0 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/tchap/go-patricia/v2 v2.3.1 // indirect
	github.com/tektoncd/chains v0.19.0 // indirect
	github.com/thales-e-security/pool v0.0.2 // indirect
	github.com/theupdateframework/go-tuf v0.7.0 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/transparency-dev/merkle v0.0.2 // indirect
	github.com/vbatts/tar-split v0.11.5 // indirect
	github.com/veraison/go-cose v1.2.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xanzy/go-gitlab v0.102.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	github.com/yashtewari/glob-intersection v0.2.0 // indirect
	github.com/zeebo/errs v1.3.0 // indirect
	go.etcd.io/etcd/api/v3 v3.5.13 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.13 // indirect
	go.etcd.io/etcd/client/v3 v3.5.13 // indirect
	go.mongodb.org/mongo-driver v1.14.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	go.opentelemetry.io/proto/otlp v1.2.0 // indirect
	go.starlark.net v0.0.0-20240123142251-f86470692795 // indirect
	go.step.sm/crypto v0.44.2 // indirect
	golang.org/x/exp v0.0.0-20240222234643-814bf88cf225
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/oauth2 v0.20.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/term v0.20.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.21.0 // indirect
	google.golang.org/api v0.181.0 // indirect
	google.golang.org/genproto v0.0.0-20240401170217-c3f982113cda // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240513163218-0867130af1f8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240513163218-0867130af1f8 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/go-jose/go-jose.v2 v2.6.3 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/component-base v0.30.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340
	k8s.io/kubectl v0.29.2 // indirect
	oras.land/oras-go/v2 v2.5.0 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.29.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
)

replace (
	github.com/google/cel-go v0.20.1 => github.com/google/cel-go v0.17.8
	github.com/prometheus/client_golang v1.19.0 => github.com/prometheus/client_golang v1.18.0
	github.com/prometheus/common v0.48.0 => github.com/prometheus/common v0.44.0
	github.com/sigstore/cosign/v2 v2.2.4 => github.com/kyverno/cosign/v2 v2.2.4-deps-fix
	k8s.io/pod-security-admission v0.30.1 => github.com/YTGhost/pod-security-admission v0.22.0-beta.0.0.20240603173423-11663473ae49
)

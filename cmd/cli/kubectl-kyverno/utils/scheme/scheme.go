package scheme

import (
	"sync"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	scheme  = runtime.NewScheme()
	codecs  = serializer.NewCodecFactory(scheme)
	Decoder = codecs.UniversalDeserializer()
	once    sync.Once
)

func Setup() {
	once.Do(func() {
		_ = apiextensionsv1.AddToScheme(scheme)
	})
}

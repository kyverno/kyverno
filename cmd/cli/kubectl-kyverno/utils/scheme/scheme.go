package scheme

import (
	"sync"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	Decoder runtime.Decoder
	once    sync.Once
)

func Setup() {
	once.Do(func() {
		scheme := runtime.NewScheme()
		_ = apiextensionsv1.AddToScheme(scheme)
		codecs := serializer.NewCodecFactory(scheme)
		Decoder = codecs.UniversalDeserializer()
	})
}

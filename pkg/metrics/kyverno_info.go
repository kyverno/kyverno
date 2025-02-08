package metrics

import "github.com/kyverno/kyverno/pkg/version"

type KyvernoInfoStruct struct {
	Version string
}

func GetKyvernoInfo() KyvernoInfoStruct {
	ver := version.Version()
	return KyvernoInfoStruct{ver}
}

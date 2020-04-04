package apply

import (
	"fmt"
	"strings"

	"github.com/nirmata/kyverno/pkg/openapi"
)

func getListEndpointForKind(kind string, openAPIController *openapi.Controller) (string, error) {

	definitionName := openAPIController.GetDefinitionNameFromKind(kind)
	definitionNameWithoutPrefix := strings.Replace(definitionName, "io.k8s.", "", -1)

	parts := strings.Split(definitionNameWithoutPrefix, ".")
	definitionPrefix := strings.Join(parts[:len(parts)-1], ".")

	defPrefixToApiPrefix := map[string]string{
		"api.core.v1":                  "/api/v1",
		"api.apps.v1":                  "/apis/apps/v1",
		"api.batch.v1":                 "/apis/batch/v1",
		"api.admissionregistration.v1": "/apis/admissionregistration.k8s.io/v1",
		"kube-aggregator.pkg.apis.apiregistration.v1":       "/apis/apiregistration.k8s.io/v1",
		"apiextensions-apiserver.pkg.apis.apiextensions.v1": "/apis/apiextensions.k8s.io/v1",
		"api.autoscaling.v1":                                "/apis/autoscaling/v1/",
		"api.storage.v1":                                    "/apis/storage.k8s.io/v1",
		"api.coordination.v1":                               "/apis/coordination.k8s.io/v1",
		"api.scheduling.v1":                                 "/apis/scheduling.k8s.io/v1",
		"api.rbac.v1":                                       "/apis/rbac.authorization.k8s.io/v1",
	}

	if defPrefixToApiPrefix[definitionPrefix] == "" {
		return "", fmt.Errorf("Unsupported resource type %v", kind)
	}

	return defPrefixToApiPrefix[definitionPrefix] + "/" + strings.ToLower(kind) + "s", nil
}

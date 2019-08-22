package engine

import (
	"encoding/json"
	"strings"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
)

func patchOverlay(rule kyverno.Rule, rawResource []byte) ([][]byte, error) {
	var resource interface{}
	if err := json.Unmarshal(rawResource, &resource); err != nil {
		return nil, err
	}
	//TODO: evaluate, Unmarshall called thrice
	resourceInfo := ParseResourceInfoFromObject(rawResource)
	patches, err := processOverlayPatches(resource, rule.Mutation.Overlay)
	if err != nil && strings.Contains(err.Error(), "Conditions are not met") {
		glog.Infof("Resource does not meet conditions in overlay pattern, resource=%s, rule=%s\n", resourceInfo, rule.Name)
		return nil, nil
	}

	return patches, err
}

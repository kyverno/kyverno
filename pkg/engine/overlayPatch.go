package engine

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/golang/glog"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
)

func patchOverlay(rule kubepolicy.Rule, rawResource []byte) ([][]byte, error) {
	var resource interface{}
	if err := json.Unmarshal(rawResource, &resource); err != nil {
		return nil, err
	}

	resourceInfo := ParseResourceInfoFromObject(rawResource)
	patches, err := processOverlayPatches(resource, *rule.Mutation.Overlay)
	if err != nil && strings.Contains(err.Error(), "Conditions are not met") {
		glog.Infof("Resource does not meet conditions in overlay pattern, resource=%s, rule=%s\n", resourceInfo, rule.Name)
		return nil, nil
	}

	return patches, err
}

func processOverlayPatches(resource, overlay interface{}) ([][]byte, error) {

	if !meetConditions(resource, overlay) {
		return nil, errors.New("Conditions are not met")
	}

	return mutateResourceWithOverlay(resource, overlay)
}

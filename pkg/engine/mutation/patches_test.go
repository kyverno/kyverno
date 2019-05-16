package mutation

import (
	"testing"

	"gotest.tools/assert"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
)

const endpointsDocument string = `{
	"kind": "Endpoints",
	"apiVersion": "v1",
	"metadata": {
		"name": "my-endpoint-service",
		"labels": {
			"originalLabel": "isHere"
		}
	},
	"subsets": [
		{
			"addresses": [
				{
					"ip": "1.2.3.4"
				}
			],
			"ports": [
				{
					"port": 9376
				}
			]
		}
	]
}`

func TestProcessPatches_EmptyPatches(t *testing.T) {
	var empty []types.Patch
	patches, err := ProcessPatches(empty, []byte(endpointsDocument))
	assert.NilError(t, err)
	assert.Assert(t, len(patches) == 0)
}

func makeAddIsMutatedLabelPatch() types.Patch {
	return types.Patch{
		Path:      "/metadata/labels/is-mutated",
		Operation: "add",
		Value:     "true",
	}
}

func TestProcessPatches_EmptyDocument(t *testing.T) {
	var patches []types.Patch
	patches = append(patches, makeAddIsMutatedLabelPatch())
	patchesBytes, err := ProcessPatches(patches, nil)
	assert.Assert(t, err != nil)
	assert.Assert(t, len(patchesBytes) == 0)
}

func TestProcessPatches_AllEmpty(t *testing.T) {
	patchesBytes, err := ProcessPatches(nil, nil)
	assert.Assert(t, err != nil)
	assert.Assert(t, len(patchesBytes) == 0)
}

func TestProcessPatches_AddPathDoesntExist(t *testing.T) {
	patch := makeAddIsMutatedLabelPatch()
	patch.Path = "/metadata/additional/is-mutated"
	patches := []types.Patch{patch}
	patchesBytes, err := ProcessPatches(patches, []byte(endpointsDocument))
	assert.NilError(t, err)
	assert.Assert(t, len(patchesBytes) == 0)
}

func TestProcessPatches_RemovePathDoesntExist(t *testing.T) {
	patch := types.Patch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patches := []types.Patch{patch}
	patchesBytes, err := ProcessPatches(patches, []byte(endpointsDocument))
	assert.NilError(t, err)
	assert.Assert(t, len(patchesBytes) == 0)
}

func TestProcessPatches_AddAndRemovePathsDontExist_EmptyResult(t *testing.T) {
	patch1 := types.Patch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := types.Patch{Path: "/spec/labels/label3", Operation: "add", Value: "label3Value"}
	patches := []types.Patch{patch1, patch2}
	patchesBytes, err := ProcessPatches(patches, []byte(endpointsDocument))
	assert.NilError(t, err)
	assert.Assert(t, len(patchesBytes) == 0)
}

func TestProcessPatches_AddAndRemovePathsDontExist_ContinueOnError_NotEmptyResult(t *testing.T) {
	patch1 := types.Patch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := types.Patch{Path: "/spec/labels/label2", Operation: "remove", Value: "label2Value"}
	patch3 := types.Patch{Path: "/metadata/labels/label3", Operation: "add", Value: "label3Value"}
	patches := []types.Patch{patch1, patch2, patch3}
	patchesBytes, err := ProcessPatches(patches, []byte(endpointsDocument))
	assert.NilError(t, err)
	assert.Assert(t, len(patchesBytes) == 1)
	assertEqStringAndData(t, `{"path":"/metadata/labels/label3","op":"add","value":"label3Value"}`, patchesBytes[0])
}

func TestProcessPatches_RemovePathDoesntExist_EmptyResult(t *testing.T) {
	patch := types.Patch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patches := []types.Patch{patch}
	patchesBytes, err := ProcessPatches(patches, []byte(endpointsDocument))
	assert.NilError(t, err)
	assert.Assert(t, len(patchesBytes) == 0)
}

func TestProcessPatches_RemovePathDoesntExist_NotEmptyResult(t *testing.T) {
	patch1 := types.Patch{Path: "/metadata/labels/is-mutated", Operation: "remove"}
	patch2 := types.Patch{Path: "/metadata/labels/label2", Operation: "add", Value: "label2Value"}
	patches := []types.Patch{patch1, patch2}
	patchesBytes, err := ProcessPatches(patches, []byte(endpointsDocument))
	assert.NilError(t, err)
	assert.Assert(t, len(patchesBytes) == 1)
	assertEqStringAndData(t, `{"path":"/metadata/labels/label2","op":"add","value":"label2Value"}`, patchesBytes[0])
}

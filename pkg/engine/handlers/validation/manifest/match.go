//
// Copyright 2020 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Adapted from sigstore/k8s-manifest-sigstore/pkg/k8smanifest/verify_resource.go
// at commit 4152ec03ed3d. Upstream does not export its resource-matching helpers.
// Keeping this matching/dry-run logic local lets the adapter guard registry and
// Sigstore I/O without copying the dependency's network or signature code.
package manifest

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	kubeutil "github.com/sigstore/k8s-manifest-sigstore/pkg/util/kubeutil"
	mapnode "github.com/sigstore/k8s-manifest-sigstore/pkg/util/mapnode"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	defaultDryRunNamespace           = "default"
	imageCanonicalizePatternRegistry = "docker.io/"
	imageCanonicalizePatternTag      = ":latest"
)

func matchResourceWithManifest(obj unstructured.Unstructured, foundManifestBytes []byte, ignoreFields []string, dryRunNamespace string, disableDryRun, checkDryRunForApply, checkMutatingResource bool) (bool, *mapnode.DiffResult, error) {
	apiVersion := obj.GetAPIVersion()
	kind := obj.GetKind()
	name := obj.GetName()
	namespace := obj.GetNamespace()
	clusterScope := false
	if namespace == "" {
		clusterScope = true
	}
	if !clusterScope && dryRunNamespace == "" {
		dryRunNamespace = defaultDryRunNamespace
	}
	isCRD := kind == "CustomResourceDefinition"

	log.Debug("obj: apiVersion", apiVersion, "kind", kind, "name", name)
	log.Debug("manifest in the message:", string(foundManifestBytes))

	var err error
	var matched bool
	var diff *mapnode.DiffResult
	objBytes, _ := json.Marshal(obj.Object)

	// CASE1: direct match
	log.Debug("try direct matching")
	// diff: found differences between two objects
	// `before`: attributes in the original YAML manifest (or dryrun data based on it),
	// `after`: attributes in the specified resource (or the resource in admission request in case of admission check)
	matched, diff, err = directMatch(foundManifestBytes, objBytes)
	if err != nil {
		return false, nil, errors.Wrap(err, "error occurred during direct match")
	}
	if diff != nil && len(ignoreFields) > 0 {
		_, diff, _ = diff.Filter(ignoreFields)
	}
	// namespace in the original manifest could be empty, so ignore it
	if diff != nil && diff.Size() > 0 {
		namespacePattern := &mapnode.DiffPattern{
			Key: "metadata.namespace", Values: map[string]interface{}{"before": nil},
		}
		diff = diff.Remove([]*mapnode.DiffPattern{namespacePattern})
	}
	if diff == nil || diff.Size() == 0 {
		matched = true
		diff = nil
	}
	if matched {
		return true, nil, nil
	} else {
		diffStr := ""
		if diff != nil {
			diffStr = diff.ToJson()
		}
		log.Debugf("found diff by direct match: %s", diffStr)
	}

	var dryRunBytes []byte
	// CASE2: dryrun create match
	if !disableDryRun {
		log.Debug("try dryrun create matching")
		matched, diff, dryRunBytes, err = dryrunCreateMatch(foundManifestBytes, objBytes, clusterScope, isCRD, dryRunNamespace)
		if err != nil {
			return false, nil, errors.Wrap(err, "error occurred during dryrun create match")
		}
		if diff != nil && len(ignoreFields) > 0 {
			_, diff, _ = diff.Filter(ignoreFields)
		}
		if diff == nil || diff.Size() == 0 {
			matched = true
			diff = nil
		}
		if matched {
			return true, nil, nil
		} else {
			diffStr := ""
			if diff != nil {
				diffStr = diff.ToJson()
			}
			log.Debugf("found diff by dryrun create matching: %s", diffStr)
		}
	}

	// CASE3: dryrun apply match
	if !disableDryRun && checkDryRunForApply {
		log.Debug("try dryrun apply matching")
		matched, diff, err = dryrunApplyMatch(foundManifestBytes, objBytes, clusterScope, isCRD, dryRunNamespace)
		if err != nil {
			return false, nil, errors.Wrap(err, "error occurred during dryrun apply match")
		}
		if diff != nil && len(ignoreFields) > 0 {
			_, diff, _ = diff.Filter(ignoreFields)
		}
		if diff == nil || diff.Size() == 0 {
			matched = true
			diff = nil
		}
		if matched {
			return true, nil, nil
		} else {
			diffStr := ""
			if diff != nil {
				diffStr = diff.ToJson()
			}
			log.Debugf("found diff by dryrun apply matching: %s", diffStr)
		}
	}

	// CASE4: manifest match for a resource in mutating admission controller
	if checkMutatingResource {
		log.Debug("try mutating resource matching (check inclusion relation between manifest, resource and dryrun result)")
		matched, diff, err = inclusionMatch(foundManifestBytes, objBytes, dryRunBytes, clusterScope, isCRD, disableDryRun)
		if err != nil {
			return false, nil, errors.Wrap(err, "error occurred during mutating resource matching")
		}
		if diff != nil && len(ignoreFields) > 0 {
			_, diff, _ = diff.Filter(ignoreFields)
		}
		if diff == nil || diff.Size() == 0 {
			matched = true
			diff = nil
		}
		if matched {
			return true, nil, nil
		} else {
			diffStr := ""
			if diff != nil {
				diffStr = diff.ToJson()
			}
			log.Debugf("found diff by mutating resource matching: %s", diffStr)
		}
	}

	return false, diff, nil
}

func directMatch(messageYAMLBytes, resourceJSONBytes []byte) (bool, *mapnode.DiffResult, error) {
	mnfNode, err := mapnode.NewFromYamlBytes(messageYAMLBytes)
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to initialize manifest node")
	}
	objNode, err := mapnode.NewFromBytes(resourceJSONBytes)
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to initialize object node")
	}
	diff := mnfNode.Diff(objNode)
	diff = removeCanonicalizedImageDiff(diff)
	if diff == nil || diff.Size() == 0 {
		return true, nil, nil
	}
	return false, diff, nil
}

func dryrunCreateMatch(messageYAMLBytes, resourceJSONBytes []byte, clusterScope, isCRD bool, dryRunNamespace string) (bool, *mapnode.DiffResult, []byte, error) {
	mnfNode, err := mapnode.NewFromYamlBytes(messageYAMLBytes)
	if err != nil {
		return false, nil, nil, errors.Wrap(err, "failed to initialize manifest node")
	}
	objNode, err := mapnode.NewFromBytes(resourceJSONBytes)
	if err != nil {
		return false, nil, nil, errors.Wrap(err, "failed to initialize object node")
	}
	nsMaskedManifestBytes := mnfNode.Mask([]string{"metadata.namespace"}).ToYaml()
	var simBytes []byte
	if clusterScope {
		simBytes, err = kubeutil.DryRunCreate([]byte(nsMaskedManifestBytes), "")
	} else {
		simBytes, err = kubeutil.DryRunCreate([]byte(nsMaskedManifestBytes), dryRunNamespace)
	}
	if err != nil {
		return false, nil, nil, errors.Wrap(err, "failed to dryrun with the found YAML in image")
	}
	simNode, err := mapnode.NewFromYamlBytes(simBytes)
	if err != nil {
		return false, nil, simBytes, errors.Wrap(err, "failed to initialize dry-run-generated object node")
	}
	mask := []string{}
	mask = append(mask, "metadata.name") // name is overwritten for dryrun like `sample-configmap-dryrun`
	if !clusterScope {
		mask = append(mask, "metadata.namespace") // namespace is overwritten for dryrun
	}
	if isCRD {
		mask = append(mask, "spec.names.kind")
		mask = append(mask, "spec.names.listKind")
		mask = append(mask, "spec.names.singular")
		mask = append(mask, "spec.names.plural")
	}
	maskedObjNode := objNode.Mask(mask)
	maskedSimNode := simNode.Mask(mask)
	diff := maskedSimNode.Diff(maskedObjNode)
	diff = removeCanonicalizedImageDiff(diff)
	if diff == nil || diff.Size() == 0 {
		return true, nil, simBytes, nil
	}
	return false, diff, simBytes, nil
}

func dryrunApplyMatch(messageYAMLBytes, resourceJSONBytes []byte, clusterScope, isCRD bool, dryRunNamespace string) (bool, *mapnode.DiffResult, error) {
	objNode, err := mapnode.NewFromBytes(resourceJSONBytes)
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to initialize object node")
	}
	objNamespace := objNode.GetString("metadata.namespace")
	_, patchedBytes, err := kubeutil.GetApplyPatchBytes(messageYAMLBytes, objNamespace)
	if err != nil {
		return false, nil, errors.Wrap(err, "error during getting applied bytes")
	}
	patchedNode, _ := mapnode.NewFromBytes(patchedBytes)
	nsMaskedPatchedNode := patchedNode.Mask([]string{"metadata.namespace"})
	var simPatchedObj []byte
	if clusterScope {
		simPatchedObj, err = kubeutil.DryRunCreate([]byte(nsMaskedPatchedNode.ToYaml()), "")
	} else {
		simPatchedObj, err = kubeutil.DryRunCreate([]byte(nsMaskedPatchedNode.ToYaml()), dryRunNamespace)
	}
	if err != nil {
		return false, nil, errors.Wrap(err, "error during DryRunCreate for apply")
	}
	simNode, _ := mapnode.NewFromYamlBytes(simPatchedObj)
	mask := []string{}
	mask = append(mask, "metadata.name") // name is overwritten for dryrun like `sample-configmap-dryrun`
	if !clusterScope {
		mask = append(mask, "metadata.namespace") // namespace is overwritten for dryrun
	}
	if isCRD {
		mask = append(mask, "spec.names.kind")
		mask = append(mask, "spec.names.listKind")
		mask = append(mask, "spec.names.singular")
		mask = append(mask, "spec.names.plural")
	}
	maskedObjNode := objNode.Mask(mask)
	maskedSimNode := simNode.Mask(mask)
	diff := maskedSimNode.Diff(maskedObjNode)
	diff = removeCanonicalizedImageDiff(diff)
	if diff == nil || diff.Size() == 0 {
		return true, nil, nil
	}
	return false, diff, nil
}

// Check inclusion relation between manifest, object and dryrun result.
// This function covers verification for an object which is mutated by the following steps.
// STEPS) Manifest --(mutating webhook A)--> Checking Object --(mutating webhook B)--> DryRun Result
// This is basically useful for verification in mutating admission controller.
func inclusionMatch(messageYAMLBytes, resourceJSONBytes, dryRunBytes []byte, clusterScope, isCRD, disableDryRun bool) (bool, *mapnode.DiffResult, error) {
	mnfNode, err := mapnode.NewFromYamlBytes(messageYAMLBytes)
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to initialize manifest node")
	}
	objNode, err := mapnode.NewFromBytes(resourceJSONBytes)
	if err != nil {
		return false, nil, errors.Wrap(err, "failed to initialize object node")
	}
	mask := []string{}
	mask = append(mask, "metadata.name") // name is overwritten for dryrun like `sample-configmap-dryrun`
	if !clusterScope {
		mask = append(mask, "metadata.namespace") // namespace is overwritten for dryrun
	}
	if isCRD {
		mask = append(mask, "spec.names.kind")
		mask = append(mask, "spec.names.listKind")
		mask = append(mask, "spec.names.singular")
		mask = append(mask, "spec.names.plural")
	}
	maskedMnfNode := mnfNode.Mask(mask)
	maskedObjNode := objNode.Mask(mask)
	var diff1, diff2 *mapnode.DiffResult
	// find diffs but ignore newly added attributes
	diff1 = maskedMnfNode.FindUpdatedAndDeleted(maskedObjNode)
	diff1 = removeCanonicalizedImageDiff(diff1)
	diff1ok := (diff1 == nil || diff1.Size() == 0)
	var diff2ok bool
	if disableDryRun {
		// if dry run is disabled, just evaluate diffs between the manifest and the object in admission request
		diff2ok = true
	} else {
		simNode, err := mapnode.NewFromYamlBytes(dryRunBytes)
		if err != nil {
			return false, nil, errors.Wrap(err, "failed to initialize dry-run-generated object node")
		}
		maskedSimNode := simNode.Mask(mask)
		diff2 = inverseDiff(maskedObjNode.FindUpdatedAndDeleted(maskedSimNode)) // simNode is based on manifest, so inverse diff results
		diff2 = removeCanonicalizedImageDiff(diff2)
		diff2ok = (diff2 == nil || diff2.Size() == 0)
	}

	if diff1ok && diff2ok {
		return true, nil, nil
	}
	reportDiff := diff1
	if diff2 != nil {
		reportDiff = diff2
	}
	return false, reportDiff, nil
}

// remove a diff result caused by image canonicalization
// diffs like below can be ignored, so remove them from the result
// e.g.) nginx:1.14.2 --> docker.io/nginx:1.14.2
//
//	ubuntu --> ubuntu:latest
func removeCanonicalizedImageDiff(diff *mapnode.DiffResult) *mapnode.DiffResult {
	if diff == nil || diff.Size() == 0 {
		return nil
	}
	items := []mapnode.Difference{}
	for _, d := range diff.Items {
		keep := true
		// check only if the key has suffix "image"
		if strings.HasSuffix(d.Key, "image") {
			var image1, image2 string
			var ok1, ok2 bool
			image1, ok1 = d.Values["before"].(string)
			image2, ok2 = d.Values["after"].(string)
			if ok1 && ok2 {
				// pattern 1) nginx:1.14.2 --> docker.io/nginx:1.14.2
				ignoreCondition1 := (imageCanonicalizePatternRegistry+image1 == image2)
				// pattern 2) ubuntu --> ubuntu:latest
				ignoreCondition2 := (image1+imageCanonicalizePatternTag == image2)
				// if the diff matches any pattern, this will be removed
				if ignoreCondition1 || ignoreCondition2 {
					keep = false
				}
			}
		}
		if keep {
			items = append(items, d)
		}
	}
	return &mapnode.DiffResult{Items: items}
}

// a util function to inverse `before` and `after` in diff result
func inverseDiff(diff *mapnode.DiffResult) *mapnode.DiffResult {
	if diff == nil || diff.Size() == 0 {
		return nil
	}
	items := []mapnode.Difference{}
	for _, d := range diff.Items {
		val1 := d.Values["before"]
		val2 := d.Values["after"]
		newD := mapnode.Difference{
			Key: d.Key,
			Values: map[string]interface{}{
				"before": val2,
				"after":  val1,
			},
		}
		items = append(items, newD)
	}
	return &mapnode.DiffResult{Items: items}
}

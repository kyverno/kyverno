package policy

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/fix"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	gitutils "github.com/kyverno/kyverno/pkg/utils/git"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type options struct {
	save bool
}

func (o options) validate(dirs ...string) error {
	if len(dirs) == 0 {
		return errors.New("at least one directory is required")
	}
	return nil
}

func find(path string) ([]string, error) {
	var files []string
	err := filepath.Walk(path, func(file string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if gitutils.IsYaml(info) {
			files = append(files, file)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (o options) execute(out io.Writer, dirs ...string) error {
	for _, dir := range dirs {
		files, err := find(dir)
		if err != nil {
			return err
		}
		for _, file := range files {
			o.processFile(out, file)
		}
	}
	fmt.Fprintln(out, "Done.")
	return nil
}

func (o options) processFile(out io.Writer, path string) {
	policies, vaps, err := policy.LoadWithLoader(policy.KubectlValidateLoader, nil, "", path)
	if err != nil {
		return
	}
	if len(policies) == 0 {
		return
	}
	var fixed []kyvernov1.PolicyInterface
	for _, policy := range policies {
		copy := policy.CreateDeepCopy()
		fmt.Fprintf(out, "Processing file (%s)...\n", path)
		messages, err := fix.FixPolicy(copy)
		for _, warning := range messages {
			fmt.Fprintln(out, "  WARNING:", warning)
		}
		if err != nil {
			fmt.Fprintln(out, "  ERROR:", err)
			return
		}
		fixed = append(fixed, copy)
	}
	needsSave := !reflect.DeepEqual(policies, fixed)
	if o.save && needsSave {
		fmt.Fprintf(out, "  Saving file (%s)...", path)
		fmt.Fprintln(out)
		var yamlBytes []byte
		for _, policy := range fixed {
			untyped, err := kubeutils.ObjToUnstructured(policy)
			if err != nil {
				fmt.Fprintf(out, "    ERROR: converting to unstructured: %s", err)
				fmt.Fprintln(out)
				return
			}
			// prune some fields
			unstructured.RemoveNestedField(untyped.UnstructuredContent(), "status")
			unstructured.RemoveNestedField(untyped.UnstructuredContent(), "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(untyped.UnstructuredContent(), "metadata", "generation")
			unstructured.RemoveNestedField(untyped.UnstructuredContent(), "metadata", "uid")
			rules, ok, err := unstructured.NestedFieldNoCopy(untyped.UnstructuredContent(), "spec", "rules")
			if !ok || err != nil {
				return
			}
			for _, rule := range rules.([]interface{}) {
				rule := rule.(map[string]interface{})
				unstructured.RemoveNestedField(rule, "exclude", "resources")
				unstructured.RemoveNestedField(rule, "match", "resources")
				if any, ok, err := unstructured.NestedFieldNoCopy(rule, "match", "any"); ok && err == nil {
					cleanResourceFilters(any.([]interface{}))
				}
				if all, ok, err := unstructured.NestedFieldNoCopy(rule, "match", "all"); ok && err == nil {
					cleanResourceFilters(all.([]interface{}))
				}
				if any, ok, err := unstructured.NestedFieldNoCopy(rule, "exclude", "any"); ok && err == nil {
					cleanResourceFilters(any.([]interface{}))
				}
				if all, ok, err := unstructured.NestedFieldNoCopy(rule, "exclude", "all"); ok && err == nil {
					cleanResourceFilters(all.([]interface{}))
				}
				if item, _, _ := unstructured.NestedMap(rule, "generate", "clone"); len(item) == 0 {
					unstructured.RemoveNestedField(rule, "generate", "clone")
				}
				if item, _, _ := unstructured.NestedMap(rule, "generate", "cloneList"); len(item) == 0 {
					unstructured.RemoveNestedField(rule, "generate", "cloneList")
				}
				if item, _, _ := unstructured.NestedMap(rule, "generate"); len(item) == 0 {
					unstructured.RemoveNestedField(rule, "generate")
				}
				if item, _, _ := unstructured.NestedMap(rule, "mutate"); len(item) == 0 {
					unstructured.RemoveNestedField(rule, "mutate")
				}
				if item, _, _ := unstructured.NestedMap(rule, "validate", "manifests", "dryRun"); len(item) == 0 {
					unstructured.RemoveNestedField(rule, "validate", "manifests", "dryRun")
				}
				if item, _, _ := unstructured.NestedMap(rule, "validate"); len(item) == 0 {
					unstructured.RemoveNestedField(rule, "validate")
				}
				if item, _, _ := unstructured.NestedMap(rule, "exclude"); len(item) == 0 {
					unstructured.RemoveNestedField(rule, "exclude")
				}
				if item, _, _ := unstructured.NestedMap(rule, "match"); len(item) == 0 {
					unstructured.RemoveNestedField(rule, "match")
				}
			}
			jsonBytes, err := untyped.MarshalJSON()
			if err != nil {
				fmt.Fprintf(out, "    ERROR: converting to json: %s", err)
				fmt.Fprintln(out)
				return
			}
			finalBytes, err := yaml.JSONToYAML(jsonBytes)
			if err != nil {
				fmt.Fprintf(out, "    ERROR: converting to yaml: %s", err)
				fmt.Fprintln(out)
				return
			}
			yamlBytes = append(yamlBytes, []byte("---\n")...)
			yamlBytes = append(yamlBytes, finalBytes...)
		}
		for _, vap := range vaps {
			finalBytes, err := yaml.Marshal(vap)
			if err != nil {
				fmt.Fprintf(out, "    ERROR: converting to yaml: %s", err)
				fmt.Fprintln(out)
				return
			}
			yamlBytes = append(yamlBytes, []byte("---\n")...)
			yamlBytes = append(yamlBytes, finalBytes...)
		}
		if err := os.WriteFile(path, yamlBytes, os.ModePerm); err != nil {
			fmt.Fprintf(out, "    ERROR: saving file (%s): %s", path, err)
			fmt.Fprintln(out)
			return
		}
		fmt.Fprintln(out, "    OK")
	}
}

func cleanResourceFilters(rf []interface{}) {
	for _, f := range rf {
		a := f.(map[string]interface{})
		if item, _, _ := unstructured.NestedMap(a, "resources"); len(item) == 0 {
			unstructured.RemoveNestedField(a, "resources")
		}
	}
}

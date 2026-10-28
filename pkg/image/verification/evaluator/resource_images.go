package evaluator

import (
	"reflect"
	"strconv"

	"github.com/google/cel-go/cel"
	engine "github.com/kyverno/kyverno/pkg/cel/compiler"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func compileResourceImagePaths(env *cel.Env, extractors map[string]engine.ImageExtractor, variables []admissionregistrationv1.Variable) [][]string {
	bindings := make(map[string][][]string, len(variables))
	for _, variable := range variables {
		expression, issues := env.Compile(variable.Expression)
		if issues.Err() == nil {
			bindings[variable.Name] = engine.ImageResourcePaths(expression)
		}
	}
	var paths [][]string
	var expand func([]string, map[string]bool)
	expand = func(path []string, visited map[string]bool) {
		if len(path) == 0 {
			return
		}
		if path[0] != engine.VariablesKey {
			paths = append(paths, path)
			return
		}
		if len(path) < 2 || visited[path[1]] {
			return
		}
		visited[path[1]] = true
		for _, source := range bindings[path[1]] {
			expand(append(append([]string(nil), source...), path[2:]...), visited)
		}
		delete(visited, path[1])
	}
	for _, extractor := range extractors {
		for _, path := range extractor.ResourcePaths {
			expand(path, make(map[string]bool))
		}
	}
	return paths
}

func resourceImageReferences(data map[string]any, customPaths [][]string) []string {
	var images []string
	var collect func(reflect.Value, []string)
	collect = func(value reflect.Value, path []string) {
		for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
			value = value.Elem()
		}
		if !value.IsValid() {
			return
		}
		if len(path) == 0 {
			switch value.Kind() {
			case reflect.String:
				if image := value.String(); image != "" {
					images = append(images, image)
				}
			case reflect.Slice, reflect.Array:
				for index := 0; index < value.Len(); index++ {
					collect(value.Index(index), nil)
				}
			}
			return
		}
		switch value.Kind() {
		case reflect.Map:
			if path[0] == "*" {
				entries := value.MapRange()
				for entries.Next() {
					collect(entries.Value(), path[1:])
				}
			} else if value.Type().Key().Kind() == reflect.String {
				collect(value.MapIndex(reflect.ValueOf(path[0]).Convert(value.Type().Key())), path[1:])
			}
		case reflect.Slice, reflect.Array:
			if path[0] == "*" {
				for index := 0; index < value.Len(); index++ {
					collect(value.Index(index), path[1:])
				}
			} else if index, err := strconv.Atoi(path[0]); err == nil && index >= 0 && index < value.Len() {
				collect(value.Index(index), path[1:])
			}
		}
	}
	resource := reflect.ValueOf(data)
	for _, root := range []string{engine.ObjectKey, engine.OldObjectKey} {
		for _, location := range [][]string{{"spec"}, {"spec", "template", "spec"}, {"spec", "jobTemplate", "spec", "template", "spec"}} {
			for _, category := range []string{"containers", "initContainers", "ephemeralContainers"} {
				path := append(append([]string{root}, location...), category, "*", "image")
				collect(resource, path)
			}
		}
	}
	for _, path := range customPaths {
		collect(resource, path)
	}
	return images
}

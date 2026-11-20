package schema

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"k8s.io/kube-openapi/pkg/validation/spec"
)

type numberKind int

const (
	numberUntyped numberKind = iota
	numberInteger
	numberFloat
)

// walkNumbers rewrites every json.Number in v according to the schema node that
// describes it. Before validation (typed=false) integral values for integer
// nodes are rewritten to canonical integer text, since JSON Schema treats 1.0
// and 1e0 as integers. After validation (typed=true) numbers become the Go
// type matching the declared CEL type: int64 for integer, float64 for number.
// Values with no schema node fall back to int64 when integral, else float64.
func walkNumbers(v any, sc *spec.Schema, typed bool) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, c := range t {
			r, err := walkNumbers(c, childSchema(sc, k), typed)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	case []any:
		var item *spec.Schema
		if sc != nil && sc.Items != nil {
			item = sc.Items.Schema
		}
		out := make([]any, len(t))
		for i, c := range t {
			r, err := walkNumbers(c, item, typed)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	case json.Number:
		return convertNumber(t, kindOf(sc), typed)
	}
	return v, nil
}

func childSchema(sc *spec.Schema, key string) *spec.Schema {
	if sc == nil {
		return nil
	}
	if p, ok := sc.Properties[key]; ok {
		return &p
	}
	if sc.AdditionalProperties != nil && sc.AdditionalProperties.Schema != nil {
		return sc.AdditionalProperties.Schema
	}
	return nil
}

func kindOf(sc *spec.Schema) numberKind {
	switch {
	case sc == nil:
		return numberUntyped
	case sc.Type.Contains("number"):
		return numberFloat
	case sc.Type.Contains("integer"):
		return numberInteger
	}
	return numberUntyped
}

func convertNumber(n json.Number, kind numberKind, typed bool) (any, error) {
	if !typed {
		if kind == numberInteger {
			if i, ok := integral(n); ok {
				return json.Number(strconv.FormatInt(i, 10)), nil
			}
		}
		return n, nil
	}
	switch kind {
	case numberInteger:
		if i, ok := integral(n); ok {
			return i, nil
		}
		return nil, fmt.Errorf("number %s is not a representable integer", n)
	case numberFloat:
		return toFloat(n)
	}
	if i, ok := integral(n); ok {
		return i, nil
	}
	return toFloat(n)
}

func toFloat(n json.Number) (float64, error) {
	f, err := strconv.ParseFloat(n.String(), 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, fmt.Errorf("number %s is not a finite double", n)
	}
	return f, nil
}

// integral reports whether n is a mathematically integral value that fits
// int64 exactly. Floats beyond 2^53 are refused rather than rounded.
func integral(n json.Number) (int64, bool) {
	if i, err := n.Int64(); err == nil {
		return i, true
	}
	f, err := strconv.ParseFloat(n.String(), 64)
	if err != nil || math.IsInf(f, 0) || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
		return 0, false
	}
	return int64(f), true
}
